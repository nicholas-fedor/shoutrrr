package basic

import (
	"bufio"
	"errors"
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

// mockServiceNilConfig is a test service with nil Config field (simulating uninitialized service).
type mockServiceNilConfig struct {
	Config *mockConfig // nil by default
}

// mockServiceWithoutConfig is a test service without a Config field.
type mockServiceWithoutConfig struct{}

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

func (m *mockServiceNilConfig) GetID() string {
	return "mockNilConfig"
}

func (m *mockServiceNilConfig) GetTemplate(_ string) (*template.Template, bool) {
	return nil, false
}

func (m *mockServiceNilConfig) Initialize(_ *url.URL, _ types.StdLogger) error {
	return nil
}

func (m *mockServiceNilConfig) Send(_ string, _ *types.Params) error {
	return nil
}

func (m *mockServiceNilConfig) SetLogger(_ types.StdLogger) {}

func (m *mockServiceNilConfig) SetTemplateFile(_, _ string) error {
	return nil
}

func (m *mockServiceNilConfig) SetTemplateString(_, _ string) error {
	return nil
}

func (m *mockServiceWithoutConfig) GetID() string {
	return "mockNoConfig"
}

func (m *mockServiceWithoutConfig) GetTemplate(_ string) (*template.Template, bool) {
	return nil, false
}

func (m *mockServiceWithoutConfig) Initialize(_ *url.URL, _ types.StdLogger) error {
	return nil
}

func (m *mockServiceWithoutConfig) Send(_ string, _ *types.Params) error {
	return nil
}

func (m *mockServiceWithoutConfig) SetLogger(_ types.StdLogger) {}

func (m *mockServiceWithoutConfig) SetTemplateFile(_, _ string) error {
	return nil
}

func (m *mockServiceWithoutConfig) SetTemplateString(_, _ string) error {
	return nil
}

func TestGenerator_Generate(t *testing.T) {
	t.Parallel()

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
			t.Parallel()

			// Set up pipe for stdin simulation
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}

			// Use dependency injection instead of global os.Stdin manipulation
			g := &Generator{Input: r}

			defer func() {
				_ = w.Close()
				_ = r.Close()
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
	t.Parallel()

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
			t.Parallel()

			g := &Generator{}
			scanner := bufio.NewScanner(strings.NewReader(tt.input))

			err := g.promptUserForFields(tt.config, tt.props, scanner)
			if (err != nil) != tt.wantErr {
				t.Errorf("promptUserForFields() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err == nil && tt.config.Kind() == reflect.Pointer &&
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
	t.Parallel()

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
			t.Parallel()

			g := &Generator{}
			scanner := bufio.NewScanner(strings.NewReader(tt.input))
			consumed := make(map[string]struct{})

			got, err := g.getInputValue(tt.field, tt.propKey, tt.props, consumed, scanner)
			if (err != nil) != tt.wantErr {
				t.Errorf("getInputValue() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if got != tt.want {
				t.Errorf("getInputValue() = %v, want %v", got, tt.want)
			}

			// Verify that props from input are marked as consumed
			if _, wasConsumed := consumed[tt.propKey]; tt.props[tt.propKey] != "" && !wasConsumed {
				t.Errorf("getInputValue() did not mark prop as consumed")
			}
		})
	}
}

func TestGenerator_formatPrompt(t *testing.T) {
	t.Parallel()

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
			t.Parallel()

			g := &Generator{}

			got := g.formatPrompt(tt.field)
			if got != tt.want {
				t.Errorf("formatPrompt() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGenerator_setFieldValue(t *testing.T) {
	t.Parallel()

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
			t.Parallel()

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
	t.Parallel()

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
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			g := &Generator{}

			g.printError(tt.fieldName, tt.errorMsg)
		})
	}
}

func TestGenerator_printInvalidType(t *testing.T) {
	t.Parallel()

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
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			g := &Generator{}

			g.printInvalidType(tt.fieldName, tt.typeName)
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

// TestGenerator_NilConfigInitialization tests that a service with nil Config gets properly initialized.
// This is a regression test for the nil config fix in basic.go:45-53.
func TestGenerator_NilConfigInitialization(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		service types.Service
		props   map[string]string
		input   string
		want    *mockConfig
		wantErr bool
	}{
		{
			name:    "nil config gets initialized with defaults",
			service: &mockServiceNilConfig{Config: nil},
			props:   map[string]string{},
			input:   "\n8080\n",
			want: &mockConfig{
				Host: "localhost",
				Port: 8080,
			},
			wantErr: false,
		},
		{
			name:    "nil config gets initialized with provided props",
			service: &mockServiceNilConfig{Config: nil},
			props:   map[string]string{"host": "example.com", "port": "9090"},
			input:   "",
			want: &mockConfig{
				Host: "example.com",
				Port: 9090,
			},
			wantErr: false,
		},
		{
			name:    "nil config error on invalid input",
			service: &mockServiceNilConfig{Config: nil},
			props:   map[string]string{},
			input:   "\ninvalid\n",
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Set up pipe for stdin simulation
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}

			g := &Generator{Input: r}

			defer func() {
				_ = w.Close()
				_ = r.Close()
			}()

			// Write input to the pipe
			_, err = w.WriteString(tt.input)
			if err != nil {
				t.Fatal(err)
			}

			_ = w.Close()

			got, err := g.Generate(tt.service, tt.props, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("Generate() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if !tt.wantErr {
				gotConfig, ok := got.(*mockConfig)
				if !ok {
					t.Errorf("Generate() returned type %T, want *mockConfig", got)

					return
				}

				// Verify the config was initialized (not nil)
				if gotConfig == nil {
					t.Error("Generate() returned nil config, expected initialized config")

					return
				}

				if gotConfig.Host != tt.want.Host {
					t.Errorf("Generate() host = %v, want %v", gotConfig.Host, tt.want.Host)
				}

				if gotConfig.Port != tt.want.Port {
					t.Errorf("Generate() port = %v, want %v", gotConfig.Port, tt.want.Port)
				}

				// Also verify that the service's internal Config field was initialized
				if mockSvc, ok := tt.service.(*mockServiceNilConfig); ok {
					if mockSvc.Config == nil {
						t.Error("Generate() left service.Config nil, expected initialized config")
					}

					// Optionally verify Host/Port match
					if mockSvc.Config != nil && mockSvc.Config.Host != tt.want.Host {
						t.Errorf("service.Config.Host = %v, want %v", mockSvc.Config.Host, tt.want.Host)
					}

					if mockSvc.Config != nil && mockSvc.Config.Port != tt.want.Port {
						t.Errorf("service.Config.Port = %v, want %v", mockSvc.Config.Port, tt.want.Port)
					}
				}
			}
		})
	}
}

