package cmdi

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

//go:embed templates/page.html
var templatesFS embed.FS

var pageTmpl = template.Must(template.ParseFS(templatesFS, "templates/page.html"))

const flagTag = "BD-FLAG-cmdi-"

var forbidden = []string{";", "&", "|", "\n", "\r"}

func filter(s string) string {
	for _, f := range forbidden {
		s = strings.ReplaceAll(s, f, "")
	}
	return s
}

type pageData struct {
	Solved      bool
	Host        string
	Output      string
	HasResult   bool
}

func isSolved(r *http.Request) bool {
	c, err := r.Cookie("solved_cmdi")
	return err == nil && c.Value == "1"
}

func render(w http.ResponseWriter, r *http.Request, data pageData) {
	if err := pageTmpl.Execute(w, data); err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
	}
}

func Index(w http.ResponseWriter, r *http.Request) {
	render(w, r, pageData{Solved: isSolved(r)})
}

func Ping(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	raw := r.PostFormValue("host")
	host := filter(raw)

	if strings.TrimSpace(host) == "" {
		render(w, r, pageData{Solved: isSolved(r), Host: raw})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sh", "-c", fmt.Sprintf("ping -c 1 %s", host))
	output, _ := cmd.CombinedOutput()
	out := string(output)

	if strings.Contains(out, flagTag) {
		http.SetCookie(w, &http.Cookie{
			Name:     "solved_cmdi",
			Value:    "1",
			Path:     "/",
			MaxAge:   60 * 60 * 24 * 365,
			SameSite: http.SameSiteLaxMode,
		})
	}

	render(w, r, pageData{
		Solved:    isSolved(r) || strings.Contains(out, flagTag),
		Host:      raw,
		Output:    out,
		HasResult: true,
	})
}
