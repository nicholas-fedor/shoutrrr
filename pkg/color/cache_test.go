package color

import (
	"bytes"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_getCachedColor(t *testing.T) {
	// Clear the cache before testing
	colorsCacheMu.Lock()
	colorsCache = make(map[Attribute]*Color)
	colorsCacheMu.Unlock()

	tests := []struct {
		name           string
		attribute      Attribute
		checkColor     func(*testing.T, *Color)
		checkCacheSize int
	}{
		{
			name:      "creates new color for foreground black",
			attribute: FgBlack,
			checkColor: func(t *testing.T, c *Color) {
				t.Helper()
				require.NotNil(t, c)
				assert.True(t, c.attrExists(FgBlack))
			},
			checkCacheSize: 1,
		},
		{
			name:      "creates new color for foreground red",
			attribute: FgRed,
			checkColor: func(t *testing.T, c *Color) {
				t.Helper()
				require.NotNil(t, c)
				assert.True(t, c.attrExists(FgRed))
			},
			checkCacheSize: 2,
		},
		{
			name:      "creates new color for foreground green",
			attribute: FgGreen,
			checkColor: func(t *testing.T, c *Color) {
				t.Helper()
				require.NotNil(t, c)
				assert.True(t, c.attrExists(FgGreen))
			},
			checkCacheSize: 3,
		},
		{
			name:      "creates new color for foreground blue",
			attribute: FgBlue,
			checkColor: func(t *testing.T, c *Color) {
				t.Helper()
				require.NotNil(t, c)
				assert.True(t, c.attrExists(FgBlue))
			},
			checkCacheSize: 4,
		},
		{
			name:      "creates new color for background black",
			attribute: BgBlack,
			checkColor: func(t *testing.T, c *Color) {
				t.Helper()
				require.NotNil(t, c)
				assert.True(t, c.attrExists(BgBlack))
			},
			checkCacheSize: 5,
		},
		{
			name:      "creates new color for bold",
			attribute: Bold,
			checkColor: func(t *testing.T, c *Color) {
				t.Helper()
				require.NotNil(t, c)
				assert.True(t, c.attrExists(Bold))
			},
			checkCacheSize: 6,
		},
		{
			name:      "creates new color for underline",
			attribute: Underline,
			checkColor: func(t *testing.T, c *Color) {
				t.Helper()
				require.NotNil(t, c)
				assert.True(t, c.attrExists(Underline))
			},
			checkCacheSize: 7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getCachedColor(tt.attribute)
			tt.checkColor(t, result)

			// Check cache size
			colorsCacheMu.RLock()

			cacheSize := len(colorsCache)

			colorsCacheMu.RUnlock()
			assert.Equal(t, tt.checkCacheSize, cacheSize)
		})
	}
}

func Test_getCachedColor_returnsSameInstance(t *testing.T) {
	// Clear the cache before testing
	colorsCacheMu.Lock()
	colorsCache = make(map[Attribute]*Color)
	colorsCacheMu.Unlock()

	// Get the same color attribute twice
	firstCall := getCachedColor(FgRed)
	secondCall := getCachedColor(FgRed)

	// Should return the exact same instance
	assert.Same(t, firstCall, secondCall)
}

func Test_getCachedColor_concurrentAccess(t *testing.T) {
	// Clear the cache before testing
	colorsCacheMu.Lock()
	colorsCache = make(map[Attribute]*Color)
	colorsCacheMu.Unlock()

	var wg sync.WaitGroup

	results := make([]*Color, 100)
	errors := make([]error, 100)

	// Simulate concurrent access to getCachedColor
	for i := range 100 {
		wg.Add(1)

		go func(idx int) {
			defer wg.Done()
			// Use different attributes to fill the cache
			attr := Attribute(30 + idx%16)

			start := time.Now()
			for time.Since(start) < 10*time.Millisecond {
				results[idx] = getCachedColor(attr)
				if results[idx] == nil {
					errors[idx] = assert.AnError
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify no errors occurred
	for i, err := range errors {
		if err != nil {
			t.Errorf("goroutine %d returned error: %v", i, err)
		}
	}

	// Verify cache has entries
	colorsCacheMu.RLock()

	cacheSize := len(colorsCache)

	colorsCacheMu.RUnlock()
	assert.Positive(t, cacheSize, "cache should have entries after concurrent access")
}

func Test_getCachedColor_threadSafety(t *testing.T) {
	// Clear the cache before testing
	colorsCacheMu.Lock()
	colorsCache = make(map[Attribute]*Color)
	colorsCacheMu.Unlock()

	var wg sync.WaitGroup

	numGoroutines := 50
	numIterations := 100

	// Use a mutex to track successful retrievals
	var mu sync.Mutex

	uniqueColors := make(map[Attribute]bool)

	// Run concurrent reads and writes
	for i := range numGoroutines {
		wg.Add(1)

		go func(goroutineID int) {
			defer wg.Done()

			for j := range numIterations {
				// Each goroutine works with different attributes
				attr := Attribute((goroutineID*10 + j) % 256)

				c := getCachedColor(attr)
				if c != nil {
					mu.Lock()
					uniqueColors[attr] = true
					mu.Unlock()
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify cache integrity
	colorsCacheMu.RLock()

	cacheSize := len(colorsCache)

	colorsCacheMu.RUnlock()

	assert.Equal(t, len(uniqueColors), cacheSize, "cache size should match unique colors")
}

// Integration test to verify that cached colors work correctly with formatting

func Test_getCachedColor_integration(t *testing.T) {
	// Clear the cache before testing
	colorsCacheMu.Lock()
	colorsCache = make(map[Attribute]*Color)
	colorsCacheMu.Unlock()

	// Get a cached color
	c := getCachedColor(FgRed)
	require.NotNil(t, c)

	// Verify it can be used for formatting
	buf := &bytes.Buffer{}
	n, err := c.Fprint(buf, "test")
	require.NoError(t, err)
	assert.Positive(t, n)
}
