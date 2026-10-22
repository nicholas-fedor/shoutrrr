package router_test

import (
	"io"
	"log"
	"slices"
	"strconv"
	"testing"

	"github.com/nicholas-fedor/shoutrrr/pkg/router"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// fanOutSizes lists the service counts BenchmarkSend fans a message out to.
var fanOutSizes = []int{1, 8, 32}

// BenchmarkLocate measures building a service from its URL, which parses the
// URL into the service config, for each contract fixture.
func BenchmarkLocate(b *testing.B) {
	serviceRouter, err := router.NewWithOptions(log.New(io.Discard, "", 0), types.SenderOptions{})
	if err != nil {
		b.Fatal(err)
	}

	for _, fx := range fixtures {
		b.Run(fx.scheme, func(b *testing.B) {
			b.ReportAllocs()

			for b.Loop() {
				service, err := serviceRouter.Locate(fx.url)
				if err != nil {
					b.Fatal(err)
				}

				if closer, ok := service.(io.Closer); ok {
					_ = closer.Close()
				}
			}
		})
	}
}

// BenchmarkSend measures the router's fan-out of one message to several
// services. The logger service sends without network I/O, so the result is the
// router's own cost per send.
func BenchmarkSend(b *testing.B) {
	for _, size := range fanOutSizes {
		b.Run(strconv.Itoa(size)+" services", func(b *testing.B) {
			serviceRouter, err := router.NewWithOptions(
				log.New(io.Discard, "", 0),
				types.SenderOptions{},
				slices.Repeat([]string{"logger://"}, size)...,
			)
			if err != nil {
				b.Fatal(err)
			}

			b.ReportAllocs()

			for b.Loop() {
				for _, err := range serviceRouter.Send("benchmark message", nil) {
					if err != nil {
						b.Fatal(err)
					}
				}
			}
		})
	}
}
