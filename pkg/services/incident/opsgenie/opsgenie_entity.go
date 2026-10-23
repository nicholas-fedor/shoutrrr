package opsgenie

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Entity represents an OpsGenie entity (e.g., user, team) with type and identifier.
// Example JSON: { "username":"trinity@opsgenie.com", "type":"user" }.
type Entity struct {
	// Type is the entity type: team, user, escalation, or schedule.
	Type string `json:"type"`
	// ID is the entity's Opsgenie ID, left out when empty.
	ID string `json:"id,omitempty"`
	// Name is the name of a team, escalation, or schedule, left out when
	// empty.
	Name string `json:"name,omitempty"`
	// Username is a user's username, left out when empty.
	Username string `json:"username,omitempty"`
}

// Entity string format.
const (
	// EntityPartsCount is the expected number of parts in an entity string (type:identifier).
	EntityPartsCount = 2
)

// Errors returned when reading or writing an entity.
var (
	// ErrInvalidEntityFormat indicates that the entity string does not have two elements separated by a colon.
	ErrInvalidEntityFormat = errors.New(
		"invalid entity, should have two elements separated by colon",
	)
	// ErrInvalidEntityIDName indicates an identifier that could not be checked
	// against the Opsgenie ID format.
	ErrInvalidEntityIDName = errors.New("invalid entity, cannot parse id/name")
	// ErrUnexpectedEntityType indicates an entity type that cannot take the
	// identifier, or that the field does not accept.
	ErrUnexpectedEntityType = errors.New("invalid entity, unexpected entity type")
	// ErrMissingEntityIdentity indicates an entity without an ID, name, or
	// username.
	ErrMissingEntityIdentity = errors.New("invalid entity, should have either ID, name or username")
)

// GetPropValue serializes an entity back into a string in the format "type:identifier".
//
// Returns:
//   - string: the entity string, with the ID, the name, or the username as
//     the identifier, in that order of preference.
//   - error: [ErrMissingEntityIdentity] for an entity without an identifier.
func (e *Entity) GetPropValue() (string, error) {
	var identifier string

	switch {
	case e.ID != "":
		identifier = e.ID
	case e.Name != "":
		identifier = e.Name
	case e.Username != "":
		identifier = e.Username
	default:
		return "", ErrMissingEntityIdentity
	}

	return fmt.Sprintf("%s:%s", e.Type, identifier), nil
}

// SetFromProp deserializes an entity from a string in the format "type:identifier".
//
// An identifier in the OpsGenie ID format is read as the ID for any type.
// Otherwise it is the username of a user, or the name of a team, escalation,
// or schedule.
//
// Parameters:
//   - propValue: The entity in the format "type:identifier".
//
// Returns:
//   - error: [ErrInvalidEntityFormat] without exactly one colon,
//     [ErrMissingEntityIdentity] without an identifier, which Opsgenie requires
//     for every entity, or [ErrUnexpectedEntityType] for a name with another
//     type.
func (e *Entity) SetFromProp(propValue string) error {
	elements := strings.Split(propValue, ":")

	if len(elements) != EntityPartsCount {
		return fmt.Errorf("%w: %q", ErrInvalidEntityFormat, propValue)
	}

	e.Type = elements[0]
	identifier := elements[1]

	if identifier == "" {
		return fmt.Errorf("%w: %q", ErrMissingEntityIdentity, propValue)
	}

	isID, err := isOpsGenieID(identifier)
	if err != nil {
		return fmt.Errorf("%w: %q", ErrInvalidEntityIDName, identifier)
	}

	switch {
	case isID:
		e.ID = identifier
	case e.Type == "user":
		e.Username = identifier
	case e.Type == "team", e.Type == "escalation", e.Type == "schedule":
		e.Name = identifier
	default:
		return fmt.Errorf("%w: %q", ErrUnexpectedEntityType, e.Type)
	}

	return nil
}

// isOpsGenieID checks if a string matches the OpsGenie ID format (e.g., 4513b7ea-3b91-438f-b7e4-e3e54af9147c).
//
// Parameters:
//   - str: the identifier to check.
//
// Returns:
//   - bool: true when str is in the ID format.
//   - error: the error from compiling the pattern.
func isOpsGenieID(str string) (bool, error) {
	matched, err := regexp.MatchString(
		`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`,
		str,
	)
	if err != nil {
		return false, fmt.Errorf("matching OpsGenie ID format for %q: %w", str, err)
	}

	return matched, nil
}
