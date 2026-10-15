package logger

import (
	"bytes"
	"log"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// newTestService returns an initialized service that logs to the returned
// buffer.
//
// Parameters:
//   - t: the test.
//
// Returns:
//   - *Service: the service.
//   - *bytes.Buffer: the log output.
func newTestService(t *testing.T) (*Service, *bytes.Buffer) {
	t.Helper()

	logs := &bytes.Buffer{}
	service := &Service{}
	require.NoError(t, service.Initialize(&url.URL{Scheme: Scheme}, log.New(logs, "", 0)))

	return service, logs
}

// TestGetID verifies that the service identifies itself by its scheme.
func TestGetID(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "logger", (&Service{}).GetID())
}

// TestInitialize verifies that Initialize sets an empty config.
func TestInitialize(t *testing.T) {
	t.Parallel()

	service, _ := newTestService(t)

	assert.Equal(t, &Config{}, service.Config)
}

// TestSend verifies that a send logs the message as is without a template,
// and renders the message template with the params and message when one is
// set.
func TestSend(t *testing.T) {
	t.Parallel()

	// tests lists templates and params with the log line each send should
	// write.
	tests := []struct {
		params   *types.Params
		name     string
		template string
		want     string
	}{
		{name: "no template", want: "Failed - Requires Toaster Repair Level 10\n"},
		{
			name:   "no template ignores params",
			params: &types.Params{"level": "warning"},
			want:   "Failed - Requires Toaster Repair Level 10\n",
		},
		{
			name:     "template with params",
			template: "{{.level}}: {{.message}}",
			params:   &types.Params{"level": "warning"},
			want:     "warning: Failed - Requires Toaster Repair Level 10\n",
		},
		{
			name:     "message overrides a message param",
			template: "{{.message}}",
			params:   &types.Params{"message": "ignored"},
			want:     "Failed - Requires Toaster Repair Level 10\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service, logs := newTestService(t)
			if tt.template != "" {
				require.NoError(t, service.SetTemplateString("message", tt.template))
			}

			require.NoError(t, service.Send("Failed - Requires Toaster Repair Level 10", tt.params))

			assert.Equal(t, tt.want, logs.String())
		})
	}
}

// TestSendLeavesParamsUnchanged verifies that a send adds the message to a
// copy of the params, not to the caller's map.
func TestSendLeavesParamsUnchanged(t *testing.T) {
	t.Parallel()

	service, _ := newTestService(t)
	params := types.Params{"level": "warning"}

	require.NoError(t, service.Send("Failed - Requires Toaster Repair Level 10", &params))

	assert.Equal(t, types.Params{"level": "warning"}, params)
}

// TestSendTemplateError verifies that a template that fails to execute fails
// the send and logs nothing.
func TestSendTemplateError(t *testing.T) {
	t.Parallel()

	service, logs := newTestService(t)
	require.NoError(t, service.SetTemplateString("message", "{{range .message}}x{{end}}"))

	err := service.Send("Critical Failure", &types.Params{"level": "error"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to write template to log")
	assert.Empty(t, logs.String())
}

// TestServiceTimeout verifies that the service reports the router's default
// send budget.
func TestServiceTimeout(t *testing.T) {
	t.Parallel()

	assert.Equal(t, types.DefaultSendTimeout, (&Service{}).ServiceTimeout(nil))
}
