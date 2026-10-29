package pico

import "encoding/json"

// ExternalProp is rendered normally on the server but loaded from Source by
// Pattr during hydration instead of being embedded in every page's root data.
type ExternalProp struct {
	Value      any
	Source     string
	serialized string
}

// NewExternalProp prepares a shared prop and caches its JavaScript/JSON literal
// so repeated SSR renders do not need to marshal the same value again.
func NewExternalProp(value any, source string) ExternalProp {
	serialized, _ := json.Marshal(value)
	return ExternalProp{Value: value, Source: source, serialized: string(serialized)}
}
