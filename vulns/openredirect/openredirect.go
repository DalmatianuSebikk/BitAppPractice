package openredirect

import (
	"embed"
	"html/template"
	"net/http"
)

//go:embed templates/page.html
var templatesFS embed.FS

var pageTmpl = template.Must(template.ParseFS(templatesFS, "templates/page.html"))

type article struct {
	Source string
	Title  string
	Desc   string
	URL    string
	Href   template.URL
}

var articles = []article{
	{Source: "Wikipedia", Title: "Phishing", Desc: "An overview of social-engineering attacks against authentication systems.", URL: "https://en.wikipedia.org/wiki/Phishing"},
	{Source: "OWASP", Title: "Unvalidated Redirects & Forwards", Desc: "The community cheat sheet on the very class of bug you're staring at.", URL: "https://owasp.org/www-community/attacks/Unvalidated_Redirects_and_Forwards_Cheat_Sheet"},
	{Source: "IETF", Title: "RFC 6749 — OAuth 2.0", Desc: "Why open redirects on auth flows are an existential threat.", URL: "https://datatracker.ietf.org/doc/html/rfc6749"},
}

func init() {
	for i := range articles {
		articles[i].Href = template.URL("/openredirect/go?url=" + articles[i].URL)
	}
}

type pageData struct {
	Articles []article
	Solved   bool
}

func Index(w http.ResponseWriter, r *http.Request) {
	data := pageData{Articles: articles, Solved: isSolved(r)}
	if err := pageTmpl.Execute(w, data); err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
	}
}

func Go(w http.ResponseWriter, r *http.Request) {
	target := r.URL.Query().Get("url")
	if target == "" {
		http.Error(w, "missing url", http.StatusBadRequest)
		return
	}
	if !isCurated(target) {
		http.SetCookie(w, &http.Cookie{
			Name:     "solved_openredirect",
			Value:    "1",
			Path:     "/",
			MaxAge:   60 * 60 * 24 * 365,
			SameSite: http.SameSiteLaxMode,
		})
	}
	http.Redirect(w, r, target, http.StatusFound)
}

func isSolved(r *http.Request) bool {
	c, err := r.Cookie("solved_openredirect")
	return err == nil && c.Value == "1"
}

func isCurated(u string) bool {
	for _, a := range articles {
		if a.URL == u {
			return true
		}
	}
	return false
}
