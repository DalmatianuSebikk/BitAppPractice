package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"practicabitapp/routes"
)

const schema = `
CREATE TABLE IF NOT EXISTS users (
	id       INTEGER PRIMARY KEY AUTOINCREMENT,
	username TEXT UNIQUE NOT NULL,
	password TEXT NOT NULL,
	has_mfa  INTEGER NOT NULL DEFAULT 0,
	role     TEXT NOT NULL DEFAULT 'user'
);
CREATE TABLE IF NOT EXISTS documents (
	id       INTEGER PRIMARY KEY,
	owner_id INTEGER NOT NULL,
	title    TEXT NOT NULL,
	body     TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS comments (
	id      INTEGER PRIMARY KEY AUTOINCREMENT,
	author  TEXT NOT NULL,
	body    TEXT NOT NULL,
	created INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS articles (
	id        INTEGER PRIMARY KEY,
	title     TEXT NOT NULL,
	body      TEXT NOT NULL,
	author    TEXT NOT NULL,
	published INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS products (
	id          INTEGER PRIMARY KEY,
	name        TEXT NOT NULL,
	price       INTEGER NOT NULL,
	description TEXT NOT NULL
);
`

type seedUser struct {
	username string
	password string
	hasMFA   bool
	role     string
}

var seedUsers = []seedUser{
	{"admin", "iloveyou", false, "senior_admin"},
	{"bob", "bobspass2025", true, "user"},
	{"alice", "alice-internal-2026-acct", false, "user"},
	{"mary", "mary-internal-2026-acct", false, "junior_admin"},
}

type seedDoc struct {
	id    int
	owner string
	title string
	body  string
}

var seedDocs = []seedDoc{
	{1, "admin", "Master Backup Key", "System master backup key (do not share): BD-FLAG-9f3a2c1e7d4b. Required for full-restore from cold storage."},
	{2, "alice", "Q4 marketing plan", "TL;DR: ship more, complain less. Budget approved through end of quarter."},
	{3, "alice", "Onsite checklist", "1. badges  2. coffee  3. laptop  4. snacks  5. extension cords."},
	{4, "bob", "Personal", "Reminder to update my resume before next year's review."},
}

type seedComment struct {
	id      int
	author  string
	body    string
	minsAgo int
}

var seedComments = []seedComment{
	{1, "alice", "saved my life last week debugging a memory leak that turned out to be a goroutine still listening on a closed channel", 240},
	{2, "mary", "hot take: most prod bugs at 2am are dumb typos", 480},
	{3, "bob", "the part about gdb at 4:32 changed my life", 30},
	{4, "chen42", "first comment, didn't watch", 90},
}

type seedArticle struct {
	id      int
	title   string
	body    string
	author  string
	daysAgo int
}

var seedArticles = []seedArticle{
	{1, "Q4 cloud spending hits new high", "Enterprise cloud bills jumped 23% YoY, driven by AI workloads and increased data egress costs across major hyperscalers.", "alice", 1},
	{2, "Open-source SBOM tooling sees surge", "Security teams are leaning on automated SBOM tooling to keep up with the EU's CRA reporting deadlines.", "mary", 3},
	{3, "TLS 1.4 draft enters working group review", "The IETF TLS WG accepted the first draft of TLS 1.4, focusing on post-quantum key exchange and a cleaner record layer.", "bob", 7},
	{4, "Why your incident retros are broken", "An opinion piece on blameful retrospectives, post-mortem fatigue, and whether action items survive the next on-call shift.", "alice", 14},
	{5, "Webhook signing 101", "A quick guide to HMAC-based webhook signing, replay protection, and why timing-safe comparison still matters.", "bob", 21},
}

type seedProduct struct {
	id    int
	name  string
	price int
	desc  string
}

var seedProducts = []seedProduct{
	{1, "Mechanical Keyboard 87-key", 14999, "Hot-swappable, RGB, USB-C cable included."},
	{2, "Standing Desk Mat", 4499, "Anti-fatigue mat, 30x18, beveled edges."},
	{3, "Laptop Privacy Screen 14\"", 2999, "Removable, 60° viewing angle, anti-glare coating."},
	{4, "Cable Sleeve Kit", 1299, "Set of 6 neoprene sleeves, mixed sizes."},
	{5, "Webcam Cover Pack", 599, "Slide-style, low-profile, pack of 5."},
}

func initDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(`ALTER TABLE users ADD COLUMN role TEXT NOT NULL DEFAULT 'user'`); err != nil {
		if !strings.Contains(err.Error(), "duplicate column") {
			db.Close()
			return nil, err
		}
	}
	for _, u := range seedUsers {
		mfa := 0
		if u.hasMFA {
			mfa = 1
		}
		if _, err := db.Exec(
			`INSERT OR IGNORE INTO users (username, password, has_mfa, role) VALUES (?, ?, ?, ?)`,
			u.username, u.password, mfa, u.role,
		); err != nil {
			db.Close()
			return nil, err
		}
		if _, err := db.Exec(
			`UPDATE users SET role = ? WHERE username = ?`,
			u.role, u.username,
		); err != nil {
			db.Close()
			return nil, err
		}
	}
	ownerIDs := map[string]int{}
	rows, err := db.Query(`SELECT username, id FROM users`)
	if err != nil {
		db.Close()
		return nil, err
	}
	for rows.Next() {
		var u string
		var id int
		if err := rows.Scan(&u, &id); err != nil {
			rows.Close()
			db.Close()
			return nil, err
		}
		ownerIDs[u] = id
	}
	rows.Close()
	for _, d := range seedDocs {
		oid, ok := ownerIDs[d.owner]
		if !ok {
			continue
		}
		if _, err := db.Exec(
			`INSERT OR IGNORE INTO documents (id, owner_id, title, body) VALUES (?, ?, ?, ?)`,
			d.id, oid, d.title, d.body,
		); err != nil {
			db.Close()
			return nil, err
		}
	}
	now := time.Now()
	for _, c := range seedComments {
		ts := now.Add(-time.Duration(c.minsAgo) * time.Minute).Unix()
		if _, err := db.Exec(
			`INSERT OR REPLACE INTO comments (id, author, body, created) VALUES (?, ?, ?, ?)`,
			c.id, c.author, c.body, ts,
		); err != nil {
			db.Close()
			return nil, err
		}
	}
	for _, a := range seedArticles {
		ts := now.Add(-time.Duration(a.daysAgo) * 24 * time.Hour).Unix()
		if _, err := db.Exec(
			`INSERT OR REPLACE INTO articles (id, title, body, author, published) VALUES (?, ?, ?, ?, ?)`,
			a.id, a.title, a.body, a.author, ts,
		); err != nil {
			db.Close()
			return nil, err
		}
	}
	for _, p := range seedProducts {
		if _, err := db.Exec(
			`INSERT OR REPLACE INTO products (id, name, price, description) VALUES (?, ?, ?, ?)`,
			p.id, p.name, p.price, p.desc,
		); err != nil {
			db.Close()
			return nil, err
		}
	}
	return db, nil
}

var labdataFiles = map[string]string{
	"./labdata/docs/manual.txt":       "PracticaBitApp Operations Manual\n\nThis viewer renders text files from the docs directory.",
	"./labdata/docs/readme.txt":       "Read me first. Or don't — the sidebar lists everything anyway.",
	"./labdata/docs/faq.txt":          "Q: How do I configure cookies?\nA: Carefully.\n\nQ: What about the chapter list?\nA: It's the index page.",
	"./labdata/secrets/traversal.txt": "BD-FLAG-traversal-9b3f1aef",
	"./labdata/secrets/cmdi.txt":      "BD-FLAG-cmdi-7e2a8c4d",
	"./labdata/secrets/sstirce.txt":   "BD-FLAG-sstirce-2a8c1f4d",
}

func ensureDataFiles() error {
	for path, content := range labdataFiles {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	if err := ensureDataFiles(); err != nil {
		log.Fatalf("labdata: %v", err)
	}

	db, err := initDB("practica.db")
	if err != nil {
		log.Fatalf("db init: %v", err)
	}
	defer db.Close()

	mux := http.NewServeMux()
	routes.Register(mux, db)

	addr := ":9997"
	log.Printf("listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
