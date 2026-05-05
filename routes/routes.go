package routes

import (
	"database/sql"
	"embed"
	"html/template"
	"io/fs"
	"net/http"

	"practicabitapp/vulns/authbrute"
	"practicabitapp/vulns/authjwt"
	"practicabitapp/vulns/authmfa"
	"practicabitapp/vulns/bac"
	"practicabitapp/vulns/cmdi"
	"practicabitapp/vulns/idor"
	"practicabitapp/vulns/midor"
	"practicabitapp/vulns/openredirect"
	"practicabitapp/vulns/sqlierror"
	"practicabitapp/vulns/sqlitime"
	"practicabitapp/vulns/sqliunion"
	"practicabitapp/vulns/ssrf"
	"practicabitapp/vulns/ssti"
	"practicabitapp/vulns/sstirce"
	"practicabitapp/vulns/traversal"
	"practicabitapp/vulns/xssrefl"
	"practicabitapp/vulns/xsswebhook"
	"practicabitapp/vulns/xssstored"
)

//go:embed templates/index.html
var templatesFS embed.FS

var indexTmpl = template.Must(template.ParseFS(templatesFS, "templates/index.html"))

//go:embed static
var staticFS embed.FS

type Chapter struct {
	Number      int
	Title       string
	Slug        string
	Description string
	Available   bool
	Solved      bool
}

var chapters = []Chapter{
	{Number: 1, Title: "Open Redirect", Slug: "openredirect", Description: "Turn an innocent click-tracker into a phishing primitive.", Available: true},
	{Number: 2, Title: "Bruteforce Login", Slug: "authbrute", Description: "Get into the staff portal as the admin.", Available: true},
	{Number: 3, Title: "MFA Bypass", Slug: "authmfa", Description: "Get past the second factor on a real-looking 2FA flow.", Available: true},
	{Number: 4, Title: "JWT — Weak Secret", Slug: "authjwt", Description: "Crack the signing key on an internal dashboard's token.", Available: true},
	{Number: 5, Title: "SQLi — UNION", Slug: "sqliunion", Description: "Pull data from a hidden table by piggybacking on a vulnerable query.", Available: true},
	{Number: 6, Title: "SQLi — Error-based", Slug: "sqlierror", Description: "Coax a database error into leaking the data you can't see.", Available: true},
	{Number: 7, Title: "SQLi — Time-based", Slug: "sqlitime", Description: "Extract data one bit at a time from a query that returns nothing.", Available: true},
	{Number: 8, Title: "Reflected XSS", Slug: "xssrefl", Description: "Pop an alert by smuggling script through a search query.", Available: true},
	{Number: 9, Title: "Stored XSS", Slug: "xssstored", Description: "Slip past a comment filter and persist a payload.", Available: true},
	{Number: 10, Title: "DOM XSS", Slug: "xssdom", Description: "Drop a payload into a page that builds the DOM from the URL hash."},
	{Number: 11, Title: "IDOR — ID Swap", Slug: "idor", Description: "Read another user's document by tweaking the URL.", Available: true},
	{Number: 12, Title: "Broken Access Control", Slug: "bac", Description: "Edit a senior admin's profile through a junior admin's panel.", Available: true},
	{Number: 13, Title: "Path Traversal", Slug: "traversal", Description: "Escape a docs viewer through a half-baked sanitizer.", Available: true},
	{Number: 14, Title: "Command Injection", Slug: "cmdi", Description: "Run arbitrary shell commands via a network-tools form with weak filters.", Available: true},
	{Number: 15, Title: "SSRF", Slug: "ssrf", Description: "Make the server fetch something it shouldn't.", Available: true},
	{Number: 16, Title: "SSTI — Data Leak", Slug: "ssti", Description: "Slip template syntax through a personalization feature.", Available: true},
	{Number: 17, Title: "SSTI — RCE", Slug: "sstirce", Description: "Reach an exec helper through a vulnerable template.", Available: true},
	{Number: 18, Title: "IDOR — Method Tampering", Slug: "midor", Description: "Slip past a partial auth check by changing the HTTP verb.", Available: true},
	{Number: 19, Title: "XSS — Webhook Exfil", Slug: "xsswebhook", Description: "Beat a stricter filter and exfil to a webhook of your choice.", Available: true},
}

func Register(mux *http.ServeMux, db *sql.DB) {
	authbrute.Init(db)
	authmfa.Init(db)
	bac.Init(db)
	idor.Init(db)
	midor.Init(db)
	sqlierror.Init(db)
	sqliunion.Init(db)
	sqlitime.Init(db)
	xssstored.Init(db)

	staticSub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic("routes: static FS sub: " + err.Error())
	}
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServerFS(staticSub)))

	mux.HandleFunc("/{$}", index)
	mux.HandleFunc("/openredirect/{$}", openredirect.Index)
	mux.HandleFunc("/openredirect/go", openredirect.Go)
	mux.HandleFunc("/authbrute/{$}", authbrute.Index)
	mux.HandleFunc("POST /authbrute/login", authbrute.Login)
	mux.HandleFunc("/authmfa/{$}", authmfa.Index)
	mux.HandleFunc("POST /authmfa/login", authmfa.Login)
	mux.HandleFunc("POST /authmfa/verify", authmfa.Verify)
	mux.HandleFunc("/authjwt/{$}", authjwt.Index)
	mux.HandleFunc("POST /authjwt/login", authjwt.Login)
	mux.HandleFunc("/authjwt/admin", authjwt.Admin)
	mux.HandleFunc("/idor/{$}", idor.Index)
	mux.HandleFunc("/idor/doc", idor.ShowDoc)
	mux.HandleFunc("/bac/{$}", bac.Index)
	mux.HandleFunc("POST /bac/users/{id}/role", bac.ChangeRole)
	mux.HandleFunc("/xssrefl/{$}", xssrefl.Index)
	mux.HandleFunc("/xssstored/{$}", xssstored.Index)
	mux.HandleFunc("POST /xssstored/comment", xssstored.Post)
	mux.HandleFunc("/sqliunion/{$}", sqliunion.Index)
	mux.HandleFunc("/sqlierror/{$}", sqlierror.Index)
	mux.HandleFunc("/sqlitime/{$}", sqlitime.Index)
	mux.HandleFunc("/traversal/{$}", traversal.Index)
	mux.HandleFunc("/cmdi/{$}", cmdi.Index)
	mux.HandleFunc("POST /cmdi/ping", cmdi.Ping)
	mux.HandleFunc("/ssrf/{$}", ssrf.Index)
	mux.HandleFunc("/ssrf/internal/{path...}", ssrf.Internal)
	mux.HandleFunc("/ssti/{$}", ssti.Index)
	mux.HandleFunc("/sstirce/{$}", sstirce.Index)
	mux.HandleFunc("/midor/{$}", midor.Index)
	mux.HandleFunc("/midor/prefs", midor.Prefs)
	mux.HandleFunc("/xsswebhook/{$}", xsswebhook.Index)
	mux.HandleFunc("POST /xsswebhook/submit", xsswebhook.Post)
}

func index(w http.ResponseWriter, r *http.Request) {
	rendered := make([]Chapter, len(chapters))
	for i, c := range chapters {
		c.Solved = isSolved(r, c.Slug)
		rendered[i] = c
	}
	if err := indexTmpl.Execute(w, rendered); err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
	}
}

func isSolved(r *http.Request, slug string) bool {
	c, err := r.Cookie("solved_" + slug)
	return err == nil && c.Value == "1"
}
