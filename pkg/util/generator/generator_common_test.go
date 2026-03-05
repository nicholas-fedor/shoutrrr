package generator

import (
	"bytes"
	"errors"
	"io"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewUserDialog(t *testing.T) {
	t.Parallel()

	type args struct {
		reader io.Reader
		props  map[string]string
	}

	tests := []struct {
		name       string
		args       args
		wantWriter string
	}{
		{
			name: "with nil props creates empty map",
			args: args{
				reader: bytes.NewBufferString("test input\n"),
				props:  nil,
			},
			wantWriter: "",
		},
		{
			name: "with existing props preserves them",
			args: args{
				reader: bytes.NewBufferString("test input\n"),
				props: map[string]string{
					"key1": "value1",
					"key2": "value2",
				},
			},
			wantWriter: "",
		},
		{
			name: "with empty props map",
			args: args{
				reader: bytes.NewBufferString("test input\n"),
				props:  map[string]string{},
			},
			wantWriter: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			writer := &bytes.Buffer{}
			got := NewUserDialog(tt.args.reader, writer, tt.args.props)

			require.NotNil(t, got)
			assert.NotNil(t, got.scanner)
			assert.Equal(t, tt.args.reader, got.reader)
			assert.Equal(t, writer, got.writer)

			if tt.args.props == nil {
				assert.NotNil(t, got.props)
				assert.Empty(t, got.props)
			} else {
				assert.Equal(t, tt.args.props, got.props)
			}

			assert.Equal(t, tt.wantWriter, writer.String())
		})
	}
}

func TestUserDialog_Query(t *testing.T) {
	t.Parallel()

	emailRegex := regexp.MustCompile(`^([a-zA-Z0-9._%+-]+)@([a-zA-Z0-9.-]+)\.([a-zA-Z]{2,})$`)
	ipRegex := regexp.MustCompile(`^(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})$`)
	testRegex := regexp.MustCompile(`^test_(\w+)$`)

	type args struct {
		prompt    string
		validator *regexp.Regexp
		key       string
	}

	tests := []struct {
		name    string
		ud      *UserDialog
		args    args
		want    []string
		wantErr bool
		wantOut string
	}{
		{
			name: "valid email with capture groups",
			ud: NewUserDialog(
				bytes.NewBufferString("user@example.com\n"),
				&bytes.Buffer{},
				nil,
			),
			args: args{
				prompt:    "Enter email:",
				validator: emailRegex,
				key:       "email",
			},
			want: []string{"user@example.com", "user", "example", "com"},
		},
		{
			name: "valid IP address with capture groups",
			ud: NewUserDialog(
				bytes.NewBufferString("192.168.1.1\n"),
				&bytes.Buffer{},
				nil,
			),
			args: args{
				prompt:    "Enter IP:",
				validator: ipRegex,
				key:       "ip",
			},
			want: []string{"192.168.1.1", "192", "168", "1", "1"},
		},
		{
			name: "invalid input then valid input",
			ud: NewUserDialog(
				bytes.NewBufferString("invalid\nuser@example.com\n"),
				&bytes.Buffer{},
				nil,
			),
			args: args{
				prompt:    "Enter email:",
				validator: emailRegex,
				key:       "email",
			},
			want:    []string{"user@example.com", "user", "example", "com"},
			wantOut: "invalid format\n\n",
		},
		{
			name: "using prop value",
			ud: NewUserDialog(
				bytes.NewBufferString(""),
				&bytes.Buffer{},
				map[string]string{"email": "prop@example.com"},
			),
			args: args{
				prompt:    "Enter email:",
				validator: emailRegex,
				key:       "email",
			},
			want:    []string{"prop@example.com", "prop", "example", "com"},
			wantOut: "Using prop value prop@example.com for email",
		},
		{
			name: "invalid prop value falls back to interactive",
			ud: NewUserDialog(
				bytes.NewBufferString("valid@example.com\n"),
				&bytes.Buffer{},
				map[string]string{"email": "not-an-email"},
			),
			args: args{
				prompt:    "Enter email:",
				validator: emailRegex,
				key:       "email",
			},
			want: []string{"valid@example.com", "valid", "example", "com"},
			wantOut: "Supplied prop value not-an-email is not valid for email: " +
				"invalid format\nEnter email: ",
		},
		{
			name: "regex with word capture",
			ud: NewUserDialog(
				bytes.NewBufferString("test_hello\n"),
				&bytes.Buffer{},
				nil,
			),
			args: args{
				prompt:    "Enter test:",
				validator: testRegex,
				key:       "test",
			},
			want: []string{"test_hello", "hello"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.ud.Query(tt.args.prompt, tt.args.validator, tt.args.key)
			assert.Equal(t, tt.want, got)

			if tt.wantOut != "" {
				writer, ok := tt.ud.writer.(*bytes.Buffer)
				require.True(t, ok)
				assert.Contains(t, writer.String(), tt.wantOut)
			}
		})
	}
}

