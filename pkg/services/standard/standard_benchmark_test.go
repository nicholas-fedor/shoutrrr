package standard

import (
	"log/slog"
	"testing"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// BenchmarkLoggerDropped measures a Debug record that the logger drops, as a
// send logs one when it succeeds. It covers a service without a logger and
// one with a slog logger whose level is Info.
func BenchmarkLoggerDropped(b *testing.B) {
	b.Run("no logger", func(b *testing.B) {
		logger := &Logger{}

		for b.Loop() {
			logger.Debug(b.Context(), "request sent")
		}
	})

	b.Run("info level", func(b *testing.B) {
		logger := &Logger{}
		logger.SetLogger(types.NewSlogLogger(slog.New(&recordingHandler{level: slog.LevelInfo})))

		for b.Loop() {
			logger.Debug(b.Context(), "request sent")
		}
	})
}
