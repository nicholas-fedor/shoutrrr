package logging

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"

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
	// logged holds the keys of the [Once.Do] warnings already logged.
	logged sync.Map
	// fields holds the names of the deprecated fields already warned about.
	fields sync.Map
	// keys holds the names of the deprecated keys already warned about.
	keys sync.Map
	// insecure is true once the insecure warning was logged.
	insecure atomic.Bool
	// plaintext is true once the plaintext warning was logged.
	plaintext atomic.Bool
}

// Attribute keys for log records.
const (
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
	logDeprecations(ctx, logger, &o.fields, deprecatedFieldWarning, KeyField, deprecations)
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
	logDeprecations(ctx, logger, &o.keys, deprecatedKeyWarning, KeyKey, deprecations)
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
	// A repeated call returns after reading the flag, without a function call.
	if !o.insecure.Load() {
		logOnce(ctx, logger, &o.insecure, insecureWarning)
	}
}

// Plaintext logs, once, that a send does not use TLS, so its connection and
// any credentials it carries are not encrypted.
//
// Parameters:
//   - ctx: the context of the call, passed to the log handler.
//   - logger: the service's logger.
func (o *Once) Plaintext(ctx context.Context, logger Logger) {
	// A repeated call returns after reading the flag, without a function call.
	if !o.plaintext.Load() {
		logOnce(ctx, logger, &o.plaintext, plaintextWarning)
	}
}

// Reset forgets the logged warnings, so each can be logged again.
func (o *Once) Reset() {
	o.logged.Clear()
	o.fields.Clear()
	o.keys.Clear()
	o.insecure.Store(false)
	o.plaintext.Store(false)
}

// logOnce logs msg unless logged is already true, and sets it. Of concurrent
// first calls, only the one that sets logged logs. The warning's source is the
// service's call to the Once method that calls logOnce.
//
// Parameters:
//   - ctx: the context of the call, passed to the log handler.
//   - logger: the service's logger.
//   - logged: whether this warning was logged.
//   - msg: the warning message.
func logOnce(ctx context.Context, logger Logger, logged *atomic.Bool, msg string) {
	if !logged.CompareAndSwap(false, true) {
		return
	}

	// Two calls up is the service's call to the Once method.
	logbridge.Log(ctx, logger.Slog(), 2, slog.LevelWarn, msg) //nolint:mnd // Explained above.
}

// logDeprecations logs a warning for each deprecation that logged has not
// recorded yet, and records it. The warning names the deprecated name and its
// replacement. Its source is the service's call to the Once method that calls
// logDeprecations.
//
// Parameters:
//   - ctx: the context of the call, passed to the log handler.
//   - logger: the service's logger.
//   - logged: the names already warned about, for one kind of deprecation.
//   - msg: the warning message.
//   - nameKey: the attribute key of the deprecated name.
//   - deprecations: the deprecations to warn about.
func logDeprecations(
	ctx context.Context,
	logger Logger,
	logged *sync.Map,
	msg, nameKey string,
	deprecations []format.Deprecation,
) {
	for _, deprecation := range deprecations {
		// Load does not keep its key, so a name warned about before costs no
		// allocation. LoadOrStore then decides the race between new warnings.
		if _, warned := logged.Load(deprecation.Name); warned {
			continue
		}

		if _, loaded := logged.LoadOrStore(deprecation.Name, struct{}{}); !loaded {
			// Two calls up is the service's call to the Once method.
			logbridge.LogAttrs(ctx, logger.Slog(), 2, slog.LevelWarn, msg, //nolint:mnd // Explained above.
				slog.String(nameKey, deprecation.Name), slog.String(KeyReplacement, deprecation.Replacement))
		}
	}
}
