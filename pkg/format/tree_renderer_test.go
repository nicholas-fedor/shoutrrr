package format

import (
	"regexp"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// testEnummer is a config with an enum field for the renderer tests.
type testEnummer struct {
	// Choice is a keyed enum with a default.
	Choice int `default:"Maybe" key:"choice"`
}

// ansiEscape matches the color escape codes the renderers write when stdout
// is a terminal.
var ansiEscape = regexp.MustCompile("\x1b\\[[0-9;]*m")

// testRenderTree renders the config format of v with r for the mock scheme.
//
// Parameters:
//   - r: the renderer.
//   - v: the config.
//
// Returns:
//   - string: the rendered tree without color escape codes.
func testRenderTree(r TreeRenderer, v any) string {
	return ansiEscape.ReplaceAllString(r.RenderTree(getRootNode(v), "mock"), "")
}

// Enums returns the enum formatter of the Choice field.
//
// Returns:
//   - map[string]types.EnumFormatter: the enum formatters by field name.
func (testEnummer) Enums() map[string]types.EnumFormatter {
	return map[string]types.EnumFormatter{
		"Choice": CreateEnumFormatter([]string{"Yes", "No", "Maybe"}),
	}
}
