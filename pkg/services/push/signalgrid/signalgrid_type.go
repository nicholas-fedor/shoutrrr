package signalgrid

import (
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// notificationType is the visual severity of a Signalgrid notification.
type notificationType int

// notificationTypeVals holds the named severity values and their enum formatter.
type notificationTypeVals struct {
	// INFO is an informational event.
	INFO notificationType
	// WARN is a warning that may require attention.
	WARN notificationType
	// SUCCESS is a successful or positive result.
	SUCCESS notificationType
	// CRIT is a critical incident such as an outage.
	CRIT notificationType
	// Enum formats and parses notification type values.
	Enum types.EnumFormatter
}

const (
	// TypeINFO is an informational notification.
	TypeINFO notificationType = iota
	// TypeWARN is a warning notification.
	TypeWARN
	// TypeSUCCESS is a successful-operation notification.
	TypeSUCCESS
	// TypeCRIT is a critical-severity notification.
	TypeCRIT
)

// NotificationType defines Signalgrid Push API severity values.
var NotificationType = &notificationTypeVals{
	INFO:    TypeINFO,
	WARN:    TypeWARN,
	SUCCESS: TypeSUCCESS,
	CRIT:    TypeCRIT,
	Enum: format.CreateEnumFormatter(
		[]string{
			"INFO",
			"WARN",
			"SUCCESS",
			"CRIT",
		},
		map[string]int{
			"info":    int(TypeINFO),
			"warn":    int(TypeWARN),
			"success": int(TypeSUCCESS),
			"crit":    int(TypeCRIT),
		},
	),
}

// String returns the canonical Push API value for this notification type.
//
// Returns:
//   - One of INFO, WARN, SUCCESS, or CRIT.
func (t notificationType) String() string {
	return NotificationType.Enum.Print(int(t))
}
