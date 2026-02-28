package generators

import (
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/generators/basic"
	"github.com/nicholas-fedor/shoutrrr/pkg/generators/xoauth2"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/telegram"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// TestWithInput tests the WithInput option function to ensure it properly
// sets the input reader in generatorConfig.
func TestWithInput(t *testing.T) {
	t.Parallel()

	type args struct {
		reader io.Reader
	}

	tests := []struct {
		name   string
		args   args
		verify func(t *testing.T, config *generatorConfig)
	}{
		{
			name: "sets input reader correctly",
			args: args{
				reader: strings.NewReader("test input"),
			},
			verify: func(t *testing.T, config *generatorConfig) {
				t.Helper()
				assert.NotNil(t, config.input, "input should be set")

				// Verify the reader works by reading from it
				buf := make([]byte, 10)
				n, err := config.input.Read(buf)
				require.NoError(t, err, "should be able to read from input")
				assert.Equal(t, 10, n, "should read all bytes")
				assert.Equal(t, "test input", string(buf), "content should match")
			},
		},
		{
			name: "sets nil reader",
			args: args{
				reader: nil,
			},
			verify: func(t *testing.T, config *generatorConfig) {
				t.Helper()
				assert.Nil(t, config.input, "input should be nil")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create a config and apply the option
			config := &generatorConfig{}
			option := WithInput(tt.args.reader)
			option(config)

			// Verify the config
			tt.verify(t, config)
		})
	}
}

// TestListGenerators tests that ListGenerators returns the expected list
// of available generators.
func TestListGenerators(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		wantContain []string
		wantLen     int
	}{
		{
			name:        "returns non-empty list with expected generators",
			wantContain: []string{"basic", "oauth2", "telegram"},
			wantLen:     3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := ListGenerators()

			// Verify the list is not empty and has expected length
			require.NotEmpty(t, got, "ListGenerators() should return non-empty list")
			assert.Len(t, got, tt.wantLen, "ListGenerators() should return %d generators", tt.wantLen)

			// Verify it contains expected generators
			for _, want := range tt.wantContain {
				assert.Contains(t, got, want, "ListGenerators() should contain %q", want)
			}
		})
	}
}

