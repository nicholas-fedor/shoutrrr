package pushbullet

import "testing"

// BenchmarkSetTarget measures setting the target of a push, which each send
// does once for every target.
func BenchmarkSetTarget(b *testing.B) {
	// targets lists one target of each kind a URL can name.
	targets := []struct {
		name   string
		target string
	}{
		{name: "email", target: "alerts.team@example.com"},
		{name: "channel", target: "#deployments"},
		{name: "device", target: "ujpah72o0sjAoRtnM0jc"},
	}

	for _, tt := range targets {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()

			for b.Loop() {
				push := PushRequest{}
				push.SetTarget(tt.target)
			}
		})
	}
}
