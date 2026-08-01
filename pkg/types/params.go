package types

// Params is the string map used to provide additional variables to the service templates.
type Params map[string]string

const (
	// TitleKey is the common key for the title prop.
	TitleKey = "title"
	// MessageKey is the common key for the message prop.
	MessageKey = "message"
	// LevelKey is the common key for the message level prop.
	LevelKey = "level"
)

// Level returns the MessageLevel stored under the "level" param.
//
// Returns:
//   - MessageLevel: the parsed level.
//   - bool: true if the level was found and parsed successfully.
func (p Params) Level() (MessageLevel, bool) {
	levelStr, found := p[LevelKey]
	if !found {
		return Unknown, false
	}

	return ParseMessageLevel(levelStr)
}

// SetLevel sets the "level" param to the string representation of the MessageLevel.
//
// Parameters:
//   - level: the message level to set.
func (p Params) SetLevel(level MessageLevel) {
	p[LevelKey] = level.String()
}

// SetMessage sets the "message" param to the specified value.
//
// Parameters:
//   - message: the message text to set.
func (p Params) SetMessage(message string) {
	p[MessageKey] = message
}

// SetTitle sets the "title" param to the specified value.
//
// Parameters:
//   - title: the title text to set.
func (p Params) SetTitle(title string) {
	p[TitleKey] = title
}

// Title returns the "title" param.
//
// Returns:
//   - string: the title value.
//   - bool: true if the title was found.
func (p Params) Title() (string, bool) {
	title, found := p[TitleKey]

	return title, found
}
