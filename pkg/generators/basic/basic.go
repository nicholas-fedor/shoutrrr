// Package basic provides a basic generator implementation for creating service configurations.
package basic

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/color"
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util/generator"
)

// Generator is the Basic Generator implementation for creating service configurations.
type Generator struct {
	// Input is the reader used for user input during configuration generation.
	// If nil, os.Stdin is used by default.
	Input io.Reader
}

// Errors returned when a config is generated.
var (
	// ErrInvalidConfigField indicates a service without a Config field.
	ErrInvalidConfigField = errors.New("config field is invalid or nil")
	// ErrRequiredFieldMissing indicates a required field without a default
	// that got no value before the input ended.
	ErrRequiredFieldMissing = errors.New("field is required and has no default value")
)

// Generate creates a service configuration by prompting the user for field values or using provided properties.
//
// A service with a config gets that config filled in. A service built without
// one, as the router's NewService builds it, gets a new zero config from
// [format.GetServiceConfig], which the service does not keep.
//
// Parameters:
//   - service: the service to generate a config for.
//   - props: field values by lowercased field name, used instead of prompting.
//
// Returns:
//   - types.ServiceConfig: the generated config.
//   - error: [ErrInvalidConfigField] for a service without a Config field, or
//     the first error from reading a field's value.
func (g *Generator) Generate(
	service types.Service,
	props map[string]string,
	_ []string,
) (types.ServiceConfig, error) {
	// format.GetServiceConfig panics for a service without a Config field, so
	// the field is checked first.
	if !reflect.Indirect(reflect.ValueOf(service)).FieldByName("Config").IsValid() {
		return nil, ErrInvalidConfigField
	}

	config := format.GetServiceConfig(service)

	// Use injected Input if provided, otherwise fall back to os.Stdin
	reader := g.Input
	if reader == nil {
		reader = os.Stdin
	}

	scanner := bufio.NewScanner(reader)
	if err := g.promptUserForFields(config, props, scanner); err != nil {
		return nil, err
	}

	return config, nil
}

// formatPrompt creates a user prompt based on the field’s name and default value.
//
// Parameters:
//   - field: the field to prompt for.
//
// Returns:
//   - string: the field name, with its default in brackets when it has one,
//     followed by a colon.
func (g *Generator) formatPrompt(field *format.FieldInfo) string {
	if field.DefaultValue != "" {
		return fmt.Sprintf("%s[%s]: ", color.HiWhiteString(field.Name), field.DefaultValue)
	}

	return color.HiWhiteString(field.Name) + ": "
}

// getInputValue retrieves the value for a field from props or user input.
//
// A prop is used once. When its value is rejected, the field is prompted for
// instead. An answer is returned as typed, and the caller parses it. When the
// input ends, an optional field takes its default value, while
// a required field without a default, or a field whose earlier value was
// rejected, fails with [generator.ErrInputClosed].
//
// Parameters:
//   - field: the field to fill.
//   - propKey: the prop key for the field.
//   - props: the props supplied on the command line.
//   - consumed: the prop keys already used, updated when a prop is used.
//   - scanner: the reader for user input.
//   - retry: whether an earlier value for the field was rejected.
//
// Returns:
//   - string: the value for the field.
//   - error: an error wrapping [ErrRequiredFieldMissing] for an empty answer to
//     a required field without a default, a read error, or
//     [generator.ErrInputClosed].
func (g *Generator) getInputValue(
	field *format.FieldInfo,
	propKey string,
	props map[string]string,
	consumed map[string]struct{},
	scanner *bufio.Scanner,
	retry bool,
) (string, error) {
	cfg := color.DefaultConfig()

	_, used := consumed[propKey]
	if propValue, ok := props[propKey]; ok && propValue != "" && !used {
		_, _ = fmt.Fprint(
			cfg.Output,
			"Using property ",
			color.HiCyanString(propValue),
			" for ",
			color.HiMagentaString(field.Name),
			" field\n",
		)
		// Mark this prop as consumed instead of mutating the original map
		consumed[propKey] = struct{}{}

		return propValue, nil
	}

	prompt := g.formatPrompt(field)
	_, _ = fmt.Fprint(cfg.Output, prompt)

	if scanner.Scan() {
		input := scanner.Text()
		if input == "" {
			if field.DefaultValue != "" {
				return field.DefaultValue, nil
			}

			if field.Required {
				return "", fmt.Errorf("%s: %w", field.Name, ErrRequiredFieldMissing)
			}

			return "", nil
		}

		return input, nil
	} else if scanErr := scanner.Err(); scanErr != nil {
		return "", fmt.Errorf("scanner error: %w", scanErr)
	}

	_, _ = fmt.Fprint(cfg.Output, "\n")

	if retry || (field.Required && field.DefaultValue == "") {
		return "", fmt.Errorf("%s: %w", field.Name, generator.ErrInputClosed)
	}

	return field.DefaultValue, nil
}

