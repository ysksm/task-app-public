package main

import "testing"

func TestOwnerNameFromEnv(t *testing.T) {
	t.Setenv("TASKAPP_OWNER", "Alice Smith!")
	if got := ownerName(); got != "alicesmith" {
		t.Errorf("ownerName() = %q, want %q", got, "alicesmith")
	}
	t.Setenv("TASKAPP_OWNER", "")
	if got := ownerName(); got == "" {
		t.Error("ownerName() is empty without TASKAPP_OWNER")
	}
}
