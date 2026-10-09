package basic

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/discord"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util/generator"
)

// generateTimeout bounds each generation in these tests, so a generator that
// loops fails the test instead of hanging until the test binary times out.
const generateTimeout = 5 * time.Second

// TestGenerateWithClosedInput verifies that generation stops when the input ends
// before a required field has a valid value, and still succeeds when props supply
// every required field.
func TestGenerateWithClosedInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		service types.Service
		props   map[string]string
		wantErr error
	}{
		{
			name:    "a required field without a value",
			service: &discord.Service{},
			props:   map[string]string{},
			wantErr: generator.ErrInputClosed,
		},
		{
			name:    "a rejected prop value",
			service: &mockServiceConfig{Config: &mockConfig{}},
			props:   map[string]string{"port": "not-a-number"},
			wantErr: generator.ErrInputClosed,
		},
		{
			name:    "props for every required field",
			service: &discord.Service{},
			props:   map[string]string{"token": "abc", "webhookid": "123"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			g := &Generator{Input: strings.NewReader("")}

			done := make(chan error, 1)

			go func() {
				_, err := g.Generate(tt.service, tt.props, nil)
				done <- err
			}()

			select {
			case err := <-done:
				if tt.wantErr != nil {
					require.ErrorIs(t, err, tt.wantErr)

					return
				}

				require.NoError(t, err)
			case <-time.After(generateTimeout):
				t.Fatal("Generate did not return after the input closed")
			}
		})
	}
}