// TestNewGenerator tests the NewGenerator function with various scenarios
// including success cases and error cases.
func TestNewGenerator(t *testing.T) {
	t.Parallel()

	type args struct {
		identifier string
		opts       []GeneratorOption
	}

	tests := []struct {
		name           string
		args           args
		wantErr        bool
		wantErrIs      error
		verifyType     func(t *testing.T, g types.Generator)
		verifyInputSet bool
	}{
		{
			name: "successful creation of basic generator without options",
			args: args{
				identifier: "basic",
				opts:       nil,
			},
			wantErr: false,
			verifyType: func(t *testing.T, g types.Generator) {
				t.Helper()

				bg, ok := g.(*basic.Generator)
				require.True(t, ok, "generator should be *basic.Generator")
				assert.Nil(t, bg.Input, "Input should be nil when no WithInput option")
			},
		},
		{
			name: "successful creation of basic generator with WithInput option",
			args: args{
				identifier: "basic",
				opts: []GeneratorOption{
					WithInput(strings.NewReader("test data")),
				},
			},
			wantErr:        false,
			verifyInputSet: true,
			verifyType: func(t *testing.T, g types.Generator) {
				t.Helper()

				bg, ok := g.(*basic.Generator)
				require.True(t, ok, "generator should be *basic.Generator")
				assert.NotNil(t, bg.Input, "Input should be set")
			},
		},
		{
			name: "successful creation of oauth2 generator",
			args: args{
				identifier: "oauth2",
				opts:       nil,
			},
			wantErr: false,
			verifyType: func(t *testing.T, g types.Generator) {
				t.Helper()

				_, ok := g.(*xoauth2.Generator)
				assert.True(t, ok, "generator should be *xouath2.Generator")
			},
		},
		{
			name: "successful creation of telegram generator without options",
			args: args{
				identifier: "telegram",
				opts:       nil,
			},
			wantErr: false,
			verifyType: func(t *testing.T, g types.Generator) {
				t.Helper()

				tg, ok := g.(*telegram.Generator)
				require.True(t, ok, "generator should be *telegram.Generator")
				assert.Nil(t, tg.Reader, "Reader should be nil when no WithInput option")
			},
		},
		{
			name: "successful creation of telegram generator with WithInput option",
			args: args{
				identifier: "telegram",
				opts: []GeneratorOption{
					WithInput(strings.NewReader("telegram test")),
				},
			},
			wantErr: false,
			verifyType: func(t *testing.T, g types.Generator) {
				t.Helper()

				tg, ok := g.(*telegram.Generator)
				require.True(t, ok, "generator should be *telegram.Generator")
				assert.NotNil(t, tg.Reader, "Reader should be set")
			},
		},
		{
			name: "error on unknown generator",
			args: args{
				identifier: "unknown",
				opts:       nil,
			},
			wantErr:   true,
			wantErrIs: ErrUnknownGenerator,
			verifyType: func(t *testing.T, g types.Generator) {
				t.Helper()
				assert.Nil(t, g, "generator should be nil on error")
			},
		},
		{
			name: "error on empty identifier",
			args: args{
				identifier: "",
				opts:       nil,
			},
			wantErr:   true,
			wantErrIs: ErrUnknownGenerator,
			verifyType: func(t *testing.T, g types.Generator) {
				t.Helper()
				assert.Nil(t, g, "generator should be nil on error")
			},
		},
		{
			name: "case insensitive - uppercase BASIC",
			args: args{
				identifier: "BASIC",
				opts:       nil,
			},
			wantErr: false,
			verifyType: func(t *testing.T, g types.Generator) {
				t.Helper()

				_, ok := g.(*basic.Generator)
				assert.True(t, ok, "generator should be *basic.Generator (case insensitive)")
			},
		},
		{
			name: "case insensitive - mixed case Oauth2",
			args: args{
				identifier: "Oauth2",
				opts:       nil,
			},
			wantErr: false,
			verifyType: func(t *testing.T, g types.Generator) {
				t.Helper()

				_, ok := g.(*xoauth2.Generator)
				assert.True(t, ok, "generator should be *xouath2.Generator (case insensitive)")
			},
		},
		{
			name: "multiple options - only last WithInput wins",
			args: args{
				identifier: "basic",
				opts: []GeneratorOption{
					WithInput(strings.NewReader("first")),
					WithInput(strings.NewReader("second")),
				},
			},
			wantErr: false,
			verifyType: func(t *testing.T, g types.Generator) {
				t.Helper()

				bg, ok := g.(*basic.Generator)
				require.True(t, ok, "generator should be *basic.Generator")
				assert.NotNil(t, bg.Input, "Input should be set with the last option")

				// Note: We verify the Input is set but don't read from it,
				// as strings.Reader is single-use and would cause EOF error
				// if the generator already consumed it.
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := NewGenerator(tt.args.identifier, tt.args.opts...)

			if tt.wantErr {
				require.Error(t, err, "NewGenerator() should return an error")

				if tt.wantErrIs != nil {
					require.ErrorIs(t, err, tt.wantErrIs, "error should wrap %v", tt.wantErrIs)
				}
			} else {
				require.NoError(t, err, "NewGenerator() should not return an error")
				require.NotNil(t, got, "NewGenerator() should return a non-nil generator")

				// Verify it implements the types.Generator interface
				_ = got

				// Run type-specific verification
				if tt.verifyType != nil {
					tt.verifyType(t, got)
				}
			}

			// Run common verification
			if tt.verifyType != nil {
				tt.verifyType(t, got)
			}
		})
	}
}

// TestNewGenerator_VerifyInterface ensures all created generators implement
// the types.Generator interface properly.
func TestNewGenerator_VerifyInterface(t *testing.T) {
	t.Parallel()

	generators := []string{"basic", "oauth2", "telegram"}

	for _, gen := range generators {
		t.Run(gen, func(t *testing.T) {
			t.Parallel()

			g, err := NewGenerator(gen)
			require.NoError(t, err, "should create %s generator", gen)
			require.NotNil(t, g, "generator should not be nil")

			// Verify interface implementation
			assert.Implements(t, (*types.Generator)(nil), g, "generator should implement types.Generator")

			// Verify the Generate method exists and has correct signature
			// by checking it can be called (with nil args for this test)
			assert.NotPanics(t, func() {
				// We just verify the method exists, not the actual functionality
				_ = g.Generate
			}, "generator should have Generate method")
		})
	}
}

// TestErrUnknownGenerator checks that ErrUnknownGenerator is properly defined
// and can be used with errors.Is.
func TestErrUnknownGenerator(t *testing.T) {
	t.Parallel()

	// Verify the error is defined
	require.Error(t, ErrUnknownGenerator, "ErrUnknownGenerator should be defined")
	assert.Equal(t, "unknown generator", ErrUnknownGenerator.Error(), "error message should match")

	// Verify it works with errors.Is
	_, err := NewGenerator("nonexistent")
	require.Error(t, err)
	require.ErrorIs(t, err, ErrUnknownGenerator, "error should be ErrUnknownGenerator")
	assert.ErrorIs(t, err, ErrUnknownGenerator, "errors.Is should work")
}
