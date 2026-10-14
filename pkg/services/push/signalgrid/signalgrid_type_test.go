package signalgrid

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
)

// TestNotificationTypeString verifies that each notification type prints as
// the value the Push API expects.
func TestNotificationTypeString(t *testing.T) {
	t.Parallel()

	// tests lists notification types with the value each should print.
	tests := []struct {
		want  string
		value notificationType
	}{
		{value: NotificationType.INFO, want: "INFO"},
		{value: NotificationType.WARN, want: "WARN"},
		{value: NotificationType.SUCCESS, want: "SUCCESS"},
		{value: NotificationType.CRIT, want: "CRIT"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tt.value.String())
		})
	}
}

// TestNotificationTypeEnumParse verifies that the enum formatter accepts the
// printed names and the lowercase aliases, and rejects other values.
func TestNotificationTypeEnumParse(t *testing.T) {
	t.Parallel()

	// tests lists strings with the notification type each should parse to.
	tests := []struct {
		input string
		want  int
	}{
		{input: "INFO", want: int(TypeINFO)},
		{input: "warn", want: int(TypeWARN)},
		{input: "success", want: int(TypeSUCCESS)},
		{input: "crit", want: int(TypeCRIT)},
		{input: "nope", want: format.EnumInvalid},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, NotificationType.Enum.Parse(tt.input))
		})
	}
}
