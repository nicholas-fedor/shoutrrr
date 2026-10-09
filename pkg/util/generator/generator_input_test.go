package generator

import (
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// queryTimeout bounds each query in these tests, so a query that loops fails the
// test instead of hanging until the test binary times out.
const queryTimeout = 5 * time.Second

// errReadFailed is the read error the failing reader returns.
var errReadFailed = errors.New("read failed")

// TestQueryStringStopsWhenInputCloses verifies that a query returns once the
// input ends, records ErrInputClosed, and that later queries return without
// prompting.
func TestQueryStringStopsWhenInputCloses(t *testing.T) {
	t.Parallel()

	var out strings.Builder

	dialog := NewUserDialog(strings.NewReader(""), &out, nil)

	assert.Empty(t, queryWithTimeout(t, dialog, "token"))
	require.ErrorIs(t, dialog.Err(), ErrInputClosed)

	prompts := strings.Count(out.String(), "Enter:")

	assert.Empty(t, queryWithTimeout(t, dialog, "chat"))
	assert.Equal(t, prompts, strings.Count(out.String(), "Enter:"), "a later query must not prompt")
}

// TestQueryStringUsesPropsAfterInputCloses verifies that a valid prop answers a
// query even after the input has ended.
func TestQueryStringUsesPropsAfterInputCloses(t *testing.T) {
	t.Parallel()

	dialog := NewUserDialog(strings.NewReader(""), io.Discard, map[string]string{"chat": "12345"})

	assert.Empty(t, queryWithTimeout(t, dialog, "token"))
	assert.Equal(t, "12345", queryWithTimeout(t, dialog, "chat"))
}

// TestQueryStringStopsOnReadError verifies that a read error ends the query and
// is reported by Err, instead of prompting again.
func TestQueryStringStopsOnReadError(t *testing.T) {
	t.Parallel()

	dialog := NewUserDialog(iotest.ErrReader(errReadFailed), io.Discard, nil)

	assert.Empty(t, queryWithTimeout(t, dialog, "token"))
	require.ErrorIs(t, dialog.Err(), errReadFailed)
}

// TestQueryStringKeepsPromptingForValidInput verifies that an invalid answer is
// asked again while input is available, and that Err stays nil.
func TestQueryStringKeepsPromptingForValidInput(t *testing.T) {
	t.Parallel()

	dialog := NewUserDialog(strings.NewReader("\nvalue\n"), io.Discard, nil)

	assert.Equal(t, "value", queryWithTimeout(t, dialog, "token"))
	assert.NoError(t, dialog.Err())
}

// queryWithTimeout asks for a required answer and fails the test when the query
// does not return within [queryTimeout].
//
// Parameters:
//   - t: the test.
//   - dialog: the dialog to query.
//   - key: the prop key for the answer.
//
// Returns:
//   - string: the answer.
func queryWithTimeout(t *testing.T, dialog *UserDialog, key string) string {
	t.Helper()

	answer := make(chan string, 1)

	go func() { answer <- dialog.QueryString("Enter:", Required, key) }()

	select {
	case got := <-answer:
		return got
	case <-time.After(queryTimeout):
		t.Fatalf("query for %q did not return", key)

		return ""
	}
}
