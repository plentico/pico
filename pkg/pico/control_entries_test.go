package pico

import (
	"strings"
	"testing"
)

func TestEntriesIteratorSSR(t *testing.T) {
	markup := `{for let [a, b] of ["ok", "test", "another"].entries()}<div>{a}: {b}</div>{/for}`
	props := map[string]any{}

	controlTree, err := buildControlTree(markup)
	if err != nil {
		t.Fatalf("Failed to build control tree: %v", err)
	}

	fence := ""
	pScopeExp := ""
	components := []Component{}
	scopeStack := []scopeStackItem{}

	result, _ := evalControlTree(controlTree, scopeStack, props, pScopeExp, fence, components, false, ".")

	// Should render 3 divs with index: value pairs
	if !strings.Contains(result, "0: ok") {
		t.Errorf("Expected to find '0: ok', got: %s", result)
	}
	if !strings.Contains(result, "1: test") {
		t.Errorf("Expected to find '1: test', got: %s", result)
	}
	if !strings.Contains(result, "2: another") {
		t.Errorf("Expected to find '2: another', got: %s", result)
	}
}

func TestEntriesWithPattr(t *testing.T) {
	markup := `{for let [a, b] of ["ok", "test", "another"].entries()}<div>{a}: {b}</div>{/for}`
	props := map[string]any{}

	controlTree, err := buildControlTree(markup)
	if err != nil {
		t.Fatalf("Failed to build control tree: %v", err)
	}

	fence := ""
	pScopeExp := ""
	components := []Component{}
	scopeStack := []scopeStackItem{}

	result, _ := evalControlTree(controlTree, scopeStack, props, pScopeExp, fence, components, true, ".")

	// Should include template tag (check for HTML-escaped quotes)
	if !strings.Contains(result, `p-for="[a, b] of`) || !strings.Contains(result, `.entries()"`) {
		t.Errorf("Expected template tag with p-for attribute, got: %s", result)
	}

	// Should also include SSR elements
	if !strings.Contains(result, "0: ok") {
		t.Errorf("Expected SSR to find '0: ok', got: %s", result)
	}
	if !strings.Contains(result, "1: test") {
		t.Errorf("Expected SSR to find '1: test', got: %s", result)
	}
	if !strings.Contains(result, "2: another") {
		t.Errorf("Expected SSR to find '2: another', got: %s", result)
	}
}

func TestObjectEntriesSSR(t *testing.T) {
	markup := `{for let [k, v] of Object.entries({name: "Alice", age: 30})}<div>{k}: {v}</div>{/for}`
	props := map[string]any{}

	controlTree, err := buildControlTree(markup)
	if err != nil {
		t.Fatalf("Failed to build control tree: %v", err)
	}

	fence := ""
	pScopeExp := ""
	components := []Component{}
	scopeStack := []scopeStackItem{}

	result, _ := evalControlTree(controlTree, scopeStack, props, pScopeExp, fence, components, false, ".")

	// Should render entries (order not guaranteed for objects)
	if !strings.Contains(result, "name") || !strings.Contains(result, "Alice") {
		t.Errorf("Expected to find 'name' and 'Alice', got: %s", result)
	}
	if !strings.Contains(result, "age") || !strings.Contains(result, "30") {
		t.Errorf("Expected to find 'age' and '30', got: %s", result)
	}
}
