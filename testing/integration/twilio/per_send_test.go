package twilio_test

import (
	"io"
	"net/http"
	"net/url"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// TestSendParamsApplyToOneSend verifies that a title param shapes only the send it
// is passed to: a later send without params uses the configured message alone.
func TestSendParamsApplyToOneSend(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		for range 2 {
			mockClient.On("Do", mock.Anything).
				Return(createMockResponse(http.StatusCreated, `{"sid": "SM123"}`), nil).
				Once()
		}

		service := createTestService(t, validTwilioURL, mockClient)

		require.NoError(t, service.Send("first", &types.Params{"title": "Alert"}))
		require.NoError(t, service.Send("second", nil))

		bodies := make([]string, 0, len(mockClient.Calls))

		for i := range mockClient.Calls {
			call := &mockClient.Calls[i]

			req, ok := call.Arguments[0].(*http.Request)
			require.True(t, ok)

			raw, err := io.ReadAll(req.Body)
			require.NoError(t, err)

			form, err := url.ParseQuery(string(raw))
			require.NoError(t, err)

			bodies = append(bodies, form.Get("Body"))
		}

		assert.Equal(t, []string{"Alert\nfirst", "second"}, bodies)
		assert.Empty(t, service.Config.Title)
	})
}
