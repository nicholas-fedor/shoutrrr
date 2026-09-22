package types

import "time"

// ServiceTimeout is implemented by a service that declares its own send budget.
//
// Embedding standard.Standard reports [DefaultSendTimeout]. A longer result extends
// the router wait when [SenderOptions.Timeout] is unset. A non-positive result does not.
type ServiceTimeout interface {
	// ServiceTimeout returns the budget for one send.
	//
	// Parameters:
	//   - params: Optional send-time overrides, applied the same way as Send.
	//
	// Returns:
	//   - The budget. Non-positive does not extend the router wait.
	ServiceTimeout(params *Params) time.Duration
}

// DefaultSendTimeout is the send budget used when a service does not report a longer one.
const DefaultSendTimeout = 10 * time.Second
