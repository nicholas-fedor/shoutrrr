// Package generators provides a router for creating generator instances by identifier.
package generators

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/generators/basic"
	"github.com/nicholas-fedor/shoutrrr/pkg/generators/xouath2"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/telegram"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// generatorConfig holds configuration options for generator creation.
type generatorConfig struct {
	input io.Reader
}

// GeneratorOption configures how a generator is created.
type GeneratorOption func(*generatorConfig)

// ErrUnknownGenerator is returned when an unknown generator identifier is provided.
var ErrUnknownGenerator = errors.New("unknown generator")

// generatorMap is a registry that maps generator type names to their
// factory functions. It is used by NewGenerator to create instances of
// specific generator implementations based on the requested type.
var generatorMap = map[string]func(config generatorConfig) types.Generator{
	"basic": func(config generatorConfig) types.Generator {
		return &basic.Generator{
			Input: config.input,
		}
	},
	"oauth2": func(config generatorConfig) types.Generator {
		return &xouath2.Generator{}
	},
	"telegram": func(config generatorConfig) types.Generator {
		return &telegram.Generator{
			Reader: config.input,
			Writer: nil,
		}
	},
}

// WithInput sets the input reader for generators that support it.
// This is useful for dependency injection in tests or for providing
// input from sources other than os.Stdin.
func WithInput(reader io.Reader) GeneratorOption {
	return func(c *generatorConfig) {
		c.input = reader
	}
}

// ListGenerators lists all available generators.
func ListGenerators() []string {
	generators := make([]string, len(generatorMap))

	i := 0

	for key := range generatorMap {
		generators[i] = key
		i++
	}

	return generators
}

// NewGenerator creates an instance of the generator that corresponds to the provided identifier.
// Optional GeneratorOption parameters can be used to configure the generator (e.g., WithInput).
func NewGenerator(identifier string, opts ...GeneratorOption) (types.Generator, error) {
	config := generatorConfig{
		input: nil,
	}
	for _, opt := range opts {
		opt(&config)
	}

	generatorFactory, valid := generatorMap[strings.ToLower(identifier)]
	if !valid {
		return nil, fmt.Errorf("%w: %q", ErrUnknownGenerator, identifier)
	}

	return generatorFactory(config), nil
}