func TestUserDialog_QueryAll(t *testing.T) {
	t.Parallel()

	// Matches comma-separated words
	wordRegex := regexp.MustCompile(`\b(\w+)\b`)
	// Matches email addresses
	emailRegex := regexp.MustCompile(`([a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,})`)

	type args struct {
		prompt     string
		validator  *regexp.Regexp
		key        string
		maxMatches int
	}

	tests := []struct {
		name    string
		ud      *UserDialog
		args    args
		want    [][]string
		wantOut string
	}{
		{
			name: "multiple words with max matches",
			ud: NewUserDialog(
				bytes.NewBufferString("apple, banana, cherry\n"),
				&bytes.Buffer{},
				nil,
			),
			args: args{
				prompt:     "Enter fruits:",
				validator:  wordRegex,
				key:        "fruits",
				maxMatches: 3,
			},
			want: [][]string{
				{"apple", "apple"},
				{"banana", "banana"},
				{"cherry", "cherry"},
			},
		},
		{
			name: "multiple emails with unlimited matches",
			ud: NewUserDialog(
				bytes.NewBufferString("user1@test.com, user2@test.com\n"),
				&bytes.Buffer{},
				nil,
			),
			args: args{
				prompt:     "Enter emails:",
				validator:  emailRegex,
				key:        "emails",
				maxMatches: -1,
			},
			want: [][]string{
				{"user1@test.com", "user1@test.com"},
				{"user2@test.com", "user2@test.com"},
			},
		},
		{
			name: "limited matches",
			ud: NewUserDialog(
				bytes.NewBufferString("one two three four\n"),
				&bytes.Buffer{},
				nil,
			),
			args: args{
				prompt:     "Enter words:",
				validator:  wordRegex,
				key:        "words",
				maxMatches: 2,
			},
			want: [][]string{
				{"one", "one"},
				{"two", "two"},
			},
		},
		{
			name: "using prop value",
			ud: NewUserDialog(
				bytes.NewBufferString(""),
				&bytes.Buffer{},
				map[string]string{"items": "a, b, c"},
			),
			args: args{
				prompt:     "Enter items:",
				validator:  wordRegex,
				key:        "items",
				maxMatches: 3,
			},
			want: [][]string{
				{"a", "a"},
				{"b", "b"},
				{"c", "c"},
			},
		},
		{
			name: "no matches returns nil",
			ud: NewUserDialog(
				bytes.NewBufferString("!!!###@@@\nvalid\n"),
				&bytes.Buffer{},
				nil,
			),
			args: args{
				prompt:     "Enter words:",
				validator:  wordRegex,
				key:        "words",
				maxMatches: 1,
			},
			want:    [][]string{{"valid", "valid"}},
			wantOut: "invalid format\n\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.ud.QueryAll(tt.args.prompt, tt.args.validator, tt.args.key, tt.args.maxMatches)
			assert.Equal(t, tt.want, got)

			if tt.wantOut != "" {
				writer, ok := tt.ud.writer.(*bytes.Buffer)
				require.True(t, ok)
				assert.Contains(t, writer.String(), tt.wantOut)
			}
		})
	}
}

