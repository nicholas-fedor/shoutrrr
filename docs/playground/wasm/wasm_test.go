package main

import (
	"encoding/json"
	"testing"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/router"
)

// TestAllServicesHaveSchema verifies that every registered service produces a
// valid config schema via the public API. If a new service is added or an
// existing one changes, this test adapts automatically.
func TestAllServicesHaveSchema(t *testing.T) {
	t.Parallel()

	r := router.ServiceRouter{}
	schemes := r.ListServices()

	if len(schemes) == 0 {
		t.Fatal("no services registered in serviceMap")
	}

	for _, scheme := range schemes {
		t.Run(scheme, func(t *testing.T) {
			t.Parallel()

			service, err := r.NewService(scheme)
			if err != nil {
				t.Fatalf("NewService(%q) failed: %v", scheme, err)
			}

			if service == nil {
				t.Fatalf("NewService(%q) returned nil", scheme)
			}

			config := format.GetServiceConfig(service)
			if config == nil {
				t.Fatalf("GetServiceConfig(%q) returned nil", scheme)
			}

			configNode := format.GetConfigFormat(config)
			if configNode == nil {
				t.Fatalf("GetConfigFormat(%q) returned nil", scheme)
			}

			if len(configNode.Items) == 0 {
				// Logger service has no config fields by design.
				if scheme != "logger" {
					t.Errorf("service %q has no config fields", scheme)
				}
			}
		})
	}
}

// TestSchemaSerialization verifies that config schemas serialize to valid JSON.
func TestSchemaSerialization(t *testing.T) {
	t.Parallel()

	r := router.ServiceRouter{}
	schemes := r.ListServices()

	for _, scheme := range schemes {
		t.Run(scheme, func(t *testing.T) {
			t.Parallel()

			jsonStr := configSchemaJSON(scheme)

			// Check for error response first.
			var errResp errorResult
			if err := json.Unmarshal([]byte(jsonStr), &errResp); err == nil && errResp.Error != "" {
				t.Fatalf("schema returned error for %q: %s", scheme, errResp.Error)
			}

			var decoded configSchema
			if err := json.Unmarshal([]byte(jsonStr), &decoded); err != nil {
				t.Fatalf("failed to unmarshal schema for %q: %v\nJSON: %s", scheme, err, jsonStr)
			}

			if decoded.Service != scheme {
				t.Errorf("service mismatch: got %q, want %q", decoded.Service, scheme)
			}

			if len(decoded.Fields) == 0 {
				// Logger service has no config fields by design.
				if scheme != "logger" {
					t.Errorf("no fields in schema for %q", scheme)
				}
			}
		})
	}
}

// TestGetServicesSerialization verifies that the services list serializes to valid JSON.
func TestGetServicesSerialization(t *testing.T) {
	t.Parallel()

	jsonStr := listServicesJSON()

	var decoded []string
	if err := json.Unmarshal([]byte(jsonStr), &decoded); err != nil {
		t.Fatalf("failed to unmarshal services list: %v", err)
	}

	if len(decoded) == 0 {
		t.Error("services list is empty")
	}
}

// TestExtractScheme verifies URL scheme extraction.
func TestExtractScheme(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{"discord://token@webhook", "discord"},
		{"teams+https://example.com/path", "teams"},
		{"smtp://user:pass@host:587", "smtp"},
		{"ntfy://ntfy.sh/topic", "ntfy"},
		{"invalid", ""},
		{"", ""},
		{"://missing-scheme", ""},
		{"a://single-char-scheme", "a"},
		{"HTTPS://uppercase", "HTTPS"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			got := extractScheme(tt.input)
			if got != tt.want {
				t.Errorf("extractScheme(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
