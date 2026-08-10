package smtp

import (
	"errors"
	"net/smtp"
	"strings"
	"testing"
)

func TestLoginAuthStart(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		host    string
		server  *smtp.ServerInfo
		wantErr error
	}{
		{
			name:   "TLS and matching host",
			host:   "mail.example.com",
			server: &smtp.ServerInfo{Name: "mail.example.com", TLS: true},
		},
		{
			name:   "localhost without TLS",
			host:   "localhost",
			server: &smtp.ServerInfo{Name: "localhost", TLS: false},
		},
		{
			name:   "127.0.0.1 without TLS",
			host:   "127.0.0.1",
			server: &smtp.ServerInfo{Name: "127.0.0.1", TLS: false},
		},
		{
			name:    "unencrypted remote",
			host:    "mail.example.com",
			server:  &smtp.ServerInfo{Name: "mail.example.com", TLS: false},
			wantErr: errUnencryptedConnection,
		},
		{
			name:    "wrong host",
			host:    "mail.example.com",
			server:  &smtp.ServerInfo{Name: "evil.example.com", TLS: true},
			wantErr: errWrongHostName,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			auth := LoginAuth("user", "pass", tt.host)

			mech, resp, err := auth.Start(tt.server)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Start() error = %v, want %v", err, tt.wantErr)
			}

			if tt.wantErr != nil {
				return
			}

			if mech != "LOGIN" {
				t.Errorf("Start() mech = %q, want LOGIN", mech)
			}

			if resp != nil {
				t.Errorf("Start() resp = %v, want nil", resp)
			}
		})
	}
}

func TestLoginAuthNext(t *testing.T) {
	t.Parallel()

	auth := LoginAuth("alice", "s3cret", "mail.example.com").(*loginAuth)

	// Successful two-step exchange. Challenge text is ignored.
	resp, err := auth.Next([]byte("User Name\x00"), true)
	if err != nil {
		t.Fatalf("step 0: unexpected error: %v", err)
	}

	if string(resp) != "alice" {
		t.Fatalf("step 0: resp = %q, want alice", resp)
	}

	resp, err = auth.Next([]byte("password:"), true)
	if err != nil {
		t.Fatalf("step 1: unexpected error: %v", err)
	}

	if string(resp) != "s3cret" {
		t.Fatalf("step 1: resp = %q, want s3cret", resp)
	}

	// An extra challenge must abort with a non-nil error, not a dummy response.
	resp, err = auth.Next([]byte("Username:"), true)
	if err == nil {
		t.Fatal("step 2: expected error, got nil")
	}

	if resp != nil {
		t.Fatalf("step 2: resp = %v, want nil so net/smtp aborts", resp)
	}

	if !errors.Is(err, errUnexpectedServerChallenge) {
		t.Fatalf("step 2: error = %v, want %v", err, errUnexpectedServerChallenge)
	}

	if got := err.Error(); !strings.Contains(got, "Username:") {
		t.Fatalf("step 2: error = %q, want challenge quoted in message", got)
	}

	// more=false ends the exchange cleanly.
	resp, err = auth.Next(nil, false)
	if err != nil || resp != nil {
		t.Fatalf("more=false: resp=%v err=%v, want nil,nil", resp, err)
	}
}

func TestLoginAuthNextResetsWithStart(t *testing.T) {
	t.Parallel()

	auth := LoginAuth("u", "p", "localhost").(*loginAuth)
	if _, err := auth.Next(nil, true); err != nil {
		t.Fatalf("first Next: %v", err)
	}

	if _, _, err := auth.Start(&smtp.ServerInfo{Name: "localhost", TLS: false}); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if auth.respStep != 0 {
		t.Fatalf("respStep after Start = %d, want 0", auth.respStep)
	}

	resp, err := auth.Next([]byte("Username:"), true)
	if err != nil {
		t.Fatalf("Next after Start: %v", err)
	}

	if string(resp) != "u" {
		t.Fatalf("Next after Start: resp = %q, want u", resp)
	}
}

func TestIsLocalhost(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"localhost", "127.0.0.1", "::1"} {
		if !isLocalhost(name) {
			t.Errorf("isLocalhost(%q) = false, want true", name)
		}
	}

	if isLocalhost("example.com") {
		t.Error("isLocalhost(example.com) = true, want false")
	}
}