func TestUserDialog_QueryBool(t *testing.T) {
	t.Parallel()

	type args struct {
		prompt string
		key    string
	}

	tests := []struct {
		name    string
		ud      *UserDialog
		args    args
		want    bool
		wantOut string
	}{
		{
			name: "yes returns true",
			ud: NewUserDialog(
				bytes.NewBufferString("yes\n"),
				&bytes.Buffer{},
				nil,
			),
			args: args{
				prompt: "Continue?",
				key:    "continue",
			},
			want: true,
		},
		{
			name: "y returns true",
			ud: NewUserDialog(
				bytes.NewBufferString("y\n"),
				&bytes.Buffer{},
				nil,
			),
			args: args{
				prompt: "Continue?",
				key:    "continue",
			},
			want: true,
		},
		{
			name: "true returns true",
			ud: NewUserDialog(
				bytes.NewBufferString("true\n"),
				&bytes.Buffer{},
				nil,
			),
			args: args{
				prompt: "Enable?",
				key:    "enable",
			},
			want: true,
		},
		{
			name: "1 returns true",
			ud: NewUserDialog(
				bytes.NewBufferString("1\n"),
				&bytes.Buffer{},
				nil,
			),
			args: args{
				prompt: "Enable?",
				key:    "enable",
			},
			want: true,
		},
		{
			name: "no returns false",
			ud: NewUserDialog(
				bytes.NewBufferString("no\n"),
				&bytes.Buffer{},
				nil,
			),
			args: args{
				prompt: "Skip?",
				key:    "skip",
			},
			want: false,
		},
		{
			name: "n returns false",
			ud: NewUserDialog(
				bytes.NewBufferString("n\n"),
				&bytes.Buffer{},
				nil,
			),
			args: args{
				prompt: "Skip?",
				key:    "skip",
			},
			want: false,
		},
		{
			name: "false returns false",
			ud: NewUserDialog(
				bytes.NewBufferString("false\n"),
				&bytes.Buffer{},
				nil,
			),
			args: args{
				prompt: "Disable?",
				key:    "disable",
			},
			want: false,
		},
		{
			name: "0 returns false",
			ud: NewUserDialog(
				bytes.NewBufferString("0\n"),
				&bytes.Buffer{},
				nil,
			),
			args: args{
				prompt: "Disable?",
				key:    "disable",
			},
			want: false,
		},
		{
			name: "invalid then valid",
			ud: NewUserDialog(
				bytes.NewBufferString("maybe\nyes\n"),
				&bytes.Buffer{},
				nil,
			),
			args: args{
				prompt: "Continue?",
				key:    "continue",
			},
			want:    true,
			wantOut: "answer must be yes or no",
		},
		{
			name: "using prop value yes",
			ud: NewUserDialog(
				bytes.NewBufferString(""),
				&bytes.Buffer{},
				map[string]string{"enabled": "yes"},
			),
			args: args{
				prompt: "Enabled?",
				key:    "enabled",
			},
			want: true,
		},
		{
			name: "using prop value no",
			ud: NewUserDialog(
				bytes.NewBufferString(""),
				&bytes.Buffer{},
				map[string]string{"disabled": "no"},
			),
			args: args{
				prompt: "Disabled?",
				key:    "disabled",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.ud.QueryBool(tt.args.prompt, tt.args.key)
			assert.Equal(t, tt.want, got)

			if tt.wantOut != "" {
				writer, ok := tt.ud.writer.(*bytes.Buffer)
				require.True(t, ok)
				assert.Contains(t, writer.String(), tt.wantOut)
			}
		})
	}
}

