package authjwt

import (
	"embed"
	"errors"
	"html/template"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const jwtSecret = "secret"
// hashcat -m 16500 -a 0 token.jwt /usr/share/wordlists/rockyou.txt

const (
	validUser = "alice"
	validPass = "alice"
)

//go:embed templates/page.html
var templatesFS embed.FS

var pageTmpl = template.Must(template.ParseFS(templatesFS, "templates/page.html"))

type tokenClaims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

func issueToken(username, role string) (string, error) {
	c := tokenClaims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   username,
			Issuer:    "practicabitapp",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	return t.SignedString([]byte(jwtSecret))
}

func parseToken(tokenStr string) (*tokenClaims, error) {
	c := &tokenClaims{}
	_, err := jwt.ParseWithClaims(tokenStr, c, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(jwtSecret), nil
	})
	if err != nil {
		return nil, err
	}
	return c, nil
}

func currentClaims(r *http.Request) *tokenClaims {
	c, err := r.Cookie("authjwt_token")
	if err != nil {
		return nil
	}
	cl, err := parseToken(c.Value)
	if err != nil {
		return nil
	}
	return cl
}

type pageData struct {
	Solved   bool
	State    string
	Username string
	Role     string
	Token    string
	Error    string
}

func isSolved(r *http.Request) bool {
	c, err := r.Cookie("solved_authjwt")
	return err == nil && c.Value == "1"
}

func render(w http.ResponseWriter, r *http.Request, errMsg string) {
	data := pageData{Solved: isSolved(r), State: "loggedout", Error: errMsg}
	if c := currentClaims(r); c != nil {
		data.State = "loggedin"
		data.Username = c.Subject
		data.Role = c.Role
		if cookie, err := r.Cookie("authjwt_token"); err == nil {
			data.Token = cookie.Value
		}
	}
	if err := pageTmpl.Execute(w, data); err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
	}
}

func Index(w http.ResponseWriter, r *http.Request) {
	render(w, r, "")
}

func Login(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	u := r.PostFormValue("username")
	p := r.PostFormValue("password")
	if u != validUser || p != validPass {
		w.WriteHeader(http.StatusUnauthorized)
		render(w, r, "Invalid credentials.")
		return
	}
	token, err := issueToken(u, "user")
	if err != nil {
		http.Error(w, "sign error", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "authjwt_token",
		Value:    token,
		Path:     "/authjwt/",
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, "/authjwt/", http.StatusFound)
}

func Admin(w http.ResponseWriter, r *http.Request) {
	c := currentClaims(r)
	if c == nil {
		http.Error(w, "Authentication required.", http.StatusUnauthorized)
		return
	}
	if c.Role != "admin" {
		http.Error(w, "Admins only.", http.StatusForbidden)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "solved_authjwt",
		Value:    "1",
		Path:     "/",
		MaxAge:   60 * 60 * 24 * 365,
		SameSite: http.SameSiteLaxMode,
	})
	data := pageData{
		Solved:   true,
		State:    "admin",
		Username: c.Subject,
		Role:     c.Role,
	}
	if err := pageTmpl.Execute(w, data); err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
	}
}
