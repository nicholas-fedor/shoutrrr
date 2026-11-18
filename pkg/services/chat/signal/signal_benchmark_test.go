package signal

import "testing"

// BenchmarkRecipients measures checking and batching the recipients of a send,
// as each send does.
func BenchmarkRecipients(b *testing.B) {
	// A send to two phone numbers, two groups, and a username.
	recipients := []string{
		"+15550000001",
		"+15550000002",
		"group.dGVzdGdyb3VwaWQx",
		"group.YWJjZGVmZ2hpams+bG1ub3A/cXI=",
		"u:alice.01",
	}

	b.Run("validate", func(b *testing.B) {
		b.ReportAllocs()

		for b.Loop() {
			if err := validateRecipients(recipients); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("batch", func(b *testing.B) {
		b.ReportAllocs()

		for b.Loop() {
			_ = batchRecipients(recipients)
		}
	})
}
