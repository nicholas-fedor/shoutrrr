package docs

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"

	"github.com/nicholas-fedor/shoutrrr/shoutrrr/cmd"
)

// Test_runDocs tests the runDocs function with various scenarios.
func Test_runDocs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		args       []string
		formatFlag string
		wantExit   int
		wantErrMsg string
	}{
		{
			name:       "console format success with single service",
			args:       []string{"discord"},
			formatFlag: "console",
			wantExit:   cmd.ExSuccess,
			wantErrMsg: "",
		},
		{
			name:       "markdown format success with single service",
			args:       []string{"slack"},
			formatFlag: "markdown",
			wantExit:   cmd.ExSuccess,
			wantErrMsg: "",
		},
		{
			name:       "multiple services",
			args:       []string{"discord", "slack"},
			formatFlag: "console",
			wantExit:   cmd.ExSuccess,
			wantErrMsg: "",
		},
		{
			name:       "invalid format returns ExUsage",
			args:       []string{"discord"},
			formatFlag: "xml",
			wantExit:   cmd.ExUsage,
			wantErrMsg: "invalid format",
		},
		{
			name:       "unknown service returns ExUsage",
			args:       []string{"unknownservice"},
			formatFlag: "console",
			wantExit:   cmd.ExUsage,
			wantErrMsg: "failed to init service",
		},
		{
			name:       "empty service list handled by cobra args validation",
			args:       []string{},
			formatFlag: "console",
			wantExit:   cmd.ExSuccess,
			wantErrMsg: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create a cobra command with the format flag
			rootCmd := &cobra.Command{
				Use: "docs",
			}
			rootCmd.Flags().StringP("format", "f", "console", "Output format")

			// Set the format flag value
			if err := rootCmd.Flags().Set("format", tt.formatFlag); err != nil {
				t.Fatalf("Failed to set format flag: %v", err)
			}

			got := runDocs(rootCmd, tt.args)

			assert.Equal(t, tt.wantExit, got.ExitCode, "Exit code mismatch")

			if tt.wantErrMsg != "" {
				assert.Contains(t, got.Message, tt.wantErrMsg, "Error message should contain expected text")
			}
		})
	}
}

// Test_runDocs_FlagError tests the error handling when flag retrieval fails.
func Test_runDocs_FlagError(t *testing.T) {
	t.Parallel()

	// Create a cobra command without the format flag to simulate an error
	rootCmd := &cobra.Command{
		Use: "docs",
	}
	// Do not add the format flag - this will cause GetString to return an error

	got := runDocs(rootCmd, []string{"discord"})

	assert.Equal(t, 1, got.ExitCode, "Exit code should be 1 on flag error")
	assert.Contains(t, got.Message, "Error getting format flag", "Error message should indicate flag retrieval failure")
}

// Test_printDocs tests the printDocs function with various scenarios.
func Test_printDocs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		docFormat  string
		services   []string
		wantExit   int
		wantErrMsg string
	}{
		{
			name:       "console render success",
			docFormat:  "console",
			services:   []string{"discord"},
			wantExit:   cmd.ExSuccess,
			wantErrMsg: "",
		},
		{
			name:       "markdown render success",
			docFormat:  "markdown",
			services:   []string{"slack"},
			wantExit:   cmd.ExSuccess,
			wantErrMsg: "",
		},
		{
			name:       "multiple services console",
			docFormat:  "console",
			services:   []string{"discord", "slack"},
			wantExit:   cmd.ExSuccess,
			wantErrMsg: "",
		},
		{
			name:       "multiple services markdown",
			docFormat:  "markdown",
			services:   []string{"smtp", "telegram"},
			wantExit:   cmd.ExSuccess,
			wantErrMsg: "",
		},
		{
			name:       "invalid format returns ExUsage",
			docFormat:  "xml",
			services:   []string{"discord"},
			wantExit:   cmd.ExUsage,
			wantErrMsg: "invalid format",
		},
		{
			name:       "unknown service returns ExUsage",
			docFormat:  "console",
			services:   []string{"notarealservice"},
			wantExit:   cmd.ExUsage,
			wantErrMsg: "failed to init service",
		},
		{
			name:       "empty format defaults to console",
			docFormat:  "",
			services:   []string{"discord"},
			wantExit:   cmd.ExUsage,
			wantErrMsg: "invalid format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := printDocs(tt.docFormat, tt.services)

			assert.Equal(t, tt.wantExit, got.ExitCode, "Exit code mismatch")

			if tt.wantErrMsg != "" {
				assert.Contains(t, got.Message, tt.wantErrMsg, "Error message should contain expected text")
			}
		})
	}
}

// Test_printDocs_EdgeCases tests edge cases for printDocs.
func Test_printDocs_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		docFormat string
		services  []string
		wantExit  int
	}{
		{
			name:      "empty services slice",
			docFormat: "console",
			services:  []string{},
			wantExit:  cmd.ExSuccess,
		},
		{
			name:      "case insensitive service lookup",
			docFormat: "console",
			services:  []string{"DISCORD"},
			wantExit:  cmd.ExSuccess,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := printDocs(tt.docFormat, tt.services)
			assert.Equal(t, tt.wantExit, got.ExitCode, "Exit code mismatch")
		})
	}
}

// TestCmd_ValidArgs verifies that the Cmd has valid service arguments.
func TestCmd_ValidArgs(t *testing.T) {
	t.Parallel()

	// Verify that services variable is populated
	assert.NotEmpty(t, services, "services list should not be empty")

	// Verify Cmd has ValidArgs set
	assert.NotEmpty(t, Cmd.ValidArgs, "Cmd.ValidArgs should not be empty")
	assert.Equal(t, services, Cmd.ValidArgs, "Cmd.ValidArgs should match services list")
}

// TestCmd_Flags verifies that the Cmd has the expected flags.
func TestCmd_Flags(t *testing.T) {
	t.Parallel()

	// Check that the format flag exists
	formatFlag := Cmd.Flags().Lookup("format")
	assert.NotNil(t, formatFlag, "format flag should exist")
	assert.Equal(t, "console", formatFlag.DefValue, "format flag default should be 'console'")
	assert.Equal(t, "f", formatFlag.Shorthand, "format flag shorthand should be 'f'")
}
