package authbrute

import (
	"crypto/rand"
	"database/sql"
	"embed"
	"encoding/hex"
	"html/template"
	"net/http"
	"sync"
)

//go:embed templates/page.html
var templatesFS embed.FS

var pageTmpl = template.Must(template.ParseFS(templatesFS, "templates/page.html"))

var db *sql.DB

func Init(d *sql.DB) { db = d }

var sessions = struct {
	sync.Mutex
	m map[string]string
}{m: map[string]string{}}

func newSession(username string) string {
	b := make([]byte, 16)
	rand.Read(b)
	sid := hex.EncodeToString(b)
	sessions.Lock()
	sessions.m[sid] = username
	sessions.Unlock()
	return sid
}

func sessionUser(sid string) string {
	sessions.Lock()
	defer sessions.Unlock()
	return sessions.m[sid]
}

type pageData struct {
	Solved      bool
	CurrentUser string
	Error       string
}

func isSolved(r *http.Request) bool {
	c, err := r.Cookie("solved_authbrute")
	return err == nil && c.Value == "1"
}

func currentUser(r *http.Request) string {
	c, err := r.Cookie("authbrute_session")
	if err != nil {
		return ""
	}
	return sessionUser(c.Value)
}

func render(w http.ResponseWriter, r *http.Request, errMsg string) {
	data := pageData{
		Solved:      isSolved(r),
		CurrentUser: currentUser(r),
		Error:       errMsg,
	}
	if err := pageTmpl.Execute(w, data); err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
	}
}

func Index(w http.ResponseWriter, r *http.Request) {
	render(w, r, "")
}

func Login(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	username := r.PostFormValue("username")
	password := r.PostFormValue("password")

	var stored string
	err := db.QueryRow(
		`SELECT password FROM users WHERE username = ? AND has_mfa = 0`, username,
	).Scan(&stored)
	if err != nil || stored != password {
		w.WriteHeader(http.StatusUnauthorized)
		render(w, r, "Invalid credentials.")
		return
	}

	sid := newSession(username)
	http.SetCookie(w, &http.Cookie{
		Name:     "authbrute_session",
		Value:    sid,
		Path:     "/authbrute/",
		SameSite: http.SameSiteLaxMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "solved_authbrute",
		Value:    "1",
		Path:     "/",
		MaxAge:   60 * 60 * 24 * 365,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, "/authbrute/", http.StatusFound)
}
