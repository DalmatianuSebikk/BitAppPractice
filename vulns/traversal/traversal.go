package traversal

import (
	"embed"
	"html/template"
	"net/http"
	"os"
	"strings"
)

//go:embed templates/page.html
var templatesFS embed.FS

var pageTmpl = template.Must(template.ParseFS(templatesFS, "templates/page.html"))

const (
	docsRoot = "./labdata/docs/"
	flagTag  = "BD-FLAG-traversal-"
)

var defaultFiles = []string{"manual.txt", "readme.txt", "faq.txt"}

type pageData struct {
	Solved  bool
	Files   []string
	Current string
	Content string
	Error   string
}

func isSolved(r *http.Request) bool {
	c, err := r.Cookie("solved_traversal")
	return err == nil && c.Value == "1"
}

func filter(s string) string {
	return strings.ReplaceAll(s, "../", "")
}

func Index(w http.ResponseWriter, r *http.Request) {
	data := pageData{
		Solved: isSolved(r),
		Files:  defaultFiles,
	}

	raw := r.URL.Query().Get("file")
	if raw != "" {
		filtered := filter(raw)
		data.Current = raw
		content, err := os.ReadFile(docsRoot + filtered)
		if err != nil {
			data.Error = err.Error()
		} else {
			data.Content = string(content)
			if strings.Contains(data.Content, flagTag) {
				http.SetCookie(w, &http.Cookie{
					Name:     "solved_traversal",
					Value:    "1",
					Path:     "/",
					MaxAge:   60 * 60 * 24 * 365,
					SameSite: http.SameSiteLaxMode,
				})
				data.Solved = true
			}
		}
	}

	if err := pageTmpl.Execute(w, data); err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
	}
}
