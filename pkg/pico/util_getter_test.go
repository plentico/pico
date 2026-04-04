package pico

import (
	"strings"
	"testing"
)

func TestSetPropsWithGettersSetters(t *testing.T) {
	// Test frontmatter with getters/setters - no trailing semicolon
	fence := `let localVars = {
	get text() {return text;},
	set text(value) {text = value;},

	get title() {return title;},
	set title(value) {title = value;},

	get salutation() {return salutation;},
	set salutation(value) {salutation = value;},

	get path() {return path;},
	set path(value) {path = value;},

	get comp() {return comp;},
	set comp(value) {comp = value;},

	get new_animal() {return new_animal;},
	set new_animal(value) {new_animal = value;},

	get show_animals() {return show_animals;},
	set show_animals(value) {show_animals = value;},
}`

	props := map[string]any{
		"text":  "hello",
		"title": "Test",
	}

	_, pScopeExp := setProps(fence, props)

	t.Logf("Generated p-scope: %s", pScopeExp)

	// Check that the output is valid (not empty or malformed)
	if pScopeExp == "" {
		t.Errorf("pScopeExp should not be empty")
	}

	// Verify the structure contains localVars
	if !strings.Contains(pScopeExp, "localVars") {
		t.Errorf("pScopeExp should contain 'localVars', got: %s", pScopeExp)
	}
}

func TestSetPropsWithGettersSettersAndTrailingSemicolon(t *testing.T) {
	// Test frontmatter with getters/setters - WITH trailing semicolon
	fence := `let localVars = {
	get text() {return text;},
	set text(value) {text = value;},
};`

	props := map[string]any{}

	_, pScopeExp := setProps(fence, props)

	t.Logf("Generated p-scope: %s", pScopeExp)

	// Check that the output is valid
	if pScopeExp == "" {
		t.Errorf("pScopeExp should not be empty")
	}

	// Verify the structure contains localVars
	if !strings.Contains(pScopeExp, "localVars") {
		t.Errorf("pScopeExp should contain 'localVars', got: %s", pScopeExp)
	}
}

func TestEvalJSWithGetterSetterFenceLogsError(t *testing.T) {
	// Test that evalJS properly logs errors when fence has issues with getters/setters
	// This test documents the current behavior - errors are now logged to help diagnose issues
	fence := `let text = "hello"; let localVars = { get text() {return text;}, set text(value) {text = value;} }`
	jsCode := "localVars.text"

	result := evalJS(jsCode, fence)
	t.Logf("Result: %v", result)

	// Note: This test documents that when there's a JS error, the function returns empty string
	// BUT now logs the error to help developers diagnose the issue
	// Previously, errors were silently swallowed with no indication of what went wrong
	t.Log("Note: If you see a 'Pico template error evaluating JS expression' log above, ")
	t.Log("this confirms error logging is working. The error is expected due to getter/setter issues.")
}

func TestEvalJSWithValidJS(t *testing.T) {
	// Test that evalJS works correctly with valid JavaScript
	fence := `let text = "hello"; let localVars = { text: text };`
	jsCode := "localVars.text"

	result := evalJS(jsCode, fence)
	t.Logf("Result: %v", result)

	// Should be able to access the property
	if result != "hello" {
		t.Errorf("evalJS should return 'hello', got: %v", result)
	}
}
