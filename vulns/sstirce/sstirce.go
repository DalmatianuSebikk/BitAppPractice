package sstirce

import (
	"bytes"
	"embed"
	"html/template"
	"net/http"
	"os/exec"
	"strings"

	usrtmpl "text/template"
)

//go:embed templates/page.html
var templatesFS embed.FS

var pageTmpl = template.Must(template.ParseFS(templatesFS, "templates/page.html"))

const flagTag = "BD-FLAG-sstirce-"

type adminContext struct {
	Username string
	Team     string
	Status   string
}

// Run is exposed for "ops debug" — it shells out and returns combined output.
// In a real app this would never have made it past code review.
func (c *adminContext) Run(cmd string) string {
	out, _ := exec.Command("sh", "-c", cmd).CombinedOutput()
	return string(out)
}

var defaultContext = &adminContext{
	Username: "ops-admin",
	Team:     "platform",
	Status:   "all systems operational",
}

type pageData struct {
	Solved   bool
	Input    string
	Rendered string
	Error    string
	HasInput bool
}

func isSolved(r *http.Request) bool {
	c, err := r.Cookie("solved_sstirce")
	return err == nil && c.Value == "1"
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

		tmpl, err := usrtmpl.New("banner").Parse(raw)
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
						Name:     "solved_sstirce",
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
