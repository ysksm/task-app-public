// Package httpapi serves the internal REST API (/api/v1), server-sent events
// and the embedded web UI.
package httpapi

import (
	"encoding/json"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/ysksm/task-app/internal/backup"
	"github.com/ysksm/task-app/internal/domain"
	"github.com/ysksm/task-app/internal/events"
	"github.com/ysksm/task-app/internal/jira"
	"github.com/ysksm/task-app/internal/mcpserver"
	"github.com/ysksm/task-app/internal/service"
)

// Options configures the server.
type Options struct {
	// LocalLogin lets a browser on this machine (loopback address and Host)
	// obtain an owner session without a password. Disable with --require-login.
	LocalLogin bool
	// Static serves the built web UI. nil disables it.
	Static fs.FS
	Logger *slog.Logger
	// Info is returned by GET /api/v1/server (binary path and data directory,
	// used by the settings page to show ready-to-copy MCP commands).
	Info map[string]string
	// LogRequests logs every API request (including the Jira-compatible
	// API) at info level with its query string and User-Agent.
	LogRequests bool
	// Backups serves /api/v1/backups (status and "back up now"). nil disables it.
	Backups *backup.Manager
	// DataDir is measured by /api/v1/capacity (database, backups, disk).
	DataDir string
	// BehindProxy accepts MCP requests that a reverse proxy on this machine
	// forwards with the public Host name (serve --behind-proxy).
	BehindProxy bool
}

// Server holds handler dependencies.
type Server struct {
	svc  *service.Service
	opts Options

	mu       sync.Mutex
	failures map[string]*loginFailures
}

const sessionCookie = "taskapp_session"

// New builds the HTTP handler.
func New(svc *service.Service, opts Options) http.Handler {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	s := &Server{svc: svc, opts: opts, failures: map[string]*loginFailures{}}
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(s.logRequests)

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		})
		r.Post("/auth/local", s.localLogin)
		r.Get("/auth/options", s.authOptions)
		r.Post("/auth/login", s.login)
		r.Post("/auth/logout", s.logout)
		r.Group(func(r chi.Router) {
			r.Use(s.authenticate)
			r.Get("/me", s.getMe)
			r.Get("/server", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, http.StatusOK, s.opts.Info) })
			r.Get("/users", s.listUsers)

			r.Get("/projects", s.listProjects)
			r.Post("/projects", s.createProject)
			r.Get("/projects/{key}", s.getProject)
			r.Patch("/projects/{key}", s.updateProject)
			r.Get("/projects/{key}/states", s.listStates)
			r.Get("/projects/{key}/labels", s.listLabels)
			r.Post("/projects/{key}/labels", s.createLabel)
			r.Patch("/projects/{key}/labels/{id}", s.updateLabel)
			r.Delete("/projects/{key}/labels/{id}", s.deleteLabel)
			r.Post("/projects/{key}/states", s.createState)
			r.Patch("/projects/{key}/states/{id}", s.updateState)
			r.Delete("/projects/{key}/states/{id}", s.deleteState)

			r.Get("/issues", s.listIssues)
			r.Post("/issues", s.createIssue)
			r.Get("/issues/{key}", s.getIssue)
			r.Patch("/issues/{key}", s.updateIssue)
			r.Delete("/issues/{key}", s.deleteIssue)
			r.Get("/issues/{key}/comments", s.listComments)
			r.Post("/issues/{key}/comments", s.addComment)
			r.Get("/issues/{key}/activity", s.listActivity)
			r.Get("/issues/{key}/checkpoints", s.listCheckpoints)
			r.Post("/issues/{key}/checkpoints", s.recordCheckpoint)
			r.Get("/issues/{key}/acceptance", s.getAcceptance)
			r.Put("/issues/{key}/acceptance/criteria", s.setCriteria)
			r.Post("/issues/{key}/acceptance/reviews", s.reviewAcceptance)
			r.Get("/acceptance", s.acceptanceQueue)

			s.reqRoutes(r)

			r.Get("/views", s.listViews)
			r.Post("/views", s.createView)
			r.Get("/views/{id}", s.getView)
			r.Patch("/views/{id}", s.updateView)
			r.Delete("/views/{id}", s.deleteView)

			r.Get("/qa", s.listQA)
			r.Post("/qa", s.createQA)
			r.Get("/qa/{key}", s.getQA)
			r.Post("/qa/{key}/answers", s.answerQA)
			r.Post("/qa/{key}/close", s.closeQA)

			r.Get("/agents/activity", s.agentActivity)

			r.Post("/users", s.addUser)
			r.Put("/users/{username}/password", s.setPassword)
			r.Get("/tokens", s.listTokens)
			r.Post("/tokens", s.issueToken)
			r.Delete("/tokens/{id}", s.revokeToken)
			r.Post("/tokens/{id}/reveal", s.revealToken)

			r.Get("/export", s.exportData)
			r.Get("/capacity", s.capacity)
			r.Get("/backups", s.backupStatus)
			r.Post("/backups", s.createBackup)
			r.Get("/events", s.streamEvents)
			r.Post("/internal/notify", s.relayEvent)
		})
		r.NotFound(func(w http.ResponseWriter, r *http.Request) {
			writeError(w, service.Errorf(service.CodeNotFound, "no such endpoint: %s %s", r.Method, r.URL.Path))
		})
	})

	// MCP over Streamable HTTP, authenticated with the same API tokens.
	r.Handle("/mcp", mcpserver.HTTPHandler(svc, opts.BehindProxy))

	// Jira REST API v3 compatible layer (ORCA and other Jira clients).
	r.Mount("/rest", jira.Handler(svc))
	r.Get("/browse/{key}", jira.Browse(svc))

	if opts.Static != nil {
		r.Handle("/*", spaHandler(opts.Static))
	}
	return r
}

