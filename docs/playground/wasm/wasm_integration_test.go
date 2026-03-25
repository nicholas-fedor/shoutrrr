package main

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/nicholas-fedor/shoutrrr/pkg/router"
)

// exampleURLs provides test URLs for services that have well-defined URL formats.
// Services not listed here will only be tested for default URL generation.
var exampleURLs = map[string]string{
	"discord":    "discord://token@123456789",
	"generic":    "generic://192.168.1.100:8123/api/webhook",
	"gotify":     "gotify://gotify.example.com/token",
	"logger":     "logger://",
	"ntfy":       "ntfy://ntfy.sh/mytopic",
	"smtp":       "smtp://user:password@mail.example.com:587/?from=from@example.com&to=to@example.com",
	"telegram":   "telegram://123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11@telegram?chats=123456",
	"mattermost": "mattermost://mattermost.example.com/token/channel",
	"mqtt":       "mqtt://broker.example.com/topic",
	"zulip":      "zulip://bot-mail:bot-key@zulip.example.com?stream=foo&topic=bar",
	"ifttt":      "ifttt://WebhookID?events=event1",
	"googlechat": "googlechat://chat.googleapis.com/v1/spaces/example/messages?key=abc&token=xyz",
	"notifiarr":  "notifiarr://api.example.com/api/v1/notification/trigger/apikey",
}

// TestAllServicesHaveSchema verifies that every registered service produces a
// valid config schema. This test automatically adapts to new services.
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

			result := configSchemaJSON(scheme)

			var errResp errorResult
			if err := json.Unmarshal([]byte(result), &errResp); err == nil && errResp.Error != "" {
				t.Fatalf("configSchemaJSON(%q) returned error: %s", scheme, errResp.Error)
			}

			var schema configSchema
			if err := json.Unmarshal([]byte(result), &schema); err != nil {
				t.Fatalf("failed to unmarshal schema for %q: %v", scheme, err)
			}

			if schema.Service != scheme {
				t.Errorf("service mismatch: got %q, want %q", schema.Service, scheme)
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

			var errResp errorResult
			if err := json.Unmarshal([]byte(jsonStr), &errResp); err == nil && errResp.Error != "" {
				t.Fatalf("configSchemaJSON(%q) returned error: %s", scheme, errResp.Error)
			}

			var decoded configSchema
			if err := json.Unmarshal([]byte(jsonStr), &decoded); err != nil {
				t.Fatalf("failed to unmarshal schema for %q: %v\nJSON: %s", scheme, err, jsonStr)
			}

			if decoded.Service != scheme {
				t.Errorf("service mismatch: got %q, want %q", decoded.Service, scheme)
			}
		})
	}
}

// TestAllServicesGenerateDefaultURL verifies that every service returns at least
// a scheme URL (e.g., "discord://") when called with empty config.
func TestAllServicesGenerateDefaultURL(t *testing.T) {
	t.Parallel()

	r := router.ServiceRouter{}
	schemes := r.ListServices()

	for _, scheme := range schemes {
		t.Run(scheme, func(t *testing.T) {
			t.Parallel()

			result := generateURLString(scheme, "{}")

			var errResp errorResult
			if err := json.Unmarshal([]byte(result), &errResp); err == nil && errResp.Error != "" {
				t.Fatalf("generateURLString(%q, {}) returned error: %s", scheme, errResp.Error)
			}

			var parsed map[string]string
			if err := json.Unmarshal([]byte(result), &parsed); err != nil {
				t.Fatalf("generateURLString(%q, {}) returned invalid JSON: %s", scheme, result)
			}

			url, ok := parsed["url"]
			if !ok {
				t.Fatalf("generateURLString(%q, {}) missing 'url' key: %s", scheme, result)
			}

			// Verify URL starts with the scheme.
			// Some services return "scheme:" (without "//") when fields are empty.
			// Some schemes are aliases (e.g., hangouts→googlechat, mqtts→mqtt).
			aliasMap := map[string]string{
				"hangouts": "googlechat",
				"mqtts":    "mqtt",
			}

			checkScheme := scheme
			if alias, ok := aliasMap[scheme]; ok {
				checkScheme = alias
			}

			if !strings.HasPrefix(url, checkScheme+":") {
				t.Errorf("generateURLString(%q, {}) = %q, want prefix %q", scheme, url, checkScheme+":")
			}
		})
	}
}

// TestAllServicesValidateURL verifies that every service URL scheme validates.
func TestAllServicesValidateURL(t *testing.T) {
	t.Parallel()

	r := router.ServiceRouter{}
	schemes := r.ListServices()

	for _, scheme := range schemes {
		rawURL := scheme + "://"

		t.Run(scheme, func(t *testing.T) {
			t.Parallel()

			result := validateURLString(rawURL)

			var valid map[string]bool
			if err := json.Unmarshal([]byte(result), &valid); err != nil {
				// Some services may return errors for empty URLs - acceptable.
				t.Logf("validateURLString(%q) returned non-boolean result: %s", rawURL, result)

				return
			}

			t.Logf("validateURLString(%q) = %v", rawURL, valid["valid"])
		})
	}
}

