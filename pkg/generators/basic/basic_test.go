package basic

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"text/template"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// mockConfig implements types.ServiceConfig.
type mockConfig struct {
	Host string `default:"localhost" key:"host"`
	Port int    `default:"8080"      key:"port" required:"true"`
	url  *url.URL
}

// mockServiceConfig is a test implementation of Service.
type mockServiceConfig struct {
	Config *mockConfig
}

func (m *mockConfig) Enums() map[string]types.EnumFormatter {
	return nil
}

// ConfigQueryResolver methods.
func (m *mockConfig) Get(key string) (string, error) {
	switch strings.ToLower(key) {
	case "host":
		return m.Host, nil
	case "port":
		return strconv.Itoa(m.Port), nil
	default:
		return "", fmt.Errorf("unknown key: %s", key)
	}
}

func (m *mockConfig) GetPropValue() (string, error) {
	// Minimal implementation for testing
	return fmt.Sprintf("%s:%d", m.Host, m.Port), nil
}

func (m *mockConfig) GetURL() *url.URL {
	if m.url == nil {
		u, _ := url.Parse("mock://url")
		m.url = u
	}

	return m.url
}

func (m *mockConfig) QueryFields() []string {
	return []string{"host", "port"}
}

func (m *mockConfig) Set(key, value string) error {
	switch strings.ToLower(key) {
	case "host":
		m.Host = value

		return nil
	case "port":
		port, err := strconv.Atoi(value)
		if err != nil {
			return err
		}

		m.Port = port

		return nil
	default:
		return fmt.Errorf("unknown key: %s", key)
	}
}

// ConfigProp methods.
func (m *mockConfig) SetFromProp(propValue string) error {
	// Minimal implementation for testing; typically parses propValue
	parts := strings.SplitN(propValue, ":", 2)
	if len(parts) == 2 {
		m.Host = parts[0]

		port, err := strconv.Atoi(parts[1])
		if err != nil {
			return err
		}

		m.Port = port
	}

	return nil
}

func (m *mockConfig) SetLogger(_ types.StdLogger) {
	// Minimal implementation, no-op
}

func (m *mockConfig) SetTemplateFile(_, _ string) error {
	return nil
}

func (m *mockConfig) SetTemplateString(_, _ string) error {
	return nil
}

func (m *mockConfig) SetURL(u *url.URL) error {
	m.url = u

	return nil
}

func (m *mockServiceConfig) GetID() string {
	return "mockID"
}

func (m *mockServiceConfig) GetTemplate(_ string) (*template.Template, bool) {
	return nil, false
}

func (m *mockServiceConfig) Initialize(_ *url.URL, _ types.StdLogger) error {
	return nil
}

func (m *mockServiceConfig) Send(_ string, _ *types.Params) error {
	return nil
}

func (m *mockServiceConfig) SetLogger(_ types.StdLogger) {}

func (m *mockServiceConfig) SetTemplateFile(_, _ string) error {
	return nil
}

func (m *mockServiceConfig) SetTemplateString(_, _ string) error {
	return nil
}

