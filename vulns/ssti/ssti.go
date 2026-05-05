package ssti

import (
	"bytes"
	"embed"
	"html/template"
	"net/http"
	"strings"

	usrtmpl "text/template"
)

//go:embed templates/page.html
var templatesFS embed.FS

var pageTmpl = template.Must(template.ParseFS(templatesFS, "templates/page.html"))

const (
	flagTag = "BD-FLAG-ssti-"
	flag    = "BD-FLAG-ssti-d8c1a73f"
)

var defaultContext = map[string]string{
	"Username": "alice",
	"Team":     "platform",
	"Date":     "2026-05-04",
	"Flag":     flag,
}

type pageData struct {
	Solved   bool
	Input    string
	Rendered string
	Error    string
	HasInput bool
}

func isSolved(r *http.Request) bool {
	c, err := r.Cookie("solved_ssti")
	return err == nil && c.Value == "1"
}

func filter(s string) string {
	return strings.ReplaceAll(s, ".Flag", "")
}

func Index(w http.ResponseWriter, r *http.Request) {
	data := pageData{Solved: isSolved(r)}

	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
		raw := r.PostFormValue("banner")
		data.Input = raw
		data.HasInput = true

		filtered := filter(raw)

		tmpl, err := usrtmpl.New("banner").Parse(filtered)
		if err != nil {
			data.Error = err.Error()
		} else {
			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, defaultContext); err != nil {
				data.Error = err.Error()
			} else {
				data.Rendered = buf.String()
				if strings.Contains(data.Rendered, flagTag) {
					http.SetCookie(w, &http.Cookie{
						Name:     "solved_ssti",
						Value:    "1",
						Path:     "/",
						MaxAge:   60 * 60 * 24 * 365,
						SameSite: http.SameSiteLaxMode,
					})
					data.Solved = true
				}
			}
		}
	}

	if err := pageTmpl.Execute(w, data); err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
	}
}
