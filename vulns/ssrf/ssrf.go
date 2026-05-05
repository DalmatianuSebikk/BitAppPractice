package ssrf

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

//go:embed templates/page.html
var templatesFS embed.FS

var pageTmpl = template.Must(template.ParseFS(templatesFS, "templates/page.html"))

const (
	flagTag = "BD-FLAG-ssrf-"
	flag    = "BD-FLAG-ssrf-c4e7a91f"
)

var (
	ssrfToken  string
	httpClient = &http.Client{
		Timeout: 5 * time.Second,
	}
)

func init() {
	b := make([]byte, 16)
	rand.Read(b)
	ssrfToken = hex.EncodeToString(b)
}

type pageData struct {
	Solved    bool
	URL       string
	Status    string
	Body      string
	Error     string
	HasResult bool
}

func isSolved(r *http.Request) bool {
	c, err := r.Cookie("solved_ssrf")
	return err == nil && c.Value == "1"
}

func Index(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Backend-Path", "/ssrf/internal/")

	rawURL := r.URL.Query().Get("url")
	data := pageData{Solved: isSolved(r)}

	if rawURL == "" {
		if err := pageTmpl.Execute(w, data); err != nil {
			http.Error(w, "render error", http.StatusInternalServerError)
		}
		return
	}

	data.URL = rawURL
	data.HasResult = true

	parsed, err := url.Parse(rawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		data.Error = "URL must be http or https."
		if err := pageTmpl.Execute(w, data); err != nil {
			http.Error(w, "render error", http.StatusInternalServerError)
		}
		return
	}

	req, err := http.NewRequestWithContext(r.Context(), "GET", rawURL, nil)
	if err != nil {
		data.Error = err.Error()
		if err := pageTmpl.Execute(w, data); err != nil {
			http.Error(w, "render error", http.StatusInternalServerError)
		}
		return
	}
	req.Header.Set("X-SSRF-Token", ssrfToken)
	req.Header.Set("User-Agent", "BitFetch/1.0")

	resp, err := httpClient.Do(req)
	if err != nil {
		data.Error = err.Error()
		if err := pageTmpl.Execute(w, data); err != nil {
			http.Error(w, "render error", http.StatusInternalServerError)
		}
		return
	}
	defer resp.Body.Close()

	data.Status = resp.Status
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	data.Body = string(body)

	if strings.Contains(data.Body, flagTag) {
		http.SetCookie(w, &http.Cookie{
			Name:     "solved_ssrf",
			Value:    "1",
			Path:     "/",
			MaxAge:   60 * 60 * 24 * 365,
			SameSite: http.SameSiteLaxMode,
		})
		data.Solved = true
	}

	if err := pageTmpl.Execute(w, data); err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
	}
}

func Internal(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-SSRF-Token") != ssrfToken {
		http.NotFound(w, r)
		return
	}

	path := r.PathValue("path")
	if path == "admin" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintf(w, "BitFetch admin gateway\n\nflag: %s\n", flag)
		return
	}

	http.NotFound(w, r)
}
