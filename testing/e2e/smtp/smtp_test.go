package e2e_test

import (
	"maps"
	"net/textproto"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// TestSMTPConnections verifies that a message reaches Mailpit over STARTTLS
// with PLAIN and LOGIN authentication, over plaintext with PLAIN
// authentication, and over implicit TLS without authentication. Each test
// reads the stored message back.
//
//nolint:paralleltest // Sends to a live server one message at a time.
func TestSMTPConnections(t *testing.T) {
	// connections lists the server and settings for each connection, with the
	// account the server should record.
	connections := []struct {
		query                 map[string]string
		name, host, api, user string
		wantUser              string
	}{
		{
			name: "STARTTLS with PLAIN auth", host: starttlsHost, api: starttlsAPI, user: username, wantUser: username,
			query: map[string]string{"encryption": "ExplicitTLS", "auth": "Plain"},
		},
		{
			name: "STARTTLS with LOGIN auth", host: starttlsHost, api: starttlsAPI, user: username, wantUser: username,
			query: map[string]string{"encryption": "ExplicitTLS", "auth": "Login"},
		},
		{
			// Go sends PLAIN credentials over plaintext only to localhost.
			name: "plaintext with PLAIN auth", host: starttlsHost, api: starttlsAPI, user: username, wantUser: username,
			query: map[string]string{"encryption": "None", "usestarttls": "no", "auth": "Plain"},
		},
		{
			name: "implicit TLS without auth", host: tlsHost, api: tlsAPI,
			query: map[string]string{"encryption": "ImplicitTLS", "auth": "None"},
		},
	}

	for _, tt := range connections {
		t.Run(tt.name, func(t *testing.T) {
			requireMailpit(t, tt.api)

			subject := uniqueSubject("Shoutrrr E2E " + tt.name)
			query := map[string]string{"subject": subject}
			maps.Copy(query, tt.query)

			service := newService(t, serviceURL(tt.host, tt.user, password, query))
			require.NoError(t, service.Send("E2E Test: "+tt.name, nil))

			message := fetchMessage(t, tt.api, subject)
			assert.Equal(t, fromAddress, message.From.Address)
			assert.Equal(t, []address{{Name: "", Address: toAddress}}, message.To)
			assert.Contains(t, message.Text, "E2E Test: "+tt.name)
			assert.Equal(t, tt.wantUser, message.Username)
		})
	}
}

// TestSMTPAuthentication verifies that a server that requires an account
// rejects a wrong password with reply code 535 and a send without credentials
// with reply code 530 (RFC 4954), which a connection failure would not produce.
//
//nolint:paralleltest // Sends to a live server one message at a time.
func TestSMTPAuthentication(t *testing.T) {
	t.Run("wrong password", func(t *testing.T) {
		requireMailpit(t, starttlsAPI)

		service := newService(t, serviceURL(starttlsHost, username, "wrong-password",
			map[string]string{"encryption": "ExplicitTLS", "auth": "Plain"}))

		var replyErr *textproto.Error

		require.ErrorAs(t, service.Send("E2E Test: wrong password", nil), &replyErr)
		assert.Equal(t, 535, replyErr.Code, replyErr.Msg)
	})

	t.Run("no credentials", func(t *testing.T) {
		requireMailpit(t, starttlsAPI)

		service := newService(t, serviceURL(starttlsHost, "", "",
			map[string]string{"encryption": "ExplicitTLS", "auth": "None"}))

		var replyErr *textproto.Error

		require.ErrorAs(t, service.Send("E2E Test: no credentials", nil), &replyErr)
		assert.Equal(t, 530, replyErr.Code, replyErr.Msg)
	})
}

// TestSMTPMessage verifies that a delivered message carries the subject from
// the title param, the sender name, and an HTML part, and that a plus-tagged
// and a second recipient each receive their own message.
//
//nolint:paralleltest // Sends to a live server one message at a time.
func TestSMTPMessage(t *testing.T) {
	// base returns the query for the STARTTLS instance with the given values added.
	base := func(extra map[string]string) map[string]string {
		query := map[string]string{"encryption": "ExplicitTLS", "auth": "Plain"}
		maps.Copy(query, extra)

		return query
	}

	t.Run("subject from the title param", func(t *testing.T) {
		requireMailpit(t, starttlsAPI)

		subject := uniqueSubject("Shoutrrr E2E title param")
		service := newService(t, serviceURL(starttlsHost, username, password,
			base(map[string]string{"subject": "Configured subject"})))

		require.NoError(t, service.Send("E2E Test: title param", &types.Params{"title": subject}))

		assert.Equal(t, subject, fetchMessage(t, starttlsAPI, subject).Subject)
	})

	t.Run("sender name", func(t *testing.T) {
		requireMailpit(t, starttlsAPI)

		subject := uniqueSubject("Shoutrrr E2E sender name")
		service := newService(t, serviceURL(starttlsHost, username, password,
			base(map[string]string{"subject": subject, "fromname": "Shoutrrr E2E"})))

		require.NoError(t, service.Send("E2E Test: sender name", nil))

		message := fetchMessage(t, starttlsAPI, subject)
		assert.Equal(t, address{Name: "Shoutrrr E2E", Address: fromAddress}, message.From)
	})

	t.Run("HTML body", func(t *testing.T) {
		requireMailpit(t, starttlsAPI)

		subject := uniqueSubject("Shoutrrr E2E HTML")
		service := newService(t, serviceURL(starttlsHost, username, password,
			base(map[string]string{"subject": subject, "usehtml": "yes"})))

		require.NoError(t, service.Send("E2E Test: HTML body", nil))

		message := fetchMessage(t, starttlsAPI, subject)
		assert.Contains(t, message.HTML, "E2E Test: HTML body")
		assert.Contains(t, message.Text, "E2E Test: HTML body")
	})

	t.Run("plus-tagged and second recipients", func(t *testing.T) {
		requireMailpit(t, starttlsAPI)

		subject := uniqueSubject("Shoutrrr E2E recipients")
		service := newService(t, serviceURL(starttlsHost, username, password, base(map[string]string{
			"subject":     subject,
			"toaddresses": "recipient+alerts@example.com,other@example.com",
		})))

		require.NoError(t, service.Send("E2E Test: recipients", nil))

		// The service sends a separate message to each recipient.
		var recipients []address
		for _, message := range fetchMessages(t, starttlsAPI, subject, 2) {
			recipients = append(recipients, message.To...)
		}

		assert.ElementsMatch(t, []address{
			{Name: "", Address: "recipient+alerts@example.com"},
			{Name: "", Address: "other@example.com"},
		}, recipients)
	})
}
