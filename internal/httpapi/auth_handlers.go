package httpapi

import (
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/ysksm/task-app/internal/service"
)

// Web UI sessions (01 §4.12). A browser on this machine may take an owner
// session without a password; other devices log in with a human's password.
// Sessions are kept in the database (service/sessions.go), so a restart keeps
// people logged in; "remember me" makes the cookie outlive the browser.

// isLocal tells whether r comes from this machine: the connection's peer is
// a loopback address and the Host header names a loopback host. Checking
// Host as well keeps a DNS-rebinding page (Host: attacker.example) or a
// reverse proxy on this machine (Host: pc.tailnet.ts.net) from passing.
func isLocal(r *http.Request) bool {
	return isLoopbackHost(r.RemoteAddr) && isLoopbackHost(r.Host)
}

func isLoopbackHost(hostport string) bool {
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (s *Server) localLoginAllowed(r *http.Request) bool {
	return s.opts.LocalLogin && isLocal(r)
}

// localLogin issues an owner session cookie to a browser on this machine.
func (s *Server) localLogin(w http.ResponseWriter, r *http.Request) {
	if !s.localLoginAllowed(r) {
		writeError(w, service.Errorf(service.CodeForbidden, "automatic login is only available on this machine; log in with a password"))
		return
	}
	owner, err := s.svc.Owner(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.startSession(w, r, owner.ID, false); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, owner)
}

// authOptions tells the login page which ways of logging in are available.
func (s *Server) authOptions(w http.ResponseWriter, r *http.Request) {
	n, err := s.svc.HumansWithPassword(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"local_login": s.localLoginAllowed(r), "password_login": n > 0})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Remember bool   `json:"remember"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	ip := clientIP(r)
	if wait := s.loginBlocked(ip); wait > 0 {
		writeError(w, service.Errorf(service.CodeRateLimited, "too many failed logins; try again in %d seconds", int(wait.Seconds()+0.999)))
		return
	}
	u, err := s.svc.Login(r.Context(), body.Username, body.Password)
	if err != nil {
		s.loginFailed(ip)
		writeError(w, err)
		return
	}
	s.loginSucceeded(ip)
	if err := s.startSession(w, r, u.ID, body.Remember); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, u)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		if err := s.svc.DeleteSession(r.Context(), c.Value); err != nil {
			writeError(w, err)
			return
		}
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: isHTTPS(r)})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) setPassword(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Password        string `json:"password"`
		CurrentPassword string `json:"current_password"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	u, err := s.svc.SetPassword(r.Context(), service.SetPasswordInput{Username: chi.URLParam(r, "username"), Password: body.Password, Current: body.CurrentPassword})
	if err != nil {
		writeError(w, err)
		return
	}
	// Log the user out everywhere else.
	keep := ""
	if c, err := r.Cookie(sessionCookie); err == nil {
		keep = c.Value
	}
	if err := s.svc.DeleteOtherSessions(r.Context(), u.ID, keep); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, u)
}

// startSession sets the session cookie. With remember the cookie persists
// (Max-Age) for as long as the session; otherwise it ends with the browser.
func (s *Server) startSession(w http.ResponseWriter, r *http.Request, userID string, remember bool) error {
	sid, err := s.svc.CreateSession(r.Context(), userID, remember)
	if err != nil {
		return err
	}
	setSessionCookie(w, r, sid, remember)
	return nil
}

func setSessionCookie(w http.ResponseWriter, r *http.Request, sid string, remember bool) {
	c := &http.Cookie{Name: sessionCookie, Value: sid, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: isHTTPS(r)}
	if remember {
		c.MaxAge = int(service.RememberSessionTTL.Seconds())
	}
	http.SetCookie(w, c)
}

// isHTTPS: the request came over TLS, directly or through a proxy on this
// machine (tailscale serve sets X-Forwarded-Proto). Over plain http on a LAN
// the cookie must not be Secure, or the browser would drop it.
func isHTTPS(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

// ---- slowing down password guessing ----

const (
	loginMaxFailures = 5
	loginLockout     = time.Minute
)

type loginFailures struct {
	count int
	until time.Time
}

func clientIP(r *http.Request) string {
	if h, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return h
	}
	return r.RemoteAddr
}

func (s *Server) loginBlocked(ip string) time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	if f := s.failures[ip]; f != nil {
		return time.Until(f.until)
	}
	return 0
}

// loginFailed locks ip out for a minute after every fifth failure in a row.
func (s *Server) loginFailed(ip string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f := s.failures[ip]
	if f == nil {
		f = &loginFailures{}
		s.failures[ip] = f
	}
	f.count++
	if f.count%loginMaxFailures == 0 {
		f.until = time.Now().Add(loginLockout)
	}
}

func (s *Server) loginSucceeded(ip string) {
	s.mu.Lock()
	delete(s.failures, ip)
	s.mu.Unlock()
}
