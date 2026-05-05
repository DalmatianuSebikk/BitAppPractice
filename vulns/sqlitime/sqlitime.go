package sqlitime

import (
	"database/sql"
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"time"
)

//go:embed templates/page.html
var templatesFS embed.FS

var pageTmpl = template.Must(template.ParseFS(templatesFS, "templates/page.html"))

var db *sql.DB

func Init(d *sql.DB) { db = d }

const slowThreshold = time.Second

type pageData struct {
	Solved   bool
	HasQuery bool
	QueryID  string
	Found    bool
}

func isSolved(r *http.Request) bool {
	c, err := r.Cookie("solved_sqlitime")
	return err == nil && c.Value == "1"
}

func Index(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")

	if idStr == "" {
		if err := pageTmpl.Execute(w, pageData{Solved: isSolved(r)}); err != nil {
			http.Error(w, "render error", http.StatusInternalServerError)
		}
		return
	}

	query := fmt.Sprintf("SELECT 1 FROM articles WHERE id = %s LIMIT 1", idStr)

	start := time.Now()
	rows, err := db.Query(query)
	found := false
	if err == nil {
		for rows.Next() {
			found = true
		}
		rows.Close()
	}
	elapsed := time.Since(start)

	if elapsed > slowThreshold {
		http.SetCookie(w, &http.Cookie{
			Name:     "solved_sqlitime",
			Value:    "1",
			Path:     "/",
			MaxAge:   60 * 60 * 24 * 365,
			SameSite: http.SameSiteLaxMode,
		})
	}

	data := pageData{
		Solved:   isSolved(r) || elapsed > slowThreshold,
		HasQuery: true,
		QueryID:  idStr,
		Found:    found,
	}
	if err := pageTmpl.Execute(w, data); err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
	}
}
