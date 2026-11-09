// Package service implements the use cases. Every entry point (internal API,
// Jira-compatible API, MCP) goes through here so that hierarchy rules, key
// numbering, activity logging and events are enforced in one place.
package service

import (
	"context"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/ysksm/task-app/internal/domain"
	"github.com/ysksm/task-app/internal/events"
)

// Service is the application core.
type Service struct {
	db  *sql.DB
	bus *events.Bus
	now func() time.Time
	// tokenAEAD encrypts the stored copy of API tokens; nil = keep only the hash.
	tokenAEAD cipher.AEAD
}

// New creates a Service. bus may be nil (e.g. for CLI commands).
func New(db *sql.DB, bus *events.Bus) *Service {
	if bus == nil {
		bus = events.NewBus()
	}
	return &Service{db: db, bus: bus, now: func() time.Time { return time.Now().UTC() }}
}

// Bus returns the event bus.
func (s *Service) Bus() *events.Bus { return s.bus }

// DB returns the underlying database (for CLI maintenance commands).
func (s *Service) DB() *sql.DB { return s.db }

// ---- actor ----

// Actor is who performs an operation and through which channel.
type Actor struct {
	User   User
	Source domain.Source
}

type actorKey struct{}

// WithActor stores the actor in the context.
func WithActor(ctx context.Context, a Actor) context.Context {
	return context.WithValue(ctx, actorKey{}, a)
}

// ActorFrom returns the actor stored in ctx.
func ActorFrom(ctx context.Context) (Actor, bool) {
	a, ok := ctx.Value(actorKey{}).(Actor)
	return a, ok
}

func requireActor(ctx context.Context) (Actor, error) {
	a, ok := ActorFrom(ctx)
	if !ok {
		return Actor{}, Errorf(CodeUnauthorized, "authentication required")
	}
	return a, nil
}

// ---- errors ----

// Error codes returned by the API as {"error": {"code", "message"}}.
const (
	CodeInvalidArgument    = "invalid_argument"
	CodeNotFound           = "not_found"
	CodeConflict           = "conflict"
	CodeHierarchyViolation = "hierarchy_violation"
	CodeUnauthorized       = "unauthorized"
	CodeForbidden          = "forbidden"
	CodeRateLimited        = "rate_limited"
	CodeInternal           = "internal"
)

// Error is a failure meant to be shown to the caller.
type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

// HTTPStatus maps the code to an HTTP status.
func (e *Error) HTTPStatus() int {
	switch e.Code {
	case CodeInvalidArgument:
		return http.StatusBadRequest
	case CodeNotFound:
		return http.StatusNotFound
	case CodeConflict:
		return http.StatusConflict
	case CodeHierarchyViolation:
		return http.StatusUnprocessableEntity
	case CodeUnauthorized:
		return http.StatusUnauthorized
	case CodeForbidden:
		return http.StatusForbidden
	case CodeRateLimited:
		return http.StatusTooManyRequests
	case CodeNotAvailable:
		return http.StatusConflict
	}
	return http.StatusInternalServerError
}

// Errorf creates an *Error.
func Errorf(code, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

// AsError converts any error into an *Error (unknown errors become internal).
func AsError(err error) *Error {
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	var he *domain.HierarchyError
	if errors.As(err, &he) {
		return &Error{Code: CodeHierarchyViolation, Message: he.Error()}
	}
	return &Error{Code: CodeInternal, Message: err.Error()}
}

// ---- helpers ----

func newID() string {
	return ulid.MustNew(ulid.Now(), rand.Reader).String()
}

const timeFormat = "2006-01-02T15:04:05.000Z"

func (s *Service) timestamp() string {
	return s.now().Format(timeFormat)
}

// withTx runs fn in a transaction.
func (s *Service) withTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

// queryer is satisfied by *sql.DB and *sql.Tx.
type queryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func nullString(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}

func strPtr(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	v := ns.String
	return &v
}
