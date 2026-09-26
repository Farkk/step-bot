package tasks

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"step-bot/internal/admin"
)

func (h Handler) featureRoutes(m *http.ServeMux) {
	m.HandleFunc("GET /api/v1/admin/fields", h.fields)
	m.HandleFunc("POST /api/v1/admin/fields", h.saveField)
	m.HandleFunc("GET /api/v1/admin/tasks/{id}/events", h.events)
	m.HandleFunc("GET /api/v1/admin/tasks/{id}/rating", h.getRating)
	m.HandleFunc("PUT /api/v1/admin/tasks/{id}/rating", h.putRating)
	m.HandleFunc("GET /api/v1/worker/reputation", h.reputation)
	m.HandleFunc("GET /api/v1/admin/analytics", h.analytics)
	m.HandleFunc("GET /api/v1/admin/analytics.csv", h.analyticsCSV)
	m.HandleFunc("POST /api/v1/admin/tasks/{id}/attachments", h.uploadAttachment)
	m.HandleFunc("GET /api/v1/admin/tasks/{id}/attachments", h.attachments)
	m.HandleFunc("GET /api/v1/worker/tasks/{id}/attachments", h.attachments)
	m.HandleFunc("GET /api/v1/worker/orders/{id}/attachments", h.attachments)
	m.HandleFunc("GET /api/v1/attachments/{id}", h.downloadAttachment)
}

