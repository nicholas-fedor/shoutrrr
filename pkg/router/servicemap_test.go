package router

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
