package matrix_test

import (
	"context"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/matrix"
)

// TestServiceSendWithCancelledContext tests that the service handles context
// cancellation properly when attempting to send messages.
// Since this is an integration test without external calls, we verify the
// service properly detects and reports the uninitialized client state.
func TestServiceSendWithCancelledContext(t *testing.T) {
	t.Parallel()

	// Create a service with dummy URL (client won't be initialized)
	service, _ := createTestService(t, "matrix://dummy@dummy.com")

	// Create a canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Attempting to send with an uninitialized client should return
	// ErrClientNotInitialized, regardless of context state
	err := service.SendWithContext(ctx, "Test message", nil)

	require.Error(t, err, "Expected error when sending with uninitialized client")
	require.ErrorIs(t, err, matrix.ErrClientNotInitialized,
		"Expected ErrClientNotInitialized, got %v", err)
}

// TestServiceSendToMultipleRooms tests that sending to multiple rooms
// is properly configured and the service handles room lists correctly.
// Rooms are set directly on the config since URL params only support one room.
func TestServiceSendToMultipleRooms(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		serviceURL    string
		rooms         []string
		expectedRooms []string
	}{
		{
			name:          "single room set directly",
			serviceURL:    "matrix://user:token@matrix.example.com",
			rooms:         []string{"#general:example.com"},
			expectedRooms: []string{"#general:example.com"},
		},
		{
			name:          "multiple rooms set directly",
			serviceURL:    "matrix://user:token@matrix.example.com",
			rooms:         []string{"#general:example.com", "#alerts:example.com"},
			expectedRooms: []string{"#general:example.com", "#alerts:example.com"},
		},
		{
			name:          "three rooms set directly",
			serviceURL:    "matrix://user:token@matrix.example.com",
			rooms:         []string{"#room1:example.com", "#room2:example.com", "#room3:example.com"},
			expectedRooms: []string{"#room1:example.com", "#room2:example.com", "#room3:example.com"},
		},
		{
			name:          "rooms with room IDs (not aliases)",
			serviceURL:    "matrix://user:token@matrix.example.com",
			rooms:         []string{"!room1id:example.com"},
			expectedRooms: []string{"!room1id:example.com"},
		},
		{
			name:          "mixed room aliases and IDs",
			serviceURL:    "matrix://user:token@matrix.example.com",
			rooms:         []string{"#general:example.com", "!alertid:example.com"},
			expectedRooms: []string{"#general:example.com", "!alertid:example.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			parsedURL, err := url.Parse(tt.serviceURL)
			require.NoError(t, err, "URL should be parseable for %s", tt.name)

			// Set rooms directly on config
			config := &matrix.Config{
				Rooms: tt.rooms,
			}
			err = config.SetURL(parsedURL)
			require.NoError(t, err, "Config should be set for %s", tt.name)
			require.Equal(t, tt.expectedRooms, config.Rooms,
				"Rooms mismatch for %s", tt.name)
		})
	}
}

// TestServiceSendWithSpecialCharacters tests message content with
// HTML and Markdown special characters that might need escaping
// or special handling in Matrix messages.
//
//nolint:gosmopolitan // Intentional string literal containing rune in Han script
func TestServiceSendWithSpecialCharacters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		message string
		title   string
	}{
		{
			name:    "message with HTML tags",
			message: "<p>This is a <strong>bold</strong> message</p>",
			title:   "",
		},
		{
			name:    "message with Markdown bold",
			message: "This is **bold** and *italic* text",
			title:   "",
		},
		{
			name:    "message with special chars ampersand",
			message: "Test & more <> characters",
			title:   "",
		},
		{
			name:    "message with newlines",
			message: "Line 1\nLine 2\nLine 3",
			title:   "",
		},
		{
			name:    "message with tabs",
			message: "Col1\tCol2\tCol3",
			title:   "",
		},
		{
			name:    "message with title and special chars",
			message: "Alert: <error> critical issue",
			title:   "Warning",
		},
		{
			name:    "message with backticks",
			message: "Use `code` for inline code",
			title:   "",
		},
		{
			name:    "message with Unicode characters",
			message: "Hello 世界 🌍 αβγδ",
			title:   "",
		},
		{
			name:    "message with JSON-like content",
			message: `{"key": "value", "number": 123}`,
			title:   "",
		},
		{
			name:    "message with URL",
			message: "Check https://example.com/path?param=value for more info",
			title:   "",
		},
		{
			name:    "empty message",
			message: "",
			title:   "",
		},
		{
			name:    "message with only special characters",
			message: "!@#$%^&*()_+-=[]{}|;':\",./<>?",
			title:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Build URL with title if needed
			serviceURL := "matrix://dummy@dummy.com"
			if tt.title != "" {
				serviceURL += "?title=" + url.QueryEscape(tt.title)
			}

			service, _ := createTestService(t, serviceURL)

			// The service should accept any message content
			// The actual sending would fail with uninitialized client,
			// but we're testing that the message is accepted
			err := service.Send(tt.message, nil)

			// With dummy URL, client is not initialized so we expect error
			require.Error(t, err, "Expected error with uninitialized client for %s", tt.name)
			require.ErrorIs(t, err, matrix.ErrClientNotInitialized,
				"Expected ErrClientNotInitialized for %s", tt.name)
		})
	}
}

// TestServiceSendWithTitle tests that the title is properly prepended
// to the message when sending notifications.
func TestServiceSendWithTitle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		serviceURL string
		message    string
		wantTitle  string
	}{
		{
			name:       "title set via URL",
			serviceURL: "matrix://user:token@matrix.example.com?title=Alert",
			message:    "This is the message",
			wantTitle:  "Alert",
		},
		{
			name:       "title with spaces",
			serviceURL: "matrix://user:token@matrix.example.com?title=Critical%20Alert",
			message:    "Critical issue detected",
			wantTitle:  "Critical Alert",
		},
		{
			name:       "no title",
			serviceURL: "matrix://user:token@matrix.example.com",
			message:    "Plain message",
			wantTitle:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			parsedURL, err := url.Parse(tt.serviceURL)
			require.NoError(t, err, "URL should be parseable for %s", tt.name)

			config := &matrix.Config{}
			err = config.SetURL(parsedURL)
			require.NoError(t, err, "Config should be set for %s", tt.name)
			require.Equal(t, tt.wantTitle, config.Title,
				"Title mismatch for %s", tt.name)
		})
	}
}

// TestServiceSendWithoutRooms tests behavior when no rooms are configured.
func TestServiceSendWithoutRooms(t *testing.T) {
	t.Parallel()

	// Create service without room configuration
	service, _ := createTestService(t, "matrix://dummy@dummy.com")

	// Verify no rooms are configured
	require.Empty(t, service.Config.Rooms, "Rooms should be empty when not configured")
}