// printError displays an error message for an invalid field value.
//
// Parameters:
//   - fieldName: the name of the field.
//   - errorMsg: the reason the value was rejected.
func (g *Generator) printError(fieldName, errorMsg string) {
	cfg := color.DefaultConfig()
	_, _ = fmt.Fprint(
		cfg.Output,
		"Invalid format for field ",
		color.HiCyanString(fieldName),
		": ",
		errorMsg,
		"\n\n",
	)
}

// printInvalidType displays a type mismatch error for a field.
//
// Parameters:
//   - fieldName: the name of the field.
//   - typeName: the kind of value the field holds.
func (g *Generator) printInvalidType(fieldName, typeName string) {
	cfg := color.DefaultConfig()
	_, _ = fmt.Fprint(
		cfg.Output,
		"Invalid type ",
		color.HiYellowString(typeName),
		" for field ",
		color.HiCyanString(fieldName),
		"\n\n",
	)
}

// promptUserForFields iterates over config fields, prompting the user or using props to set values.
//
// Parameters:
//   - serviceConfig: the service config to fill, a pointer to its struct.
//   - props: the props supplied on the command line.
//   - scanner: the reader for user input.
//
// Returns:
//   - error: the first error from reading a field's value, or nil.
func (g *Generator) promptUserForFields(
	serviceConfig types.ServiceConfig,
	props map[string]string,
	scanner *bufio.Scanner,
) error {
	configNode := format.GetConfigFormat(serviceConfig)
	config := reflect.Indirect(reflect.ValueOf(serviceConfig)) // Dereference for setting fields

	// Track which props have been consumed to avoid mutating the input map
	consumed := make(map[string]struct{})

	for _, item := range configNode.Items {
		field := item.Field()
		propKey := strings.ToLower(field.Name)

		for retry := false; ; retry = true {
			inputValue, err := g.getInputValue(field, propKey, props, consumed, scanner, retry)
			if err != nil {
				return err // Propagate the error immediately
			}

			if valid, err := g.setFieldValue(config, field, inputValue); valid {
				break
			} else if err != nil {
				g.printError(field.Name, err.Error())
			} else {
				g.printInvalidType(field.Name, field.Type.Kind().String())
			}
		}
	}

	return nil
}

// setFieldValue attempts to set a field's value and handles required field validation.
//
// An empty value sets an optional field to its default, or leaves it unset
// when it has none. An empty value for a required field is reported and
// rejected.
//
// Parameters:
//   - config: the config struct value to set.
//   - field: the field to set.
//   - inputValue: the value as text, or empty.
//
// Returns:
//   - bool: true when the field was set or may stay unset.
//   - error: the error from setting the value, naming the field, or nil.
func (g *Generator) setFieldValue(
	config reflect.Value,
	field *format.FieldInfo,
	inputValue string,
) (bool, error) {
	cfg := color.DefaultConfig()

	if inputValue == "" {
		if field.Required {
			_, _ = fmt.Fprint(
				cfg.Output,
				"Field ",
				color.HiCyanString(field.Name),
				" is required!\n\n",
			)

			return false, nil
		}

		if field.DefaultValue == "" {
			return true, nil
		}

		inputValue = field.DefaultValue
	}

	valid, err := format.SetConfigField(config, field, inputValue)
	if err != nil {
		return false, fmt.Errorf("failed to set field %s: %w", field.Name, err)
	}

	return valid, nil
}
