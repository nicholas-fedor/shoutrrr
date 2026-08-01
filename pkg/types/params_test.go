package types

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseMessageLevel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected MessageLevel
		found    bool
	}{
		{"unknown uppercase", "Unknown", Unknown, true},
		{"debug", "Debug", Debug, true},
		{"info", "Info", Info, true},
		{"warning", "Warning", Warning, true},
		{"error", "Error", Error, true},
		{"unknown lowercase", "unknown", Unknown, true},
		{"empty string", "", Unknown, false},
		{"info lowercase", "info", Info, true},
		{"debug mixed case", "DEBUG", Debug, true},
		{"warning mixed case", "WaRnInG", Warning, true},
		{"error mixed case", "eRrOr", Error, true},
		{"unknown mixed case", "UnKnOwN", Unknown, true},
		{"random string", "foobar", Unknown, false},
		{"numeric string", "123", Unknown, false},
		{"info with spaces", "  info  ", Unknown, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, found := ParseMessageLevel(tt.input)
			assert.Equal(t, tt.expected, got)
			assert.Equal(t, tt.found, found)
		})
	}
}

func TestMessageLevelString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		level    MessageLevel
		expected string
	}{
		{"unknown", Unknown, "Unknown"},
		{"debug", Debug, "Debug"},
		{"info", Info, "Info"},
		{"warning", Warning, "Warning"},
		{"error", Error, "Error"},
		{"out of range low", MessageLevel(0), "Unknown"},
		{"out of range high", MessageLevel(99), "Unknown"},
		{"out of range mid", MessageLevel(50), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.level.String()
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestMessageLevelConstants(t *testing.T) {
	t.Parallel()

	assert.Equal(t, Unknown, MessageLevel(0))
	assert.Equal(t, Debug, MessageLevel(1))
	assert.Equal(t, Info, MessageLevel(2))
	assert.Equal(t, Warning, MessageLevel(3))
	assert.Equal(t, Error, MessageLevel(4))
	assert.Equal(t, 5, MessageLevelCount)
}

func TestMessageLevelStringRoundTrip(t *testing.T) {
	t.Parallel()

	levels := []MessageLevel{Unknown, Debug, Info, Warning, Error}
	for _, level := range levels {
		t.Run(level.String(), func(t *testing.T) {
			t.Parallel()

			parsed, found := ParseMessageLevel(level.String())
			assert.True(t, found)
			assert.Equal(t, level, parsed)
		})
	}
}

func TestParamsLevel(t *testing.T) {
	t.Parallel()

	t.Run("not set returns false", func(t *testing.T) {
		t.Parallel()

		p := Params{}
		_, found := p.Level()
		assert.False(t, found)
	})

	t.Run("set and get warning", func(t *testing.T) {
		t.Parallel()

		p := Params{}
		p.SetLevel(Warning)
		got, found := p.Level()
		assert.True(t, found)
		assert.Equal(t, Warning, got)
	})

	t.Run("overwrite level", func(t *testing.T) {
		t.Parallel()

		p := Params{}
		p.SetLevel(Info)
		p.SetLevel(Error)
		got, found := p.Level()
		assert.True(t, found)
		assert.Equal(t, Error, got)
	})

	t.Run("set unknown level", func(t *testing.T) {
		t.Parallel()

		p := Params{}
		p.SetLevel(Unknown)
		got, found := p.Level()
		assert.True(t, found)
		assert.Equal(t, Unknown, got)
	})
}

func TestParamsTitle(t *testing.T) {
	t.Parallel()

	t.Run("set and get title", func(t *testing.T) {
		t.Parallel()

		p := Params{}
		p.SetTitle("alert")
		got, found := p.Title()
		assert.True(t, found)
		assert.Equal(t, "alert", got)
	})

	t.Run("empty title", func(t *testing.T) {
		t.Parallel()

		p := Params{}
		p.SetTitle("")
		got, found := p.Title()
		assert.True(t, found)
		assert.Empty(t, got)
	})

	t.Run("overwrite title", func(t *testing.T) {
		t.Parallel()

		p := Params{}
		p.SetTitle("first")
		p.SetTitle("second")
		got, found := p.Title()
		assert.True(t, found)
		assert.Equal(t, "second", got)
	})

	t.Run("not set returns false", func(t *testing.T) {
		t.Parallel()

		p := Params{}
		_, found := p.Title()
		assert.False(t, found)
	})

	t.Run("unicode title", func(t *testing.T) {
		t.Parallel()

		p := Params{}
		p.SetTitle(" alerts")
		got, found := p.Title()
		assert.True(t, found)
		assert.Equal(t, " alerts", got)
	})
}

func TestParamsMessage(t *testing.T) {
	t.Parallel()

	t.Run("set and get message", func(t *testing.T) {
		t.Parallel()

		p := Params{}
		p.SetMessage("hello world")
		val, found := p[MessageKey]
		assert.True(t, found)
		assert.Equal(t, "hello world", val)
	})

	t.Run("empty message", func(t *testing.T) {
		t.Parallel()

		p := Params{}
		p.SetMessage("")
		val, found := p[MessageKey]
		assert.True(t, found)
		assert.Empty(t, val)
	})

	t.Run("overwrite message", func(t *testing.T) {
		t.Parallel()

		p := Params{}
		p.SetMessage("first")
		p.SetMessage("second")
		val, found := p[MessageKey]
		assert.True(t, found)
		assert.Equal(t, "second", val)
	})

	t.Run("unicode message", func(t *testing.T) {
		t.Parallel()

		p := Params{}
		p.SetMessage(" hello")
		val, found := p[MessageKey]
		assert.True(t, found)
		assert.Equal(t, " hello", val)
	})
}

func TestParamsMultipleKeys(t *testing.T) {
	t.Parallel()

	t.Run("all three keys coexist", func(t *testing.T) {
		t.Parallel()

		p := Params{}
		p.SetTitle("title")
		p.SetMessage("message")
		p.SetLevel(Info)

		title, found := p.Title()
		assert.True(t, found)
		assert.Equal(t, "title", title)

		assert.Equal(t, "message", p[MessageKey])

		level, found := p.Level()
		assert.True(t, found)
		assert.Equal(t, Info, level)
	})

	t.Run("level key is LevelKey", func(t *testing.T) {
		t.Parallel()

		p := Params{}
		p.SetLevel(Error)
		assert.Equal(t, "Error", p[LevelKey])
	})

	t.Run("title key is TitleKey", func(t *testing.T) {
		t.Parallel()

		p := Params{}
		p.SetTitle("t")
		assert.Equal(t, "t", p[TitleKey])
	})

	t.Run("message key is MessageKey", func(t *testing.T) {
		t.Parallel()

		p := Params{}
		p.SetMessage("m")
		assert.Equal(t, "m", p[MessageKey])
	})
}

func TestTargetError(t *testing.T) {
	t.Parallel()

	t.Run("error format", func(t *testing.T) {
		t.Parallel()

		inner := errors.New("connection refused")
		err := &TargetError{URL: "slack://webhook", Err: inner}

		expected := "slack://webhook: connection refused"
		assert.Equal(t, expected, err.Error())
	})

	t.Run("unwrap returns inner error", func(t *testing.T) {
		t.Parallel()

		inner := errors.New("connection refused")
		err := &TargetError{URL: "slack://webhook", Err: inner}

		unwrapped := err.Unwrap()
		assert.Equal(t, inner, unwrapped)
	})

	t.Run("errors.Is finds wrapped error", func(t *testing.T) {
		t.Parallel()

		inner := errors.New("connection refused")
		err := &TargetError{URL: "slack://webhook", Err: inner}

		assert.ErrorIs(t, err, inner)
	})

	t.Run("errors.As recovers target error", func(t *testing.T) {
		t.Parallel()

		inner := errors.New("connection refused")
		err := &TargetError{URL: "slack://webhook", Err: inner}

		var targetErr *TargetError
		require.ErrorAs(t, err, &targetErr)
		require.NotNil(t, targetErr)
		assert.Equal(t, "slack://webhook", targetErr.URL)
		assert.Equal(t, inner, targetErr.Err)
	})

	t.Run("nil Err field", func(t *testing.T) {
		t.Parallel()

		err := &TargetError{URL: "slack://webhook", Err: nil}

		assert.Equal(t, "slack://webhook: <nil>", err.Error())
		assert.NoError(t, err.Unwrap())
	})

	t.Run("errors.Is with nil inner", func(t *testing.T) {
		t.Parallel()

		err := &TargetError{URL: "slack://webhook", Err: nil}

		assert.NotErrorIs(t, err, errors.New("anything"))
	})
}

func TestTargetErrorNil(t *testing.T) {
	t.Parallel()

	t.Run("errors.As matches nil target error", func(t *testing.T) {
		t.Parallel()

		var (
			err *TargetError
			e   error = err
		)

		var targetErr *TargetError
		require.ErrorAs(t, e, &targetErr)
		assert.Nil(t, targetErr)
	})

	t.Run("errors.As does not match non-target error", func(t *testing.T) {
		t.Parallel()

		e := errors.New("some error")

		var targetErr *TargetError
		assert.NotErrorAs(t, e, &targetErr)
		assert.Nil(t, targetErr)
	})
}

func TestItemsToPlain(t *testing.T) {
	t.Parallel()

	t.Run("empty items", func(t *testing.T) {
		t.Parallel()

		got := ItemsToPlain([]MessageItem{})
		assert.Empty(t, got)
	})

	t.Run("single item", func(t *testing.T) {
		t.Parallel()

		items := []MessageItem{{Text: "hello"}}
		got := ItemsToPlain(items)
		assert.Equal(t, "hello\n", got)
	})

	t.Run("multiple items", func(t *testing.T) {
		t.Parallel()

		items := []MessageItem{
			{Text: "first"},
			{Text: "second"},
			{Text: "third"},
		}
		got := ItemsToPlain(items)
		assert.Equal(t, "first\nsecond\nthird\n", got)
	})

	t.Run("empty text items", func(t *testing.T) {
		t.Parallel()

		items := []MessageItem{
			{Text: ""},
			{Text: ""},
		}
		got := ItemsToPlain(items)
		assert.Equal(t, "\n\n", got)
	})

	t.Run("unicode text", func(t *testing.T) {
		t.Parallel()

		items := []MessageItem{{Text: "你好"}}
		got := ItemsToPlain(items)
		assert.Equal(t, "你好\n", got)
	})

	t.Run("preserves only text", func(t *testing.T) {
		t.Parallel()

		items := []MessageItem{
			{Text: "msg1", Level: Error},
			{Text: "msg2", Level: Info},
		}
		got := ItemsToPlain(items)
		assert.Equal(t, "msg1\nmsg2\n", got)
		assert.NotContains(t, got, "Error")
		assert.NotContains(t, got, "Info")
	})
}

func TestMessageItemWithField(t *testing.T) {
	t.Parallel()

	t.Run("adds single field", func(t *testing.T) {
		t.Parallel()

		mi := &MessageItem{Text: "msg"}
		result := mi.WithField("key", "value")

		require.Len(t, mi.Fields, 1)
		assert.Equal(t, Field{Key: "key", Value: "value"}, mi.Fields[0])
		assert.Equal(t, mi, result)
	})

	t.Run("adds multiple fields", func(t *testing.T) {
		t.Parallel()

		mi := &MessageItem{Text: "msg"}
		mi.WithField("a", "1").WithField("b", "2")

		require.Len(t, mi.Fields, 2)
		assert.Equal(t, "a", mi.Fields[0].Key)
		assert.Equal(t, "1", mi.Fields[0].Value)
		assert.Equal(t, "b", mi.Fields[1].Key)
		assert.Equal(t, "2", mi.Fields[1].Value)
	})

	t.Run("empty key and value", func(t *testing.T) {
		t.Parallel()

		mi := &MessageItem{Text: "msg"}
		mi.WithField("", "")

		require.Len(t, mi.Fields, 1)
		assert.Empty(t, mi.Fields[0].Key)
		assert.Empty(t, mi.Fields[0].Value)
	})

	t.Run("unicode field", func(t *testing.T) {
		t.Parallel()

		mi := &MessageItem{Text: "msg"}
		mi.WithField(" alerts", "值")

		require.Len(t, mi.Fields, 1)
		assert.Equal(t, " alerts", mi.Fields[0].Key)
		assert.Equal(t, "值", mi.Fields[0].Value)
	})
}

func TestFieldsFromMap(t *testing.T) {
	t.Parallel()

	t.Run("empty map", func(t *testing.T) {
		t.Parallel()

		got := FieldsFromMap(map[string]string{}, false)
		assert.Empty(t, got)
	})

	t.Run("single entry unsorted", func(t *testing.T) {
		t.Parallel()

		got := FieldsFromMap(map[string]string{"b": "2"}, false)
		require.Len(t, got, 1)
		assert.Equal(t, "b", got[0].Key)
		assert.Equal(t, "2", got[0].Value)
	})

	t.Run("multiple entries sorted", func(t *testing.T) {
		t.Parallel()

		fieldMap := map[string]string{
			"c": "3",
			"a": "1",
			"b": "2",
		}
		got := FieldsFromMap(fieldMap, true)
		require.Len(t, got, 3)
		assert.Equal(t, "a", got[0].Key)
		assert.Equal(t, "1", got[0].Value)
		assert.Equal(t, "b", got[1].Key)
		assert.Equal(t, "2", got[1].Value)
		assert.Equal(t, "c", got[2].Key)
		assert.Equal(t, "3", got[2].Value)
	})

	t.Run("multiple entries unsorted", func(t *testing.T) {
		t.Parallel()

		fieldMap := map[string]string{
			"c": "3",
			"a": "1",
			"b": "2",
		}
		got := FieldsFromMap(fieldMap, false)
		require.Len(t, got, 3)

		keys := make([]string, len(got))
		for i, f := range got {
			keys[i] = f.Key
			assert.Equal(t, fieldMap[f.Key], f.Value)
		}

		assert.ElementsMatch(t, []string{"a", "b", "c"}, keys)
	})

	t.Run("empty values", func(t *testing.T) {
		t.Parallel()

		fieldMap := map[string]string{"a": "", "b": ""}
		got := FieldsFromMap(fieldMap, false)
		require.Len(t, got, 2)

		for _, f := range got {
			assert.Empty(t, f.Value)
		}
	})

	t.Run("unicode keys and values", func(t *testing.T) {
		t.Parallel()

		fieldMap := map[string]string{" alerts": "值"}
		got := FieldsFromMap(fieldMap, false)
		require.Len(t, got, 1)
		assert.Equal(t, " alerts", got[0].Key)
		assert.Equal(t, "值", got[0].Value)
	})
}

func TestMessageItemStruct(t *testing.T) {
	t.Parallel()

	t.Run("zero value", func(t *testing.T) {
		t.Parallel()

		mi := MessageItem{}
		assert.Empty(t, mi.Text)
		assert.True(t, mi.Timestamp.IsZero())
		assert.Equal(t, Unknown, mi.Level)
		assert.Nil(t, mi.File)
	})

	t.Run("populated values", func(t *testing.T) {
		t.Parallel()

		ts := time.Now()
		mi := MessageItem{
			Text:      "hello",
			Timestamp: ts,
			Level:     Error,
			Fields:    []Field{{Key: "k", Value: "v"}},
			File:      &File{Name: "test.txt", Data: []byte("data")},
		}

		assert.Equal(t, "hello", mi.Text)
		assert.Equal(t, ts, mi.Timestamp)
		assert.Equal(t, Error, mi.Level)
		require.Len(t, mi.Fields, 1)
		assert.Equal(t, "k", mi.Fields[0].Key)
		assert.Equal(t, "v", mi.Fields[0].Value)
		require.NotNil(t, mi.File)
		assert.Equal(t, "test.txt", mi.File.Name)
		assert.Equal(t, []byte("data"), mi.File.Data)
	})
}

func TestFileStruct(t *testing.T) {
	t.Parallel()

	t.Run("zero value", func(t *testing.T) {
		t.Parallel()

		f := File{}
		assert.Empty(t, f.Name)
		assert.Nil(t, f.Data)
	})

	t.Run("populated values", func(t *testing.T) {
		t.Parallel()

		f := File{Name: "report.pdf", Data: []byte("pdf content")}
		assert.Equal(t, "report.pdf", f.Name)
		assert.Equal(t, []byte("pdf content"), f.Data)
	})

	t.Run("empty data", func(t *testing.T) {
		t.Parallel()

		f := File{Name: "empty.txt", Data: []byte{}}
		assert.Equal(t, "empty.txt", f.Name)
		assert.Empty(t, f.Data)
	})
}

func TestMessageLimitStruct(t *testing.T) {
	t.Parallel()

	t.Run("zero value", func(t *testing.T) {
		t.Parallel()

		ml := MessageLimit{}
		assert.Equal(t, 0, ml.ChunkSize)
		assert.Equal(t, 0, ml.TotalChunkSize)
		assert.Equal(t, 0, ml.ChunkCount)
	})

	t.Run("populated values", func(t *testing.T) {
		t.Parallel()

		ml := MessageLimit{ChunkSize: 100, TotalChunkSize: 1000, ChunkCount: 10}
		assert.Equal(t, 100, ml.ChunkSize)
		assert.Equal(t, 1000, ml.TotalChunkSize)
		assert.Equal(t, 10, ml.ChunkCount)
	})
}