func TestUserDialog_QueryInt(t *testing.T) {
	t.Parallel()

	type args struct {
		prompt  string
		key     string
		bitSize int
	}

	tests := []struct {
		name    string
		ud      *UserDialog
		args    args
		want    int64
		wantOut string
	}{
		{
			name: "decimal positive number",
			ud: NewUserDialog(
				bytes.NewBufferString("42\n"),
				&bytes.Buffer{},
				nil,
			),
			args: args{
				prompt:  "Enter number:",
				key:     "number",
				bitSize: 64,
			},
			want: 42,
		},
		{
			name: "decimal negative number",
			ud: NewUserDialog(
				bytes.NewBufferString("-123\n"),
				&bytes.Buffer{},
				nil,
			),
			args: args{
				prompt:  "Enter number:",
				key:     "number",
				bitSize: 64,
			},
			want: -123,
		},
		{
			name: "zero",
			ud: NewUserDialog(
				bytes.NewBufferString("0\n"),
				&bytes.Buffer{},
				nil,
			),
			args: args{
				prompt:  "Enter number:",
				key:     "number",
				bitSize: 64,
			},
			want: 0,
		},
		{
			name: "hex with 0x prefix",
			ud: NewUserDialog(
				bytes.NewBufferString("0xFF\n"),
				&bytes.Buffer{},
				nil,
			),
			args: args{
				prompt:  "Enter hex:",
				key:     "hex",
				bitSize: 64,
			},
			want: 255,
		},
		{
			name: "hex with # prefix",
			ud: NewUserDialog(
				bytes.NewBufferString("#ffa080\n"),
				&bytes.Buffer{},
				nil,
			),
			args: args{
				prompt:  "Enter color:",
				key:     "color",
				bitSize: 64,
			},
			want: 0xFFA080,
		},
		{
			name: "hex lowercase",
			ud: NewUserDialog(
				bytes.NewBufferString("0xabc\n"),
				&bytes.Buffer{},
				nil,
			),
			args: args{
				prompt:  "Enter hex:",
				key:     "hex",
				bitSize: 64,
			},
			want: 2748,
		},
		{
			name: "invalid then valid",
			ud: NewUserDialog(
				bytes.NewBufferString("abc\n100\n"),
				&bytes.Buffer{},
				nil,
			),
			args: args{
				prompt:  "Enter number:",
				key:     "number",
				bitSize: 64,
			},
			want:    100,
			wantOut: "not a number\n\n",
		},
		{
			name: "using prop value",
			ud: NewUserDialog(
				bytes.NewBufferString(""),
				&bytes.Buffer{},
				map[string]string{"port": "8080"},
			),
			args: args{
				prompt:  "Enter port:",
				key:     "port",
				bitSize: 16,
			},
			want: 8080,
		},
		{
			name: "with int32 bitSize",
			ud: NewUserDialog(
				bytes.NewBufferString("12345\n"),
				&bytes.Buffer{},
				nil,
			),
			args: args{
				prompt:  "Enter number:",
				key:     "number",
				bitSize: 32,
			},
			want: 12345,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.ud.QueryInt(tt.args.prompt, tt.args.key, tt.args.bitSize)
			assert.Equal(t, tt.want, got)

			if tt.wantOut != "" {
				writer, ok := tt.ud.writer.(*bytes.Buffer)
				require.True(t, ok)
				assert.Contains(t, writer.String(), tt.wantOut)
			}
		})
	}
}

