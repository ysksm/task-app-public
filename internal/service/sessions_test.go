package service

import (
	"context"
	"testing"
	"time"

	"github.com/ysksm/task-app/internal/store"
)

func TestSessionsSlideAndExpire(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := store.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	s := New(db, nil)
	clock := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return clock }
	u, err := s.CreateUser(ctx, CreateUserInput{Kind: "human", Username: "alice", DisplayName: "Alice"})
	if err != nil {
		t.Fatal(err)
	}
	kept, _ := s.CreateSession(ctx, u.ID, true)
	plain, _ := s.CreateSession(ctx, u.ID, false)
	check := func(sid string, wantOK, wantRenew bool) {
		t.Helper()
		_, renew, err := s.SessionUser(ctx, sid)
		if (err == nil) != wantOK || renew != wantRenew {
			t.Fatalf("at %s: ok=%v renew=%v, want ok=%v renew=%v", clock, err == nil, renew, wantOK, wantRenew)
		}
	}

	check(kept, true, false) // just created: nothing to slide yet
	clock = clock.Add(5 * time.Minute)
	check(kept, true, false) // within 10 minutes: no write
	clock = clock.Add(6 * time.Minute)
	check(kept, true, true)   // slid: the cookie is re-sent
	check(plain, true, false) // slid too, but a browser-session cookie needs no Max-Age

	// Used every 20 days, a remembered session outlives 30 days after login.
	for i := 0; i < 3; i++ {
		clock = clock.Add(20 * 24 * time.Hour)
		check(kept, true, true)
	}
	// The plain one expired a day after its last use; a remembered one 30 days after.
	check(plain, false, false)
	clock = clock.Add(31 * 24 * time.Hour)
	check(kept, false, false)
}
