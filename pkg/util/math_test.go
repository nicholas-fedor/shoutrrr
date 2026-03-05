package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		a    int
		b    int
		want int
	}{
		{
			name: "returns a when a is smaller",
			a:    5,
			b:    10,
			want: 5,
		},
		{
			name: "returns b when b is smaller",
			a:    10,
			b:    5,
			want: 5,
		},
		{
			name: "returns a when values are equal",
			a:    7,
			b:    7,
			want: 7,
		},
		{
			name: "works with negative numbers",
			a:    -10,
			b:    -5,
			want: -10,
		},
		{
			name: "works with mixed signs",
			a:    -5,
			b:    5,
			want: -5,
		},
		{
			name: "works with zero",
			a:    0,
			b:    5,
			want: 0,
		},
		{
			name: "works with both zeros",
			a:    0,
			b:    0,
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := Min(tt.a, tt.b)
			assert.Equal(t, tt.want, got, "Min(%d, %d) should return %d", tt.a, tt.b, tt.want)
		})
	}
}

func TestMax(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		a    int
		b    int
		want int
	}{
		{
			name: "returns a when a is larger",
			a:    10,
			b:    5,
			want: 10,
		},
		{
			name: "returns b when b is larger",
			a:    5,
			b:    10,
			want: 10,
		},
		{
			name: "returns a when values are equal",
			a:    7,
			b:    7,
			want: 7,
		},
		{
			name: "works with negative numbers",
			a:    -5,
			b:    -10,
			want: -5,
		},
		{
			name: "works with mixed signs",
			a:    -5,
			b:    5,
			want: 5,
		},
		{
			name: "works with zero",
			a:    0,
			b:    5,
			want: 5,
		},
		{
			name: "works with both zeros",
			a:    0,
			b:    0,
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := Max(tt.a, tt.b)
			assert.Equal(t, tt.want, got, "Max(%d, %d) should return %d", tt.a, tt.b, tt.want)
		})
	}
}
