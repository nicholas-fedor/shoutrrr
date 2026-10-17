package ntfy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestPriorityString verifies the name of each priority, which the service
// sends as the Priority header.
func TestPriorityString(t *testing.T) {
	t.Parallel()

	// tests lists priorities with their names.
	tests := []struct {
		want     string
		priority priority
	}{
		{priority: PriorityMin, want: "Min"},
		{priority: PriorityLow, want: "Low"},
		{priority: PriorityDefault, want: "Default"},
		{priority: PriorityHigh, want: "High"},
		{priority: PriorityMax, want: "Max"},
		{priority: priority(0), want: ""},
		{priority: priority(9), want: "Invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tt.priority.String())
		})
	}
}

// TestPriorityEnum verifies that the priority enum parses the names, the
// numbers 1 to 5, and the urgent alias, matching ntfy's priority values.
func TestPriorityEnum(t *testing.T) {
	t.Parallel()

	assert.Equal(t, &priorityVals{
		Min: PriorityMin, Low: PriorityLow, Default: PriorityDefault, High: PriorityHigh, Max: PriorityMax, Enum: Priority.Enum,
	}, Priority)

	for value, want := range map[string]priority{
		"min": PriorityMin, "Low": PriorityLow, "DEFAULT": PriorityDefault, "high": PriorityHigh, "max": PriorityMax,
		"1": PriorityMin, "2": PriorityLow, "3": PriorityDefault, "4": PriorityHigh, "5": PriorityMax, "urgent": PriorityMax,
	} {
		assert.Equal(t, int(want), Priority.Enum.Parse(value), value)
	}
}
