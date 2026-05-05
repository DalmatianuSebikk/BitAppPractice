package bac

import (
	"database/sql"
	"embed"
	"errors"
	"html/template"
	"net/http"
	"strconv"
)

//go:embed templates/page.html
var templatesFS embed.FS

var pageTmpl = template.Must(template.ParseFS(templatesFS, "templates/page.html"))

const requesterUsername = "mary"

var (
	db          *sql.DB
	requesterID int
)

func Init(d *sql.DB) {
	db = d
	if err := db.QueryRow(`SELECT id FROM users WHERE username = ?`, requesterUsername).Scan(&requesterID); err != nil {
		panic("bac: " + requesterUsername + " not seeded: " + err.Error())
	}
}

type userRow struct {
	ID       int
	Username string
	Role     string
}

type pageData struct {
	Solved      bool
	Requester   string
	RequesterID int
	Role        string
	Users       []userRow
}

func isSolved(r *http.Request) bool {
	c, err := r.Cookie("solved_bac")
	return err == nil && c.Value == "1"
}

func currentRole() string {
	var role string
	db.QueryRow(`SELECT role FROM users WHERE id = ?`, requesterID).Scan(&role)
	return role
}

func loadAllUsers() ([]userRow, error) {
	rows, err := db.Query(`SELECT id, username, role FROM users ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []userRow
	for rows.Next() {
		var u userRow
		if err := rows.Scan(&u.ID, &u.Username, &u.Role); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

func Index(w http.ResponseWriter, r *http.Request) {
	users, err := loadAllUsers()
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	data := pageData{
		Solved:      isSolved(r),
		Requester:   requesterUsername,
		RequesterID: requesterID,
		Role:        currentRole(),
		Users:       users,
	}
	if err := pageTmpl.Execute(w, data); err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
	}
}

func validRole(role string) bool {
	return role == "user" || role == "junior_admin" || role == "senior_admin"
}

func ChangeRole(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	newRole := r.PostFormValue("role")
	if !validRole(newRole) {
		http.Error(w, "bad role", http.StatusBadRequest)
		return
	}

	role := currentRole()
	if role != "junior_admin" && role != "senior_admin" {
		http.Error(w, "admins only", http.StatusForbidden)
		return
	}

	var oldRole string
	err = db.QueryRow(`SELECT role FROM users WHERE id = ?`, id).Scan(&oldRole)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	if _, err := db.Exec(`UPDATE users SET role = ? WHERE id = ?`, newRole, id); err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	if newRole == "senior_admin" || oldRole == "senior_admin" {
		http.SetCookie(w, &http.Cookie{
			Name:     "solved_bac",
			Value:    "1",
			Path:     "/",
			MaxAge:   60 * 60 * 24 * 365,
			SameSite: http.SameSiteLaxMode,
		})
	}

	http.Redirect(w, r, "/bac/", http.StatusFound)
}
