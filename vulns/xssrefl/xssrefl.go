package xssrefl

import (
	"embed"
	"html/template"
	"net/http"
	"strings"
)

//go:embed templates/page.html
var templatesFS embed.FS

var pageTmpl = template.Must(template.ParseFS(templatesFS, "templates/page.html"))

type SearchResult struct {
	Title string
	URL   template.URL
	Snip  string
}

var fakeResults = []SearchResult{
	{"OWASP — Cross-Site Scripting", "https://owasp.org/www-community/attacks/xss/", "An overview of XSS attacks and their variants in modern applications."},
	{"Cross-Site Scripting — Wikipedia", "https://en.wikipedia.org/wiki/Cross-site_scripting", "XSS is a type of security vulnerability typically found in web applications."},
	{"XSS Cheat Sheet — PortSwigger", "https://portswigger.net/web-security/cross-site-scripting/cheat-sheet", "A reference for XSS payloads, bypasses, and quirks across browsers."},
}

type pageData struct {
	Solved       bool
	Query        template.HTML
	QueryEscaped string
	HasQuery     bool
	Results      []SearchResult
}

func isSolved(r *http.Request) bool {
	c, err := r.Cookie("solved_xssrefl")
	return err == nil && c.Value == "1"
}

var xssTriggers = []string{
	"<script", "onerror=", "onload=", "onclick=", "onmouseover=",
	"javascript:", "<iframe", "<svg", "<img",
}

func looksLikeXSS(q string) bool {
	lower := strings.ToLower(q)
	for _, t := range xssTriggers {
		if strings.Contains(lower, t) {
			return true
		}
	}
	return false
}

func Index(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")

	if q != "" && looksLikeXSS(q) {
		http.SetCookie(w, &http.Cookie{
			Name:     "solved_xssrefl",
			Value:    "1",
			Path:     "/",
			MaxAge:   60 * 60 * 24 * 365,
			SameSite: http.SameSiteLaxMode,
		})
	}

	data := pageData{
		Solved:       isSolved(r),
		Query:        template.HTML(q),
		QueryEscaped: q,
		HasQuery:     q != "",
	}
	if q != "" {
		data.Results = fakeResults
	}

	if err := pageTmpl.Execute(w, data); err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
	}
}
