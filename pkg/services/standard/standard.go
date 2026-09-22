package standard

import (
	"time"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Standard implements the Logger and Templater parts of the Service interface.
type Standard struct {
	Logger
	Templater
}

// ServiceTimeout reports [types.DefaultSendTimeout].
//
// A service with a longer limit overrides this method.
//
// Parameters:
//   - params: Unused. The default budget does not depend on send parameters.
//
// Returns:
//   - [types.DefaultSendTimeout].
func (Standard) ServiceTimeout(*types.Params) time.Duration {
	return types.DefaultSendTimeout
}
