package color

// getCachedColor retrieves a cached color object for the given attribute, creating it if necessary.
// Note: This only caches single-attribute colors. Combined attributes create new Color objects.
func getCachedColor(attribute Attribute) *Color {
	colorsCacheMu.RLock()

	c, ok := colorsCache[attribute]

	colorsCacheMu.RUnlock()

	if ok {
		return c
	}

	colorsCacheMu.Lock()
	defer colorsCacheMu.Unlock()

	// Double-check after acquiring write lock
	if c, ok = colorsCache[attribute]; ok {
		return c
	}

	c = New(attribute)
	colorsCache[attribute] = c

	return c
}