func TestUserDialog_QueryString(t *testing.T) {
	t.Parallel()

	noSpacesValidator := func(s string) error {
		for _, r := range s {
			if r == ' ' {
				return errors.New("spaces not allowed")
			}
		}

		return nil
	}

	type args struct {
		prompt    string
		validator func(string) error
		key       string
	}

	tests := []struct {
		name    string
		ud      *UserDialog
		args    args
		want    string
		wantOut string
	}{
		{
			name: "valid input without validator",
			ud: NewUserDialog(
				bytes.NewBufferString("hello\n"),
				&bytes.Buffer{},
				nil,
			),
			args: args{
				prompt:    "Enter name:",
				validator: nil,
				key:       "name",
			},
			want: "hello",
		},
		{
			name: "valid input with validator",
			ud: NewUserDialog(
				bytes.NewBufferString("testuser\n"),
				&bytes.Buffer{},
				nil,
			),
			args: args{
				prompt:    "Enter username:",
				validator: noSpacesValidator,
				key:       "username",
			},
			want: "testuser",
		},
		{
			name: "invalid then valid",
			ud: NewUserDialog(
				bytes.NewBufferString("hello world\ngoodbye\n"),
				&bytes.Buffer{},
				nil,
			),
			args: args{
				prompt:    "Enter name:",
				validator: noSpacesValidator,
				key:       "name",
			},
			want:    "goodbye",
			wantOut: "spaces not allowed\n\n",
		},
		{
			name: "empty input allowed without validator",
			ud: NewUserDialog(
				bytes.NewBufferString("\n"),
				&bytes.Buffer{},
				nil,
			),
			args: args{
				prompt:    "Enter (optional):",
				validator: nil,
				key:       "optional",
			},
			want: "",
		},
		{
			name: "using prop value",
			ud: NewUserDialog(
				bytes.NewBufferString(""),
				&bytes.Buffer{},
				map[string]string{"name": "propvalue"},
			),
			args: args{
				prompt:    "Enter name:",
				validator: nil,
				key:       "name",
			},
			want:    "propvalue",
			wantOut: "Using prop value",
		},
		{
			name: "invalid prop value falls back to interactive",
			ud: NewUserDialog(
				bytes.NewBufferString("validinput\n"),
				&bytes.Buffer{},
				map[string]string{"name": "has space"},
			),
			args: args{
				prompt:    "Enter name:",
				validator: noSpacesValidator,
				key:       "name",
			},
			want:    "validinput",
			wantOut: "is not valid for",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.ud.QueryString(tt.args.prompt, tt.args.validator, tt.args.key)
			assert.Equal(t, tt.want, got)

			if tt.wantOut != "" {
				writer, ok := tt.ud.writer.(*bytes.Buffer)
				require.True(t, ok)
				assert.Contains(t, writer.String(), tt.wantOut)
			}
		})
	}
}

func TestUserDialog_QueryStringPattern(t *testing.T) {
	t.Parallel()

	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	alphaRegex := regexp.MustCompile(`^[a-zA-Z]+$`)
	digitRegex := regexp.MustCompile(`^\d+$`)

	type args struct {
		prompt    string
		validator *regexp.Regexp
		key       string
	}

	tests := []struct {
		name      string
		ud        *UserDialog
		args      args
		want      string
		wantPanic bool
		wantOut   string
	}{
		{
			name: "valid email",
			ud: NewUserDialog(
				bytes.NewBufferString("user@example.com\n"),
				&bytes.Buffer{},
				nil,
			),
			args: args{
				prompt:    "Enter email:",
				validator: emailRegex,
				key:       "email",
			},
			want: "user@example.com",
		},
		{
			name: "valid alphabetic",
			ud: NewUserDialog(
				bytes.NewBufferString("HelloWorld\n"),
				&bytes.Buffer{},
				nil,
			),
			args: args{
				prompt:    "Enter text:",
				validator: alphaRegex,
				key:       "text",
			},
			want: "HelloWorld",
		},
		{
			name: "valid digits",
			ud: NewUserDialog(
				bytes.NewBufferString("12345\n"),
				&bytes.Buffer{},
				nil,
			),
			args: args{
				prompt:    "Enter number:",
				validator: digitRegex,
				key:       "number",
			},
			want: "12345",
		},
		{
			name: "invalid then valid",
			ud: NewUserDialog(
				bytes.NewBufferString("123\nvalidemail@test.com\n"),
				&bytes.Buffer{},
				nil,
			),
			args: args{
				prompt:    "Enter email:",
				validator: emailRegex,
				key:       "email",
			},
			want:    "validemail@test.com",
			wantOut: "invalid format\n\n",
		},
		{
			name: "using prop value",
			ud: NewUserDialog(
				bytes.NewBufferString(""),
				&bytes.Buffer{},
				map[string]string{"email": "prop@test.com"},
			),
			args: args{
				prompt:    "Enter email:",
				validator: emailRegex,
				key:       "email",
			},
			want:    "prop@test.com",
			wantOut: "Using prop value",
		},
		{
			name: "nil validator panics",
			ud: NewUserDialog(
				bytes.NewBufferString("test\n"),
				&bytes.Buffer{},
				nil,
			),
			args: args{
				prompt:    "Enter value:",
				validator: nil,
				key:       "value",
			},
			wantPanic: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.wantPanic {
				assert.Panics(t, func() {
					tt.ud.QueryStringPattern(tt.args.prompt, tt.args.validator, tt.args.key)
				})

				return
			}

			got := tt.ud.QueryStringPattern(tt.args.prompt, tt.args.validator, tt.args.key)
			assert.Equal(t, tt.want, got)

			if tt.wantOut != "" {
				writer, ok := tt.ud.writer.(*bytes.Buffer)
				require.True(t, ok)
				assert.Contains(t, writer.String(), tt.wantOut)
			}
		})
	}
}

