package httpapi

import (
	"net/http/httptest"
	"testing"
)

func TestIsLocal(t *testing.T) {
	cases := []struct {
		remote, host string
		want         bool
	}{
		{"127.0.0.1:50000", "127.0.0.1:7700", true},
		{"127.0.0.1:50000", "localhost:7700", true},
		{"[::1]:50000", "[::1]:7700", true},
		{"127.0.0.1:50000", "LOCALHOST", true},
		// another device on the LAN or the tailnet
		{"192.168.1.20:50000", "192.168.1.10:7700", false},
		{"100.101.102.103:50000", "127.0.0.1:7700", false},
		// DNS rebinding or a reverse proxy on this machine
		{"127.0.0.1:50000", "attacker.example", false},
		{"127.0.0.1:50000", "my-pc.tailnet.ts.net", false},
		{"127.0.0.1:50000", "", false},
	}
	for _, c := range cases {
		r := httptest.NewRequest("POST", "/api/v1/auth/local", nil)
		r.RemoteAddr, r.Host = c.remote, c.host
		if got := isLocal(r); got != c.want {
			t.Errorf("isLocal(remote=%q, host=%q) = %v, want %v", c.remote, c.host, got, c.want)
		}
	}
}
