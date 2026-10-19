package format

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// unknownNode is a node that is neither a value nor a container.
type unknownNode struct {
	ValueNode
}

// TestConsoleTreeRendererRenderTree verifies the console rendering of each
// field: the name padded past the longest name, the type or value padded to
// its column, the description padded to its column, and the URL part,
// template, default, required, alias, and enum markers.
func TestConsoleTreeRendererRenderTree(t *testing.T) {
	t.Parallel()

	types := ConsoleTreeRenderer{WithValues: false}
	values := ConsoleTreeRenderer{WithValues: true}
	pad := strings.Repeat

	// tests lists renderers and configs with the tree each should render.
	tests := []struct {
		config   any
		name     string
		want     string
		renderer ConsoleTreeRenderer
	}{
		{
			name:     "types",
			renderer: types,
			config: &struct {
				Name string `default:"notempty"`
				Host string `url:"host"`
			}{},
			want: "Host string" + pad(" ", 10) + pad(" ", 60) + " <URL: Host> <Required>\n" +
				"Name string" + pad(" ", 10) + pad(" ", 60) + " <Default: notempty>\n",
		},
		{
			name:     "enum as option",
			renderer: types,
			config:   &testEnummer{},
			want:     "Choice option" + pad(" ", 10) + pad(" ", 60) + " <Default: Maybe> [Yes, No, Maybe]\n",
		},
		{
			name:     "URL paths in name order",
			renderer: types,
			config: &struct {
				Host  string `url:"host"`
				Path1 string `url:"path1"`
				Path3 string `url:"path3"`
				Path2 string `url:"path2"`
			}{},
			want: "Host  string" + pad(" ", 10) + pad(" ", 60) + " <URL: Host> <Required>\n" +
				"Path1 string" + pad(" ", 10) + pad(" ", 60) + " <URL: Path> <Required>\n" +
				"Path2 string" + pad(" ", 10) + pad(" ", 60) + " <URL: Path> <Required>\n" +
				"Path3 string" + pad(" ", 10) + pad(" ", 60) + " <URL: Path> <Required>\n",
		},
		{
			name:     "values with markers",
			renderer: values,
			config: &struct {
				Name  string `default:"x"     desc:"Desc" key:"name,n,alias" tpl:"{{.x}}"`
				Host  string `url:"host,port"`
				Query string `url:"query"`
			}{Name: "val", Host: "example.com"},
			want: "Host  example.com" + pad(" ", 19) + pad(" ", 60) + " <URL: Host, Port> <Required>\n" +
				"Name  val" + pad(" ", 27) + "Desc" + pad(" ", 56) + " <Template: {{.x}}> <Default: x> <Aliases: n, alias>\n" +
				"Query " + pad(" ", 30) + pad(" ", 60) + " <Required>\n",
		},
		{
			name:     "enum value",
			renderer: values,
			config:   &testEnummer{Choice: 1},
			want:     "Choice No" + pad(" ", 28) + pad(" ", 60) + " <Default: Maybe> [Yes, No, Maybe]\n",
		},
		{
			name:     "long value and description",
			renderer: values,
			config: &struct {
				Token string `desc:"A description that is longer than the sixty column description width"`
			}{Token: strings.Repeat("t", 40)},
			want: "Token " + strings.Repeat("t", 40) + " A description that is longer than the sixty column description width  <Required>\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, testRenderTree(tt.renderer, tt.config))
		})
	}

	assert.Empty(t, testRenderTree(types, &struct{}{}))
}

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
