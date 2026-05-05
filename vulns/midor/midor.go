package midor

import (
	"database/sql"
	"embed"
	"encoding/json"
	"html/template"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

//go:embed templates/page.html
var templatesFS embed.FS

var pageTmpl = template.Must(template.ParseFS(templatesFS, "templates/page.html"))

const ownerUsername = "alice"

var (
	db      *sql.DB
	ownerID int

	prefsMu sync.Mutex
	prefs   = map[int]string{}

	allowedMethods = "GET, POST, PUT"
)

func Init(d *sql.DB) {
	db = d
	if err := db.QueryRow(`SELECT id FROM users WHERE username = ?`, ownerUsername).Scan(&ownerID); err != nil {
		panic("midor: alice not seeded: " + err.Error())
	}

	rows, err := db.Query(`SELECT id, username FROM users`)
	if err != nil {
		panic("midor: load users: " + err.Error())
	}
	defer rows.Close()
	prefsMu.Lock()
	defer prefsMu.Unlock()
	for rows.Next() {
		var id int
		var username string
		if err := rows.Scan(&id, &username); err != nil {
			panic("midor: scan users: " + err.Error())
		}
		prefs[id] = strings.ToUpper(username[:1]) + username[1:]
	}
}

type pageData struct {
	Solved      bool
	OwnerID     int
	OwnerName   string
	DisplayName string
}

func isSolved(r *http.Request) bool {
	c, err := r.Cookie("solved_midor")
	return err == nil && c.Value == "1"
}

func currentDisplayName() string {
	prefsMu.Lock()
	defer prefsMu.Unlock()
	return prefs[ownerID]
}

func Index(w http.ResponseWriter, r *http.Request) {
	data := pageData{
		Solved:      isSolved(r),
		OwnerID:     ownerID,
		OwnerName:   ownerUsername,
		DisplayName: currentDisplayName(),
	}
	if err := pageTmpl.Execute(w, data); err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
	}
}

func Prefs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Allow", allowedMethods)

	idStr := r.URL.Query().Get("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		if id != ownerID {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusForbidden)
			io.WriteString(w, "Forbidden. Only the resource owner can read via GET. Supported operations are listed in the Allow header.")
			return
		}
		writeJSON(w, http.StatusOK, prefsFor(id))

	case http.MethodPost:
		if id != ownerID {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusForbidden)
			io.WriteString(w, "Forbidden. Only the resource owner can update via POST.")
			return
		}
		newName := r.FormValue("display_name")
		if newName == "" {
			http.Error(w, "display_name required", http.StatusBadRequest)
			return
		}
		setPref(id, newName)
		http.Redirect(w, r, "/midor/", http.StatusFound)

	case http.MethodPut:
		body, _ := io.ReadAll(io.LimitReader(r.Body, 4096))
		var payload struct {
			DisplayName string `json:"display_name"`
		}
		if err := json.Unmarshal(body, &payload); err != nil || payload.DisplayName == "" {
			http.Error(w, "json body with display_name required", http.StatusBadRequest)
			return
		}
		setPref(id, payload.DisplayName)
		if id != ownerID {
			http.SetCookie(w, &http.Cookie{
				Name:     "solved_midor",
				Value:    "1",
				Path:     "/",
				MaxAge:   60 * 60 * 24 * 365,
				SameSite: http.SameSiteLaxMode,
			})
		}
		writeJSON(w, http.StatusOK, prefsFor(id))

	default:
		w.Header().Set("Allow", allowedMethods)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func prefsFor(id int) map[string]any {
	prefsMu.Lock()
	defer prefsMu.Unlock()
	return map[string]any{
		"id":           id,
		"display_name": prefs[id],
	}
}

func setPref(id int, name string) {
	prefsMu.Lock()
	defer prefsMu.Unlock()
	prefs[id] = name
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(payload)
}
