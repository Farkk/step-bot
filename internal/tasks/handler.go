package tasks

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"step-bot/internal/admin"
	"step-bot/internal/profile"
	"step-bot/internal/storage"
)

type CreateInput struct {
	Title       string         `json:"title"`
	Category    string         `json:"category"`
	Description string         `json:"description"`
	Budget      int64          `json:"budget"`
	Deadline    time.Time      `json:"deadline"`
	Location    string         `json:"location"`
	Latitude    *float64       `json:"latitude,omitempty"`
	Longitude   *float64       `json:"longitude,omitempty"`
	Fields      map[string]any `json:"fields,omitempty"`
	Publish     *bool          `json:"publish,omitempty"`
}

func (v CreateInput) Validate(now time.Time) error {
	if len([]rune(strings.TrimSpace(v.Title))) < 3 || len([]rune(v.Title)) > 160 || len([]rune(strings.TrimSpace(v.Category))) < 2 || len([]rune(v.Category)) > 80 || len([]rune(strings.TrimSpace(v.Description))) < 10 || len([]rune(v.Description)) > 10000 || v.Budget < 1 || v.Deadline.Before(now) || len([]rune(v.Location)) > 300 {
		return errors.New("Проверьте название, тип, описание, бюджет и срок")
	}
	if (v.Latitude == nil) != (v.Longitude == nil) {
		return errors.New("Укажите обе координаты")
	}
	if v.Latitude != nil && (*v.Latitude < -90 || *v.Latitude > 90 || *v.Longitude < -180 || *v.Longitude > 180) {
		return errors.New("Некорректные координаты")
	}
	return nil
}

type FieldDefinition struct {
	Key      string   `json:"key"`
	Label    string   `json:"label"`
	Type     string   `json:"type"`
	Required bool     `json:"required"`
	Options  []string `json:"options"`
}

func validateFields(defs []FieldDefinition, values map[string]any) error {
	allowed := make(map[string]FieldDefinition, len(defs))
	for _, d := range defs {
		allowed[d.Key] = d
		if d.Required && (values[d.Key] == nil || values[d.Key] == "" || strings.TrimSpace(fmt.Sprint(values[d.Key])) == "") {
			return errors.New("Заполните поле «" + d.Label + "»")
		}
	}
	for key, value := range values {
		d, ok := allowed[key]
		if !ok {
			return errors.New("Неизвестное поле заявки")
		}
		if value == nil {
			if d.Required {
				return errors.New("Обязательное поле пусто")
			}
			continue
		}
		switch d.Type {
		case "text", "date":
			s, ok := value.(string)
			if !ok || len([]rune(s)) > 1000 {
				return errors.New("Некорректное значение поля")
			}
			if d.Type == "date" && s != "" {
				if _, err := time.Parse("2006-01-02", s); err != nil {
					return err
				}
			}
		case "number":
			if _, ok := value.(float64); !ok {
				return errors.New("Ожидается число")
			}
		case "boolean":
			if _, ok := value.(bool); !ok {
				return errors.New("Ожидается логическое значение")
			}
		case "select":
			s, ok := value.(string)
			if !ok {
				return errors.New("Ожидается вариант")
			}
			found := false
			for _, option := range d.Options {
				if s == option {
					found = true
				}
			}
			if s != "" && !found {
				return errors.New("Недопустимый вариант")
			}
		default:
			return errors.New("Неизвестный тип поля")
		}
	}
	return nil
}

func validRating(score int, comment string) bool {
	return score >= 1 && score <= 5 && len([]rune(strings.TrimSpace(comment))) >= 1 && len([]rune(comment)) <= 2000
}
func ratingAllowed(completedAt, now time.Time) bool {
	return !completedAt.IsZero() && !now.After(completedAt.Add(14*24*time.Hour))
}

type Task struct {
	ID            int64             `json:"id"`
	Company       string            `json:"company"`
	Title         string            `json:"title"`
	Category      string            `json:"category"`
	Description   string            `json:"description"`
	Budget        int64             `json:"budget"`
	Deadline      time.Time         `json:"deadline"`
	Location      string            `json:"location"`
	Status        string            `json:"status"`
	Applications  int               `json:"applications"`
	MyApplication string            `json:"myApplication,omitempty"`
	Fields        map[string]any    `json:"fields"`
	FieldSchema   []FieldDefinition `json:"fieldSchema"`
	Latitude      *float64          `json:"latitude,omitempty"`
	Longitude     *float64          `json:"longitude,omitempty"`
}
type Handler struct {
	DB      *sql.DB
	Admin   admin.Handler
	Profile profile.Handler
	Store   *storage.ObjectStore
}

