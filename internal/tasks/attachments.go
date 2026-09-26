package tasks

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"
)

const maxAttachment = 5 << 20

type attachment struct {
	ID          int64     `json:"id"`
	Filename    string    `json:"filename"`
	ContentType string    `json:"contentType"`
	CreatedAt   time.Time `json:"createdAt"`
}

func (h Handler) uploadAttachment(w http.ResponseWriter, r *http.Request) {
	m, ok := h.Admin.Authorize(w, r, true)
	if !ok {
		return
	}
	id, ok := h.ownedTask(w, r, m)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxAttachment+4096)
	if err := r.ParseMultipartForm(maxAttachment); err != nil {
		http.Error(w, "Файл слишком большой", 413)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Выберите файл", 400)
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxAttachment+1))
	if err != nil || len(data) > maxAttachment || len(data) == 0 {
		http.Error(w, "Файл должен быть не больше 5 МБ", 413)
		return
	}
	filename := filepath.Base(strings.ReplaceAll(header.Filename, "\\", "/"))
	if filename == "." || len([]rune(filename)) > 200 {
		http.Error(w, "Некорректное имя файла", 400)
		return
	}
	contentType := http.DetectContentType(data)
	if !strings.HasPrefix(contentType, "image/") && contentType != "application/pdf" && contentType != "text/plain; charset=utf-8" && contentType != "application/zip" {
		http.Error(w, "Допустимы изображения, PDF, текст и ZIP", 400)
		return
	}
	keyBytes := make([]byte, 16)
	if _, err = rand.Read(keyBytes); err != nil {
		http.Error(w, "Ошибка сервера", 500)
		return
	}
	objectKey := hex.EncodeToString(keyBytes)
	if err = h.Store.Put(r.Context(), objectKey, data, contentType); err != nil {
		http.Error(w, "Хранилище файлов недоступно", 503)
		return
	}
	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		http.Error(w, "Ошибка базы", 500)
		return
	}
	defer tx.Rollback()
	var attachmentID int64
	err = tx.QueryRowContext(r.Context(), `INSERT INTO task_attachments(task_id,filename,content_type,object_key) VALUES($1,$2,$3,$4) RETURNING id`, id, filename, contentType, objectKey).Scan(&attachmentID)
	if err == nil {
		_, err = tx.ExecContext(r.Context(), `INSERT INTO task_events(task_id,kind,actor_member_id) VALUES($1,'attachment_added',$2)`, id, m.ID)
	}
	if err != nil || tx.Commit() != nil {
		h.Store.Delete(r.Context(), objectKey)
		http.Error(w, "Не удалось сохранить файл", 500)
		return
	}
	w.WriteHeader(201)
	respond(w, map[string]int64{"id": attachmentID})
}
func (h Handler) attachmentTask(w http.ResponseWriter, r *http.Request) (int64, bool) {
	if strings.HasPrefix(r.URL.Path, "/api/v1/admin/") {
		m, ok := h.Admin.Authorize(w, r, false)
		if !ok {
			return 0, false
		}
		return h.ownedTask(w, r, m)
	}
	userID, ok := h.workerID(w, r)
	if !ok {
		return 0, false
	}
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Некорректный номер", 400)
		return 0, false
	}
	var exists bool
	err = h.DB.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM tasks WHERE id=$1 AND (status='open' OR assigned_user_id=$2))`, id, userID).Scan(&exists)
	if err != nil {
		http.Error(w, "Ошибка базы", 500)
		return 0, false
	}
	if !exists {
		http.Error(w, "Заявка не найдена", 404)
		return 0, false
	}
	return id, true
}
func (h Handler) attachments(w http.ResponseWriter, r *http.Request) {
	id, ok := h.attachmentTask(w, r)
	if !ok {
		return
	}
	rows, err := h.DB.QueryContext(r.Context(), `SELECT id,filename,content_type,created_at FROM task_attachments WHERE task_id=$1 ORDER BY id`, id)
	if err != nil {
		http.Error(w, "Ошибка базы", 500)
		return
	}
	defer rows.Close()
	items := []attachment{}
	for rows.Next() {
		var a attachment
		if rows.Scan(&a.ID, &a.Filename, &a.ContentType, &a.CreatedAt) != nil {
			http.Error(w, "Ошибка базы", 500)
			return
		}
		items = append(items, a)
	}
	if rows.Err() != nil {
		http.Error(w, "Ошибка базы", 500)
		return
	}
	respond(w, items)
}
func (h Handler) downloadAttachment(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Некорректный номер", 400)
		return
	}
	var taskID int64
	var filename, contentType, status, objectKey string
	var companyID, assigned sql.NullInt64
	err = h.DB.QueryRowContext(r.Context(), `SELECT a.task_id,a.object_key,a.filename,a.content_type,t.status,t.company_id,t.assigned_user_id FROM task_attachments a JOIN tasks t ON t.id=a.task_id WHERE a.id=$1`, id).Scan(&taskID, &objectKey, &filename, &contentType, &status, &companyID, &assigned)
	if err == sql.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "Ошибка базы", 500)
		return
	}
	authorized := false
	if m, _, err := h.Admin.Current(r); err == nil && companyID.Valid && m.CompanyID == companyID.Int64 {
		authorized = true
	}
	if !authorized {
		if userID, ok := h.workerID(w, r); ok && (status == "open" || (assigned.Valid && assigned.Int64 == userID)) {
			authorized = true
		} else {
			return
		}
	}
	data, err := h.Store.Get(r.Context(), objectKey)
	if err != nil {
		http.Error(w, "Хранилище файлов недоступно", 503)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", "attachment; filename*=UTF-8''"+url.PathEscape(filename))
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(data)
}
