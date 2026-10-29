package logging

import (
	"context"
	"log/slog"
	"sync"

	"github.com/nicholas-fedor/shoutrrr/internal/logbridge"
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
)

// Logger receives the warnings. The standard service logger implements it.
type Logger interface {
	// Slog returns the slog logger that receives the service's records.
	//
	// Returns:
	//   - *slog.Logger: the logger.
	Slog() *slog.Logger
}

// Once logs each warning at most once, at [slog.LevelWarn]. The source of each
// warning is the service's call to the Once method, for handlers that add the
// source. The zero value is ready to use, and a Once is safe for concurrent
// use. A Once must not be copied after first use.
type Once struct {
	// logged holds the keys of the warnings already logged.
	logged sync.Map
}

// Attribute keys for log records.
const (
	// KeyAlias names a room alias that a service resolved.
	KeyAlias = "alias"
	// KeyCode holds a reason code that a server returned.
	KeyCode = "code"
	// KeyDevice names the device a session logged in as.
	KeyDevice = "device"
	// KeyError holds the error that a record reports.
	KeyError = "error"
	// KeyField names a config field.
	KeyField = "field"
	// KeyFlow names a login flow.
	KeyFlow = "flow"
	// KeyHomeserver names the server that a login reported.
	KeyHomeserver = "homeserver"
	// KeyKey names a config key in a URL or in params.
	KeyKey = "key"
	// KeyLimit holds a size limit in bytes.
	KeyLimit = "limit"
	// KeyPort holds a port number.
	KeyPort = "port"
	// KeyReplacement names what replaces a deprecated field or key.
	KeyReplacement = "replacement"
	// KeyRoom names a chat room.
	KeyRoom = "room"
	// KeyServer names the host a service connects to.
	KeyServer = "server"
	// KeyService names the service that logged the record, by its ID.
	KeyService = "service"
	// KeyStatus holds the status of an HTTP response, such as "200 OK".
	KeyStatus = "status"
	// KeyTimestamp holds a timestamp that a server returned.
	KeyTimestamp = "timestamp"
	// KeyTopic names a topic that a message was published to.
	KeyTopic = "topic"
	// KeyWarning holds a warning that a server returned.
	KeyWarning = "warning"
)

// Messages of the warnings.
const (
	// insecureWarning is logged when a send skips TLS certificate
	// verification.
	insecureWarning = "skipping TLS certificate verification"
	// plaintextWarning is logged when a send does not use TLS.
	plaintextWarning = "sending without TLS"
	// deprecatedFieldWarning is logged when a deprecated config field is set.
	deprecatedFieldWarning = "config field is deprecated"
	// deprecatedKeyWarning is logged when a URL or params use a deprecated
	// config key.
	deprecatedKeyWarning = "config key is deprecated"
)

// DeprecatedFields logs a warning for each deprecated config field, once per
// field. The warning names the field and its replacement.
//
// Parameters:
//   - ctx: the context of the call, passed to the log handler.
//   - logger: the service's logger.
//   - deprecations: the deprecated fields that are set, such as those that
//     [format.SyncDeprecatedFields] reports.
func (o *Once) DeprecatedFields(ctx context.Context, logger Logger, deprecations []format.Deprecation) {
	pc := logbridge.CallerPC(1)

	for _, deprecation := range deprecations {
		o.Do("field:"+deprecation.Name, func() {
			logbridge.Log(ctx, logger.Slog(), pc, slog.LevelWarn, deprecatedFieldWarning,
				KeyField, deprecation.Name, KeyReplacement, deprecation.Replacement)
		})
	}
}

// DeprecatedKeys logs a warning for each deprecated config key, once per key.
// The warning names the key and its replacement.
//
// Parameters:
//   - ctx: the context of the call, passed to the log handler.
//   - logger: the service's logger.
//   - deprecations: the deprecated keys that a URL or params used, such as
//     those that [format.PropKeyResolver.DeprecatedKeys] reports.
func (o *Once) DeprecatedKeys(ctx context.Context, logger Logger, deprecations []format.Deprecation) {
	pc := logbridge.CallerPC(1)

	for _, deprecation := range deprecations {
		o.Do("key:"+deprecation.Name, func() {
			logbridge.Log(ctx, logger.Slog(), pc, slog.LevelWarn, deprecatedKeyWarning,
				KeyKey, deprecation.Name, KeyReplacement, deprecation.Replacement)
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
//   - ctx: the context of the call, passed to the log handler.
//   - logger: the service's logger.
func (o *Once) Insecure(ctx context.Context, logger Logger) {
	pc := logbridge.CallerPC(1)

	o.Do("insecure", func() { logbridge.Log(ctx, logger.Slog(), pc, slog.LevelWarn, insecureWarning) })
}

// Plaintext logs, once, that a send does not use TLS, so its connection and
// any credentials it carries are not encrypted.
//
// Parameters:
//   - ctx: the context of the call, passed to the log handler.
//   - logger: the service's logger.
func (o *Once) Plaintext(ctx context.Context, logger Logger) {
	pc := logbridge.CallerPC(1)

	o.Do("plaintext", func() { logbridge.Log(ctx, logger.Slog(), pc, slog.LevelWarn, plaintextWarning) })
}

// Reset forgets the logged warnings, so each can be logged again.
func (o *Once) Reset() {
	o.logged.Clear()
}
