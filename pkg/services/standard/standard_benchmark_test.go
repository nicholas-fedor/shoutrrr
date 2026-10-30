package standard

import (
	"log/slog"
	"net/http"
	"testing"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// BenchmarkLoggerDropped measures a Debug record that the logger drops, as a
// send logs one when it succeeds. It covers a service without a logger and
// one with a slog logger whose level is Info. With that slog logger, it also
// covers a record with a string attribute logged through LogAttrs.
func BenchmarkLoggerDropped(b *testing.B) {
	b.Run("no logger", func(b *testing.B) {
		logger := &Logger{}

		for b.Loop() {
			logger.Debug(b.Context(), "request sent")
		}
	})

	infoLevel := &Logger{}
	infoLevel.SetLogger(types.NewSlogLogger(slog.New(&recordingHandler{level: slog.LevelInfo})))

	b.Run("info level", func(b *testing.B) {
		for b.Loop() {
			infoLevel.Debug(b.Context(), "request sent")
		}
	})

	b.Run("info level with attribute", func(b *testing.B) {
		status := http.StatusText(http.StatusOK)

		for b.Loop() {
			infoLevel.LogAttrs(b.Context(), slog.LevelDebug, "response received", slog.String("status", status))
		}
	})
}
