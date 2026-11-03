package pico

import (
	"encoding/json"
	"fmt"
)

// externalPropBindings carries external values directly into a Goja VM. Keeping
// them out of the JavaScript fence avoids reparsing a large JSON literal for
// every expression evaluated during SSR.
var externalPropBindings = map[string]map[string]any{}

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

// evalComponentProp retains externalization when a component forwards a prop
// directly. Evaluating it to a plain Go value would serialize the entire value
// into the receiving component's fence and hydration scope.
func evalComponentProp(props map[string]any, expression any, fence string) any {
	if name, ok := expression.(string); ok {
		if external, ok := props[name].(ExternalProp); ok {
			return external
		}
	}
	return evalJS(fmt.Sprintf(`%s`, expression), fence)
}
