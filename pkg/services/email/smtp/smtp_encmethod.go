package smtp

import (
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// encMethod is an SMTP transport encryption method.
type encMethod int

// encMethodVals holds named encryption methods and their enum formatter.
type encMethodVals struct {
	// None means no encryption.
	None encMethod
	// ExplicitTLS means that TLS is initiated with STARTTLS.
	ExplicitTLS encMethod
	// ImplicitTLS means that TLS is used for the whole session.
	ImplicitTLS encMethod
	// Auto means implicit TLS on port [ImplicitTLSPort], otherwise explicit TLS when supported.
	Auto encMethod
	// Enum formats and parses encryption method values.
	Enum types.EnumFormatter
}

const (
	// EncNone represents no encryption.
	EncNone encMethod = iota
	// EncExplicitTLS represents explicit TLS initiated with STARTTLS.
	EncExplicitTLS
	// EncImplicitTLS represents implicit TLS used throughout the session.
	EncImplicitTLS
	// EncAuto represents automatic TLS selection based on port.
	EncAuto
)

// ImplicitTLSPort is the de facto standard SMTPS port for implicit TLS.
const ImplicitTLSPort = 465

// EncMethods is the enum helper for populating the [Config.Encryption] field.
var EncMethods = &encMethodVals{
	None:        EncNone,
	ExplicitTLS: EncExplicitTLS,
	ImplicitTLS: EncImplicitTLS,
	Auto:        EncAuto,

	Enum: format.CreateEnumFormatter(
		[]string{
			"None",
			"ExplicitTLS",
			"ImplicitTLS",
			"Auto",
		},
	),
}

// String returns the encryption method name.
//
// Returns:
//   - The canonical name of the encryption method, such as "Auto" or "ImplicitTLS".
func (em encMethod) String() string {
	return EncMethods.Enum.Print(int(em))
}

// useImplicitTLS reports whether implicit TLS should be used.
//
// Parameters:
//   - encryption: The configured encryption method.
//   - port: The SMTP server port.
//
// Returns:
//   - true when [EncImplicitTLS] is selected, or when [EncAuto] is selected and port is [ImplicitTLSPort].
func useImplicitTLS(encryption encMethod, port uint16) bool {
	switch encryption {
	case EncNone:
		return false
	case EncExplicitTLS:
		return false
	case EncImplicitTLS:
		return true
	case EncAuto:
		return port == ImplicitTLSPort
	default:
		// Unreachable due to enum constraints, but included for safety.
		return false
	}
}
