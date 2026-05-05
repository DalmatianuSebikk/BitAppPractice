package idor

import (
	"database/sql"
	"embed"
	"errors"
	"html/template"
	"net/http"
	"strconv"
)

//go:embed templates/page.html templates/doc.html
var templatesFS embed.FS

var (
	pageTmpl = template.Must(template.ParseFS(templatesFS, "templates/page.html"))
	docTmpl  = template.Must(template.ParseFS(templatesFS, "templates/doc.html"))
)

const ownerUsername = "alice"

var (
	db      *sql.DB
	ownerID int
)

func Init(d *sql.DB) {
	db = d
	if err := db.QueryRow(`SELECT id FROM users WHERE username = ?`, ownerUsername).Scan(&ownerID); err != nil {
		panic("idor: alice not seeded: " + err.Error())
	}
}

type Doc struct {
	ID      int
	OwnerID int
	Title   string
	Body    string
}

type pageData struct {
	Solved bool
	Owner  string
	Docs   []Doc
}

type docData struct {
	Solved bool
	Doc    Doc
	Mine   bool
}

func isSolved(r *http.Request) bool {
	c, err := r.Cookie("solved_idor")
	return err == nil && c.Value == "1"
}

func Index(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query(
		`SELECT id, owner_id, title, body FROM documents WHERE owner_id = ? ORDER BY id`,
		ownerID,
	)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var docs []Doc
	for rows.Next() {
		var d Doc
		if err := rows.Scan(&d.ID, &d.OwnerID, &d.Title, &d.Body); err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		docs = append(docs, d)
	}

	data := pageData{Solved: isSolved(r), Owner: ownerUsername, Docs: docs}
	if err := pageTmpl.Execute(w, data); err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
	}
}

func ShowDoc(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}

	var d Doc
	err = db.QueryRow(
		`SELECT id, owner_id, title, body FROM documents WHERE id = ?`, id,
	).Scan(&d.ID, &d.OwnerID, &d.Title, &d.Body)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	mine := d.OwnerID == ownerID
	if !mine {
		http.SetCookie(w, &http.Cookie{
			Name:     "solved_idor",
			Value:    "1",
			Path:     "/",
			MaxAge:   60 * 60 * 24 * 365,
			SameSite: http.SameSiteLaxMode,
		})
	}

	data := docData{
		Solved: isSolved(r) || !mine,
		Doc:    d,
		Mine:   mine,
	}
	if err := docTmpl.Execute(w, data); err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
	}
}