func TestGenerator_Generate(t *testing.T) {
	tests := []struct {
		name    string
		props   map[string]string
		input   string
		want    types.ServiceConfig
		wantErr bool
	}{
		{
			name:  "successful generation with defaults",
			props: map[string]string{},
			input: "\n8080\n",
			want: &mockConfig{
				Host: "localhost",
				Port: 8080,
			},
			wantErr: false,
		},
		{
			name:  "successful generation with props",
			props: map[string]string{"host": "example.com", "port": "9090"},
			input: "",
			want: &mockConfig{
				Host: "example.com",
				Port: 9090,
			},
			wantErr: false,
		},
		{
			name:    "error_on_invalid_port",
			props:   map[string]string{},
			input:   "\ninvalid\n",
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &Generator{}

			// Set up pipe for stdin
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}

			originalStdin := os.Stdin
			os.Stdin = r

			defer func() {
				os.Stdin = originalStdin

				_ = w.Close()
			}()

			// Write input to the pipe
			_, err = w.WriteString(tt.input)
			if err != nil {
				t.Fatal(err)
			}

			_ = w.Close()

			service := newMockServiceConfig()

			got, err := g.Generate(service, tt.props, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("Generate() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Generate() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestGenerator_promptUserForFields(t *testing.T) {
	tests := []struct {
		name    string
		config  reflect.Value
		props   map[string]string
		input   string
		wantErr bool
	}{
		{
			name:    "valid input with defaults",
			config:  reflect.ValueOf(newMockServiceConfig().Config), // Pass *mockConfig
			props:   map[string]string{},
			input:   "\n8080\n",
			wantErr: false,
		},
		{
			name:    "valid props",
			config:  reflect.ValueOf(newMockServiceConfig().Config), // Pass *mockConfig
			props:   map[string]string{"host": "test.com", "port": "1234"},
			input:   "",
			wantErr: false,
		},
		{
			name:    "invalid config type",
			config:  reflect.ValueOf("not a config"),
			props:   map[string]string{},
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &Generator{}
			scanner := bufio.NewScanner(strings.NewReader(tt.input))

			err := g.promptUserForFields(tt.config, tt.props, scanner)
			if (err != nil) != tt.wantErr {
				t.Errorf("promptUserForFields() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err == nil && tt.config.Kind() == reflect.Ptr &&
				tt.config.Type().Elem().Kind() == reflect.Struct {
				got := tt.config.Interface().(*mockConfig)
				if tt.props["host"] != "" && got.Host != tt.props["host"] {
					t.Errorf("promptUserForFields() host = %v, want %v", got.Host, tt.props["host"])
				}

				if tt.props["port"] != "" {
					wantPort := atoiOrZero(tt.props["port"])
					if got.Port != wantPort {
						t.Errorf("promptUserForFields() port = %v, want %v", got.Port, wantPort)
					}
				}
			}
		})
	}
}

func TestGenerator_getInputValue(t *testing.T) {
	tests := []struct {
		name    string
		field   *format.FieldInfo
		propKey string
		props   map[string]string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:    "from props",
			field:   &format.FieldInfo{Name: "Host"},
			propKey: "host",
			props:   map[string]string{"host": "example.com"},
			input:   "",
			want:    "example.com",
			wantErr: false,
		},
		{
			name:    "from user input",
			field:   &format.FieldInfo{Name: "Port", Type: reflect.TypeFor[int]()}, // Add Type
			propKey: "port",
			props:   map[string]string{},
			input:   "8080\n",
			want:    "8080",
			wantErr: false,
		},
		{
			name:    "default value",
			field:   &format.FieldInfo{Name: "Host", DefaultValue: "localhost"},
			propKey: "host",
			props:   map[string]string{},
			input:   "\n",
			want:    "localhost",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &Generator{}
			scanner := bufio.NewScanner(strings.NewReader(tt.input))

			got, err := g.getInputValue(tt.field, tt.propKey, tt.props, scanner)
			if (err != nil) != tt.wantErr {
				t.Errorf("getInputValue() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if got != tt.want {
				t.Errorf("getInputValue() = %v, want %v", got, tt.want)
			}

			if tt.props[tt.propKey] != "" {
				t.Errorf("getInputValue() did not clear prop, got %v", tt.props[tt.propKey])
			}
		})
	}
}

func TestGenerator_formatPrompt(t *testing.T) {
	tests := []struct {
		name  string
		field *format.FieldInfo
		want  string
	}{
		{
			name:  "field with default",
			field: &format.FieldInfo{Name: "Host", DefaultValue: "localhost"},
			want:  "Host[localhost]: ",
		},
		{
			name:  "field without default",
			field: &format.FieldInfo{Name: "Port"},
			want:  "Port: ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &Generator{}

			got := g.formatPrompt(tt.field)
			if got != tt.want {
				t.Errorf("formatPrompt() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGenerator_setFieldValue(t *testing.T) {
	tests := []struct {
		name       string
		config     reflect.Value
		field      *format.FieldInfo
		inputValue string
		want       bool
		wantErr    bool
	}{
		{
			name:       "valid value",
			config:     reflect.ValueOf(newMockServiceConfig().Config).Elem(),
			field:      &format.FieldInfo{Name: "Port", Type: reflect.TypeFor[int](), Required: true},
			inputValue: "8080",
			want:       true,
			wantErr:    false,
		},
		{
			name:       "required field empty",
			config:     reflect.ValueOf(newMockServiceConfig().Config).Elem(),
			field:      &format.FieldInfo{Name: "Port", Type: reflect.TypeFor[int](), Required: true},
			inputValue: "",
			want:       false,
			wantErr:    false,
		},
		{
			name:       "invalid value",
			config:     reflect.ValueOf(newMockServiceConfig().Config).Elem(),
			field:      &format.FieldInfo{Name: "Port", Type: reflect.TypeFor[int]()},
			inputValue: "invalid",
			want:       false,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &Generator{}

			got, err := g.setFieldValue(tt.config, tt.field, tt.inputValue)
			if (err != nil) != tt.wantErr {
				t.Errorf("setFieldValue() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if got != tt.want {
				t.Errorf("setFieldValue() = %v, want %v", got, tt.want)
			}

			if got && !tt.wantErr {
				if tt.field.Name == "Port" {
					wantPort := atoiOrZero(tt.inputValue)
					if gotPort := tt.config.FieldByName("Port").Int(); int(gotPort) != wantPort {
						t.Errorf("setFieldValue() set Port = %v, want %v", gotPort, wantPort)
					}
				}
			}
		})
	}
}

func TestGenerator_printError(t *testing.T) {
	tests := []struct {
		name      string
		fieldName string
		errorMsg  string
	}{
		{
			name:      "basic error",
			fieldName: "Port",
			errorMsg:  "invalid format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(*testing.T) {
			g := &Generator{}

			g.printError(tt.fieldName, tt.errorMsg)
		})
	}
}

func TestGenerator_printInvalidType(t *testing.T) {
	tests := []struct {
		name      string
		fieldName string
		typeName  string
	}{
		{
			name:      "invalid type",
			fieldName: "Port",
			typeName:  "int",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(*testing.T) {
			g := &Generator{}

			g.printInvalidType(tt.fieldName, tt.typeName)
		})
	}
}

func TestGenerator_validateAndReturnConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  reflect.Value
		want    types.ServiceConfig
		wantErr bool
	}{
		{
			name:    "valid config",
			config:  reflect.ValueOf(&mockConfig{Host: "test", Port: 1234}),
			want:    &mockConfig{Host: "test", Port: 1234},
			wantErr: false,
		},
		{
			name:    "invalid config type",
			config:  reflect.ValueOf("not a config"),
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &Generator{}

			got, err := g.validateAndReturnConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateAndReturnConfig() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("validateAndReturnConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}

// atoiOrZero converts a string to an int, returning 0 on error.
func atoiOrZero(s string) int {
	i, _ := strconv.Atoi(s)

	return i
}

// newMockServiceConfig creates a new mockServiceConfig with an initialized Config.
func newMockServiceConfig() *mockServiceConfig {
	return &mockServiceConfig{
		Config: &mockConfig{},
	}
}