// TestGenerator_DefaultsAndPropsOverrideExistingConfig tests that provided props
// override the default values in an existing config.
func TestGenerator_DefaultsAndPropsOverrideExistingConfig(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		service *mockServiceConfig
		props   map[string]string
		input   string
		want    *mockConfig
		wantErr bool
	}{
		{
			name:    "props override defaults",
			service: &mockServiceConfig{Config: &mockConfig{Host: "existing.com", Port: 3000}},
			props:   map[string]string{"port": "9000"},
			input:   "",
			want: &mockConfig{
				Host: "localhost", // defaults are used when not in props
				Port: 9000,
			},
			wantErr: false,
		},
		{
			name:    "user input overrides defaults",
			service: &mockServiceConfig{Config: &mockConfig{Host: "original.com", Port: 4000}},
			props:   map[string]string{},
			input:   "\n5000\n",
			want: &mockConfig{
				Host: "localhost", // defaults are used when not in props
				Port: 5000,
			},
			wantErr: false,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Set up pipe for stdin simulation
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}

			g := &Generator{Input: r}

			defer func() {
				_ = w.Close()
				_ = r.Close()
			}()

			// Write input to the pipe
			_, err = w.WriteString(tt.input)
			if err != nil {
				t.Fatal(err)
			}

			_ = w.Close()

			got, err := g.Generate(tt.service, tt.props, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("Generate() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if !tt.wantErr {
				gotConfig, ok := got.(*mockConfig)
				if !ok {
					t.Errorf("Generate() returned type %T, want *mockConfig", got)

					return
				}

				if gotConfig.Host != tt.want.Host {
					t.Errorf("Generate() host = %v, want %v", gotConfig.Host, tt.want.Host)
				}

				if gotConfig.Port != tt.want.Port {
					t.Errorf("Generate() port = %v, want %v", gotConfig.Port, tt.want.Port)
				}
			}
		})
	}
}

// TestGenerator_ServiceWithoutConfigField tests that services without a Config field
// return ErrInvalidConfigField.
func TestGenerator_ServiceWithoutConfigField(t *testing.T) {
	t.Parallel()

	// Set up pipe for stdin simulation
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	g := &Generator{Input: r}

	defer func() {
		_ = w.Close()
		_ = r.Close()
	}()

	// Write empty input to the pipe
	_, err = w.WriteString("")
	if err != nil {
		t.Fatal(err)
	}

	_ = w.Close()

	// Service without Config field should return ErrInvalidConfigField
	_, err = g.Generate(&mockServiceWithoutConfig{}, map[string]string{}, nil)
	if err == nil {
		t.Error("Generate() expected error for service without Config field")

		return
	}

	if !errors.Is(err, ErrInvalidConfigField) {
		t.Errorf("Generate() error = %v, want ErrInvalidConfigField", err)
	}
}

// TestGenerator_IgnoresInvalidFieldNames tests that invalid/unknown field names
// in props are silently ignored and do not cause errors.
func TestGenerator_IgnoresInvalidFieldNames(t *testing.T) {
	t.Parallel()

	// Set up pipe for stdin simulation
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	g := &Generator{Input: r}

	defer func() {
		_ = w.Close()
		_ = r.Close()
	}()

	// Write input to the pipe
	_, err = w.WriteString("\n8080\n")
	if err != nil {
		t.Fatal(err)
	}

	_ = w.Close()

	// Props with invalid field name should not cause an error,
	// it should just be ignored (unknown props are silently ignored)
	service := newMockServiceConfig()

	got, err := g.Generate(service, map[string]string{"invalid_field": "value"}, nil)
	if err != nil {
		t.Errorf("Generate() error = %v, wantErr false", err)

		return
	}

	// Verify that the config was still generated with defaults
	gotConfig, ok := got.(*mockConfig)
	if !ok {
		t.Errorf("Generate() returned type %T, want *mockConfig", got)

		return
	}

	if gotConfig.Host != "localhost" {
		t.Errorf("Generate() host = %v, want localhost (default)", gotConfig.Host)
	}

	if gotConfig.Port != 8080 {
		t.Errorf("Generate() port = %v, want 8080", gotConfig.Port)
	}
}