func TestUserDialog_Write(t *testing.T) {
	t.Parallel()

	type args struct {
		message string
		v       []any
	}

	tests := []struct {
		name       string
		ud         *UserDialog
		args       args
		wantWriter string
	}{
		{
			name: "simple message",
			ud: NewUserDialog(
				bytes.NewBufferString(""),
				&bytes.Buffer{},
				nil,
			),
			args: args{
				message: "Hello, World!",
				v:       nil,
			},
			wantWriter: "Hello, World!",
		},
		{
			name: "formatted message with string",
			ud: NewUserDialog(
				bytes.NewBufferString(""),
				&bytes.Buffer{},
				nil,
			),
			args: args{
				message: "Hello, %s!",
				v:       []any{"Alice"},
			},
			wantWriter: "Hello, Alice!",
		},
		{
			name: "formatted message with integer",
			ud: NewUserDialog(
				bytes.NewBufferString(""),
				&bytes.Buffer{},
				nil,
			),
			args: args{
				message: "Count: %d",
				v:       []any{42},
			},
			wantWriter: "Count: 42",
		},
		{
			name: "formatted message with multiple values",
			ud: NewUserDialog(
				bytes.NewBufferString(""),
				&bytes.Buffer{},
				nil,
			),
			args: args{
				message: "Name: %s, Age: %d, Score: %.2f",
				v:       []any{"Bob", 25, 95.5},
			},
			wantWriter: "Name: Bob, Age: 25, Score: 95.50",
		},
		{
			name: "empty message",
			ud: NewUserDialog(
				bytes.NewBufferString(""),
				&bytes.Buffer{},
				nil,
			),
			args: args{
				message: "",
				v:       nil,
			},
			wantWriter: "",
		},
		{
			name: "message with special characters",
			ud: NewUserDialog(
				bytes.NewBufferString(""),
				&bytes.Buffer{},
				nil,
			),
			args: args{
				message: "Special: %s",
				v:       []any{"!@#$%^&*()"},
			},
			wantWriter: "Special: !@#$%^&*()",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tt.ud.Write(tt.args.message, tt.args.v...)

			writer, ok := tt.ud.writer.(*bytes.Buffer)
			require.True(t, ok)
			assert.Equal(t, tt.wantWriter, writer.String())
		})
	}
}

