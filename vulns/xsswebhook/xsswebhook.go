package xsswebhook

import (
	"bytes"
	"context"
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

const adminSecret = "BD-FLAG-xsswebhook-9c2e1f78"

var (
	scriptRegex       = regexp.MustCompile(`(?i)<\s*/?\s*script[^>]*>`)
	eventHandlerRegex = regexp.MustCompile(`(?i)\s+on[a-z]+\s*=\s*("[^"]*"|'[^']*'|[^\s>]+)`)

	urlAttrDoubleQuoted = regexp.MustCompile(`(?i)\b(?:src|href|data|action)\s*=\s*"(https?://[^"]*)`)
	urlAttrSingleQuoted = regexp.MustCompile(`(?i)\b(?:src|href|data|action)\s*=\s*'(https?://[^']*)`)
	urlAttrUnquoted     = regexp.MustCompile(`(?i)\b(?:src|href|data|action)\s*=\s*(https?://[^\s>"']+)`)

	httpClient = &http.Client{Timeout: 5 * time.Second}
)

func filter(s string) string {
	s = scriptRegex.ReplaceAllString(s, "")
	s = eventHandlerRegex.ReplaceAllString(s, "")
	return s
}

func extractURLs(html string) []string {
	var urls []string
	for _, re := range []*regexp.Regexp{urlAttrDoubleQuoted, urlAttrSingleQuoted, urlAttrUnquoted} {
		for _, m := range re.FindAllStringSubmatch(html, -1) {
			if len(m) >= 2 {
				urls = append(urls, m[1])
			}
		}
	}
	return urls
}

func isExternalHost(rawURL string) bool {
	var afterScheme string
	switch {
	case strings.HasPrefix(rawURL, "https://"):
		afterScheme = rawURL[len("https://"):]
	case strings.HasPrefix(rawURL, "http://"):
		afterScheme = rawURL[len("http://"):]
	default:
		return false
	}
	end := strings.IndexAny(afterScheme, "/?#")
	host := afterScheme
	if end != -1 {
		host = afterScheme[:end]
	}
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return false
	}
	if strings.HasPrefix(host, "localhost") || strings.HasPrefix(host, "127.") || strings.HasPrefix(host, "[::1]") {
		return false
	}
	return true
}

func sanitizeURL(rawURL string) string {
	var schemeEnd int
	switch {
	case strings.HasPrefix(rawURL, "https://"):
		schemeEnd = len("https://")
	case strings.HasPrefix(rawURL, "http://"):
		schemeEnd = len("http://")
	default:
		return ""
	}
	rest := rawURL[schemeEnd:]
	end := strings.IndexAny(rest, "/?#")
	if end == -1 {
		return rawURL
	}
	host := rest[:end]
	pathQuery := rest[end:]
	var b strings.Builder
	for i := 0; i < len(pathQuery); i++ {
		c := pathQuery[i]
		if urlSafeByte(c) {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return rawURL[:schemeEnd] + host + b.String()
}

func urlSafeByte(c byte) bool {
	switch {
	case c >= 'A' && c <= 'Z':
		return true
	case c >= 'a' && c <= 'z':
		return true
	case c >= '0' && c <= '9':
		return true
	}
	switch c {
	case '-', '_', '.', '~',
		'/', '?', '#', '&', '=', ':', '@', '+', '!', '$',
		'(', ')', '*', ',', ';', '%':
		return true
	}
	return false
}

func fireBackgroundRequest(rawURL string) {
	sanitized := sanitizeURL(rawURL)
	if sanitized == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", sanitized, nil)
	if err != nil {
		return
	}
	req.Header.Set("User-Agent", "BitBounty-RenderBot/1.0")
	resp, err := httpClient.Do(req)
	if err == nil {
		resp.Body.Close()
	}
}

type pageData struct {
	Solved      bool
	HasInput    bool
	Body        template.HTML
	AdminSecret string
}

func isSolved(r *http.Request) bool {
	c, err := r.Cookie("solved_xsswebhook")
	return err == nil && c.Value == "1"
}

func renderAndDeliver(w http.ResponseWriter, r *http.Request, data pageData) {
	var buf bytes.Buffer
	if err := pageTmpl.Execute(&buf, data); err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
		return
	}

	if data.HasInput {
		rendered := buf.String()
		urls := extractURLs(rendered)
		secretLeaked := false
		for _, u := range urls {
			if !isExternalHost(u) {
				continue
			}
			go fireBackgroundRequest(u)
			if strings.Contains(u, adminSecret) {
				secretLeaked = true
			}
		}
		if secretLeaked {
			http.SetCookie(w, &http.Cookie{
				Name:     "solved_xsswebhook",
				Value:    "1",
				Path:     "/",
				MaxAge:   60 * 60 * 24 * 365,
				SameSite: http.SameSiteLaxMode,
			})
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(buf.Bytes())
}

func Index(w http.ResponseWriter, r *http.Request) {
	renderAndDeliver(w, r, pageData{
		Solved:      isSolved(r),
		AdminSecret: adminSecret,
	})
}

func Post(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	body := r.PostFormValue("body")
	if strings.TrimSpace(body) == "" {
		http.Redirect(w, r, "/xsswebhook/", http.StatusFound)
		return
	}
	filtered := filter(body)
	renderAndDeliver(w, r, pageData{
		Solved:      isSolved(r),
		HasInput:    true,
		Body:        template.HTML(filtered),
		AdminSecret: adminSecret,
	})
}