func (h Handler) Routes(m *http.ServeMux) {
	m.HandleFunc("GET /api/v1/admin/tasks", h.adminList)
	m.HandleFunc("POST /api/v1/admin/tasks", h.create)
	m.HandleFunc("POST /api/v1/admin/tasks/{id}/publish", h.publish)
	m.HandleFunc("GET /api/v1/admin/tasks/{id}/applications", h.adminApplications)
	m.HandleFunc("POST /api/v1/admin/tasks/{id}/applications/{application}/decision", h.decide)
	m.HandleFunc("POST /api/v1/admin/tasks/{id}/status", h.adminStatus)
	m.HandleFunc("GET /api/v1/worker/tasks", h.workerList)
	m.HandleFunc("POST /api/v1/worker/tasks/{id}/applications", h.apply)
	m.HandleFunc("GET /api/v1/worker/orders", h.orders)
	m.HandleFunc("POST /api/v1/worker/orders/{id}/status", h.orderStatus)
	h.featureRoutes(m)
}
func (h Handler) adminList(w http.ResponseWriter, r *http.Request) {
	member, ok := h.Admin.Authorize(w, r, false)
	if !ok {
		return
	}
	h.list(w, r, `SELECT t.id,c.name,t.title,t.category,t.description,t.budget,t.deadline,t.location,t.status,(SELECT count(*) FROM applications a WHERE a.task_id=t.id),t.field_values,t.field_schema,t.latitude,t.longitude FROM tasks t JOIN companies c ON c.id=t.company_id WHERE t.company_id=$1 ORDER BY t.created_at DESC LIMIT 200`, member.CompanyID, 0)
}
func (h Handler) workerID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	u, err := h.Profile.Authenticate(r)
	if err != nil {
		http.Error(w, "Недействительные данные запуска MAX", 401)
		return 0, false
	}
	var id int64
	err = h.DB.QueryRowContext(r.Context(), `SELECT user_id FROM max_identities WHERE max_id=$1`, u.ID).Scan(&id)
	if err != nil {
		http.Error(w, "Сначала заполните профиль", 403)
		return 0, false
	}
	return id, true
}
func (h Handler) workerList(w http.ResponseWriter, r *http.Request) {
	id, ok := h.workerID(w, r)
	if !ok {
		return
	}
	h.list(w, r, `SELECT t.id,c.name,t.title,t.category,t.description,t.budget,t.deadline,t.location,t.status,(SELECT count(*) FROM applications a WHERE a.task_id=t.id),COALESCE((SELECT a.status FROM applications a WHERE a.task_id=t.id AND a.user_id=$1),''),t.field_values,t.field_schema,t.latitude,t.longitude FROM tasks t JOIN companies c ON c.id=t.company_id WHERE t.status='open' ORDER BY t.created_at DESC LIMIT 200`, id, 1)
}
func (h Handler) list(w http.ResponseWriter, r *http.Request, query string, arg int64, worker int) {
	rows, err := h.DB.QueryContext(r.Context(), query, arg)
	if err != nil {
		http.Error(w, "Не удалось загрузить заявки", 500)
		return
	}
	defer rows.Close()
	items := []Task{}
	for rows.Next() {
		var t Task
		var fields, schema []byte
		var err error
		if worker == 1 {
			err = rows.Scan(&t.ID, &t.Company, &t.Title, &t.Category, &t.Description, &t.Budget, &t.Deadline, &t.Location, &t.Status, &t.Applications, &t.MyApplication, &fields, &schema, &t.Latitude, &t.Longitude)
		} else {
			err = rows.Scan(&t.ID, &t.Company, &t.Title, &t.Category, &t.Description, &t.Budget, &t.Deadline, &t.Location, &t.Status, &t.Applications, &fields, &schema, &t.Latitude, &t.Longitude)
		}
		if err = json.Unmarshal(fields, &t.Fields); err != nil {
			http.Error(w, "Не удалось загрузить поля", 500)
			return
		}
		if err = json.Unmarshal(schema, &t.FieldSchema); err != nil {
			http.Error(w, "Не удалось загрузить поля", 500)
			return
		}
		if err != nil {
			http.Error(w, "Не удалось загрузить заявки", 500)
			return
		}
		items = append(items, t)
	}
	if rows.Err() != nil {
		http.Error(w, "Не удалось загрузить заявки", 500)
		return
	}
	respond(w, items)
}
func (h Handler) create(w http.ResponseWriter, r *http.Request) {
	member, ok := h.Admin.Authorize(w, r, true)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	var input CreateInput
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if d.Decode(&input) != nil || input.Validate(time.Now()) != nil {
		http.Error(w, "Проверьте поля заявки", 400)
		return
	}
	input.Title = strings.TrimSpace(input.Title)
	input.Category = strings.TrimSpace(input.Category)
	input.Description = strings.TrimSpace(input.Description)
	input.Location = strings.TrimSpace(input.Location)
	defs, err := h.definitions(r, member.CompanyID, input.Category)
	if err != nil {
		http.Error(w, "Не удалось загрузить поля", 500)
		return
	}
	if err := validateFields(defs, input.Fields); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	fieldsJSON, _ := json.Marshal(input.Fields)
	schemaJSON, _ := json.Marshal(defs)
	status := "open"
	if input.Publish != nil && !*input.Publish {
		status = "draft"
	}
	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		http.Error(w, "Не удалось создать заявку", 500)
		return
	}
	defer tx.Rollback()
	var id int64
	err = tx.QueryRowContext(r.Context(), `INSERT INTO tasks(company_id,created_by,title,category,description,budget,deadline,location,field_values,field_schema,latitude,longitude,status) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13) RETURNING id`, member.CompanyID, member.ID, input.Title, input.Category, input.Description, input.Budget, input.Deadline, input.Location, string(fieldsJSON), string(schemaJSON), input.Latitude, input.Longitude, status).Scan(&id)
	if err == nil {
		kind := "published"
		if status == "draft" {
			kind = "draft_created"
		}
		_, err = tx.ExecContext(r.Context(), `INSERT INTO task_events(task_id,kind,actor_member_id) VALUES($1,$2,$3)`, id, kind, member.ID)
	}
	if err == nil && status == "open" {
		_, err = tx.ExecContext(r.Context(), `INSERT INTO notification_outbox(task_id,user_id,text) SELECT $1,m.user_id,$2 FROM max_identities m WHERE m.max_id>0`, id, "Новая заявка №"+strconv.FormatInt(id, 10)+" «"+input.Title+"». Откройте ШАГ в MAX, чтобы увидеть детали.")
	}
	if err != nil || tx.Commit() != nil {
		http.Error(w, "Не удалось создать заявку", 500)
		return
	}
	w.WriteHeader(201)
	respond(w, map[string]int64{"id": id})
}
func (h Handler) publish(w http.ResponseWriter, r *http.Request) {
	m, ok := h.Admin.Authorize(w, r, true)
	if !ok {
		return
	}
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Некорректный номер", 400)
		return
	}
	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		http.Error(w, "Ошибка базы", 500)
		return
	}
	defer tx.Rollback()
	var title, status string
	var deadline time.Time
	err = tx.QueryRowContext(r.Context(), `SELECT title,status,deadline FROM tasks WHERE id=$1 AND company_id=$2 FOR UPDATE`, id, m.CompanyID).Scan(&title, &status, &deadline)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "Заявка не найдена", 404)
		return
	}
	if err != nil {
		http.Error(w, "Ошибка базы", 500)
		return
	}
	if status != "draft" {
		http.Error(w, "Заявка уже опубликована", 409)
		return
	}
	if !deadline.After(time.Now()) {
		http.Error(w, "Срок заявки истёк", 409)
		return
	}
	_, err = tx.ExecContext(r.Context(), `UPDATE tasks SET status='open' WHERE id=$1`, id)
	if err == nil {
		_, err = tx.ExecContext(r.Context(), `INSERT INTO task_events(task_id,kind,actor_member_id) VALUES($1,'published',$2)`, id, m.ID)
	}
	if err == nil {
		_, err = tx.ExecContext(r.Context(), `INSERT INTO notification_outbox(task_id,user_id,text) SELECT $1,m.user_id,$2 FROM max_identities m WHERE m.max_id>0`, id, "Новая заявка №"+strconv.FormatInt(id, 10)+" «"+title+"».")
	}
	if err != nil || tx.Commit() != nil {
		http.Error(w, "Не удалось опубликовать", 500)
		return
	}
	respond(w, map[string]string{"status": "open"})
}
func (h Handler) apply(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.workerID(w, r)
	if !ok {
		return
	}
	taskID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || taskID < 1 {
		http.Error(w, "Некорректный номер заявки", 400)
		return
	}
	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		http.Error(w, "Не удалось откликнуться", 500)
		return
	}
	defer tx.Rollback()
	var status string
	err = tx.QueryRowContext(r.Context(), `SELECT status FROM tasks WHERE id=$1 FOR UPDATE`, taskID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "Заявка не найдена", 404)
		return
	}
	if err != nil {
		http.Error(w, "Не удалось откликнуться", 500)
		return
	}
	if status != "open" {
		http.Error(w, "Заявка больше не открыта", 409)
		return
	}
	var applicationID int64
	err = tx.QueryRowContext(r.Context(), `INSERT INTO applications(task_id,user_id) VALUES($1,$2) ON CONFLICT(task_id,user_id) DO NOTHING RETURNING id`, taskID, userID).Scan(&applicationID)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "Вы уже откликнулись", 409)
		return
	}
	if err == nil {
		_, err = tx.ExecContext(r.Context(), `INSERT INTO task_events(task_id,kind,actor_user_id) VALUES($1,'application_created',$2)`, taskID, userID)
	}
	if err != nil || tx.Commit() != nil {
		http.Error(w, "Не удалось откликнуться", 500)
		return
	}
	w.WriteHeader(201)
	respond(w, map[string]string{"status": "pending"})
}
func respond(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

type Application struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"userId"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
	Rating    float64   `json:"rating"`
	Ratings   int       `json:"ratings"`
	Completed int       `json:"completed"`
}

