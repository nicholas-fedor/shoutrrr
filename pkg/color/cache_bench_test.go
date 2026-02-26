package color

import (
	"testing"
)

// Benchmark_getCachedColor benchmarks the getCachedColor function for cache miss scenarios.
func Benchmark_getCachedColor(b *testing.B) {
	// Clear the cache before benchmarking
	colorsCacheMu.Lock()
	colorsCache = make(map[Attribute]*Color)
	colorsCacheMu.Unlock()

	b.ResetTimer()

	for i := range b.N {
		_ = getCachedColor(Attribute(i % 256))
	}
}

// Benchmark_getCachedColor_CacheHit benchmarks the getCachedColor function for cache hit scenarios.
func Benchmark_getCachedColor_CacheHit(b *testing.B) {
	// Pre-populate cache
	colorsCacheMu.Lock()

	colorsCache = make(map[Attribute]*Color)
	for i := range 256 {
		colorsCache[Attribute(i)] = New(Attribute(i))
	}
	colorsCacheMu.Unlock()

	b.ResetTimer()

	for i := range b.N {
		_ = getCachedColor(Attribute(i % 256))
	}
}

// Benchmark_getCachedColor_Concurrent benchmarks the getCachedColor function under concurrent access.
func Benchmark_getCachedColor_Concurrent(b *testing.B) {
	// Clear the cache before benchmarking
	colorsCacheMu.Lock()
	colorsCache = make(map[Attribute]*Color)
	colorsCacheMu.Unlock()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			_ = getCachedColor(Attribute(i % 256))
			i++
		}
	})
}
