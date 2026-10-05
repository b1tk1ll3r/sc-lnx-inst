package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func newTestAuth(t *testing.T) (*guiAuth, http.Handler) {
	t.Helper()
	g, err := newGUIAuth("127.0.0.1:4242")
	if err != nil {
		t.Fatal(err)
	}
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ok")) })
	return g, g.wrap(ok)
}

func doReq(h http.Handler, method, target string, mod func(*http.Request)) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, target, nil)
	r.Host = "127.0.0.1:4242"
	if mod != nil {
		mod(r)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestGUIAuthOneTimeLoginSetsSessionCookie(t *testing.T) {
	g, h := newTestAuth(t)
	if w := doReq(h, "GET", "/api/status", nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated API call allowed: %d", w.Code)
	}
	login, err := g.loginURL()
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(login)
	w := doReq(h, "GET", u.RequestURI(), nil)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("login not redirected: %d", w.Code)
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode || cookies[0].Name != "cl_session_4242" {
		t.Fatalf("unexpected session cookie: %#v", cookies)
	}
	if strings.Contains(login, cookies[0].Value) {
		t.Fatal("session secret leaked into login URL")
	}
	withCookie := func(r *http.Request) { r.AddCookie(cookies[0]) }
	if w := doReq(h, "GET", "/api/status", withCookie); w.Code != http.StatusOK {
		t.Fatalf("authenticated call rejected: %d", w.Code)
	}
	// Single use: replaying the login URL without the cookie must fail.
	if w := doReq(h, "GET", u.RequestURI(), nil); w.Code != http.StatusForbidden {
		t.Fatalf("login token replay accepted: %d", w.Code)
	}
}

func TestGUIAuthRejectsForeignHostAndOrigin(t *testing.T) {
	g, h := newTestAuth(t)
	cookie := &http.Cookie{Name: g.cookieName, Value: g.session}
	if w := doReq(h, "GET", "/", func(r *http.Request) { r.Host = "evil.example:4242"; r.AddCookie(cookie) }); w.Code != http.StatusForbidden {
		t.Fatalf("foreign Host accepted: %d", w.Code)
	}
	if w := doReq(h, "POST", "/api/action/setup", func(r *http.Request) {
		r.Header.Set("Origin", "http://evil.example")
		r.AddCookie(cookie)
	}); w.Code != http.StatusForbidden {
		t.Fatalf("cross-origin POST accepted: %d", w.Code)
	}
	if w := doReq(h, "POST", "/api/action/setup", func(r *http.Request) {
		r.Header.Set("Origin", "http://127.0.0.1:4242")
		r.AddCookie(cookie)
	}); w.Code != http.StatusOK {
		t.Fatalf("same-origin POST rejected: %d", w.Code)
	}
}

func TestGUIControlEndpointRequiresSecretAndNoBrowser(t *testing.T) {
	g, h := newTestAuth(t)
	if w := doReq(h, "POST", guiControlPath, nil); w.Code != http.StatusForbidden {
		t.Fatalf("control without secret accepted: %d", w.Code)
	}
	if w := doReq(h, "POST", guiControlPath, func(r *http.Request) {
		r.Header.Set(guiControlHeader, g.control)
		r.Header.Set("Origin", "http://127.0.0.1:4242")
	}); w.Code != http.StatusForbidden {
		t.Fatalf("browser-originated control call accepted: %d", w.Code)
	}
	w := doReq(h, "POST", guiControlPath, func(r *http.Request) { r.Header.Set(guiControlHeader, g.control) })
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "http://127.0.0.1:4242/login?t=") {
		t.Fatalf("control call failed: %d %s", w.Code, w.Body.String())
	}
}