func parseID(raw string) (int64, error) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id < 1 {
		return 0, errors.New("invalid id")
	}
	return id, nil
}
func (h Handler) adminApplications(w http.ResponseWriter, r *http.Request) {
	m, ok := h.Admin.Authorize(w, r, false)
	if !ok {
		return
	}
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Некорректный номер", 400)
		return
	}
	var exists bool
	err = h.DB.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM tasks WHERE id=$1 AND company_id=$2)`, id, m.CompanyID).Scan(&exists)
	if err != nil {
		http.Error(w, "Ошибка базы", 500)
		return
	}
	if !exists {
		http.Error(w, "Заявка не найдена", 404)
		return
	}
	rows, err := h.DB.QueryContext(r.Context(), `SELECT a.id,a.user_id,COALESCE(NULLIF(mi.display_name,''),u.full_name),a.status,a.created_at,COALESCE((SELECT avg(tr.score) FROM task_ratings tr JOIN tasks t ON t.id=tr.task_id WHERE t.assigned_user_id=a.user_id),0), (SELECT count(*) FROM task_ratings tr JOIN tasks t ON t.id=tr.task_id WHERE t.assigned_user_id=a.user_id), (SELECT count(*) FROM tasks t WHERE t.assigned_user_id=a.user_id AND t.status='completed') FROM applications a JOIN users u ON u.id=a.user_id LEFT JOIN max_identities mi ON mi.user_id=u.id WHERE a.task_id=$1 ORDER BY a.created_at`, id)
	if err != nil {
		http.Error(w, "Ошибка базы", 500)
		return
	}
	defer rows.Close()
	items := []Application{}
	for rows.Next() {
		var a Application
		if err = rows.Scan(&a.ID, &a.UserID, &a.Name, &a.Status, &a.CreatedAt, &a.Rating, &a.Ratings, &a.Completed); err != nil {
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
func (h Handler) decide(w http.ResponseWriter, r *http.Request) {
	m, ok := h.Admin.Authorize(w, r, true)
	if !ok {
		return
	}
	taskID, err := parseID(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Некорректный номер", 400)
		return
	}
	appID, err := parseID(r.PathValue("application"))
	if err != nil {
		http.Error(w, "Некорректный номер", 400)
		return
	}
	var input struct {
		Decision string `json:"decision"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1024)
	if json.NewDecoder(r.Body).Decode(&input) != nil || (input.Decision != "accept" && input.Decision != "reject") {
		http.Error(w, "Нужно выбрать решение", 400)
		return
	}
	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		http.Error(w, "Ошибка базы", 500)
		return
	}
	defer tx.Rollback()
	var status, title string
	err = tx.QueryRowContext(r.Context(), `SELECT status,title FROM tasks WHERE id=$1 AND company_id=$2 FOR UPDATE`, taskID, m.CompanyID).Scan(&status, &title)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "Заявка не найдена", 404)
		return
	}
	if err != nil {
		http.Error(w, "Ошибка базы", 500)
		return
	}
	if status != "open" {
		http.Error(w, "Заявка уже назначена или закрыта", 409)
		return
	}
	var userID int64
	err = tx.QueryRowContext(r.Context(), `SELECT user_id FROM applications WHERE id=$1 AND task_id=$2 AND status='pending' FOR UPDATE`, appID, taskID).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "Отклик уже обработан", 409)
		return
	}
	if err != nil {
		http.Error(w, "Ошибка базы", 500)
		return
	}
	if input.Decision == "accept" {
		_, err = tx.ExecContext(r.Context(), `UPDATE tasks SET status='assigned',assigned_user_id=$1 WHERE id=$2`, userID, taskID)
		if err == nil {
			_, err = tx.ExecContext(r.Context(), `INSERT INTO task_events(task_id,kind,actor_member_id,subject_user_id) SELECT $1,'application_rejected_auto',$2,a.user_id FROM applications a WHERE a.task_id=$1 AND a.id<>$3 AND a.status='pending'`, taskID, m.ID, appID)
		}
		if err == nil {
			_, err = tx.ExecContext(r.Context(), `UPDATE applications SET status=CASE WHEN id=$1 THEN 'accepted' ELSE 'rejected' END WHERE task_id=$2 AND status='pending'`, appID, taskID)
		}
	} else {
		_, err = tx.ExecContext(r.Context(), `UPDATE applications SET status='rejected' WHERE id=$1`, appID)
	}
	if err == nil {
		_, err = tx.ExecContext(r.Context(), `INSERT INTO task_events(task_id,kind,actor_member_id,subject_user_id) VALUES($1,$2,$3,$4)`, taskID, "application_"+input.Decision+"ed", m.ID, userID)
	}
	if err == nil {
		if input.Decision == "accept" {
			err = enqueue(r.Context(), tx, taskID, userID, "assigned", title)
		} else {
			err = enqueue(r.Context(), tx, taskID, userID, "rejected", title)
		}
	}
	if err == nil && input.Decision == "accept" {
		_, err = tx.ExecContext(r.Context(), `INSERT INTO notification_outbox(task_id,user_id,text) SELECT $1,a.user_id,$2 FROM applications a JOIN max_identities m ON m.user_id=a.user_id AND m.max_id>0 WHERE a.task_id=$1 AND a.id<>$3 AND a.status='rejected'`, taskID, "Заявка №"+strconv.FormatInt(taskID, 10)+" «"+title+"»: ваш отклик отклонён. Откройте ШАГ в MAX, чтобы увидеть детали.", appID)
	}
	if err != nil || tx.Commit() != nil {
		http.Error(w, "Не удалось сохранить решение", 500)
		return
	}
	respond(w, map[string]string{"status": input.Decision})
}
func (h Handler) orders(w http.ResponseWriter, r *http.Request) {
	id, ok := h.workerID(w, r)
	if !ok {
		return
	}
	h.list(w, r, `SELECT t.id,c.name,t.title,t.category,t.description,t.budget,t.deadline,t.location,t.status,(SELECT count(*) FROM applications a WHERE a.task_id=t.id),'',t.field_values,t.field_schema,t.latitude,t.longitude FROM tasks t JOIN companies c ON c.id=t.company_id WHERE t.assigned_user_id=$1 AND t.status IN ('assigned','in_progress','paused','awaiting_confirmation','completed','cancelled') ORDER BY t.created_at DESC LIMIT 200`, id, 1)
}
func (h Handler) orderStatus(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.workerID(w, r)
	if !ok {
		return
	}
	taskID, err := parseID(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Некорректный номер", 400)
		return
	}
	var input struct {
		Action string `json:"action"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1024)
	if json.NewDecoder(r.Body).Decode(&input) != nil || (input.Action != "start" && input.Action != "complete") {
		http.Error(w, "Некорректное действие", 400)
		return
	}
	old := "assigned"
	if input.Action == "complete" {
		old = "in_progress"
	}
	next := nextStatus(old, input.Action)
	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		http.Error(w, "Ошибка базы", 500)
		return
	}
	defer tx.Rollback()
	var current, title string
	err = tx.QueryRowContext(r.Context(), `SELECT status,title FROM tasks WHERE id=$1 AND assigned_user_id=$2 FOR UPDATE`, taskID, userID).Scan(&current, &title)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "Заказ не найден", 404)
		return
	}
	if err != nil {
		http.Error(w, "Ошибка базы", 500)
		return
	}
	if current != old {
		http.Error(w, "Действие недоступно", 409)
		return
	}
	_, err = tx.ExecContext(r.Context(), `UPDATE tasks SET status=$1,completed_at=CASE WHEN $1='completed' THEN now() ELSE completed_at END WHERE id=$2`, next, taskID)
	if err == nil {
		_, err = tx.ExecContext(r.Context(), `INSERT INTO task_events(task_id,kind,actor_user_id) VALUES($1,$2,$3)`, taskID, next, userID)
	}
	if err == nil {
		err = enqueue(r.Context(), tx, taskID, userID, next, title)
	}
	if err != nil || tx.Commit() != nil {
		http.Error(w, "Не удалось обновить заказ", 500)
		return
	}
	respond(w, map[string]string{"status": next})
}

func (h Handler) adminStatus(w http.ResponseWriter, r *http.Request) {
	m, ok := h.Admin.Authorize(w, r, true)
	if !ok {
		return
	}
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Некорректный номер", 400)
		return
	}
	var input struct {
		Action string `json:"action"`
		Reason string `json:"reason"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 2048)
	if json.NewDecoder(r.Body).Decode(&input) != nil || (input.Action != "pause" && input.Action != "resume" && input.Action != "confirm" && input.Action != "cancel") {
		http.Error(w, "Некорректное действие", 400)
		return
	}
	input.Reason = strings.TrimSpace(input.Reason)
	if input.Action == "cancel" && (len([]rune(input.Reason)) < 3 || len([]rune(input.Reason)) > 500) {
		http.Error(w, "Укажите причину отмены", 400)
		return
	}
	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		http.Error(w, "Ошибка базы", 500)
		return
	}
	defer tx.Rollback()
	var current, title string
	var assignee sql.NullInt64
	err = tx.QueryRowContext(r.Context(), `SELECT status,title,assigned_user_id FROM tasks WHERE id=$1 AND company_id=$2 FOR UPDATE`, id, m.CompanyID).Scan(&current, &title, &assignee)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "Заявка не найдена", 404)
		return
	}
	if err != nil {
		http.Error(w, "Ошибка базы", 500)
		return
	}
	next := nextStatus(current, input.Action)
	if next == "" {
		http.Error(w, "Переход статуса недоступен", 409)
		return
	}
	_, err = tx.ExecContext(r.Context(), `UPDATE tasks SET status=$1,completed_at=CASE WHEN $1='completed' THEN now() ELSE completed_at END WHERE id=$2`, next, id)
	if err == nil {
		_, err = tx.ExecContext(r.Context(), `INSERT INTO task_events(task_id,kind,actor_member_id) VALUES($1,$2,$3)`, id, next+func() string {
			if input.Reason != "" {
				return ": " + input.Reason
			}
			return ""
		}(), m.ID)
	}
	if err == nil && assignee.Valid {
		err = enqueue(r.Context(), tx, id, assignee.Int64, next, title)
	}
	if err == nil && next == "cancelled" && !assignee.Valid {
		_, err = tx.ExecContext(r.Context(), `INSERT INTO notification_outbox(task_id,user_id,text) SELECT $1,a.user_id,$2 FROM applications a JOIN max_identities m ON m.user_id=a.user_id AND m.max_id>0 WHERE a.task_id=$1 AND a.status='pending'`, id, "Заявка №"+strconv.FormatInt(id, 10)+" «"+title+"»: отменена. Откройте ШАГ в MAX, чтобы увидеть детали.")
	}
	if err != nil || tx.Commit() != nil {
		http.Error(w, "Не удалось обновить заявку", 500)
		return
	}
	respond(w, map[string]string{"status": next})
}
