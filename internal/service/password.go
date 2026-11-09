package service

import (
	"context"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"
)

// Passwords are stored as
//
//	pbkdf2-sha256$<iterations>$<salt, base64 raw std>$<key, base64 raw std>
//
// (PBKDF2-HMAC-SHA256, 16-byte salt, 32-byte key). The format names its
// parameters so that another implementation can verify existing hashes.
const (
	passwordScheme     = "pbkdf2-sha256"
	passwordIterations = 600_000
	passwordMinLength  = 8
)

var b64 = base64.RawStdEncoding

func hashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	rand.Read(salt)
	key, err := pbkdf2.Key(sha256.New, password, salt, passwordIterations, 32)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s$%d$%s$%s", passwordScheme, passwordIterations, b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

func checkPassword(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != passwordScheme {
		return false
	}
	iter, err := strconv.Atoi(parts[1])
	if err != nil || iter < 1 {
		return false
	}
	salt, err1 := b64.DecodeString(parts[2])
	want, err2 := b64.DecodeString(parts[3])
	if err1 != nil || err2 != nil || len(want) == 0 {
		return false
	}
	got, err := pbkdf2.Key(sha256.New, password, salt, iter, len(want))
	return err == nil && subtle.ConstantTimeCompare(got, want) == 1
}

// dummyHash keeps the time of a failed login the same whether or not the
// user exists. Computed on first use so that starting taskapp stays fast.
var dummyHash = sync.OnceValue(func() string {
	h, _ := hashPassword("taskapp-dummy-password")
	return h
})

// Login checks a human's username and password.
func (s *Service) Login(ctx context.Context, username, password string) (User, error) {
	fail := Errorf(CodeUnauthorized, "invalid username or password")
	var hash string
	u, err := scanUserWith(s.db.QueryRowContext(ctx, `SELECT `+userSelect+`, password_hash FROM users
		WHERE username = ? AND active = 1 AND kind = 'human'`, strings.ToLower(strings.TrimSpace(username))), &hash)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && hash == "") {
		checkPassword(dummyHash(), password)
		return User{}, fail
	}
	if err != nil {
		return User{}, err
	}
	if !checkPassword(hash, password) {
		return User{}, fail
	}
	return u, nil
}

// SetPasswordInput changes a human's password. Current is required when
// users change their own password and already have one; the owner may set
// anyone's password without it.
type SetPasswordInput struct {
	Username string
	Password string
	Current  string
}

// SetPassword sets (or, with an empty Password and the owner as actor,
// clears) a human's password.
func (s *Service) SetPassword(ctx context.Context, in SetPasswordInput) (User, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return User{}, err
	}
	if in.Username == "" || in.Username == "me" {
		in.Username = actor.User.Username
	}
	var hash string
	u, err := scanUserWith(s.db.QueryRowContext(ctx, `SELECT `+userSelect+`, password_hash FROM users
		WHERE username = ? AND active = 1`, in.Username), &hash)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, Errorf(CodeNotFound, "user %q not found", in.Username)
	}
	if err != nil {
		return User{}, err
	}
	self := u.ID == actor.User.ID
	if !self && !actor.User.IsOwner {
		return User{}, Errorf(CodeForbidden, "only the owner can set other users' passwords")
	}
	if u.Kind != "human" {
		return User{}, Errorf(CodeInvalidArgument, "agents use API tokens, not passwords")
	}
	if self && !actor.User.IsOwner && hash != "" && !checkPassword(hash, in.Current) {
		return User{}, Errorf(CodeForbidden, "current password is wrong")
	}
	newHash := ""
	if in.Password != "" || !actor.User.IsOwner {
		if utf8.RuneCountInString(in.Password) < passwordMinLength {
			return User{}, Errorf(CodeInvalidArgument, "password must be at least %d characters", passwordMinLength)
		}
		if newHash, err = hashPassword(in.Password); err != nil {
			return User{}, err
		}
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE users SET password_hash = ? WHERE id = ?`, newHash, u.ID); err != nil {
		return User{}, err
	}
	u.HasPassword = newHash != ""
	return u, nil
}

// SetPasswordDirect sets a password without an actor (CLI `taskapp passwd`,
// run by whoever can read the data directory).
func (s *Service) SetPasswordDirect(ctx context.Context, username, password string) (User, error) {
	u, err := s.UserByUsername(ctx, username)
	if err != nil {
		return User{}, err
	}
	return s.SetPassword(WithActor(ctx, Actor{User: User{ID: u.ID, IsOwner: true}}), SetPasswordInput{Username: username, Password: password})
}

// HumansWithPassword counts the humans who can log in from other devices.
func (s *Service) HumansWithPassword(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE kind = 'human' AND active = 1 AND password_hash <> ''`).Scan(&n)
	return n, err
}
