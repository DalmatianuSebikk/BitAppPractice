package xssstored

import (
	"database/sql"
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"regexp"
	"strings"
	"time"
)

//go:embed templates/page.html
var templatesFS embed.FS

var pageTmpl = template.Must(template.ParseFS(templatesFS, "templates/page.html"))

var db *sql.DB

func Init(d *sql.DB) { db = d }

type Comment struct {
	ID       int
	Author   string
	Body     template.HTML
	Created  time.Time
	Relative string
}

var scriptTagRegex = regexp.MustCompile(`(?i)<\s*/?\s*script[^>]*>`)

func filter(s string) string {
	return scriptTagRegex.ReplaceAllString(s, "")
}

var xssTriggers = []string{
	"<script", "onerror=", "onload=", "onclick=", "onmouseover=",
	"<svg", "<iframe", "<img", "javascript:",
}

func looksLikeXSS(s string) bool {
	lower := strings.ToLower(s)
	for _, t := range xssTriggers {
		if strings.Contains(lower, t) {
			return true
		}
	}
	return false
}

func isSolved(r *http.Request) bool {
	c, err := r.Cookie("solved_xssstored")
	return err == nil && c.Value == "1"
}

type pageData struct {
	Solved   bool
	Count    int
	Comments []Comment
}

func humanize(d time.Duration) string {
	s := int(d.Seconds())
	switch {
	case s < 60:
		return "just now"
	case s < 3600:
		return fmt.Sprintf("%d min ago", s/60)
	case s < 86400:
		return fmt.Sprintf("%d hr ago", s/3600)
	case s < 86400*7:
		return fmt.Sprintf("%d days ago", s/86400)
	default:
		return fmt.Sprintf("%d weeks ago", s/(86400*7))
	}
}

func loadComments() ([]Comment, error) {
	rows, err := db.Query(`SELECT id, author, body, created FROM comments ORDER BY created DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var comments []Comment
	now := time.Now()
	for rows.Next() {
		var c Comment
		var ts int64
		var body string
		if err := rows.Scan(&c.ID, &c.Author, &body, &ts); err != nil {
			return nil, err
		}
		c.Body = template.HTML(body)
		c.Created = time.Unix(ts, 0)
		c.Relative = humanize(now.Sub(c.Created))
		comments = append(comments, c)
	}
	return comments, nil
}

func Index(w http.ResponseWriter, r *http.Request) {
	comments, err := loadComments()
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	data := pageData{Solved: isSolved(r), Count: len(comments), Comments: comments}
	if err := pageTmpl.Execute(w, data); err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
	}
}

func Post(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	body := r.PostFormValue("body")
	if strings.TrimSpace(body) == "" {
		http.Redirect(w, r, "/xssstored/", http.StatusFound)
		return
	}

	filtered := filter(body)
	if strings.TrimSpace(filtered) == "" {
		http.Redirect(w, r, "/xssstored/", http.StatusFound)
		return
	}

	if _, err := db.Exec(
		`INSERT INTO comments (author, body, created) VALUES (?, ?, ?)`,
		"guest", filtered, time.Now().Unix(),
	); err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	if looksLikeXSS(filtered) {
		http.SetCookie(w, &http.Cookie{
			Name:     "solved_xssstored",
			Value:    "1",
			Path:     "/",
			MaxAge:   60 * 60 * 24 * 365,
			SameSite: http.SameSiteLaxMode,
		})
	}

	http.Redirect(w, r, "/xssstored/", http.StatusFound)
}
