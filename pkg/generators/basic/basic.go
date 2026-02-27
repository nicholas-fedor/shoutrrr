// Package basic provides a basic generator implementation for creating service configurations.
package basic

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/color"
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Generator is the Basic Generator implementation for creating service configurations.
type Generator struct{}

// Errors defined as static variables for better error handling.
var (
	ErrInvalidConfigType    = errors.New("config does not implement types.ServiceConfig")
	ErrInvalidConfigField   = errors.New("config field is invalid or nil")
	ErrRequiredFieldMissing = errors.New("field is required and has no default value")
)

// Generate creates a service configuration by prompting the user for field values or using provided properties.
func (g *Generator) Generate(
	service types.Service,
	props map[string]string,
	_ []string,
) (types.ServiceConfig, error) {
	configPtr := reflect.ValueOf(service).Elem().FieldByName("Config")
	if !configPtr.IsValid() || configPtr.IsNil() {
		return nil, ErrInvalidConfigField
	}

	scanner := bufio.NewScanner(os.Stdin)
	if err := g.promptUserForFields(configPtr, props, scanner); err != nil {
		return nil, err
	}

	if config, ok := configPtr.Interface().(types.ServiceConfig); ok {
		return config, nil
	}

	return nil, ErrInvalidConfigType
}

// formatPrompt creates a user prompt based on the field’s name and default value.
func (g *Generator) formatPrompt(field *format.FieldInfo) string {
	if field.DefaultValue != "" {
		return fmt.Sprintf("%s[%s]: ", color.HiWhiteString(field.Name), field.DefaultValue)
	}

	return color.HiWhiteString(field.Name) + ": "
}

// getInputValue retrieves the value for a field from props or user input.
func (g *Generator) getInputValue(
	field *format.FieldInfo,
	propKey string,
	props map[string]string,
	scanner *bufio.Scanner,
) (string, error) {
	cfg := color.DefaultConfig()

	if propValue, ok := props[propKey]; ok && propValue != "" {
		_, _ = fmt.Fprint(
			cfg.Output,
			"Using property ",
			color.HiCyanString(propValue),
			" for ",
			color.HiMagentaString(field.Name),
			" field\n",
		)
		props[propKey] = ""

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

		// More specific type validation
		if field.Type != nil {
			kind := field.Type.Kind()
			if kind == reflect.Int || kind == reflect.Int8 || kind == reflect.Int16 ||
				kind == reflect.Int32 || kind == reflect.Int64 {
				if _, err := strconv.ParseInt(input, 10, field.Type.Bits()); err != nil {
					return "", fmt.Errorf("invalid integer value for %s: %w", field.Name, err)
				}
			}
		}

		return input, nil
	} else if scanErr := scanner.Err(); scanErr != nil {
		return "", fmt.Errorf("scanner error: %w", scanErr)
	}

	return field.DefaultValue, nil
}

// printError displays an error message for an invalid field value.
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
func (g *Generator) promptUserForFields(
	configPtr reflect.Value,
	props map[string]string,
	scanner *bufio.Scanner,
) error {
	serviceConfig, ok := configPtr.Interface().(types.ServiceConfig)
	if !ok {
		return ErrInvalidConfigType
	}

	configNode := format.GetConfigFormat(serviceConfig)
	config := configPtr.Elem() // Dereference for setting fields

	for _, item := range configNode.Items {
		field := item.Field()
		propKey := strings.ToLower(field.Name)

		for {
			inputValue, err := g.getInputValue(field, propKey, props, scanner)
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

	valid, err := format.SetConfigField(config, *field, inputValue)
	if err != nil {
		return false, fmt.Errorf("failed to set field %s: %w", field.Name, err)
	}

	return valid, nil
}

// validateAndReturnConfig ensures the config implements ServiceConfig and returns it.
func (g *Generator) validateAndReturnConfig(config reflect.Value) (types.ServiceConfig, error) {
	configInterface := config.Interface()
	if serviceConfig, ok := configInterface.(types.ServiceConfig); ok {
		return serviceConfig, nil
	}

	return nil, ErrInvalidConfigType
}
