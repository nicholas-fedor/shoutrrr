package standard

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTemplaterSetTemplateString verifies that a template string is parsed
// and stored under its ID, replacing any template already stored there.
func TestTemplaterSetTemplateString(t *testing.T) {
	t.Parallel()

	templater := &Templater{}
	require.NoError(t, templater.SetTemplateString("body", "first {{.message}}"))
	require.NoError(t, templater.SetTemplateString("body", "second {{.message}}"))

	tpl, found := templater.GetTemplate("body")
	require.True(t, found)

	var out strings.Builder

	require.NoError(t, tpl.Execute(&out, map[string]string{"message": "hello"}))
	assert.Equal(t, "second hello", out.String())
}

// TestTemplaterSetTemplateStringError verifies that a template that does not
// parse fails with an error naming its ID and stores nothing.
func TestTemplaterSetTemplateStringError(t *testing.T) {
	t.Parallel()

	templater := &Templater{}

	err := templater.SetTemplateString("body", "template {{ missing end tag")

	require.ErrorContains(t, err, `parsing template string for ID "body"`)

	_, found := templater.GetTemplate("body")
	assert.False(t, found)
}

// TestTemplaterSetTemplateFile verifies that a template file is read and
// stored under its ID.
func TestTemplaterSetTemplateFile(t *testing.T) {
	t.Parallel()

	file := filepath.Join(t.TempDir(), "body.tpl")
	require.NoError(t, os.WriteFile(file, []byte("file {{.message}}"), 0o600))

	templater := &Templater{}
	require.NoError(t, templater.SetTemplateFile("body", file))

	tpl, found := templater.GetTemplate("body")
	require.True(t, found)

	var out strings.Builder

	require.NoError(t, tpl.Execute(&out, map[string]string{"message": "hello"}))
	assert.Equal(t, "file hello", out.String())
}

// TestTemplaterSetTemplateFileErrors verifies that a missing file fails with
// [fs.ErrNotExist] and that a file that does not parse fails with the parse
// error, neither storing a template.
func TestTemplaterSetTemplateFileErrors(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	invalid := filepath.Join(dir, "invalid.tpl")
	require.NoError(t, os.WriteFile(invalid, []byte("{{ missing end tag"), 0o600))

	t.Run("missing file", func(t *testing.T) {
		t.Parallel()

		templater := &Templater{}

		err := templater.SetTemplateFile("body", filepath.Join(dir, "missing.tpl"))

		require.ErrorIs(t, err, fs.ErrNotExist)
		require.ErrorContains(t, err, `for ID "body"`)

		_, found := templater.GetTemplate("body")
		assert.False(t, found)
	})

	t.Run("invalid template", func(t *testing.T) {
		t.Parallel()

		templater := &Templater{}

		require.ErrorContains(t, templater.SetTemplateFile("body", invalid), `parsing template string for ID "body"`)

		_, found := templater.GetTemplate("body")
		assert.False(t, found)
	})
}

// TestTemplaterGetTemplate verifies that GetTemplate reports a template only
// for an ID that was set, including on a templater with none.
func TestTemplaterGetTemplate(t *testing.T) {
	t.Parallel()

	empty := &Templater{}
	tpl, found := empty.GetTemplate("body")
	assert.Nil(t, tpl)
	assert.False(t, found)

	templater := &Templater{}
	require.NoError(t, templater.SetTemplateString("body", "template body"))

	tpl, found = templater.GetTemplate("body")
	assert.NotNil(t, tpl)
	assert.True(t, found)

	tpl, found = templater.GetTemplate("other")
	assert.Nil(t, tpl)
	assert.False(t, found)
}
