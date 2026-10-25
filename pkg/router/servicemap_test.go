package router

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// errPanicked marks an error built from a recovered panic.
var errPanicked = errors.New("panicked")

// TestNewServiceForEveryScheme verifies that every listed scheme resolves to a
// service.
func TestNewServiceForEveryScheme(t *testing.T) {
	t.Parallel()

	schemes := (&ServiceRouter{}).ListServices()
	require.NotEmpty(t, schemes)

	for _, scheme := range schemes {
		t.Run(scheme, func(t *testing.T) {
			t.Parallel()

			service, err := newService(scheme)
			require.NoError(t, err)
			assert.NotNil(t, service)
		})
	}
}

// TestUninitializedServiceSends verifies that every send entry of every
// service reports [types.ErrServiceNotInitialized], without panicking, when
// Initialize has not run. ServiceTimeout must not panic either, because a
// caller can ask for the budget first.
func TestUninitializedServiceSends(t *testing.T) {
	t.Parallel()

	items := []types.MessageItem{{Text: "message"}}

	for _, scheme := range (&ServiceRouter{}).ListServices() {
		t.Run(scheme, func(t *testing.T) {
			t.Parallel()

			service, err := newService(scheme)
			require.NoError(t, err)

			// sends lists the send entries the service implements.
			sends := map[string]func() error{
				"Send": func() error { return service.Send("message", nil) },
			}

			if sender, ok := service.(types.ContextSender); ok {
				sends["SendContext"] = func() error { return sender.SendContext(t.Context(), "message", nil) }
			}

			if sender, ok := service.(types.RichSender); ok {
				sends["SendItems"] = func() error { return sender.SendItems(items, nil) }
			}

			if sender, ok := service.(types.ContextAttachmentSender); ok {
				sends["SendItemsContext"] = func() error { return sender.SendItemsContext(t.Context(), items, nil) }
			}

			for name, send := range sends {
				require.ErrorIs(t, callRecovering(send), types.ErrServiceNotInitialized, name)
			}

			if reporter, ok := service.(types.ServiceTimeout); ok {
				require.NoError(t, callRecovering(func() error {
					_ = reporter.ServiceTimeout(nil)

					return nil
				}), "ServiceTimeout")
			}
		})
	}
}

// callRecovering calls f and turns a panic into an error.
//
// Parameters:
//   - f: the function to call.
//
// Returns:
//   - error: the error f returned, or an error describing its panic.
func callRecovering(f func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%w: %v", errPanicked, r)
		}
	}()

	return f()
}
