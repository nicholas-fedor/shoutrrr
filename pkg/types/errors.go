package types

import "errors"

// ErrServiceNotInitialized indicates a send on a service whose Initialize has
// not run, so the service has no configuration to send with.
var ErrServiceNotInitialized = errors.New("service not initialized")
