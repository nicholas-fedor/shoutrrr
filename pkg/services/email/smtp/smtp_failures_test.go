package smtp

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/internal/failures"
	failuresmocks "github.com/nicholas-fedor/shoutrrr/internal/failures/mocks"
)

// TestFail verifies the message of each failure ID and that the failure
// carries the ID.
func TestFail(t *testing.T) {
	t.Parallel()

	// tests lists failure IDs with their messages.
	tests := []struct {
		want string
		id   failures.FailureID
	}{
		{id: FailUnknown, want: "an unknown error occurred"},
		{id: FailGetSMTPClient, want: "error getting SMTP client"},
		{id: FailEnableStartTLS, want: "error enabling StartTLS"},
		{id: FailAuthenticating, want: "error authenticating"},
		{id: FailClosingSession, want: "error closing session"},
		{id: FailPlainHeader, want: "error writing plain header"},
		{id: FailHTMLHeader, want: "error writing HTML header"},
		{id: FailMultiEndHeader, want: "error writing multipart end header"},
		{id: FailMessageTemplate, want: "error applying message template"},
		{id: FailMessageRaw, want: "error writing message"},
		{id: FailSetSender, want: "error setting MAIL FROM"},
		{id: FailSetRecipient, want: "error setting RCPT"},
		{id: FailOpenDataStream, want: "error starting DATA"},
		{id: FailWriteHeaders, want: "error writing message headers"},
		{id: FailCloseDataStream, want: "error closing message stream"},
		{id: FailConnectToServer, want: "error connecting to server"},
		{id: FailCreateSMTPClient, want: "error creating smtp client"},
		{id: FailApplySendParams, want: "error applying params to send config"},
		{id: FailHandshake, want: "server did not accept the handshake"},
		{id: FailResetSession, want: "error resetting session between recipients"},
		{id: failures.FailureID(999), want: "an unknown error occurred"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()

			got := fail(tt.id, nil)

			assert.Equal(t, tt.id, got.ID())
			require.EqualError(t, got, tt.want)
			assert.NoError(t, got.Unwrap())
		})
	}
}

// TestFailArguments verifies that the authentication method and recipient
// fill in their messages, keeping the failure IDs.
func TestFailArguments(t *testing.T) {
	t.Parallel()

	authType := fail(FailAuthType, nil, "bogus")
	assert.Equal(t, FailAuthType, authType.ID())
	require.EqualError(t, authType, "invalid authentication method 'bogus'")

	sendRecipient := fail(FailSendRecipient, errors.New("rejected"), "rec@example.com")
	assert.Equal(t, FailSendRecipient, sendRecipient.ID())
	require.EqualError(t, sendRecipient, `error sending message to recipient "rec@example.com": rejected`)
}

// TestFailWrapping verifies that a failure wraps its cause, adds the cause to
// its message, and matches other failures with the same ID.
func TestFailWrapping(t *testing.T) {
	t.Parallel()

	cause := failuresmocks.NewMockFailure(t)
	cause.EXPECT().Error().Return("connection reset").Once()

	got := fail(FailHandshake, cause)

	assert.Same(t, cause, got.Unwrap())
	require.EqualError(t, got, "server did not accept the handshake: connection reset")
	require.ErrorIs(t, fail(FailHandshake, errors.New("one")), fail(FailHandshake, nil))
	require.NotErrorIs(t, fail(FailHandshake, nil), fail(FailAuthenticating, nil))
}