// ---- auth ----

func (s *Server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
			u, err := s.svc.Authenticate(ctx, strings.TrimSpace(strings.TrimPrefix(h, "Bearer ")))
			if err != nil {
				writeError(w, err)
				return
			}
			next.ServeHTTP(w, r.WithContext(service.WithActor(ctx, service.Actor{User: u, Source: domain.SourceAPI})))
			return
		}
		if c, err := r.Cookie(sessionCookie); err == nil {
			if u, renew, err := s.svc.SessionUser(ctx, c.Value); err == nil {
				if renew {
					setSessionCookie(w, r, c.Value, true)
				}
				next.ServeHTTP(w, r.WithContext(service.WithActor(ctx, service.Actor{User: u, Source: domain.SourceUI})))
				return
			}
		}
		writeError(w, service.Errorf(service.CodeUnauthorized, "authentication required"))
	})
}

// relayEvent republishes a change made by another process on the same
// database (taskapp mcp over stdio) so that connected browsers refresh.
func (s *Server) relayEvent(w http.ResponseWriter, r *http.Request) {
	var e events.Event
	if err := decodeJSON(r, &e); err != nil {
		writeError(w, err)
		return
	}
	s.svc.Bus().Publish(e)
	w.WriteHeader(http.StatusNoContent)
}

// ---- helpers ----

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		api := strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/rest/") || r.URL.Path == "/mcp"
		switch {
		case api && s.opts.LogRequests:
			s.opts.Logger.Info("request", "method", r.Method, "path", r.URL.RequestURI(), "status", ww.Status(),
				"dur", time.Since(start), "ua", r.UserAgent())
		case api:
			s.opts.Logger.Debug("request", "method", r.Method, "path", r.URL.Path, "status", ww.Status(), "dur", time.Since(start))
		}
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

type errorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func writeError(w http.ResponseWriter, err error) {
	e := service.AsError(err)
	var body errorBody
	body.Error.Code, body.Error.Message = e.Code, e.Message
	writeJSON(w, e.HTTPStatus(), body)
}

func decodeJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return service.Errorf(service.CodeInvalidArgument, "invalid request body: %s", err.Error())
	}
	return nil
}

// spaHandler serves static files and falls back to index.html for client routes.
func spaHandler(static fs.FS) http.Handler {
	files := http.FileServer(http.FS(static))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path != "" {
			if f, err := static.Open(path); err == nil {
				f.Close()
				if strings.HasPrefix(path, "assets/") {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				files.ServeHTTP(w, r)
				return
			}
		}
		index, err := fs.ReadFile(static, "index.html")
		if err != nil {
			http.Error(w, "web UI is not built (run make web)", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Write(index)
	})
}
