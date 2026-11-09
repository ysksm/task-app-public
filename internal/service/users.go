package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base32"
	"encoding/hex"
	"errors"
	"regexp"
	"strings"
)

// User is a human or an AI agent.
type User struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"` // human | agent
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	AgentModel  string `json:"agent_model"`
	AvatarColor string `json:"avatar_color"`
	IsOwner     bool   `json:"is_owner"`
	Active      bool   `json:"active"`
	// HasPassword tells whether the user can log in with a password (the hash
	// itself never leaves the service).
	HasPassword bool `json:"has_password"`
}

// UserRef is the compact form embedded in other objects.
type UserRef struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	AvatarColor string `json:"avatar_color"`
}

func (u User) Ref() *UserRef {
	return &UserRef{ID: u.ID, Kind: u.Kind, Username: u.Username, DisplayName: u.DisplayName, AvatarColor: u.AvatarColor}
}

const userColumns = `id, kind, username, display_name, email, agent_model, avatar_color, is_owner, active`

// userSelect is userColumns plus has_password, for reading users.
const userSelect = userColumns + `, password_hash <> ''`

func scanUser(row interface{ Scan(...any) error }) (User, error) {
	return scanUserWith(row)
}

// scanUserWith scans userSelect followed by extra columns.
func scanUserWith(row interface{ Scan(...any) error }, extra ...any) (User, error) {
	var u User
	dest := append([]any{&u.ID, &u.Kind, &u.Username, &u.DisplayName, &u.Email, &u.AgentModel, &u.AvatarColor, &u.IsOwner, &u.Active, &u.HasPassword}, extra...)
	err := row.Scan(dest...)
	return u, err
}

var usernameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

// CreateUserInput describes a new user.
type CreateUserInput struct {
	Kind        string
	Username    string
	DisplayName string
	Email       string
	AgentModel  string
	AvatarColor string
	IsOwner     bool
}

var avatarColors = []string{"#5B58D6", "#2F7D8C", "#B85A1E", "#2B7A4B", "#A23B72", "#3B6FB6"}

// CreateUser adds a user. Used by seed and the CLI; there is no public sign-up.
func (s *Service) CreateUser(ctx context.Context, in CreateUserInput) (User, error) {
	if in.Kind != "human" && in.Kind != "agent" {
		return User{}, Errorf(CodeInvalidArgument, "kind must be human or agent")
	}
	if !usernameRe.MatchString(in.Username) {
		return User{}, Errorf(CodeInvalidArgument, "invalid username %q", in.Username)
	}
	if in.DisplayName == "" {
		in.DisplayName = in.Username
	}
	if in.Email == "" {
		// Jira clients sign in with email + API token.
		in.Email = in.Username + "@taskapp.local"
	}
	if in.AvatarColor == "" {
		sum := sha256.Sum256([]byte(in.Username))
		in.AvatarColor = avatarColors[int(sum[0])%len(avatarColors)]
	}
	u := User{ID: newID(), Kind: in.Kind, Username: in.Username, DisplayName: in.DisplayName, Email: in.Email,
		AgentModel: in.AgentModel, AvatarColor: in.AvatarColor, IsOwner: in.IsOwner, Active: true}
	_, err := s.db.ExecContext(ctx, `INSERT INTO users (`+userColumns+`, created_at) VALUES (?,?,?,?,?,?,?,?,?,?)`,
		u.ID, u.Kind, u.Username, u.DisplayName, u.Email, u.AgentModel, u.AvatarColor, u.IsOwner, u.Active, s.timestamp())
	if err != nil {
		if isUniqueViolation(err) {
			return User{}, Errorf(CodeConflict, "username %q already exists", in.Username)
		}
		return User{}, err
	}
	return u, nil
}

// EnsureOwner returns the owner, creating one named username if none exists.
func (s *Service) EnsureOwner(ctx context.Context, username string) (User, error) {
	u, err := scanUser(s.db.QueryRowContext(ctx, `SELECT `+userSelect+` FROM users WHERE is_owner = 1 ORDER BY created_at LIMIT 1`))
	if err == nil {
		return u, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return User{}, err
	}
	return s.CreateUser(ctx, CreateUserInput{Kind: "human", Username: username, IsOwner: true})
}

// Owner returns the workspace owner.
func (s *Service) Owner(ctx context.Context) (User, error) {
	u, err := scanUser(s.db.QueryRowContext(ctx, `SELECT `+userSelect+` FROM users WHERE is_owner = 1 ORDER BY created_at LIMIT 1`))
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, Errorf(CodeNotFound, "no owner user")
	}
	return u, err
}

