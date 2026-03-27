package pico

import (
	"strings"
	"testing"
)

func TestIDSelectorScoping(t *testing.T) {
	// Simulate the issue: parent div with ID, child div without ID
	scopedElements := []scopedElement{
		{tag: "div", id: "", classes: []string{}, scopedClass: "p-childClass"},            // Child added first
		{tag: "div", id: "plenti_cms", classes: []string{}, scopedClass: "p-parentClass"}, // Parent added second
	}

	css := `#plenti_cms { position: fixed; }`
	result := scopeCSS(css, scopedElements)

	// Should use parent's scoped class (p-parentClass), not child's (p-childClass)
	if !strings.Contains(result, "#plenti_cms.p-parentClass") {
		t.Errorf("Expected '#plenti_cms.p-parentClass', got: %s", result)
	}
	if strings.Contains(result, "p-childClass") {
		t.Errorf("Should not contain child's scoped class 'p-childClass', got: %s", result)
	}
}

func TestIDWithPseudoClassScoping(t *testing.T) {
	scopedElements := []scopedElement{
		{tag: "div", id: "", classes: []string{}, scopedClass: "p-childClass"},
		{tag: "div", id: "plenti_cms", classes: []string{"menu-visible"}, scopedClass: "p-parentClass"},
	}

	css := `#plenti_cms.menu-visible { right: 0; }`
	result := scopeCSS(css, scopedElements)

	// Should use parent's scoped class for the ID
	if !strings.Contains(result, "#plenti_cms.menu-visible.p-parentClass") {
		t.Errorf("Expected '#plenti_cms.menu-visible.p-parentClass', got: %s", result)
	}
}
