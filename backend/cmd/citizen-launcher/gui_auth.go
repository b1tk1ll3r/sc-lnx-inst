package main

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// GUI authentication model
//
// The browser window is opened with a one-time login URL (/login?t=…). The
// token is single-use and expires after guiLoginTTL; redeeming it sets an
// HttpOnly, SameSite=Strict session cookie and redirects to "/". The session
// secret itself never appears in a URL or on a command line, so reading
// another process' /proc/<pid>/cmdline only ever reveals a spent token.
//
// A second `citizen-launcher gui` call obtains a fresh login URL from the
// running instance via guiControlPath, authenticated with the control secret
// from the user's 0600 gui-instance.json.
const (
	guiLoginPath     = "/login"
	guiControlPath   = "/control/login-url"
	guiControlHeader = "X-Citizen-Control"
	guiLoginTTL      = 2 * time.Minute
)

type guiAuth struct {
	addr       string
	session    string
	control    string
	cookieName string
	allowed    map[string]bool

	mu     sync.Mutex
	logins map[string]time.Time
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func newGUIAuth(addr string) (*guiAuth, error) {
	session, err := randomHex(32)
	if err != nil {
		return nil, err
	}
	control, err := randomHex(32)
	if err != nil {
		return nil, err
	}
	g := &guiAuth{addr: addr, session: session, control: control, cookieName: "cl_session",
		allowed: map[string]bool{addr: true}, logins: map[string]time.Time{}}
	if _, port, err := net.SplitHostPort(addr); err == nil {
		g.allowed["localhost:"+port] = true
		// Cookies are scoped per host, not per port: keep instances apart.
		g.cookieName = "cl_session_" + port
	}
	return g, nil
}

// loginURL mints a new single-use login URL.
func (g *guiAuth) loginURL() (string, error) {
	tok, err := randomHex(24)
	if err != nil {
		return "", err
	}
	now := time.Now()
	g.mu.Lock()
	for t, exp := range g.logins {
		if now.After(exp) {
			delete(g.logins, t)
		}
	}
	g.logins[tok] = now.Add(guiLoginTTL)
	g.mu.Unlock()
	return "http://" + g.addr + guiLoginPath + "?t=" + tok, nil
}

func (g *guiAuth) consumeLogin(tok string) bool {
	if tok == "" {
		return false
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	exp, ok := g.logins[tok]
	if !ok {
		return false
	}
	delete(g.logins, tok)
	return time.Now().Before(exp)
}

func (g *guiAuth) authenticated(r *http.Request) bool {
	c, err := r.Cookie(g.cookieName)
	return err == nil && subtle.ConstantTimeCompare([]byte(c.Value), []byte(g.session)) == 1
}

func (g *guiAuth) wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// DNS-rebinding guard: a hostile page resolving its own name to
		// 127.0.0.1 still sends its own Host header.
		if !g.allowed[r.Host] {
			http.Error(w, "forbidden host", http.StatusForbidden)
			return
		}
		origin := r.Header.Get("Origin")
		if r.Method != http.MethodGet && r.Method != http.MethodHead && origin != "" &&
			!g.allowed[strings.TrimPrefix(origin, "http://")] {
			http.Error(w, "forbidden origin", http.StatusForbidden)
			return
		}
		switch r.URL.Path {
		case guiLoginPath:
			if r.Method != http.MethodGet {
				http.Error(w, "GET required", http.StatusMethodNotAllowed)
				return
			}
			if g.consumeLogin(r.URL.Query().Get("t")) {
				http.SetCookie(w, &http.Cookie{Name: g.cookieName, Value: g.session, Path: "/",
					HttpOnly: true, SameSite: http.SameSiteStrictMode})
				http.Redirect(w, r, "/", http.StatusSeeOther)
				return
			}
			if g.authenticated(r) {
				http.Redirect(w, r, "/", http.StatusSeeOther)
				return
			}
			http.Error(w, "Dieser Anmeldelink ist abgelaufen. Bitte Citizen Launcher erneut über das App-Menü öffnen.", http.StatusForbidden)
			return
		case guiControlPath:
			// Only the local CLI (no browser Origin) holding the control secret.
			if r.Method != http.MethodPost || origin != "" ||
				subtle.ConstantTimeCompare([]byte(r.Header.Get(guiControlHeader)), []byte(g.control)) != 1 {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			u, err := g.loginURL()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"url": u})
			return
		}
		if !g.authenticated(r) {
			http.Error(w, "Nicht angemeldet. Bitte Citizen Launcher über das App-Menü öffnen.", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
