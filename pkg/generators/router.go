package generators

import (
	"errors"
	"fmt"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/generators/basic"
	"github.com/nicholas-fedor/shoutrrr/pkg/generators/xouath2"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/telegram"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

var ErrUnknownGenerator = errors.New("unknown generator")

var generatorMap = map[string]func() types.Generator{
	"basic":    func() types.Generator { return &basic.Generator{} },
	"oauth2":   func() types.Generator { return &xouath2.Generator{} },
	"telegram": func() types.Generator { return &telegram.Generator{} },
}

// NewGenerator creates an instance of the generator that corresponds to the provided identifier.
func NewGenerator(identifier string) (types.Generator, error) {
	generatorFactory, valid := generatorMap[strings.ToLower(identifier)]
	if !valid {
		return nil, fmt.Errorf("%w: %q", ErrUnknownGenerator, identifier)
	}

	return generatorFactory(), nil
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
