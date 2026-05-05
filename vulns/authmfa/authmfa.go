package authmfa

import (
	"crypto/rand"
	"database/sql"
	"embed"
	"encoding/hex"
	"fmt"
	"html/template"
	"log"
	"math/big"
	"net/http"
	"sync"
)

//go:embed templates/page.html
var templatesFS embed.FS

var pageTmpl = template.Must(template.ParseFS(templatesFS, "templates/page.html"))

var db *sql.DB

func Init(d *sql.DB) { db = d }

type Session struct {
	Username  string
	OTP       string
	MFAPassed bool
}

var sessions = struct {
	sync.Mutex
	m map[string]*Session
}{m: map[string]*Session{}}

func newSessionID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func newOTP() string {
	n, _ := rand.Int(rand.Reader, big.NewInt(1000000))
	return fmt.Sprintf("%06d", n)
}

type pageData struct {
	Solved bool
	User   *Session
	Error  string
}

func isSolved(r *http.Request) bool {
	c, err := r.Cookie("solved_authmfa")
	return err == nil && c.Value == "1"
}

func currentSession(r *http.Request) *Session {
	c, err := r.Cookie("authmfa_session")
	if err != nil {
		return nil
	}
	sessions.Lock()
	defer sessions.Unlock()
	return sessions.m[c.Value]
}

func render(w http.ResponseWriter, r *http.Request, errMsg string) {
	data := pageData{
		Solved: isSolved(r),
		User:   currentSession(r),
		Error:  errMsg,
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
		`SELECT password FROM users WHERE username = ? AND has_mfa = 1`, username,
	).Scan(&stored)
	if err != nil || stored != password {
		w.WriteHeader(http.StatusUnauthorized)
		render(w, r, "Invalid credentials.")
		return
	}

	sid := newSessionID()
	otp := newOTP()
	sessions.Lock()
	sessions.m[sid] = &Session{Username: username, OTP: otp, MFAPassed: false}
	sessions.Unlock()

	log.Printf("[authmfa] OTP for %s: %s", username, otp)

	http.SetCookie(w, &http.Cookie{
		Name:     "authmfa_session",
		Value:    sid,
		Path:     "/authmfa/",
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, "/authmfa/", http.StatusFound)
}

func Verify(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	code := r.PostFormValue("code")

	sidCookie, err := r.Cookie("authmfa_session")
	if err != nil {
		http.Redirect(w, r, "/authmfa/", http.StatusFound)
		return
	}

	sessions.Lock()
	s, ok := sessions.m[sidCookie.Value]
	if !ok {
		sessions.Unlock()
		http.Redirect(w, r, "/authmfa/", http.StatusFound)
		return
	}
	if s.MFAPassed {
		sessions.Unlock()
		http.Redirect(w, r, "/authmfa/", http.StatusFound)
		return
	}
	if s.OTP != code {
		sessions.Unlock()
		w.WriteHeader(http.StatusUnauthorized)
		render(w, r, "Wrong code.")
		return
	}
	s.MFAPassed = true
	sessions.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:     "solved_authmfa",
		Value:    "1",
		Path:     "/",
		MaxAge:   60 * 60 * 24 * 365,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, "/authmfa/", http.StatusFound)
}
