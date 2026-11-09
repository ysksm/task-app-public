package service

import (
	"strings"
	"testing"
)

func TestPasswordHash(t *testing.T) {
	h, err := hashPassword("correct horse")
	if err != nil {
		t.Fatal(err)
	}
	if parts := strings.Split(h, "$"); len(parts) != 4 || parts[0] != "pbkdf2-sha256" || parts[1] != "600000" {
		t.Fatalf("unexpected format %q", h)
	}
	if !checkPassword(h, "correct horse") {
		t.Error("the right password was rejected")
	}
	for _, wrong := range []string{"", "correct horse ", "Correct horse"} {
		if checkPassword(h, wrong) {
			t.Errorf("accepted %q", wrong)
		}
	}
	h2, _ := hashPassword("correct horse")
	if h == h2 {
		t.Error("the same password gave the same hash (salt missing)")
	}
	for _, bad := range []string{"", "plain", "bcrypt$1$a$b", "pbkdf2-sha256$x$a$b", "pbkdf2-sha256$1$!!$b"} {
		if checkPassword(bad, "x") {
			t.Errorf("accepted malformed hash %q", bad)
		}
	}
}

// A hash written by another implementation (Python: hashlib.pbkdf2_hmac)
// must verify, so that a rewrite can keep existing passwords.
func TestPasswordHashInterop(t *testing.T) {
	const h = "pbkdf2-sha256$1000$c2FsdHNhbHRzYWx0c2FsdA$" + interopKey
	if !checkPassword(h, "password") {
		t.Error("interop hash rejected")
	}
}

const interopKey = "8nX7hwFEzIB8aPajJTYK8weHQc5Ngz0pFVAKvSu4jQA"