func TestUserDialog_Writelnf(t *testing.T) {
	t.Parallel()

	type args struct {
		fmtStr string
		v      []any
	}

	tests := []struct {
		name       string
		ud         *UserDialog
		args       args
		wantWriter string
	}{
		{
			name: "simple message",
			ud: NewUserDialog(
				bytes.NewBufferString(""),
				&bytes.Buffer{},
				nil,
			),
			args: args{
				fmtStr: "Hello, World!",
				v:      nil,
			},
			wantWriter: "Hello, World!\n",
		},
		{
			name: "formatted message with string",
			ud: NewUserDialog(
				bytes.NewBufferString(""),
				&bytes.Buffer{},
				nil,
			),
			args: args{
				fmtStr: "Hello, %s!",
				v:      []any{"Alice"},
			},
			wantWriter: "Hello, Alice!\n",
		},
		{
			name: "formatted message with integer",
			ud: NewUserDialog(
				bytes.NewBufferString(""),
				&bytes.Buffer{},
				nil,
			),
			args: args{
				fmtStr: "Count: %d",
				v:      []any{42},
			},
			wantWriter: "Count: 42\n",
		},
		{
			name: "formatted message with multiple values",
			ud: NewUserDialog(
				bytes.NewBufferString(""),
				&bytes.Buffer{},
				nil,
			),
			args: args{
				fmtStr: "Name: %s, Age: %d",
				v:      []any{"Bob", 25},
			},
			wantWriter: "Name: Bob, Age: 25\n",
		},
		{
			name: "empty message still adds newline",
			ud: NewUserDialog(
				bytes.NewBufferString(""),
				&bytes.Buffer{},
				nil,
			),
			args: args{
				fmtStr: "",
				v:      nil,
			},
			wantWriter: "\n",
		},
		{
			name: "error message",
			ud: NewUserDialog(
				bytes.NewBufferString(""),
				&bytes.Buffer{},
				nil,
			),
			args: args{
				fmtStr: "Error: %s",
				v:      []any{"something went wrong"},
			},
			wantWriter: "Error: something went wrong\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tt.ud.Writelnf(tt.args.fmtStr, tt.args.v...)

			writer, ok := tt.ud.writer.(*bytes.Buffer)
			require.True(t, ok)
			assert.Equal(t, tt.wantWriter, writer.String())
		})
	}
}

func TestValidateFormat(t *testing.T) {
	t.Parallel()

	type args struct {
		validator func(string) bool
	}

	tests := []struct {
		name      string
		args      args
		input     string
		wantErr   bool
		wantError error
	}{
		{
			name: "validator returns true",
			args: args{
				validator: func(s string) bool {
					return s == "valid"
				},
			},
			input:   "valid",
			wantErr: false,
		},
		{
			name: "validator returns false",
			args: args{
				validator: func(s string) bool {
					return s == "valid"
				},
			},
			input:     "invalid",
			wantErr:   true,
			wantError: errInvalidFormat,
		},
		{
			name: "validator checks non-empty",
			args: args{
				validator: func(s string) bool {
					return s != ""
				},
			},
			input:   "not empty",
			wantErr: false,
		},
		{
			name: "validator fails on empty",
			args: args{
				validator: func(s string) bool {
					return s != ""
				},
			},
			input:     "",
			wantErr:   true,
			wantError: errInvalidFormat,
		},
		{
			name: "validator checks prefix",
			args: args{
				validator: func(s string) bool {
					return len(s) >= 3 && s[:3] == "pre"
				},
			},
			input:   "prefix",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := ValidateFormat(tt.args.validator)
			require.NotNil(t, got)

			err := got(tt.input)
			if tt.wantErr {
				require.Error(t, err)

				if tt.wantError != nil {
					require.ErrorIs(t, err, tt.wantError)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestRequired(t *testing.T) {
	t.Parallel()

	type args struct {
		answer string
	}

	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "non-empty string",
			args:    args{answer: "hello"},
			wantErr: false,
		},
		{
			name:    "single character",
			args:    args{answer: "a"},
			wantErr: false,
		},
		{
			name:    "empty string",
			args:    args{answer: ""},
			wantErr: true,
		},
		{
			name:    "whitespace only",
			args:    args{answer: "   "},
			wantErr: false,
		},
		{
			name:    "string with spaces",
			args:    args{answer: "hello world"},
			wantErr: false,
		},
		{
			name:    "special characters",
			args:    args{answer: "!@#$%"},
			wantErr: false,
		},
		{
			name:    "numeric string",
			args:    args{answer: "12345"},
			wantErr: false,
		},
		{
			name:    "unicode characters",
			args:    args{answer: "Hello \u4e16\u754c"},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := Required(tt.args.answer)
			if tt.wantErr {
				require.Error(t, err)
				assert.ErrorIs(t, err, errRequired)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
