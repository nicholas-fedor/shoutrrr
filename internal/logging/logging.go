package logging

import (
	"fmt"
	"sync"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
)

// Logger receives the warnings. The standard service logger implements it.
type Logger interface {
	// Log writes v to the service's logger.
	Log(v ...any)
}

// Once logs each warning at most once. The zero value is ready to use, and a
// Once is safe for concurrent use. A Once must not be copied after first use.
type Once struct {
	// logged holds the keys of the warnings already logged.
	logged sync.Map
}

// Warnings about a service's connections.
const (
	// insecureWarning is logged when a send skips TLS certificate
	// verification.
	insecureWarning = "Warning: skipping TLS certificate verification"
	// plaintextWarning is logged when a send does not use TLS.
	plaintextWarning = "Warning: sending without TLS"
)

// DeprecatedFields logs a warning for each deprecated config field, once per
// field.
//
// Parameters:
//   - logger: the service's logger.
//   - deprecations: the deprecated fields that are set, such as those that
//     [format.SyncDeprecatedFields] reports.
func (o *Once) DeprecatedFields(logger Logger, deprecations []format.Deprecation) {
	for _, deprecation := range deprecations {
		o.Do("field:"+deprecation.Name, func() {
			logger.Log(fmt.Sprintf(
				"Warning: config field %s is deprecated, use %s instead",
				deprecation.Name, deprecation.Replacement,
			))
		})
	}
}

// DeprecatedKeys logs a warning for each deprecated config key, once per key.
//
// Parameters:
//   - logger: the service's logger.
//   - deprecations: the deprecated keys that a URL or params used, such as
//     those that [format.PropKeyResolver.DeprecatedKeys] reports.
func (o *Once) DeprecatedKeys(logger Logger, deprecations []format.Deprecation) {
	for _, deprecation := range deprecations {
		o.Do("key:"+deprecation.Name, func() {
			logger.Log(fmt.Sprintf(
				"Warning: key %q is deprecated, use %q instead",
				deprecation.Name, deprecation.Replacement,
			))
		})
	}
}

// Do calls warn unless a warning with the same key was logged before. It
// suits a warning that only one service logs.
//
// Parameters:
//   - key: identifies the warning.
//   - warn: logs the warning.
func (o *Once) Do(key string, warn func()) {
	if _, loaded := o.logged.LoadOrStore(key, struct{}{}); !loaded {
		warn()
	}
}

// Insecure logs, once, that a send skips TLS certificate verification, so its
// connection can be intercepted. A service does not log it for an injected
// client, whose owner controls its TLS settings.
//
// Parameters:
//   - logger: the service's logger.
func (o *Once) Insecure(logger Logger) {
	o.Do("insecure", func() { logger.Log(insecureWarning) })
}

// Plaintext logs, once, that a send does not use TLS, so its connection and
// any credentials it carries are not encrypted.
//
// Parameters:
//   - logger: the service's logger.
func (o *Once) Plaintext(logger Logger) {
	o.Do("plaintext", func() { logger.Log(plaintextWarning) })
}

// Reset forgets the logged warnings, so each can be logged again.
func (o *Once) Reset() {
	o.logged.Clear()
}
