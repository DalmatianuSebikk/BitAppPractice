# PracticaBitApp

Deliberately-vulnerable web security lab, written in Go. Each chapter is a
self-contained mini-app demonstrating one vulnerability class with realistic
filters, intentional bugs, and automated solve detection that flips a
per-chapter cookie when the student exploits the bug.

> **This application contains intentional vulnerabilities.** Every bug in
> `vulns/` is a teaching feature. Do not run anywhere reachable from the
> internet, do not seed real data into it, and do not derive secure code from
> it.

## Run

```
go run .
```
Server binds `:9998`. Visit `http://localhost:9998/` for the chapter index.

## Architecture

```
main.go                  # entry point: opens SQLite, seeds DB + labdata files, runs server
routes/
  routes.go              # owns ALL URL wiring (no vuln package registers its own routes)
  static/style.css       # shared Apple-styled CSS, served at /static/style.css
  templates/index.html   # chapter index page
vulns/
  <slug>/
    README.md            # this directory's chapter doc
    <slug>.go            # handlers + (intentional) bug
    templates/page.html  # chapter UI
```

State lives in:
- `practica.db` — SQLite (auto-created, idempotent seeds, safe to `rm`)
- `labdata/` — files on disk used by traversal/cmdi/sstirce chapters
  (overwritten on every startup, safe to delete)

Per-chapter solve cookies (`solved_<slug>=1`) light up "Solved" pills on the
index. They're unsigned and per-browser; honor system.

## Chapters

| #  | Title | Slug | Class |
|----|-------|------|-------|
| 01 | [Open Redirect](vulns/openredirect/README.md) | `openredirect` | Unvalidated redirect |
| 02 | [Bruteforce Login](vulns/authbrute/README.md) | `authbrute` | Auth — no rate limit, weak password |
| 03 | [MFA Bypass](vulns/authmfa/README.md) | `authmfa` | MFA — no rate limit on OTP step |
| 04 | [JWT — Weak Secret](vulns/authjwt/README.md) | `authjwt` | JWT signed with low-entropy HMAC key |
| 05 | [SQLi — UNION](vulns/sqliunion/README.md) | `sqliunion` | SQL injection (UNION-based) |
| 06 | [SQLi — Error-based](vulns/sqlierror/README.md) | `sqlierror` | SQL injection (error message exfil) |
| 07 | [SQLi — Time-based](vulns/sqlitime/README.md) | `sqlitime` | Blind SQL injection (timing oracle) |
| 08 | [Reflected XSS](vulns/xssrefl/README.md) | `xssrefl` | XSS — body-context reflection |
| 09 | [Stored XSS](vulns/xssstored/README.md) | `xssstored` | XSS past a regex denylist |
| 10 | DOM XSS *(locked, planned)* | `xssdom` | DOM-based XSS via `location.hash` |
| 11 | [IDOR — ID Swap](vulns/idor/README.md) | `idor` | Direct object reference, no owner check |
| 12 | [Broken Access Control](vulns/bac/README.md) | `bac` | Privilege escalation — missing rank check |
| 13 | [Path Traversal](vulns/traversal/README.md) | `traversal` | Path traversal past a non-iterative filter |
| 14 | [Command Injection](vulns/cmdi/README.md) | `cmdi` | Shell injection past a metacharacter denylist |
| 15 | [SSRF](vulns/ssrf/README.md) | `ssrf` | Server-side request forgery — internal endpoint discovery |
| 16 | [SSTI — Data Leak](vulns/ssti/README.md) | `ssti` | Go `text/template` injection (data exfil) |
| 17 | [SSTI — RCE](vulns/sstirce/README.md) | `sstirce` | SSTI escalating to RCE via dangerous data method |
| 18 | [IDOR — Method Tampering](vulns/midor/README.md) | `midor` | Per-method auth gap (PUT lacks the check GET has) |
| 19 | [XSS — Webhook Exfil](vulns/xsswebhook/README.md) | `xsswebhook` | Filter bypass via dangling markup, no-JS exfil |

## Conventions

- **Chapter pages are CTF-style briefs.** They describe the feature being
  attacked and state a goal — they don't explain the vulnerability class or
  point at the bug. Walk-throughs and bypass payloads live in each chapter's
  `README.md`.
- **`vulns/<slug>/` packages export handler funcs only.** All URL wiring
  happens in [`routes/routes.go`](routes/routes.go). No vuln package calls
  `mux.HandleFunc` itself.
- **In-scope vulnerability per chapter.** Out-of-scope bugs (an XSS in
  chapter 7's error page, say) should be treated as noise and fixed —
  nothing distracts from the chapter's lesson.
- **Solve cookies flow on real exploitation only.** Each chapter detects its
  own win condition server-side; flipping the cookie manually in DevTools
  works (honor system) but isn't the intended path.

## Reset

- All chapter solve cookies are per-browser; clear cookies for `localhost:9998`
  to start fresh.
- `rm practica.db` rebuilds the SQLite seed (users + documents + comments +
  articles + products) and discards student-posted data.
- `rm -rf labdata/` removes the on-disk lab files; `main.go` re-creates them
  on startup.

## Dependencies

- `modernc.org/sqlite` — pure-Go SQLite driver (no CGO)
- `github.com/golang-jwt/jwt/v5` — JWT for chapter 04

Build with stock Go (≥ 1.22 for the new `mux` route patterns).
