package slack

import "testing"

// BenchmarkSetIcon measures choosing the icon field of a payload, which each
// send with an icon does once.
func BenchmarkSetIcon(b *testing.B) {
	// icons lists one icon of each kind a URL can name.
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
				payload := MessagePayload{}
				payload.SetIcon(tt.icon)
			}
		})
	}
}
