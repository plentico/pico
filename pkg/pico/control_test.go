package pico

import (
	"strings"
	"testing"
)

func TestForInSupport(t *testing.T) {
	// Test "for in" syntax
	markup := "{for let item in items}<div>{item}</div>{/for}"
	controlTree, err := buildControlTree(markup)
	if err != nil {
		t.Fatalf("Failed to build control tree: %v", err)
	}

	if len(controlTree) != 1 {
		t.Fatalf("Expected 1 control, got %d", len(controlTree))
	}

	ctrl := controlTree[0]
	if !ctrl.isForLoop {
		t.Fatal("Expected for loop control")
	}

	if ctrl.forKeyword != "in" {
		t.Errorf("Expected forKeyword to be 'in', got '%s'", ctrl.forKeyword)
	}

	if ctrl.forVar != "item" {
		t.Errorf("Expected forVar to be 'item', got '%s'", ctrl.forVar)
	}

	if ctrl.forCollection != "items" {
		t.Errorf("Expected forCollection to be 'items', got '%s'", ctrl.forCollection)
	}
}

func TestForOfSupport(t *testing.T) {
	// Test "for of" syntax
	markup := "{for let item of items}<div>{item}</div>{/for}"
	controlTree, err := buildControlTree(markup)
	if err != nil {
		t.Fatalf("Failed to build control tree: %v", err)
	}

	if len(controlTree) != 1 {
		t.Fatalf("Expected 1 control, got %d", len(controlTree))
	}

	ctrl := controlTree[0]
	if !ctrl.isForLoop {
		t.Fatal("Expected for loop control")
	}

	if ctrl.forKeyword != "of" {
		t.Errorf("Expected forKeyword to be 'of', got '%s'", ctrl.forKeyword)
	}

	if ctrl.forVar != "item" {
		t.Errorf("Expected forVar to be 'item', got '%s'", ctrl.forVar)
	}

	if ctrl.forCollection != "items" {
		t.Errorf("Expected forCollection to be 'items', got '%s'", ctrl.forCollection)
	}
}

func TestForInPAttrGeneration(t *testing.T) {
	// Test that p-for attribute correctly uses "in" keyword
	markup := "{for let item in items}<div>{item}</div>{/for}"
	props := map[string]any{
		"items": []any{"a", "b", "c"},
	}

	controlTree, err := buildControlTree(markup)
	if err != nil {
		t.Fatalf("Failed to build control tree: %v", err)
	}

	fence := "let items = [\"a\", \"b\", \"c\"];"
	pScopeExp := ""
	components := []Component{}
	scopeStack := []scopeStackItem{}

	result, _ := evalControlTree(controlTree, scopeStack, props, pScopeExp, fence, components, true, ".")

	// Check that the p-for attribute contains "in" not "of"
	if !strings.Contains(result, "p-for=\"item in items\"") {
		t.Errorf("Expected p-for attribute with 'in' keyword, got: %s", result)
	}
}

func TestForOfPAttrGeneration(t *testing.T) {
	// Test that p-for attribute correctly uses "of" keyword
	markup := "{for let item of items}<div>{item}</div>{/for}"
	props := map[string]any{
		"items": []any{"a", "b", "c"},
	}

	controlTree, err := buildControlTree(markup)
	if err != nil {
		t.Fatalf("Failed to build control tree: %v", err)
	}

	fence := "let items = [\"a\", \"b\", \"c\"];"
	pScopeExp := ""
	components := []Component{}
	scopeStack := []scopeStackItem{}

	result, _ := evalControlTree(controlTree, scopeStack, props, pScopeExp, fence, components, true, ".")

	// Check that the p-for attribute contains "of" not "in"
	if !strings.Contains(result, "p-for=\"item of items\"") {
		t.Errorf("Expected p-for attribute with 'of' keyword, got: %s", result)
	}
}

func TestForInWithObject(t *testing.T) {
	// Test "for in" with an object - iterates over keys
	markup := "{for let i in obj}{i}{/for}"
	props := map[string]any{
		"obj": map[string]any{"ok": "ok", "test": "test"},
	}

	controlTree, err := buildControlTree(markup)
	if err != nil {
		t.Fatalf("Failed to build control tree: %v", err)
	}

	fence := "let obj = {ok: 'ok', test: 'test'};"
	pScopeExp := ""
	components := []Component{}
	scopeStack := []scopeStackItem{}

	result, _ := evalControlTree(controlTree, scopeStack, props, pScopeExp, fence, components, false, ".")

	// Check that the result contains the object keys
	if !strings.Contains(result, "ok") || !strings.Contains(result, "test") {
		t.Errorf("Expected result to contain 'ok' and 'test', got: %s", result)
	}
}

func TestArrayDestructuring(t *testing.T) {
	// Test array destructuring: [key, value]
	markup := "{for let [k, v] of entries}<div>{k}: {v}</div>{/for}"
	props := map[string]any{
		"entries": []any{
			[]any{"name", "John"},
			[]any{"age", 30},
		},
	}

	controlTree, err := buildControlTree(markup)
	if err != nil {
		t.Fatalf("Failed to build control tree: %v", err)
	}

	fence := "let entries = [['name', 'John'], ['age', 30]];"
	pScopeExp := ""
	components := []Component{}
	scopeStack := []scopeStackItem{}

	result, _ := evalControlTree(controlTree, scopeStack, props, pScopeExp, fence, components, false, ".")

	// Check that destructured values are accessible
	if !strings.Contains(result, "name") || !strings.Contains(result, "John") {
		t.Errorf("Expected result to contain 'name' and 'John', got: %s", result)
	}
	if !strings.Contains(result, "age") || !strings.Contains(result, "30") {
		t.Errorf("Expected result to contain 'age' and '30', got: %s", result)
	}
}

func TestObjectDestructuring(t *testing.T) {
	// Test object destructuring: {name, age}
	markup := "{for let {name, age} of people}<div>{name} is {age}</div>{/for}"
	props := map[string]any{
		"people": []any{
			map[string]any{"name": "Alice", "age": 25},
			map[string]any{"name": "Bob", "age": 30},
		},
	}

	controlTree, err := buildControlTree(markup)
	if err != nil {
		t.Fatalf("Failed to build control tree: %v", err)
	}

	fence := "let people = [{name: 'Alice', age: 25}, {name: 'Bob', age: 30}];"
	pScopeExp := ""
	components := []Component{}
	scopeStack := []scopeStackItem{}

	result, _ := evalControlTree(controlTree, scopeStack, props, pScopeExp, fence, components, false, ".")

	// Check that destructured values are accessible
	if !strings.Contains(result, "Alice") || !strings.Contains(result, "25") {
		t.Errorf("Expected result to contain 'Alice' and '25', got: %s", result)
	}
	if !strings.Contains(result, "Bob") || !strings.Contains(result, "30") {
		t.Errorf("Expected result to contain 'Bob' and '30', got: %s", result)
	}
}