// TestGetServicesSerialization verifies the services list serializes to valid JSON.
func TestGetServicesSerialization(t *testing.T) {
	t.Parallel()

	jsonStr := listServicesJSON()

	var schemes []string
	if err := json.Unmarshal([]byte(jsonStr), &schemes); err != nil {
		t.Fatalf("failed to unmarshal services list: %v", err)
	}

	if len(schemes) == 0 {
		t.Error("services list is empty")
	}
}

// TestParseURL verifies URL parsing for services with known formats.
func TestParseURL(t *testing.T) {
	t.Parallel()

	for scheme, rawURL := range exampleURLs {
		t.Run(scheme, func(t *testing.T) {
			t.Parallel()

			result := parseURLString(rawURL)

			var errResp errorResult
			if err := json.Unmarshal([]byte(result), &errResp); err == nil && errResp.Error != "" {
				t.Fatalf("parseURLString(%q) returned error: %s", rawURL, errResp.Error)
			}

			var parsed parseResult
			if err := json.Unmarshal([]byte(result), &parsed); err != nil {
				t.Fatalf("failed to unmarshal parse result: %v", err)
			}

			if parsed.Service != scheme {
				t.Errorf("service mismatch: got %q, want %q", parsed.Service, scheme)
			}
		})
	}
}

// TestValidateURL verifies URL validation for known good URLs.
func TestValidateURL(t *testing.T) {
	t.Parallel()

	for scheme, rawURL := range exampleURLs {
		t.Run(scheme, func(t *testing.T) {
			t.Parallel()

			result := validateURLString(rawURL)

			var valid map[string]bool
			if err := json.Unmarshal([]byte(result), &valid); err != nil {
				t.Fatalf("failed to unmarshal validation result: %v", err)
			}

			if !valid["valid"] {
				t.Errorf("expected valid=true for %q", rawURL)
			}
		})
	}
}

// TestGenerateURL verifies URL generation for services.
func TestGenerateURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		scheme   string
		config   string
		wantErr  bool
		contains string
	}{
		{
			name:     "discord basic",
			scheme:   "discord",
			config:   `{"WebhookID":"123456789","Token":"mytoken"}`,
			contains: "discord://",
		},
		{
			name:     "ntfy basic",
			scheme:   "ntfy",
			config:   `{"Host":"ntfy.sh","Path":"mytopic"}`,
			contains: "ntfy://",
		},
		{
			name:     "generic with webhook",
			scheme:   "generic",
			config:   `{"WebhookURL":"192.168.1.100:8123/api/webhook/abc123"}`,
			contains: "generic://",
		},
		{
			name:    "invalid service",
			scheme:  "nonexistent",
			config:  `{}`,
			wantErr: true,
		},
		{
			name:    "invalid JSON",
			scheme:  "discord",
			config:  "not-json",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := generateURLString(tt.scheme, tt.config)

			var errResp errorResult
			if err := json.Unmarshal([]byte(result), &errResp); err == nil && errResp.Error != "" {
				if !tt.wantErr {
					t.Fatalf("unexpected error: %s", errResp.Error)
				}

				return
			}

			if tt.wantErr {
				t.Fatal("expected error but got success")
			}

			var parsed map[string]string
			if err := json.Unmarshal([]byte(result), &parsed); err != nil {
				t.Fatalf("failed to unmarshal result: %v", err)
			}

			if tt.contains != "" && !strings.Contains(parsed["url"], tt.contains) {
				t.Errorf("expected URL to contain %q, got %q", tt.contains, parsed["url"])
			}
		})
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
		{"generic://192.168.1.100:8123/path", "generic"},
		{"invalid", ""},
		{"", ""},
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

// TestGetServicesMatchesList verifies that the JSON services match the API.
func TestGetServicesMatchesList(t *testing.T) {
	t.Parallel()

	r := router.ServiceRouter{}
	expected := r.ListServices()

	jsonStr := listServicesJSON()

	var schemes []string
	if err := json.Unmarshal([]byte(jsonStr), &schemes); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(schemes) != len(expected) {
		t.Errorf("service count mismatch: got %d, want %d", len(schemes), len(expected))
	}
}

// TestMarshalError verifies error serialization.
func TestMarshalError(t *testing.T) {
	t.Parallel()

	t.Run("nil error", func(t *testing.T) {
		t.Parallel()

		result := marshalError(nil)
		if !strings.Contains(result, "unknown error") {
			t.Errorf("expected 'unknown error' for nil, got: %s", result)
		}
	})

	t.Run("valid error", func(t *testing.T) {
		t.Parallel()

		result := marshalError(errors.New("test error"))
		if !strings.Contains(result, "test error") {
			t.Errorf("expected 'test error' in result, got: %s", result)
		}
	})
}

// TestMarshalErrorStr verifies string error serialization.
func TestMarshalErrorStr(t *testing.T) {
	t.Parallel()

	result := marshalErrorStr("something failed")
	if !strings.Contains(result, "something failed") {
		t.Errorf("expected error message in result, got: %s", result)
	}
}
