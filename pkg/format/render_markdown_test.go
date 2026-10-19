package format

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestMarkdownTreeRendererRenderTree verifies the markdown rendering of the
// URL fields, each with its place in the URL highlighted, and of the keyed
// query props with their defaults, aliases, and possible values. A URL ends
// in a slash only when it has no path. The output is compared as rendered, so
// it must carry no color escape codes.
func TestMarkdownTreeRendererRenderTree(t *testing.T) {
	t.Parallel()

	renderer := MarkdownTreeRenderer{HeaderPrefix: "### "}

	// tests lists renderers and configs with the markdown each should render.
	tests := []struct {
		config   any
		name     string
		want     string
		renderer MarkdownTreeRenderer
	}{
		{
			name:     "URL field and prop",
			renderer: renderer,
			config: &struct {
				Name string `default:"notempty" key:"name"`
				Host string `url:"host"`
			}{},
			want: "### URL Fields\n\n" +
				"* __Host__ (__Required__)\n" +
				"<br>URL part: <code class=\"service-url\">mock://<strong>host</strong>/</code>\n\n" +
				"### Query/Param Props\n\n" +
				"* __name__\n  Default: `notempty`\n",
		},
		{
			name:     "URL paths in part order",
			renderer: renderer,
			config: &struct {
				Host  string `url:"host"`
				Path1 string `url:"path1"`
				Path3 string `url:"path3"`
				Path2 string `url:"path2"`
			}{},
			want: "### URL Fields\n\n" +
				"* __Host__ (__Required__)\n" +
				"<br>URL part: <code class=\"service-url\">mock://<strong>host</strong>/path1/path2/path3</code>\n" +
				"* __Path1__ (__Required__)\n" +
				"<br>URL part: <code class=\"service-url\">mock://host/<strong>path1</strong>/path2/path3</code>\n" +
				"* __Path2__ (__Required__)\n" +
				"<br>URL part: <code class=\"service-url\">mock://host/path1/<strong>path2</strong>/path3</code>\n" +
				"* __Path3__ (__Required__)\n" +
				"<br>URL part: <code class=\"service-url\">mock://host/path1/path2/<strong>path3</strong></code>\n\n" +
				"### Query/Param Props\n\n",
		},
		{
			name:     "credentials, host and port, and paths",
			renderer: renderer,
			config: &struct {
				User   string `url:"user"`
				Pass   string `url:"pass"`
				Host   string `url:"host,port"`
				Token  string `url:"path1"`
				Target string `url:"path2"`
			}{},
			want: "### URL Fields\n\n" +
				"* __User__ (__Required__)\n" +
				"<br>URL part: <code class=\"service-url\">mock://<strong>user</strong>:pass@host:port/token/target</code>\n" +
				"* __Pass__ (__Required__)\n" +
				"<br>URL part: <code class=\"service-url\">mock://user:<strong>pass</strong>@host:port/token/target</code>\n" +
				"* __Host__ (__Required__)\n" +
				"<br>URL part: <code class=\"service-url\">mock://user:pass@<strong>host</strong>:<strong>port</strong>/token/target</code>\n" +
				"* __Token__ (__Required__)\n" +
				"<br>URL part: <code class=\"service-url\">mock://user:pass@host:port/<strong>token</strong>/target</code>\n" +
				"* __Target__ (__Required__)\n" +
				"<br>URL part: <code class=\"service-url\">mock://user:pass@host:port/token/<strong>target</strong></code>\n\n" +
				"### Query/Param Props\n\n",
		},
		{
			name:     "single path",
			renderer: renderer,
			config: &struct {
				Host  string `url:"host,port"`
				Topic string `url:"path"`
			}{},
			want: "### URL Fields\n\n" +
				"* __Host__ (__Required__)\n" +
				"<br>URL part: <code class=\"service-url\">mock://<strong>host</strong>:<strong>port</strong>/topic</code>\n" +
				"* __Topic__ (__Required__)\n" +
				"<br>URL part: <code class=\"service-url\">mock://host:port/<strong>topic</strong></code>\n\n" +
				"### Query/Param Props\n\n",
		},
		{
			name:     "paths past the fourth",
			renderer: renderer,
			config: &struct {
				Host  string `url:"host"`
				Path1 string `url:"path1"`
				Path5 string `url:"path5"`
			}{},
			want: "### URL Fields\n\n" +
				"* __Host__ (__Required__)\n" +
				"<br>URL part: <code class=\"service-url\">mock://<strong>host</strong>/path1/path5</code>\n" +
				"* __Path1__ (__Required__)\n" +
				"<br>URL part: <code class=\"service-url\">mock://host/<strong>path1</strong>/path5</code>\n" +
				"* __Path5__ (__Required__)\n" +
				"<br>URL part: <code class=\"service-url\">mock://host/path1/<strong>path5</strong></code>\n\n" +
				"### Query/Param Props\n\n",
		},
		{
			name:     "user without password",
			renderer: renderer,
			config: &struct {
				User string `url:"user"`
				Host string `url:"host"`
			}{},
			want: "### URL Fields\n\n" +
				"* __User__ (__Required__)\n" +
				"<br>URL part: <code class=\"service-url\">mock://<strong>user</strong>@host/</code>\n" +
				"* __Host__ (__Required__)\n" +
				"<br>URL part: <code class=\"service-url\">mock://user@<strong>host</strong>/</code>\n\n" +
				"### Query/Param Props\n\n",
		},
		{
			name:     "one field for user and password",
			renderer: renderer,
			config: &struct {
				Token string `url:"user,pass"`
				Host  string `url:"host"`
			}{},
			want: "### URL Fields\n\n" +
				"* __Token__ (__Required__)\n" +
				"<br>URL part: <code class=\"service-url\">mock://<strong>token</strong>:<strong>token</strong>@host/</code>\n" +
				"* __Host__ (__Required__)\n" +
				"<br>URL part: <code class=\"service-url\">mock://token:token@<strong>host</strong>/</code>\n\n" +
				"### Query/Param Props\n\n",
		},
		{
			name:     "prop aliases",
			renderer: renderer,
			config: &struct {
				Name string `key:"name,handle,title,target"`
			}{},
			want: "### URL Fields\n\n### Query/Param Props\n\n" +
				"* __name__ (__Required__)\n  Aliases: `handle`, `title`, `target`\n",
		},
		{
			name:     "possible enum values",
			renderer: renderer,
			config:   &testEnummer{},
			want: "### URL Fields\n\n### Query/Param Props\n\n" +
				"* __choice__\n  Default: `Maybe`\n  Possible values: `Yes`, `No`, `Maybe`\n",
		},
		{
			name:     "required props first, with descriptions and bool defaults",
			renderer: renderer,
			config: &struct {
				Quiet   bool   `default:"no"  key:"quiet"`
				Verbose bool   `default:"yes" desc:"Log more" key:"verbose"`
				Query   string `key:"query"   url:"query"`
				Token   string `key:"token"`
				Hidden  string
			}{},
			want: "### URL Fields\n\n### Query/Param Props\n\n" +
				"* __query__ (__Required__)\n\n" +
				"* __token__ (__Required__)\n\n" +
				"* __quiet__\n  Default: ❌ `no`\n\n" +
				"* __verbose__ - Log more\n  Default: ✔ `yes`\n",
		},
		{
			name: "props empty message",
			renderer: MarkdownTreeRenderer{
				HeaderPrefix: "### ", PropsDescription: "Feel free to set these:", PropsEmptyMessage: "There is nothing to set!",
			},
			config: &struct {
				Host string `url:"host"`
			}{},
			want: "### URL Fields\n\n" +
				"* __Host__ (__Required__)\n" +
				"<br>URL part: <code class=\"service-url\">mock://<strong>host</strong>/</code>\n\n" +
				"### Query/Param Props\n\nThere is nothing to set!",
		},
		{
			name: "props description",
			renderer: MarkdownTreeRenderer{
				HeaderPrefix: "### ", PropsDescription: "Feel free to set these:", PropsEmptyMessage: "There is nothing to set!",
			},
			config: &struct {
				CoolMode bool `key:"coolmode" optional:""`
			}{},
			want: "### URL Fields\n\n### Query/Param Props\n\nFeel free to set these:\n\n" +
				"* __coolmode__\n  Default: *empty*\n",
		},
		{
			name:     "props description ending in a newline",
			renderer: MarkdownTreeRenderer{HeaderPrefix: "### ", PropsDescription: "Set these:\n"},
			config: &struct {
				CoolMode bool `key:"coolmode" optional:""`
			}{},
			want: "### URL Fields\n\n### Query/Param Props\n\nSet these:\n\n" +
				"* __coolmode__\n  Default: *empty*\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tt.renderer.RenderTree(getRootNode(tt.config), "mock"))
		})
	}
}

// TestMarkdownTreeRendererWriteURLFields verifies that a field listed twice
// and a nil field are each written at most once.
func TestMarkdownTreeRendererWriteURLFields(t *testing.T) {
	t.Parallel()

	host := &FieldInfo{Name: "Host", URLParts: []URLPart{URLHost}, Required: true}

	var out strings.Builder

	MarkdownTreeRenderer{}.writeURLFields(&out, []*FieldInfo{host, nil, host}, "mock")

	assert.Equal(t, "URL Fields\n\n* __Host__ (__Required__)\n"+
		"<br>URL part: <code class=\"service-url\">mock://<strong>host</strong>/</code>\n", out.String())
}
