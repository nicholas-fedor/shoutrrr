package router

import (
	"fmt"
	"io"
	"net/http"
	"reflect"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/types/mocks"
)

// capability pairs a method name the router dispatches on with the interface it
// type-asserts for that method.
type capability struct {
	// method is the method name a service defines for the capability.
	method string
	// iface is the interface the router checks for.
	iface reflect.Type
}

// capabilities lists every optional method the router uses, with the interface
// whose exact signature a service must match for the router to call it.
var capabilities = []capability{
	{method: "SendContext", iface: reflect.TypeFor[types.ContextSender]()},
	{method: "SendItems", iface: reflect.TypeFor[types.RichSender]()},
	{method: "SendItemsContext", iface: reflect.TypeFor[types.ContextAttachmentSender]()},
	{method: "SetHTTPClient", iface: reflect.TypeFor[types.HTTPClientSetter]()},
	{method: "SetDialContext", iface: reflect.TypeFor[types.DialContextSetter]()},
	{method: "ServiceTimeout", iface: reflect.TypeFor[types.ServiceTimeout]()},
	{method: "Close", iface: reflect.TypeFor[io.Closer]()},
}

// capabilityExemptions lists methods whose signatures deliberately differ from the
// router's interface. Each is deprecated and kept for API compatibility until v1.
var capabilityExemptions = map[string]map[string]string{
	"discord": {"SendItems": "deprecated pointer-param form; SendItemsContext serves the router"},
	"bark":    {"SendItems": "deprecated pointer-param form; the router sends Bark plain text"},
}

// TestServiceCapabilitiesMatchRouterInterfaces verifies that a service method named
// after a router capability has the signature the router type-asserts. A mismatch
// compiles but silently drops the capability, as happened with Discord's SendItems,
// which the router never called.
func TestServiceCapabilitiesMatchRouterInterfaces(t *testing.T) {
	t.Parallel()

	for scheme, newService := range serviceMap {
		t.Run(scheme, func(t *testing.T) {
			t.Parallel()

			serviceType := reflect.TypeOf(newService())

			for _, capability := range capabilities {
				if _, defined := serviceType.MethodByName(capability.method); !defined {
					continue
				}

				if reason, exempt := capabilityExemptions[scheme][capability.method]; exempt {
					t.Logf("%s.%s is exempt: %s", scheme, capability.method, reason)

					continue
				}

				assert.True(
					t,
					serviceType.Implements(capability.iface),
					"%s defines %s but does not implement %s, so the router never calls it",
					scheme, capability.method, capability.iface,
				)
			}
		})
	}
}

// TestSendItemsDeliversRichDiscordPayload verifies that ServiceRouter.SendItems
// reaches Discord's rich path, so item fields become embed fields instead of the
// plain-text fallback.
func TestSendItemsDeliversRichDiscordPayload(t *testing.T) {
	t.Parallel()

	var body string

	httpClient := mocks.NewMockHTTPClient(t)
	httpClient.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
		raw, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, fmt.Errorf("reading request body: %w", err)
		}

		body = string(raw)

		return &http.Response{StatusCode: http.StatusNoContent, Body: http.NoBody}, nil
	}).Once()

	serviceRouter, err := NewWithOptions(
		nil,
		types.SenderOptions{HTTPClient: httpClient},
		"discord://token@123456789012345678",
	)
	require.NoError(t, err)

	items := []types.MessageItem{{
		Text:   "deploy finished",
		Fields: []types.Field{{Key: "Environment", Value: "production"}},
	}}

	for _, sendErr := range serviceRouter.SendItems(items, types.Params{}) {
		require.NoError(t, sendErr)
	}

	assert.Contains(t, body, `"fields"`)
	assert.Contains(t, body, "Environment")
	assert.Contains(t, body, "production")
}

// TestRouterDeadlineCancelsContextSenderRequest verifies that the router's send
// budget reaches every service that implements types.ContextSender: when the
// budget expires, the service's in-flight request is canceled instead of left
// running. Under synctest, a request left running fails the test as a deadlock.
// Matrix also bounds each request with its own timeout, so this test cannot tell
// whether matrix received the router's context. The capability test covers its
// dispatch.
func TestRouterDeadlineCancelsContextSenderRequest(t *testing.T) {
	t.Parallel()

	serviceURLs := map[string]string{
		"discord":   "discord://token@123456789012345678",
		"matrix":    "matrix://:token@matrix.example.com/?rooms=!room:example.com",
		"pagerduty": "pagerduty://events.example.com/5ec7e75ec7e75ec7e75ec7e75ec7e700",
		"zulip":     "zulip://bot%40example.com:key@zulip.example.com/?stream=foo&topic=bar",
	}

	for scheme, serviceURL := range serviceURLs {
		t.Run(scheme, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				var requests atomic.Int32

				httpClient := mocks.NewMockHTTPClient(t)
				httpClient.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
					requests.Add(1)
					<-req.Context().Done()

					return nil, fmt.Errorf("request canceled: %w", req.Context().Err())
				}).Maybe()

				serviceRouter, err := NewWithOptions(nil, types.SenderOptions{HTTPClient: httpClient}, serviceURL)
				require.NoError(t, err)

				errs := serviceRouter.Send("message", nil)
				require.Len(t, errs, 1)
				require.Error(t, errs[0])

				// Every request goroutine must have ended with the canceled context.
				synctest.Wait()

				require.Positive(t, requests.Load(), "the service must have sent a request for the deadline to cancel")
			})
		})
	}
}