// ListUsers returns all active users, humans first.
func (s *Service) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+userSelect+` FROM users WHERE active = 1 ORDER BY kind DESC, username`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []User{}
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// UserByID looks an active user up by id.
func (s *Service) UserByID(ctx context.Context, id string) (User, error) {
	u, err := scanUser(s.db.QueryRowContext(ctx, `SELECT `+userSelect+` FROM users WHERE id = ? AND active = 1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, Errorf(CodeNotFound, "user not found")
	}
	return u, err
}

// UserByUsername looks a user up.
func (s *Service) UserByUsername(ctx context.Context, username string) (User, error) {
	return s.userByUsername(ctx, s.db, username)
}

func (s *Service) userByUsername(ctx context.Context, q queryer, username string) (User, error) {
	u, err := scanUser(q.QueryRowContext(ctx, `SELECT `+userSelect+` FROM users WHERE username = ?`, username))
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, Errorf(CodeNotFound, "user %q not found", username)
	}
	return u, err
}

// resolveUsername turns "me" into the actor and other names into user ids.
func (s *Service) resolveUserID(ctx context.Context, q queryer, actor Actor, username string) (string, error) {
	if username == "me" {
		return actor.User.ID, nil
	}
	u, err := s.userByUsername(ctx, q, username)
	if err != nil {
		return "", err
	}
	return u.ID, nil
}

// ---- API tokens ----

// Token is token metadata (never the secret).
type Token struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Prefix     string  `json:"prefix"`
	User       UserRef `json:"user"`
	LastUsedAt *string `json:"last_used_at"`
	CreatedAt  string  `json:"created_at"`
}

const tokenPrefix = "tk_"

// CreateToken issues a token for the user. If raw is empty a random one is
// generated. Authentication uses only its hash; with a token key set an
// encrypted copy is kept too so RevealToken can show it again.
func (s *Service) CreateToken(ctx context.Context, userID, name, raw string) (string, Token, error) {
	if raw == "" {
		b := make([]byte, 20)
		if _, err := rand.Read(b); err != nil {
			return "", Token{}, err
		}
		raw = tokenPrefix + strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b))
	}
	if name == "" {
		return "", Token{}, Errorf(CodeInvalidArgument, "token name is required")
	}
	prefix := raw
	if len(prefix) > 7 {
		prefix = prefix[:7]
	}
	enc, err := s.sealToken(raw)
	if err != nil {
		return "", Token{}, err
	}
	t := Token{ID: newID(), Name: name, Prefix: prefix, CreatedAt: s.timestamp()}
	_, err = s.db.ExecContext(ctx, `INSERT INTO api_tokens (id, user_id, name, token_hash, token_prefix, token_enc, created_at) VALUES (?,?,?,?,?,?,?)`,
		t.ID, userID, name, hashToken(raw), prefix, enc, t.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return "", Token{}, Errorf(CodeConflict, "token already exists")
		}
		return "", Token{}, err
	}
	return raw, t, nil
}

// Authenticate resolves a raw token to its user.
func (s *Service) Authenticate(ctx context.Context, raw string) (User, error) {
	var tokenID string
	row := s.db.QueryRowContext(ctx, `SELECT `+prefixed("u", userSelect)+`, t.id
		FROM api_tokens t JOIN users u ON u.id = t.user_id
		WHERE t.token_hash = ? AND t.revoked_at IS NULL AND u.active = 1
		  AND (t.expires_at IS NULL OR t.expires_at > ?)`, hashToken(raw), s.timestamp())
	u, err := scanUserWith(row, &tokenID)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, Errorf(CodeUnauthorized, "invalid token")
	}
	if err != nil {
		return User{}, err
	}
	_, _ = s.db.ExecContext(ctx, `UPDATE api_tokens SET last_used_at = ? WHERE id = ?`, s.timestamp(), tokenID)
	return u, nil
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func prefixed(alias, cols string) string {
	parts := strings.Split(cols, ",")
	for i, p := range parts {
		parts[i] = alias + "." + strings.TrimSpace(p)
	}
	return strings.Join(parts, ", ")
}

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}

// ListTokens returns token metadata. The owner sees every token; others only their own.
func (s *Service) ListTokens(ctx context.Context) ([]Token, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return nil, err
	}
	q := `SELECT t.id, t.name, t.token_prefix, t.last_used_at, t.created_at, u.id, u.kind, u.username, u.display_name, u.avatar_color
		FROM api_tokens t JOIN users u ON u.id = t.user_id WHERE t.revoked_at IS NULL`
	args := []any{}
	if !actor.User.IsOwner {
		q, args = q+` AND t.user_id = ?`, append(args, actor.User.ID)
	}
	rows, err := s.db.QueryContext(ctx, q+` ORDER BY t.created_at DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Token{}
	for rows.Next() {
		var t Token
		var last sql.NullString
		if err := rows.Scan(&t.ID, &t.Name, &t.Prefix, &last, &t.CreatedAt, &t.User.ID, &t.User.Kind, &t.User.Username, &t.User.DisplayName, &t.User.AvatarColor); err != nil {
			return nil, err
		}
		t.LastUsedAt = strPtr(last)
		out = append(out, t)
	}
	return out, rows.Err()
}

// IssueToken creates a token for username (the owner may issue for anyone,
// others only for themselves) and returns the raw secret once.
func (s *Service) IssueToken(ctx context.Context, username, name string) (string, Token, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return "", Token{}, err
	}
	if username == "" || username == "me" {
		username = actor.User.Username
	}
	u, err := s.UserByUsername(ctx, username)
	if err != nil {
		return "", Token{}, err
	}
	if !actor.User.IsOwner && u.ID != actor.User.ID {
		return "", Token{}, Errorf(CodeForbidden, "only the owner can issue tokens for other users")
	}
	raw, t, err := s.CreateToken(ctx, u.ID, name, "")
	if err != nil {
		return "", Token{}, err
	}
	t.User = *u.Ref()
	return raw, t, nil
}

// RevokeToken disables a token.
func (s *Service) RevokeToken(ctx context.Context, id string) error {
	actor, err := requireActor(ctx)
	if err != nil {
		return err
	}
	q, args := `UPDATE api_tokens SET revoked_at = ? WHERE id = ? AND revoked_at IS NULL`, []any{s.timestamp(), id}
	if !actor.User.IsOwner {
		q, args = q+` AND user_id = ?`, append(args, actor.User.ID)
	}
	res, err := s.db.ExecContext(ctx, q, args...)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Errorf(CodeNotFound, "token not found")
	}
	return nil
}

// AddUser lets the owner register a person or an agent.
func (s *Service) AddUser(ctx context.Context, in CreateUserInput) (User, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return User{}, err
	}
	if !actor.User.IsOwner {
		return User{}, Errorf(CodeForbidden, "only the owner can add users")
	}
	in.IsOwner = false
	return s.CreateUser(ctx, in)
}
