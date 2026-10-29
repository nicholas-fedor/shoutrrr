// Package logging holds the warnings that notification services log about
// their settings, and the keys of the attributes that services log.
//
// A service keeps one [Once] and logs through it the warnings for skipped TLS
// certificate verification, for connections without TLS, and for deprecated
// config keys and fields. This keeps each warning's message the same in every
// service, and keeps it from repeating on every send. Resetting the Once, as
// Initialize does, lets each warning appear again.
package logging
