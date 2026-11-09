package service

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// TokenKeyFile holds the key that encrypts the stored copy of API tokens. It
// lives next to the database but is not in the database, its backups or the
// JSON export: copy it along to keep "show token" working after a restore.
const TokenKeyFile = "token.key"

// CodeNotAvailable: the token exists but its secret cannot be shown (issued
// before tokens were kept encrypted, or the key file was replaced).
const CodeNotAvailable = "not_available"

// LoadTokenKey reads <dataDir>/token.key, creating a random 32-byte key
// (mode 0600) the first time.
func LoadTokenKey(dataDir string) ([]byte, error) {
	path := filepath.Join(dataDir, TokenKeyFile)
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	switch {
	case err == nil:
		_, werr := f.Write(key)
		if cerr := f.Close(); werr == nil {
			werr = cerr
		}
		if werr != nil {
			return nil, werr
		}
		return key, nil
	case errors.Is(err, fs.ErrExist):
		key, err = os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		if len(key) != 32 {
			return nil, fmt.Errorf("%s: want 32 bytes, got %d", path, len(key))
		}
		return key, nil
	default:
		return nil, err
	}
}

// SetTokenKey enables keeping an encrypted copy of new tokens (AES-256-GCM).
func (s *Service) SetTokenKey(key []byte) error {
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	s.tokenAEAD, err = cipher.NewGCM(block)
	return err
}

// sealToken returns base64(nonce || ciphertext), or "" when no key is set.
func (s *Service) sealToken(raw string) (string, error) {
	if s.tokenAEAD == nil {
		return "", nil
	}
	nonce := make([]byte, s.tokenAEAD.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(s.tokenAEAD.Seal(nonce, nonce, []byte(raw), nil)), nil
}

func (s *Service) openToken(enc string) (string, bool) {
	if s.tokenAEAD == nil || enc == "" {
		return "", false
	}
	b, err := base64.StdEncoding.DecodeString(enc)
	n := s.tokenAEAD.NonceSize()
	if err != nil || len(b) < n {
		return "", false
	}
	raw, err := s.tokenAEAD.Open(nil, b[:n], b[n:], nil)
	if err != nil {
		return "", false
	}
	return string(raw), true
}

// RevealToken returns an active token's secret. The owner may reveal any
// token, others only their own (the same rule as listing and revoking).
func (s *Service) RevealToken(ctx context.Context, id string) (string, error) {
	actor, err := requireActor(ctx)
	if err != nil {
		return "", err
	}
	q, args := `SELECT token_enc FROM api_tokens WHERE id = ? AND revoked_at IS NULL`, []any{id}
	if !actor.User.IsOwner {
		q, args = q+` AND user_id = ?`, append(args, actor.User.ID)
	}
	var enc string
	err = s.db.QueryRowContext(ctx, q, args...).Scan(&enc)
	if errors.Is(err, sql.ErrNoRows) {
		return "", Errorf(CodeNotFound, "token not found")
	}
	if err != nil {
		return "", err
	}
	raw, ok := s.openToken(enc)
	if !ok {
		return "", Errorf(CodeNotAvailable, "this token cannot be shown (issued before tokens were kept, or the key changed); issue a new one")
	}
	return raw, nil
}
