package sqliunion

import (
	"database/sql"
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"
)

//go:embed templates/page.html
var templatesFS embed.FS

var pageTmpl = template.Must(template.ParseFS(templatesFS, "templates/page.html"))

var (
	db            *sql.DB
	adminPassword string
)

/*

articles — the source the vulnerable page reads from. Schema:


id INTEGER, title TEXT, body TEXT, author TEXT, published INTEGER
The vulnerable query is literally:


SELECT id, title, body, author, published FROM articles WHERE id = $id
5 columns, fixed shape. Each row becomes an "article card" in the rendered page (title in the heading, body in the article box, author/date in the meta line).

users — what we actually want. Schema:


id INTEGER, username TEXT, password TEXT, has_mfa INTEGER, role TEXT

The discovery path a student walks (without being told the schema):

Confirm SQLi: ?id=1 OR 1=1 -- → all articles render. Bug present.
Count columns: ?id=0 UNION SELECT 1 → fails (column mismatch). ?id=0 UNION SELECT 1,2,3,4,5 -- → a row appears with literals 1/2/3/4/5 in each slot. Now you know it's 5 columns and you can see slots 2 and 3 most prominently in the rendering.
Enumerate tables: ?id=0 UNION SELECT 1, name, sql, '', 0 FROM sqlite_master WHERE type='table' -- → SQLite's metadata table dumps every table name and its schema. users shows up.
Extract: the payload above.
*/

func Init(d *sql.DB) {
	db = d
	if err := db.QueryRow(`SELECT password FROM users WHERE username = 'admin'`).Scan(&adminPassword); err != nil {
		panic("sqliunion: admin not seeded: " + err.Error())
	}
}

type Article struct {
	ID     string
	Title  string
	Body   string
	Author string
	Posted string
}

type pageData struct {
	Solved   bool
	HasQuery bool
	QueryID  string
	Articles []Article
}

func isSolved(r *http.Request) bool {
	c, err := r.Cookie("solved_sqliunion")
	return err == nil && c.Value == "1"
}

func humanize(d time.Duration) string {
	s := int(d.Seconds())
	switch {
	case s < 60:
		return "just now"
	case s < 3600:
		return fmt.Sprintf("%dm ago", s/60)
	case s < 86400:
		return fmt.Sprintf("%dh ago", s/3600)
	case s < 86400*7:
		return fmt.Sprintf("%dd ago", s/86400)
	default:
		return fmt.Sprintf("%dw ago", s/(86400*7))
	}
}

func loadList() ([]Article, error) {
	rows, err := db.Query(`SELECT id, title, body, author, published FROM articles ORDER BY published DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	now := time.Now()
	var arts []Article
	for rows.Next() {
		var id int
		var title, body, author string
		var published int64
		if err := rows.Scan(&id, &title, &body, &author, &published); err != nil {
			return nil, err
		}
		arts = append(arts, Article{
			ID:     strconv.Itoa(id),
			Title:  title,
			Body:   body,
			Author: author,
			Posted: humanize(now.Sub(time.Unix(published, 0))),
		})
	}
	return arts, nil
}

func loadByID(idStr string) ([]Article, error) {
	query := fmt.Sprintf(
		"SELECT id, title, body, author, published FROM articles WHERE id = %s",
		idStr,
	)
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	now := time.Now()
	var arts []Article
	for rows.Next() {
		var id, title, body, author, posted sql.NullString
		if err := rows.Scan(&id, &title, &body, &author, &posted); err != nil {
			return nil, err
		}
		a := Article{
			ID:     id.String,
			Title:  title.String,
			Body:   body.String,
			Author: author.String,
			Posted: posted.String,
		}
		if ts, err := strconv.ParseInt(posted.String, 10, 64); err == nil {
			a.Posted = humanize(now.Sub(time.Unix(ts, 0)))
		}
		arts = append(arts, a)
	}
	return arts, nil
}

func Index(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")

	if idStr == "" {
		arts, err := loadList()
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		data := pageData{Solved: isSolved(r), Articles: arts}
		if err := pageTmpl.Execute(w, data); err != nil {
			http.Error(w, "render error", http.StatusInternalServerError)
		}
		return
	}

	arts, _ := loadByID(idStr)

	for _, a := range arts {
		if strings.Contains(a.ID, adminPassword) ||
			strings.Contains(a.Title, adminPassword) ||
			strings.Contains(a.Body, adminPassword) ||
			strings.Contains(a.Author, adminPassword) ||
			strings.Contains(a.Posted, adminPassword) {
			http.SetCookie(w, &http.Cookie{
				Name:     "solved_sqliunion",
				Value:    "1",
				Path:     "/",
				MaxAge:   60 * 60 * 24 * 365,
				SameSite: http.SameSiteLaxMode,
			})
			break
		}
	}

	data := pageData{
		Solved:   isSolved(r),
		HasQuery: true,
		QueryID:  idStr,
		Articles: arts,
	}
	if err := pageTmpl.Execute(w, data); err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
	}
}
