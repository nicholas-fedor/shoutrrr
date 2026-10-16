package signal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseRecipients verifies that path segments become phone numbers,
// usernames, and group IDs, joining the segments a slash in a group ID
// splits until the next phone number with a + prefix, username, or group.
func TestParseRecipients(t *testing.T) {
	t.Parallel()

	// tests lists path segments with the recipients each should produce.
	tests := []struct {
		name  string
		parts []string
		want  []string
	}{
		{name: "phone numbers", parts: []string{testRecipient, "+15559876543"}, want: []string{testRecipient, "+15559876543"}},
		{name: "phone number formatting", parts: []string{"+1 (555) 123-4567"}, want: []string{"+1 (555) 123-4567"}},
		{name: "usernames", parts: []string{"u:alice.01", "u:bob.02"}, want: []string{"u:alice.01", "u:bob.02"}},
		{name: "group IDs", parts: []string{"group.Z3JvdXAx", "group.ABCD+EFGH="}, want: []string{"group.Z3JvdXAx", "group.ABCD+EFGH="}},
		{name: "group ID split by slashes", parts: []string{"group.AB", "CD", "EF=="}, want: []string{"group.AB/CD/EF=="}},
		{
			name:  "split group ID before a phone number",
			parts: []string{"group.AB", "CD=", testRecipient},
			want:  []string{"group.AB/CD=", testRecipient},
		},
		{
			name:  "split group ID before a username",
			parts: []string{"group.AB", "CD=", "u:alice.01"},
			want:  []string{"group.AB/CD=", "u:alice.01"},
		},
		{
			name:  "split group ID before a group ID",
			parts: []string{"group.AB", "CD=", "group.EF"},
			want:  []string{"group.AB/CD=", "group.EF"},
		},
		{
			name:  "mixed types",
			parts: []string{testRecipient, "group.Z3JvdXAx", "+15559876543", "u:alice.01"},
			want:  []string{testRecipient, "group.Z3JvdXAx", "+15559876543", "u:alice.01"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			recipients, err := parseRecipients(tt.parts)

			require.NoError(t, err)
			assert.Equal(t, tt.want, recipients)
		})
	}
}

// TestParseRecipientsErrors verifies that an empty list and any invalid
// recipient are rejected.
func TestParseRecipientsErrors(t *testing.T) {
	t.Parallel()

	// tests lists path segments with the error each should produce.
	tests := []struct {
		wantErr error
		name    string
		parts   []string
	}{
		{name: "no segments", parts: []string{}, wantErr: ErrNoRecipients},
		{name: "invalid recipient", parts: []string{"alice"}, wantErr: ErrInvalidRecipient},
		{name: "invalid after a valid one", parts: []string{testRecipient, "alice"}, wantErr: ErrInvalidRecipient},
		{name: "empty username", parts: []string{"u:"}, wantErr: ErrInvalidRecipient},
		{name: "invalid group ID", parts: []string{"group.AB!"}, wantErr: ErrInvalidRecipient},
		{name: "empty group ID", parts: []string{"group."}, wantErr: ErrInvalidRecipient},
		{name: "empty segment", parts: []string{testRecipient, ""}, wantErr: ErrInvalidRecipient},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := parseRecipients(tt.parts)

			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}

// TestBatchRecipients verifies that phone numbers share the first batch,
// usernames the next, and each group gets its own batch.
func TestBatchRecipients(t *testing.T) {
	t.Parallel()

	// tests lists recipients with the batches each should produce.
	tests := []struct {
		name       string
		recipients []string
		want       [][]string
	}{
		{name: "none"},
		{name: "phone numbers", recipients: []string{testRecipient, "+15559876543"}, want: [][]string{{testRecipient, "+15559876543"}}},
		{name: "groups", recipients: []string{"group.Z3JvdXAx", "group.Z3JvdXAy"}, want: [][]string{{"group.Z3JvdXAx"}, {"group.Z3JvdXAy"}}},
		{
			name:       "mixed types",
			recipients: []string{"group.Z3JvdXAx", "u:alice.01", testRecipient, "u:bob.02", "+15559876543"},
			want:       [][]string{{testRecipient, "+15559876543"}, {"u:alice.01", "u:bob.02"}, {"group.Z3JvdXAx"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, batchRecipients(tt.recipients))
		})
	}
}

// TestRecipientValidation verifies which strings count as phone numbers,
// group IDs, usernames, and recipients.
func TestRecipientValidation(t *testing.T) {
	t.Parallel()

	// tests lists strings with the kinds of recipient each should match.
	tests := []struct {
		value     string
		wantPhone bool
		wantGroup bool
		wantUser  bool
	}{
		{value: testRecipient, wantPhone: true},
		{value: "15551234567", wantPhone: true},
		{value: "+1 (555) 123-4567", wantPhone: true},
		{value: "group.ABCD/EF+GH_-=", wantGroup: true},
		{value: "u:alice.01", wantUser: true},
		{value: "u:"},
		{value: "group."},
		{value: "group.AB!"},
		{value: "alice"},
		{value: ""},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.wantPhone, isValidPhoneNumber(tt.value), "phone number")
			assert.Equal(t, tt.wantGroup, isValidGroupID(tt.value), "group ID")
			assert.Equal(t, tt.wantUser, isValidUsername(tt.value), "username")
			assert.Equal(t, tt.wantPhone || tt.wantGroup || tt.wantUser, isValidRecipient(tt.value), "recipient")
		})
	}
}
