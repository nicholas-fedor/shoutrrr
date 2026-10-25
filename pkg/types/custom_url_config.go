package types

import "net/url"

// CustomURLService is the interface that needs to be implemented to support custom URLs in services.
type CustomURLService interface {
	// Service is the notification service that the custom URL configures.
	Service
	// GetServiceURLFromCustom converts a custom URL into the service URL that
	// [Service.Initialize] accepts.
	//
	// The router calls it when the URL scheme extends the service scheme with a
	// plus sign, such as "generic+https".
	//
	// Parameters:
	//   - customURL: the custom URL, including its extended scheme.
	//
	// Returns:
	//   - serviceURL: the service URL. The router treats nil as a failure.
	//   - err: the conversion failure, or nil on success.
	GetServiceURLFromCustom(customURL *url.URL) (serviceURL *url.URL, err error)
}
