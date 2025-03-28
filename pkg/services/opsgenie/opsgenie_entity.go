package opsgenie

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// EntityPartsCount is the expected number of parts in an entity string (type:identifier).
const (
	EntityPartsCount = 2 // Expected number of parts in an entity string (type:identifier)
)

// ErrInvalidEntityFormat indicates that the entity string does not have two elements separated by a colon.
var (
	ErrInvalidEntityFormat = errors.New(
		"invalid entity, should have two elements separated by colon",
	)
	ErrInvalidEntityIDName   = errors.New("invalid entity, cannot parse id/name")
	ErrUnexpectedEntityType  = errors.New("invalid entity, unexpected entity type")
	ErrMissingEntityIdentity = errors.New("invalid entity, should have either ID, name or username")
)

// Entity represents an OpsGenie entity (e.g., user, team) with type and identifier.
// Example JSON: { "username":"trinity@opsgenie.com", "type":"user" }.
type Entity struct {
	Type     string `json:"type"`
	ID       string `json:"id,omitempty"`
	Name     string `json:"name,omitempty"`
	Username string `json:"username,omitempty"`
}

// SetFromProp deserializes an entity from a string in the format "type:identifier".
func (e *Entity) SetFromProp(propValue string) error {
	elements := strings.Split(propValue, ":")

	if len(elements) != EntityPartsCount {
		return fmt.Errorf("%w: %q", ErrInvalidEntityFormat, propValue)
	}

	e.Type = elements[0]
	identifier := elements[1]

	isID, err := isOpsGenieID(identifier)
	if err != nil {
		return fmt.Errorf("%w: %q", ErrInvalidEntityIDName, identifier)
	}

	switch {
	case isID:
		e.ID = identifier
	case e.Type == "team":
		e.Name = identifier
	case e.Type == "user":
		e.Username = identifier
	default:
		return fmt.Errorf("%w: %q", ErrUnexpectedEntityType, e.Type)
	}

	return nil
}

// GetPropValue serializes an entity back into a string in the format "type:identifier".
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

// isOpsGenieID checks if a string matches the OpsGenie ID format (e.g., 4513b7ea-3b91-438f-b7e4-e3e54af9147c).
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
