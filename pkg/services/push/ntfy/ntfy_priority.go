package ntfy

import (
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// priority is an ntfy message priority, from 1 for min to 5 for max.
type priority int

// priorityVals holds the priority values and their formatter.
type priorityVals struct {
	// Enum reads and prints the priority names.
	Enum types.EnumFormatter
	// Min is the lowest priority, shown silently under other notifications.
	Min priority
	// Low is shown silently, only once the notification drawer is opened.
	Low priority
	// Default is shown with a short vibration and the default sound.
	Default priority
	// High is shown with a long vibration burst and a pop-over notification.
	High priority
	// Max is the highest priority, shown with very long vibration bursts and
	// a pop-over notification.
	Max priority
}

// Priority levels as constants.
const (
	// PriorityMin is priority 1.
	PriorityMin priority = 1
	// PriorityLow is priority 2.
	PriorityLow priority = 2
	// PriorityDefault is priority 3.
	PriorityDefault priority = 3
	// PriorityHigh is priority 4.
	PriorityHigh priority = 4
	// PriorityMax is priority 5, also called urgent.
	PriorityMax priority = 5
)

// Priority defines the notification priority levels.
var Priority = &priorityVals{
	Min:     PriorityMin,
	Low:     PriorityLow,
	Default: PriorityDefault,
	High:    PriorityHigh,
	Max:     PriorityMax,
	Enum: format.CreateEnumFormatter(
		[]string{
			"",
			"Min",
			"Low",
			"Default",
			"High",
			"Max",
		}, map[string]int{
			"1":      int(PriorityMin),
			"2":      int(PriorityLow),
			"3":      int(PriorityDefault),
			"4":      int(PriorityHigh),
			"5":      int(PriorityMax),
			"urgent": int(PriorityMax),
		},
	),
}

// levelPriorities maps each known message level to the ntfy priority it is
// sent with when the send keeps the default priority.
var levelPriorities = map[types.MessageLevel]priority{
	types.Debug:   PriorityLow,
	types.Info:    PriorityDefault,
	types.Warning: PriorityHigh,
	types.Error:   PriorityMax,
}

// String returns the priority's name.
//
// Returns:
//   - string: the name, such as Default.
func (p priority) String() string {
	return Priority.Enum.Print(int(p))
}
