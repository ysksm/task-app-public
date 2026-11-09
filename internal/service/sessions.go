package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"
)

// Web UI sessions (01 §4.12). They live in the database, so a restart keeps
// people logged in. "Remember me" sessions last 30 days after the last use and
// get a persistent cookie; the others last a day after the last use and get a
// browser-session cookie (gone when the browser closes).
const (
	SessionTTL         = 24 * time.Hour
	RememberSessionTTL = 30 * 24 * time.Hour
	// Sliding the expiry writes to the database; do it at most this often.
	sessionTouchEvery = 10 * time.Minute
)

func sessionTTL(remember bool) time.Duration {
	if remember {
		return RememberSessionTTL
	}
	return SessionTTL
}

// CreateSession starts a session for userID and returns the cookie value.
func (s *Service) CreateSession(ctx context.Context, userID string, remember bool) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	sid := hex.EncodeToString(b)
	now := s.now()
	// Drop expired sessions now and then (here is as good as anywhere).
	if _, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= ?`, now.Format(timeFormat)); err != nil {
		return "", err
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO sessions (id_hash, user_id, remember, created_at, last_seen_at, expires_at) VALUES (?,?,?,?,?,?)`,
		hashToken(sid), userID, remember, now.Format(timeFormat), now.Format(timeFormat), now.Add(sessionTTL(remember)).Format(timeFormat))
	return sid, err
}

// SessionUser resolves a session cookie to its (active) user and slides the
// expiry. renew is set when the expiry moved on a "remember me" session: the
// caller re-sends the cookie so that its Max-Age moves along too.
func (s *Service) SessionUser(ctx context.Context, sid string) (u User, renew bool, err error) {
	var userID, lastSeen string
	var remember bool
	now := s.now()
	err = s.db.QueryRowContext(ctx, `SELECT user_id, remember, last_seen_at FROM sessions WHERE id_hash = ? AND expires_at > ?`,
		hashToken(sid), now.Format(timeFormat)).Scan(&userID, &remember, &lastSeen)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, false, Errorf(CodeUnauthorized, "session expired")
	}
	if err != nil {
		return User{}, false, err
	}
	u, err = s.UserByID(ctx, userID)
	if err != nil {
		return User{}, false, Errorf(CodeUnauthorized, "session expired")
	}
	if t, err := time.Parse(timeFormat, lastSeen); err != nil || now.Sub(t) >= sessionTouchEvery {
		_, err := s.db.ExecContext(ctx, `UPDATE sessions SET last_seen_at = ?, expires_at = ? WHERE id_hash = ?`,
			now.Format(timeFormat), now.Add(sessionTTL(remember)).Format(timeFormat), hashToken(sid))
		renew = err == nil && remember
	}
	return u, renew, nil
}

// DeleteSession ends one session (logout).
func (s *Service) DeleteSession(ctx context.Context, sid string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE id_hash = ?`, hashToken(sid))
	return err
}

// DeleteOtherSessions ends every session of userID except keep (after a password change).
func (s *Service) DeleteOtherSessions(ctx context.Context, userID, keep string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ? AND id_hash <> ?`, userID, hashToken(keep))
	return err
}
