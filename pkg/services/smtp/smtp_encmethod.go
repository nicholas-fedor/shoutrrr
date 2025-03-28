package smtp

import (
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

const (
	// EncNone represents no encryption.
	EncNone encMethod = iota // 0
	// EncExplicitTLS represents explicit TLS initiated with StartTLS.
	EncExplicitTLS // 1
	// EncImplicitTLS represents implicit TLS used throughout the session.
	EncImplicitTLS // 2
	// EncAuto represents automatic TLS selection based on port.
	EncAuto // 3
	// ImplicitTLSPort is the de facto standard SMTPS port for implicit TLS.
	ImplicitTLSPort = 465
)

// EncMethods is the enum helper for populating the Encryption field.
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
		}),
}

type encMethod int

type encMethodVals struct {
	// None means no encryption
	None encMethod
	// ExplicitTLS means that TLS needs to be initiated by using StartTLS
	ExplicitTLS encMethod
	// ImplicitTLS means that TLS is used for the whole session
	ImplicitTLS encMethod
	// Auto means that TLS will be implicitly used for port 465, otherwise explicit TLS will be used if supported
	Auto encMethod

	// Enum is the EnumFormatter instance for EncMethods
	Enum types.EnumFormatter
}

func (at encMethod) String() string {
	return EncMethods.Enum.Print(int(at))
}

// useImplicitTLS determines if implicit TLS should be used based on encryption method and port.
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
		return false // Unreachable due to enum constraints, but included for safety
	}
}
