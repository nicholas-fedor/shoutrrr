package format

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertFormatSameFormat(t *testing.T) {
	t.Parallel()

	body := "hello world"

	got, err := ConvertFormat(body, "text", "text")
	require.NoError(t, err)
	assert.Equal(t, body, got)
}

func TestConvertFormatMarkdownToText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"heading", "# Heading", "Heading"},
		{"bold", "**bold**", "bold"},
		{"italic", "*italic*", "italic"},
		{"underscore italic", "_italic_", "italic"},
		{"code", "`code`", "code"},
		{"link", "[link](http://example.com)", "link"},
		{"image alt text", "![alt text](http://example.com/image.png)", "alt text"},
		{"plain text", "plain text", "plain text"},
		{"list item", "- list item", "list item"},
		{"escaped asterisk", "\\*x\\*", "*x*"},
		{"escaped underscore", "\\_x\\_", "_x_"},
		{"unclosed italic", "*x", "*x"},
		{"unclosed bold", "**x", "**x"},
		{"empty emphasis", "****", "*"},
		{"hr", "---", ""},
		{"multiple headings", "# H1\n## H2\n### H3", "H1\nH2\nH3"},
		{"mixed formatting", "# Title\n\nHello **bold** and *italic* world!", "Title\n\nHello bold and italic world!"},
		{"unicode content", "# 你好\n\n*测试*", "你好\n\n测试"},
		{"special chars preserved", "a<b>c&d\"e", "a<b>c&d\"e"},
		{"only delimiters", "***", "*"},
		{"empty string", "", ""},
		{"nested escaped", "\\*\\*bold\\*\\*", "**bold**"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ConvertFormat(tt.input, "markdown", "text")
			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestConvertFormatHTMLToText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"paragraph", "<p>hello</p>", "hello"},
		{"bold", "<b>bold</b>", "bold"},
		{"strong", "<strong>bold</strong>", "bold"},
		{"italic", "<i>italic</i>", "italic"},
		{"emphasis", "<em>italic</em>", "italic"},
		{"line break", "<br>", ""},
		{"self-closing br", "<br/>", ""},
		{"plain text", "plain text", "plain text"},
		{"escaped entities", "&lt;escaped&gt;", "<escaped>"},
		{"nested tags", "<div><p>nested</p></div>", "nested"},
		{"heading", "<h1>Title</h1>", "Title"},
		{"multiple headings", "<h1>A</h1><h2>B</h2>", "AB"},
		{"unicode content", "<p>你好</p>", "你好"},
		{"empty string", "", ""},
		{"script tag", "<script>alert('xss')</script>", "alert('xss')"},
		{"link tag", "<a href=\"http://example.com\">link</a>", "link"},
		{"image tag", "<img src=\"test.png\" alt=\"alt text\"/>", ""},
		{"list items", "<ul><li>A</li><li>B</li></ul>", "AB"},
		{"horizontal rule", "<hr>", ""},
		{"comment", "<!-- comment -->", ""},
		{"multiple paragraphs", "<p>A</p><p>B</p>", "AB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ConvertFormat(tt.input, "html", "text")
			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestConvertFormatTextToMarkdown(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"plain text", "plain text", "plain text"},
		{"asterisk", "use * for emphasis", "use \\* for emphasis"},
		{"underscore", "use _ for emphasis", "use \\_ for emphasis"},
		{"backtick", "code: `cmd`", "code: \\`cmd\\`"},
		{"bracket", "[not a link]", "\\[not a link]"},
		{"multiple special", "*_`[", "\\*\\_\\`\\["},
		{"empty string", "", ""},
		{"unicode safe", "你好世界", "你好世界"},
		{"mixed safe and special", "hello * world", "hello \\* world"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ConvertFormat(tt.input, "text", "markdown")
			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestConvertFormatTextToHTML(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"angle brackets", "<script>", "&lt;script&gt;"},
		{"ampersand", "a&b", "a&amp;b"},
		{"quotes", `a"b'c`, "a&#34;b&#39;c"},
		{"mixed entities", "<>&\"'", "&lt;&gt;&amp;&#34;&#39;"},
		{"plain text", "hello world", "hello world"},
		{"empty string", "", ""},
		{"unicode content", "你好世界", "你好世界"},
		{"newlines preserved", "line1\nline2", "line1\nline2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ConvertFormat(tt.input, "text", "html")
			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestConvertFormatHTMLToMarkdown(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"strong", "<strong>bold</strong>", "**bold**"},
		{"b tag", "<b>bold</b>", "**bold**"},
		{"em tag", "<em>italic</em>", "_italic_"},
		{"i tag", "<i>italic</i>", "_italic_"},
		{"h1 and h2", "<h1>A</h1><h2>B</h2>", "# A\n## B"},
		{"h3 through h6", "<h3>A</h3><h4>B</h4><h5>C</h5><h6>D</h6>", "### A\n#### B\n##### C\n###### D"},
		{"paragraphs", "<p>A</p><p>B</p>", "A\n\nB"},
		{"line break", "<br>", ""},
		{"self-closing br", "<br/>", ""},
		{"nested emphasis", "<p><strong>bold</strong> and <em>italic</em></p>", "**bold** and _italic_"},
		{"empty strong", "<strong></strong>", "****"},
		{"empty em", "<em></em>", "__"},
		{"unknown tags ignored", "<div>text</div>", "text"},
		{"self-closing img", "<img src=\"test.png\" alt=\"alt text\"/>", ""},
		{"unicode content", "<p>你好</p>", "你好"},
		{"empty string", "", ""},
		{"mixed headings and paragraphs", "<h1>Title</h1><p>Body</p>", "# Title\nBody"},
		{"multiple list items", "<ul><li>A</li><li>B</li></ul>", "AB"},
		{"horizontal rule", "<hr>", ""},
		{"comment", "<!-- comment -->", ""},
		{"heading with nested bold", "<h1><strong>Title</strong></h1>", "# **Title**"},
		{"paragraph with nested italic", "<p>text <em>italic</em> text</p>", "text _italic_ text"},
		{"anchor link", `<a href="http://example.com">link</a>`, "[link](http://example.com)"},
		{"anchor single quotes", `<a href='http://example.com'>link</a>`, "[link](http://example.com)"},
		{"html entities decoded", "&lt;script&gt;", "<script>"},
		{"anchor with nested bold", `<a href="http://example.com"><strong>bold</strong> link</a>`, "[**bold** link](http://example.com)"},
		{"anchor without href", "<a>link</a>", "[link]"},
		{"mixed entities and tags", "<p>a&amp;b</p>", "a&b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ConvertFormat(tt.input, "html", "markdown")
			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestConvertFormatMarkdownToHTML(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"h1", "# Heading", "<h1>Heading</h1>"},
		{"h2", "## Heading", "<h2>Heading</h2>"},
		{"h3", "### Heading", "<h3>Heading</h3>"},
		{"h4", "#### Heading", "<h4>Heading</h4>"},
		{"h5", "##### Heading", "<h5>Heading</h5>"},
		{"h6", "###### Heading", "<h6>Heading</h6>"},
		{"too many hashes", "####### Title", "<p>####### Title</p>"},
		{"no space after hash", "#topic", "<p>#topic</p>"},
		{"bold in heading", "# **bold** heading", "<h1><strong>bold</strong> heading</h1>"},
		{"italic in heading", "## *italic* heading", "<h2><em>italic</em> heading</h2>"},
		{"bold", "**bold**", "<p><strong>bold</strong></p>"},
		{"double underscore bold", "__bold__", "<p><strong>bold</strong></p>"},
		{"italic star", "*italic*", "<p><em>italic</em></p>"},
		{"italic underscore", "_italic_", "<p><em>italic</em></p>"},
		{"escaped asterisk", "\\*x\\*", "<p>*x*</p>"},
		{"escaped underscore", "\\_x\\_", "<p>_x_</p>"},
		{"simple italic star", "*x*", "<p><em>x</em></p>"},
		{"simple italic underscore", "_x_", "<p><em>x</em></p>"},
		{"nested bold italic", "***bold and italic***", "<p><strong><em>bold and italic</em></strong></p>"},
		{"italic with bold inside", "*text **bold** text*", "<p><em>text <strong>bold</strong> text</em></p>"},
		{"bold with italic inside", "**bold *and italic* bold**", "<p><strong>bold <em>and italic</em> bold</strong></p>"},
		{"code span", "`code`", "<p>`code`</p>"},
		{"link", "[link](http://example.com)", "<p>[link](http://example.com)</p>"},
		{"image", "![alt](http://example.com/img.png)", "<p>![alt](http://example.com/img.png)</p>"},
		{"unordered list", "- item 1\n- item 2", "<p>- item 1</p><p>- item 2</p>"},
		{"horizontal rule", "---", "<p>---</p>"},
		{"empty line becomes br", "line1\n\nline2", "<p>line1</p><br><p>line2</p>"},
		{"multiple paragraphs", "para1\n\npara2", "<p>para1</p><br><p>para2</p>"},
		{"unicode heading", "# 你好", "<h1>你好</h1>"},
		{"unicode emphasis", "*测试*", "<p><em>测试</em></p>"},
		{"mixed content", "# Title\n\nHello **world** and _universe_!", "<h1>Title</h1><br><p>Hello <strong>world</strong> and <em>universe</em>!</p>"},
		{"only emphasis delimiters", "***", "<p>***</p>"},
		{"empty string", "", "<br>"},
		{"whitespace only", "   ", "<br>"},
		{"paragraph with special chars", "<script>", "<p>&lt;script&gt;</p>"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ConvertFormat(tt.input, "markdown", "html")
			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestConvertFormatRoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"escaped asterisk", "\\*x\\*", "*x*"},
		{"simple italic star", "*x*", "_x_"},
		{"escaped underscore", "\\_x\\_", "_x_"},
		{"simple italic underscore", "_x_", "_x_"},
		{"bold roundtrip", "**bold**", "**bold**"},
		{"nested emphasis", "***bold and italic***", "**_bold and italic_**"},
		{"escaped in heading", "# \\*Title\\*", "# *Title*"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			html, err := ConvertFormat(tt.input, "markdown", "html")
			require.NoError(t, err)

			back, err := ConvertFormat(html, "html", "markdown")
			require.NoError(t, err)

			assert.Equal(t, tt.want, back)
		})
	}
}

func TestConvertFormatUnsupported(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		from         string
		targetFormat string
		expectError  bool
	}{
		{"xml to text", "xml", "text", true},
		{"unknown format", "unknown", "text", true},
		{"text to xml", "text", "xml", true},
		{"markdown to xml", "markdown", "xml", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := ConvertFormat("body", tt.from, tt.targetFormat)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConvertFormatEmptyInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		input        string
		from         string
		targetFormat string
		expected     string
	}{
		{"empty markdown to text", "", "markdown", "text", ""},
		{"empty html to text", "", "html", "text", ""},
		{"empty text to markdown", "", "text", "markdown", ""},
		{"empty text to html", "", "text", "html", ""},
		{"empty markdown to html", "", "markdown", "html", "<br>"},
		{"empty html to markdown", "", "html", "markdown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ConvertFormat(tt.input, tt.from, tt.targetFormat)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestConvertFormatUnicodePreservation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		input        string
		from         string
		targetFormat string
		expected     string
	}{
		{"chinese markdown to text", "# 你好\n\n*测试*", "markdown", "text", "你好\n\n测试"},
		{"emoji markdown to html", "Hello 🌍", "markdown", "html", "<p>Hello 🌍</p>"},
		{"cyrillic html to text", "<p>Привет</p>", "html", "text", "Привет"},
		{"arabic text to html", "مرحبا", "text", "html", "مرحبا"},
		{"mixed unicode markdown to html", "# 你好 **world**", "markdown", "html", "<h1>你好 <strong>world</strong></h1>"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ConvertFormat(tt.input, tt.from, tt.targetFormat)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}
