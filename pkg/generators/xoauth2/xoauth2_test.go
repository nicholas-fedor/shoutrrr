package xoauth2

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

func TestGenerator_Generate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		props       map[string]string
		args        []string
		wantErr     bool
		expectPanic bool
		errMsg      string
	}{
		{
			name:    "gmail provider with credentials file",
			props:   map[string]string{"provider": "gmail"},
			args:    []string{"/nonexistent/credentials.json"},
			wantErr: true,
			errMsg:  "failed to read file",
		},
		{
			name:        "gmail provider without args",
			props:       map[string]string{"provider": "gmail"},
			args:        []string{},
			expectPanic: true,
		},
		{
			name:    "non-gmail provider with file arg",
			props:   map[string]string{"provider": "other"},
			args:    []string{"/nonexistent/config.json"},
			wantErr: true,
			errMsg:  "failed to read file",
		},
		{
			name:    "no provider with file arg",
			props:   map[string]string{},
			args:    []string{"/nonexistent/config.json"},
			wantErr: true,
			errMsg:  "failed to read file",
		},
		{
			name:    "interactive generator no args fails at scan",
			props:   map[string]string{},
			args:    []string{},
			wantErr: true,
			errMsg:  "failed to scan input",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			g := &Generator{}

			if tt.expectPanic {
				assert.Panics(t, func() {
					_, _ = g.Generate(nil, tt.props, tt.args)
				})

				return
			}

			got, err := g.Generate(nil, tt.props, tt.args)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)

				return
			}

			require.NoError(t, err)
			assert.NotNil(t, got)
		})
	}
}

func Test_generateOauth2Config(t *testing.T) {
	t.Parallel()

	type args struct {
		conf *oauth2.Config
		host string
	}

	tests := []struct {
		name    string
		args    args
		wantErr bool
		errMsg  string
	}{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := generateOauth2Config(tt.args.conf, tt.args.host)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)

				return
			}

			require.NoError(t, err)
			assert.NotNil(t, got)
		})
	}
}

func Test_oauth2GeneratorFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		file    string
		wantErr bool
		errType error
		errMsg  string
	}{
		{
			name:    "nonexistent file",
			file:    "/nonexistent/path/config.json",
			wantErr: true,
			errType: ErrReadFileFailed,
			errMsg:  "failed to read file",
		},
		{
			name:    "invalid JSON file",
			file:    createTempFile(t, "invalid json content"),
			wantErr: true,
			errType: ErrUnmarshalFailed,
			errMsg:  "failed to unmarshal JSON",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := oauth2GeneratorFile(tt.file)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)

				if tt.errType != nil {
					require.ErrorIs(t, err, tt.errType)
				}

				return
			}

			require.NoError(t, err)
			assert.NotNil(t, got)
		})
	}
}

func Test_oauth2GeneratorGmail(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		file    string
		wantErr bool
		errType error
		errMsg  string
	}{
		{
			name:    "nonexistent credentials file",
			file:    "/nonexistent/path/credentials.json",
			wantErr: true,
			errType: ErrReadFileFailed,
			errMsg:  "failed to read file",
		},
		{
			name:    "invalid JSON credentials file",
			file:    createTempFile(t, "invalid json content"),
			wantErr: true,
			errMsg:  "invalid character",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := oauth2GeneratorGmail(tt.file)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)

				if tt.errType != nil {
					require.ErrorIs(t, err, tt.errType)
				}

				return
			}

			require.NoError(t, err)
			assert.NotNil(t, got)
		})
	}
}

// createTempFile creates a temporary file with the given content and returns its path.
func createTempFile(t *testing.T, content string) string {
	t.Helper()

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test-config.json")

	err := os.WriteFile(tmpFile, []byte(content), 0o600)
	require.NoError(t, err, "failed to create temp file")

	return tmpFile
}
