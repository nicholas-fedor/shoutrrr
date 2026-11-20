package format

// URLKeys records the config keys that the last service URL read set.
//
// [BuildQuery] leaves a value at its default out of a URL, so the values of a
// config cannot tell a default that its URL named from one that it left out.
// Some services map the level param onto a key, such as a priority, only while
// the URL leaves the key out, so a URL that names the default turns the
// mapping off. A config keeps that difference by embedding URLKeys through an
// unexported alias, which keeps the record out of its exported fields.
//
//	type urlKeys = format.URLKeys
//
//	type Config struct {
//		urlKeys
//
//		Priority int `default:"0" key:"priority" levelmapped:"true"`
//	}
//
// [ApplyQuery] starts the record anew and adds each key that it sets.
// [PropKeyResolver.KeepsDefault] then reports a recorded key as set.
// [BuildQuery] writes a recorded key whose field has the `levelmapped:"true"`
// tag even at its default, so the config read back from that URL records it
// too.
//
// Each key is recorded by the name of the field it sets, so an alias or a
// deprecated key counts as the field's primary key. A recorded key is never
// changed, so the copies of a config, such as the copy
// that each send makes, can share the record. The zero value holds no keys.
type URLKeys struct {
	// head is the key recorded last, or nil when the URL set none.
	head *urlKey
}

// urlKey is a key in the list that [URLKeys] holds. It links to the key
// recorded before it and is never changed, so the copies of a config share
// the keys they had in common.
type urlKey struct {
	// next is the key recorded before this one, or nil.
	next *urlKey
	// name is the name of the field that the key sets.
	name string
}

// urlKeysHolder is a config that embeds [URLKeys]. Its method is unexported,
// so only a type that embeds the record has it.
type urlKeysHolder interface {
	// record returns the embedded record.
	record() *URLKeys
}

// add records a key by its field. A nil record records nothing.
//
// Parameters:
//   - field: the name of the field that the key sets.
func (u *URLKeys) add(field string) {
	if u != nil && !u.has(field) {
		u.head = &urlKey{next: u.head, name: field}
	}
}

// has reports whether the record holds a key of a field.
//
// Parameters:
//   - field: the name of the field.
//
// Returns:
//   - bool: true when the URL set the field under any of its keys. A nil
//     record holds no keys.
func (u *URLKeys) has(field string) bool {
	if u == nil {
		return false
	}

	for recorded := u.head; recorded != nil; recorded = recorded.next {
		if recorded.name == field {
			return true
		}
	}

	return false
}

// record returns the record, so a resolver reaches it through a config that
// embeds it.
//
// Returns:
//   - *URLKeys: the record itself.
func (u *URLKeys) record() *URLKeys {
	return u
}

// reset empties the record, as before a new URL is read. A nil record is left
// as it is.
func (u *URLKeys) reset() {
	if u != nil {
		u.head = nil
	}
}

// urlKeysOf returns the record that a config embeds.
//
// Parameters:
//   - config: the config, given by pointer, or nil.
//
// Returns:
//   - *URLKeys: the record, or nil when the config embeds none.
func urlKeysOf(config any) *URLKeys {
	holder, ok := config.(urlKeysHolder)
	if !ok {
		return nil
	}

	return holder.record()
}
