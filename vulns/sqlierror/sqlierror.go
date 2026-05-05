package sqlierror

import (
	"database/sql"
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
)

//go:embed templates/page.html
var templatesFS embed.FS

/*

Sure. Two things going on: a function-specific quirk, and why we have to reach for it on SQLite specifically.

json_extract's contract
SQLite ships with the JSON1 extension built in. json_extract(doc, path) takes a JSON string and a JSONPath:


SELECT json_extract('{"x":1}', '$.x');   -- returns 1
SELECT json_extract('{"x":1}', '$.y');   -- returns NULL (path missing)
JSONPaths must start with $. If they don't, SQLite raises a runtime error and echoes the bad path verbatim into the error message:


SELECT json_extract('{}', 'iloveyou');
-- Error: bad JSON path: 'iloveyou'
That last detail is the key: the malformed path gets stamped into the error string. So if you can make the path argument come from a SELECT, whatever that subquery returns shows up in the error.
JSONPaths must start with $. If they don't, SQLite raises a runtime error and echoes the bad path verbatim into the error message:


SELECT json_extract('{}', 'iloveyou');
-- Error: bad JSON path: 'iloveyou'
That last detail is the key: the malformed path gets stamped into the error string. So if you can make the path argument come from a SELECT, whatever that subquery returns shows up in the error.

The exfiltration
Server's vulnerable query (after our ?id= is concatenated):


SELECT id, name, price, description FROM products
WHERE id = json_extract('{}', (SELECT password FROM users WHERE username='admin'))
What SQLite does row-by-row:

Resolve the inner subquery: SELECT password FROM users WHERE username='admin' → string 'iloveyou'.
Call json_extract('{}', 'iloveyou').
'iloveyou' doesn't start with $ → runtime error: bad JSON path: 'iloveyou'.
The whole top-level query aborts with that error.
The page is built to display SQL errors in the red box (that's the chapter's intentional bug), so the rendered page contains the literal admin password sitting inside the error text. Game over.

Why we need this on SQLite
Most error-based SQLi tutorials show MySQL/Postgres because those engines have several functions that error in informative ways with arbitrary input:

MySQL: extractvalue(1, concat(0x7e,(SELECT …))) — XPath syntax error includes the concat'd string.
MySQL: updatexml(1, concat(0x7e,(SELECT …)),1) — same idea.
MySQL/MariaDB: duplicate-key tricks via floor(rand()*2).
Postgres: cast((SELECT …) AS int) — text-to-int errors include the value.
SQLite is unusually forgiving and has almost none of those handles:

CAST('iloveyou' AS INTEGER) returns 0 silently.
Division by zero returns NULL, doesn't error.
Type mismatches usually get coerced.
RAISE() only works inside trigger bodies.
The narrow set of things that do error informatively on SQLite are the JSON1 functions — they're stricter about their input grammar than the rest of the library. json_extract with a malformed path is the cleanest, but json_each, json_object with odd-arg counts, etc. all have similar properties. The json_extract trick is what circulates in CTF write-ups specifically because of this gap.
*/

var pageTmpl = template.Must(template.ParseFS(templatesFS, "templates/page.html"))

var (
	db            *sql.DB
	adminPassword string
)

func Init(d *sql.DB) {
	db = d
	if err := db.QueryRow(`SELECT password FROM users WHERE username = 'admin'`).Scan(&adminPassword); err != nil {
		panic("sqlierror: admin not seeded: " + err.Error())
	}
}

type Product struct {
	ID    string
	Name  string
	Price string
	Desc  string
}

type pageData struct {
	Solved   bool
	HasQuery bool
	QueryID  string
	Products []Product
	DBError  string
}

func isSolved(r *http.Request) bool {
	c, err := r.Cookie("solved_sqlierror")
	return err == nil && c.Value == "1"
}

func formatPrice(cents int64) string {
	return fmt.Sprintf("$%d.%02d", cents/100, cents%100)
}

func loadList() ([]Product, error) {
	rows, err := db.Query(`SELECT id, name, price, description FROM products ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ps []Product
	for rows.Next() {
		var id int
		var name, desc string
		var price int64
		if err := rows.Scan(&id, &name, &price, &desc); err != nil {
			return nil, err
		}
		ps = append(ps, Product{
			ID:    strconv.Itoa(id),
			Name:  name,
			Price: formatPrice(price),
			Desc:  desc,
		})
	}
	return ps, nil
}

func loadByID(idStr string) ([]Product, error) {
	query := fmt.Sprintf(
		"SELECT id, name, price, description FROM products WHERE id = %s",
		idStr,
	)
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ps []Product
	for rows.Next() {
		var id, name, price, desc sql.NullString
		if err := rows.Scan(&id, &name, &price, &desc); err != nil {
			return nil, err
		}
		p := Product{
			ID:    id.String,
			Name:  name.String,
			Price: price.String,
			Desc:  desc.String,
		}
		if cents, err := strconv.ParseInt(price.String, 10, 64); err == nil {
			p.Price = formatPrice(cents)
		}
		ps = append(ps, p)
	}
	return ps, nil
}

func Index(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")

	if idStr == "" {
		ps, err := loadList()
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		data := pageData{Solved: isSolved(r), Products: ps}
		if err := pageTmpl.Execute(w, data); err != nil {
			http.Error(w, "render error", http.StatusInternalServerError)
		}
		return
	}

	ps, err := loadByID(idStr)
	data := pageData{
		Solved:   isSolved(r),
		HasQuery: true,
		QueryID:  idStr,
		Products: ps,
	}
	if err != nil {
		data.DBError = err.Error()
	}

	if adminPassword != "" {
		hit := strings.Contains(data.DBError, adminPassword)
		if !hit {
			for _, p := range ps {
				if strings.Contains(p.ID, adminPassword) ||
					strings.Contains(p.Name, adminPassword) ||
					strings.Contains(p.Price, adminPassword) ||
					strings.Contains(p.Desc, adminPassword) {
					hit = true
					break
				}
			}
		}
		if hit {
			http.SetCookie(w, &http.Cookie{
				Name:     "solved_sqlierror",
				Value:    "1",
				Path:     "/",
				MaxAge:   60 * 60 * 24 * 365,
				SameSite: http.SameSiteLaxMode,
			})
			data.Solved = true
		}
	}

	if err := pageTmpl.Execute(w, data); err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
	}
}
