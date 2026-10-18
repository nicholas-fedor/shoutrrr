package standard

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// TestStandardServiceTimeout verifies that Standard reports the default send
// timeout, whatever the params, and that embedding it gives a service that
// budget.
func TestStandardServiceTimeout(t *testing.T) {
	t.Parallel()

	assert.Equal(t, types.DefaultSendTimeout, Standard{}.ServiceTimeout(nil))
	assert.Equal(t, types.DefaultSendTimeout, Standard{}.ServiceTimeout(&types.Params{"timeout": "1s"}))

	var budget types.ServiceTimeout = &struct{ Standard }{}

	assert.Equal(t, types.DefaultSendTimeout, budget.ServiceTimeout(nil))
}
