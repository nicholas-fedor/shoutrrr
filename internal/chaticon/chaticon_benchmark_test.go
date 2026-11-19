package chaticon

import "testing"

// BenchmarkIsURL measures reading an icon, which each send with an icon does
// once.
func BenchmarkIsURL(b *testing.B) {
	// icons lists one icon of each kind.
	icons := []struct {
		name string
		icon string
	}{
		{name: "url", icon: "https://example.com/icons/alert.png"},
		{name: "emoji", icon: ":rotating_light:"},
	}

	for _, tt := range icons {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()

			for b.Loop() {
				IsURL(tt.icon)
			}
		})
	}
}