func (h Handler) definitions(r *http.Request, companyID int64, category string) ([]FieldDefinition, error) {
	rows, err := h.DB.QueryContext(r.Context(), `SELECT key,label,type,required,options FROM task_field_definitions WHERE company_id=$1 AND category=$2 ORDER BY id`, companyID, category)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	defs := []FieldDefinition{}
	for rows.Next() {
		var d FieldDefinition
		var options []byte
		if err = rows.Scan(&d.Key, &d.Label, &d.Type, &d.Required, &options); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(options, &d.Options); err != nil {
			return nil, err
		}
		defs = append(defs, d)
	}
	return defs, rows.Err()
}
func (h Handler) fields(w http.ResponseWriter, r *http.Request) {
	m, ok := h.Admin.Authorize(w, r, false)
	if !ok {
		return
	}
	defs, err := h.definitions(r, m.CompanyID, r.URL.Query().Get("category"))
	if err != nil {
		http.Error(w, "Ошибка базы", 500)
		return
	}
	respond(w, defs)
}
func (h Handler) saveField(w http.ResponseWriter, r *http.Request) {
	m, ok := h.Admin.Authorize(w, r, true)
	if !ok {
		return
	}
	if m.Role != "owner" {
		http.Error(w, "Только владелец настраивает поля", 403)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	var input struct {
		Category string `json:"category"`
		FieldDefinition
	}
	decodeErr := json.NewDecoder(r.Body).Decode(&input)
	input.Category = strings.TrimSpace(input.Category)
	input.Label = strings.TrimSpace(input.Label)
	if decodeErr != nil || len(input.Category) < 2 || len(input.Category) > 80 || len(input.Key) < 1 || len(input.Key) > 60 || len(input.Label) < 1 || len(input.Label) > 120 || !strings.Contains("|text|number|select|date|boolean|", "|"+input.Type+"|") || len(input.Options) > 50 || input.Type == "select" && len(input.Options) == 0 {
		http.Error(w, "Проверьте описание поля", 400)
		return
	}
	opts, _ := json.Marshal(input.Options)
	_, err := h.DB.ExecContext(r.Context(), `INSERT INTO task_field_definitions(company_id,category,key,label,type,required,options) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(company_id,category,key) DO UPDATE SET label=EXCLUDED.label,type=EXCLUDED.type,required=EXCLUDED.required,options=EXCLUDED.options`, m.CompanyID, input.Category, input.Key, input.Label, input.Type, input.Required, string(opts))
	if err != nil {
		http.Error(w, "Не удалось сохранить поле", 500)
		return
	}
	respond(w, input)
}

func (h Handler) ownedTask(w http.ResponseWriter, r *http.Request, m admin.Member) (int64, bool) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Некорректный номер", 400)
		return 0, false
	}
	var exists bool
	err = h.DB.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM tasks WHERE id=$1 AND company_id=$2)`, id, m.CompanyID).Scan(&exists)
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
func (h Handler) events(w http.ResponseWriter, r *http.Request) {
	m, ok := h.Admin.Authorize(w, r, false)
	if !ok {
		return
	}
	id, ok := h.ownedTask(w, r, m)
	if !ok {
		return
	}
	rows, err := h.DB.QueryContext(r.Context(), `SELECT e.kind,COALESCE(cm.full_name,NULLIF(mi.display_name,''),u.full_name,'Система'),COALESCE(NULLIF(subject_m.display_name,''),subject_u.full_name,''),e.created_at FROM task_events e LEFT JOIN company_members cm ON cm.id=e.actor_member_id LEFT JOIN users u ON u.id=e.actor_user_id LEFT JOIN max_identities mi ON mi.user_id=u.id LEFT JOIN users subject_u ON subject_u.id=e.subject_user_id LEFT JOIN max_identities subject_m ON subject_m.user_id=subject_u.id WHERE e.task_id=$1 ORDER BY e.created_at,e.id`, id)
	if err != nil {
		http.Error(w, "Ошибка базы", 500)
		return
	}
	defer rows.Close()
	items := []struct {
		Kind      string    `json:"kind"`
		Actor     string    `json:"actor"`
		Subject   string    `json:"subject"`
		CreatedAt time.Time `json:"createdAt"`
	}{}
	for rows.Next() {
		var item struct {
			Kind      string    `json:"kind"`
			Actor     string    `json:"actor"`
			Subject   string    `json:"subject"`
			CreatedAt time.Time `json:"createdAt"`
		}
		if rows.Scan(&item.Kind, &item.Actor, &item.Subject, &item.CreatedAt) != nil {
			http.Error(w, "Ошибка базы", 500)
			return
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		http.Error(w, "Ошибка базы", 500)
		return
	}
	respond(w, items)
}

type rating struct {
	Score         int        `json:"score"`
	Comment       string     `json:"comment"`
	CompletedAt   time.Time  `json:"completedAt"`
	EditableUntil time.Time  `json:"editableUntil"`
	UpdatedAt     *time.Time `json:"updatedAt,omitempty"`
}

func (h Handler) ratingState(w http.ResponseWriter, r *http.Request, m admin.Member) (int64, rating, bool) {
	id, ok := h.ownedTask(w, r, m)
	if !ok {
		return 0, rating{}, false
	}
	var s rating
	var completed sql.NullTime
	var updated sql.NullTime
	err := h.DB.QueryRowContext(r.Context(), `SELECT t.completed_at,COALESCE(tr.score,0),COALESCE(tr.comment,''),tr.updated_at FROM tasks t LEFT JOIN task_ratings tr ON tr.task_id=t.id WHERE t.id=$1`, id).Scan(&completed, &s.Score, &s.Comment, &updated)
	if err != nil {
		http.Error(w, "Ошибка базы", 500)
		return 0, rating{}, false
	}
	if completed.Valid {
		s.CompletedAt = completed.Time
		s.EditableUntil = completed.Time.Add(14 * 24 * time.Hour)
	}
	if updated.Valid {
		s.UpdatedAt = &updated.Time
	}
	return id, s, true
}
func (h Handler) getRating(w http.ResponseWriter, r *http.Request) {
	m, ok := h.Admin.Authorize(w, r, false)
	if !ok {
		return
	}
	_, s, ok := h.ratingState(w, r, m)
	if ok {
		respond(w, s)
	}
}
func (h Handler) putRating(w http.ResponseWriter, r *http.Request) {
	m, ok := h.Admin.Authorize(w, r, true)
	if !ok {
		return
	}
	id, s, ok := h.ratingState(w, r, m)
	if !ok {
		return
	}
	if !ratingAllowed(s.CompletedAt, time.Now().UTC()) {
		http.Error(w, "Оценку можно изменить в течение 14 дней после закрытия", 409)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	var input rating
	if json.NewDecoder(r.Body).Decode(&input) != nil || !validRating(input.Score, input.Comment) {
		http.Error(w, "Укажите 1–5 звёзд и комментарий", 400)
		return
	}
	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		http.Error(w, "Ошибка базы", 500)
		return
	}
	defer tx.Rollback()
	var completed time.Time
	err = tx.QueryRowContext(r.Context(), `SELECT completed_at FROM tasks WHERE id=$1 AND company_id=$2 AND status='completed' FOR UPDATE`, id, m.CompanyID).Scan(&completed)
	if errors.Is(err, sql.ErrNoRows) || !ratingAllowed(completed, time.Now().UTC()) {
		http.Error(w, "Срок оценки истёк", 409)
		return
	}
	if err != nil {
		http.Error(w, "Ошибка базы", 500)
		return
	}
	_, err = tx.ExecContext(r.Context(), `INSERT INTO task_ratings(task_id,score,comment,author_member_id) VALUES($1,$2,$3,$4) ON CONFLICT(task_id) DO UPDATE SET score=EXCLUDED.score,comment=EXCLUDED.comment,author_member_id=EXCLUDED.author_member_id,updated_at=now()`, id, input.Score, strings.TrimSpace(input.Comment), m.ID)
	if err == nil {
		_, err = tx.ExecContext(r.Context(), `INSERT INTO task_events(task_id,kind,actor_member_id) VALUES($1,$2,$3)`, id, map[bool]string{true: "rating_updated", false: "rating_created"}[s.Score > 0], m.ID)
	}
	if err != nil || tx.Commit() != nil {
		http.Error(w, "Не удалось сохранить оценку", 500)
		return
	}
	h.getRating(w, r)
}

func (h Handler) reputation(w http.ResponseWriter, r *http.Request) {
	id, ok := h.workerID(w, r)
	if !ok {
		return
	}
	var accepted, reviewed, ratings, completed int
	var average float64
	err := h.DB.QueryRowContext(r.Context(), `SELECT (SELECT count(*) FROM applications WHERE user_id=$1 AND status='accepted'),(SELECT count(*) FROM applications WHERE user_id=$1 AND status IN ('accepted','rejected')),(SELECT count(*) FROM task_ratings tr JOIN tasks t ON t.id=tr.task_id WHERE t.assigned_user_id=$1),(SELECT count(*) FROM tasks WHERE assigned_user_id=$1 AND status='completed'),COALESCE((SELECT avg(tr.score) FROM task_ratings tr JOIN tasks t ON t.id=tr.task_id WHERE t.assigned_user_id=$1),0)`, id).Scan(&accepted, &reviewed, &ratings, &completed, &average)
	if err != nil {
		http.Error(w, "Ошибка базы", 500)
		return
	}
	percentage := 0.0
	if reviewed > 0 {
		percentage = float64(accepted) * 100 / float64(reviewed)
	}
	respond(w, map[string]any{"acceptedPercent": percentage, "averageRating": average, "ratings": ratings, "completed": completed})
}

type analyticsData struct {
	AverageRating float64 `json:"averageRating"`
	RatingCount   int     `json:"ratingCount"`
	ReactionHours float64 `json:"reactionHours"`
	ReactionCount int     `json:"reactionCount"`
	Completed     int     `json:"completed"`
	From          string  `json:"from"`
	To            string  `json:"to"`
}

func period(r *http.Request) (time.Time, time.Time, error) {
	now := time.Now().UTC()
	from := now.AddDate(0, -1, 0)
	to := now
	var err error
	if s := r.URL.Query().Get("from"); s != "" {
		from, err = time.Parse("2006-01-02", s)
		if err != nil {
			return from, to, err
		}
	}
	if s := r.URL.Query().Get("to"); s != "" {
		to, err = time.Parse("2006-01-02", s)
		if err != nil {
			return from, to, err
		}
		to = to.Add(24 * time.Hour)
	}
	if !from.Before(to) || to.Sub(from) > 366*24*time.Hour {
		return from, to, fmt.Errorf("invalid period")
	}
	return from, to, nil
}
func (h Handler) metrics(r *http.Request, companyID int64) (analyticsData, error) {
	from, to, err := period(r)
	if err != nil {
		return analyticsData{}, err
	}
	v := analyticsData{From: from.Format("2006-01-02"), To: to.Add(-time.Nanosecond).Format("2006-01-02")}
	err = h.DB.QueryRowContext(r.Context(), `SELECT COALESCE(avg(tr.score),0),count(tr.task_id) FROM task_ratings tr JOIN tasks t ON t.id=tr.task_id WHERE t.company_id=$1 AND t.completed_at>=$2 AND t.completed_at<$3`, companyID, from, to).Scan(&v.AverageRating, &v.RatingCount)
	if err != nil {
		return v, err
	}
	err = h.DB.QueryRowContext(r.Context(), `SELECT count(*) FROM tasks WHERE company_id=$1 AND completed_at>=$2 AND completed_at<$3 AND status='completed'`, companyID, from, to).Scan(&v.Completed)
	if err != nil {
		return v, err
	}
	err = h.DB.QueryRowContext(r.Context(), `SELECT COALESCE(avg(extract(epoch FROM decision.created_at-first_app.created_at)/3600),0),count(*) FROM tasks t JOIN LATERAL (SELECT min(created_at) created_at FROM task_events WHERE task_id=t.id AND kind='application_created') first_app ON first_app.created_at IS NOT NULL JOIN LATERAL (SELECT min(created_at) created_at FROM task_events WHERE task_id=t.id AND kind IN ('application_accepted','application_rejected')) decision ON decision.created_at IS NOT NULL WHERE t.company_id=$1 AND decision.created_at>=$2 AND decision.created_at<$3`, companyID, from, to).Scan(&v.ReactionHours, &v.ReactionCount)
	return v, err
}
func (h Handler) analytics(w http.ResponseWriter, r *http.Request) {
	m, ok := h.Admin.Authorize(w, r, false)
	if !ok {
		return
	}
	v, err := h.metrics(r, m.CompanyID)
	if err != nil {
		http.Error(w, "Проверьте период", 400)
		return
	}
	respond(w, v)
}
func (h Handler) analyticsCSV(w http.ResponseWriter, r *http.Request) {
	m, ok := h.Admin.Authorize(w, r, false)
	if !ok {
		return
	}
	v, err := h.metrics(r, m.CompanyID)
	if err != nil {
		http.Error(w, "Проверьте период", 400)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="step-analytics.csv"`)
	writer := csv.NewWriter(w)
	_ = writer.Write([]string{"Период с", "Период по", "Средний рейтинг", "Количество оценок", "Время реакции, часы", "Количество реакций", "Закрытые задачи"})
	_ = writer.Write([]string{v.From, v.To, strconv.FormatFloat(v.AverageRating, 'f', 2, 64), strconv.Itoa(v.RatingCount), strconv.FormatFloat(v.ReactionHours, 'f', 2, 64), strconv.Itoa(v.ReactionCount), strconv.Itoa(v.Completed)})
	writer.Flush()
}
