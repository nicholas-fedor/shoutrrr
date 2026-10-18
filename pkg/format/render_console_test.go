package format

import (
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// unknownNode is a node that is neither a value nor a container.
type unknownNode struct {
	ValueNode
}

// ansiEscape matches the color escape codes the console renderer writes when
// stdout is a terminal.
var ansiEscape = regexp.MustCompile("\x1b\\[[0-9;]*m")

var _ = ginkgo.Describe("RenderConsole", func() {
	format.CharactersAroundMismatchToInclude = 30
	renderer := ConsoleTreeRenderer{WithValues: false}

	ginkgo.It("should render the expected output based on config reflection/tags", func() {
		actual := testRenderTree(renderer, &struct {
			Name string `default:"notempty"`
			Host string `url:"host"`
		}{})

		expected := `
Host string                                                                       <URL: Host> <Required>
Name string                                                                       <Default: notempty>
`[1:]

		gomega.Expect(actual).To(gomega.Equal(expected))
	})

	ginkgo.It(`should render enum types as "option"`, func() {
		actual := testRenderTree(renderer, &testEnummer{})

		expected := `
Choice option                                                                       <Default: Maybe> [Yes, No, Maybe]
`[1:]

		gomega.Expect(actual).To(gomega.Equal(expected))
	})

	ginkgo.It("should render url paths in sorted order", func() {
		actual := testRenderTree(renderer, &struct {
			Host  string `url:"host"`
			Path1 string `url:"path1"`
			Path3 string `url:"path3"`
			Path2 string `url:"path2"`
		}{})

		expected := `
Host  string                                                                       <URL: Host> <Required>
Path1 string                                                                       <URL: Path> <Required>
Path2 string                                                                       <URL: Path> <Required>
Path3 string                                                                       <URL: Path> <Required>
`[1:]

		gomega.Expect(actual).To(gomega.Equal(expected))
	})
})

// TestConsoleTreeRendererWriteNodeValue verifies that values are written as
// they are, slices in brackets, and maps in braces with their keys in order,
// that a node of another type is written as "?", and that the returned width
// is the length of what was written, not counting color escape codes.
func TestConsoleTreeRendererWriteNodeValue(t *testing.T) {
	t.Parallel()

	config := &testStruct{
		Signed: -45, Duration: 3 * time.Second, TestEnum: 1, SubProp: subPropStruct{Value: "whoa"},
		StrSlice: []string{"one", "two"}, StrArray: [3]string{"a", "b", "c"},
		SubPropPtrSlice: []*subPropStruct{{Value: "diet"}, {Value: "glue"}},
		StrMap:          map[string]string{"b": "2", "a": "1"}, Uint8Map: map[string]uint8{"x": 9},
	}
	items := map[string]Node{}

	for _, item := range getRootNode(config).Items {
		items[item.Field().Name] = item
	}

	// tests lists fields with how each should be written.
	tests := []struct {
		field string
		want  string
	}{
		{field: "Signed", want: "-45"},
		{field: "Duration", want: "3s"},
		{field: "TestEnum", want: "Foo"},
		{field: "SubProp", want: "@whoa"},
		{field: "StrSlice", want: "[ one, two ]"},
		{field: "StrArray", want: "[ a, b, c ]"},
		{field: "SubPropPtrSlice", want: "[ @diet, @glue ]"},
		{field: "StrMap", want: "{ a: 1, b: 2 }"},
		{field: "Uint8Map", want: "{ x: 9 }"},
		{field: "IntMap", want: "{  }"},
	}

	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			t.Parallel()

			node, ok := items[tt.field]
			require.True(t, ok)

			var out strings.Builder

			width := ConsoleTreeRenderer{}.writeNodeValue(&out, node)

			assert.Equal(t, tt.want, ansiEscape.ReplaceAllString(out.String(), ""))
			assert.Equal(t, len(tt.want), width)
		})
	}

	var out strings.Builder

	width := ConsoleTreeRenderer{}.writeNodeValue(&out, &unknownNode{ValueNode{FieldInfo: &FieldInfo{Type: reflect.TypeFor[string]()}}})
	assert.Equal(t, "?", out.String())
	assert.Equal(t, 1, width)
}
