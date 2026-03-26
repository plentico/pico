package pico

import (
	"strings"
	"testing"
)

func TestSetPropsWithFrontmatter(t *testing.T) {
	// Test frontmatter from the user's example
	fence := `prop name;
prop age;
prop animals;
prop test = "whatever";

let content = {
	name: name,
	age: age,
	animals: animals,
	test: test
}

let text = "<em>some<strong>thing</strong></em>" + name;
let title = "Pico";

var salutation = "hola";
//var salutation;

let path = "./mycomp.pico";
let comp = "mycomp";
let new_animal = "";
let show_animals = true;`

	props := map[string]any{}

	_, pScopeExp := setProps(fence, props)

	// Check that the output doesn't contain "let" keyword
	if strings.Contains(pScopeExp, "let ") {
		t.Errorf("pScopeExp should not contain 'let' keyword, got: %s", pScopeExp)
	}

	// Check that the output doesn't contain "var" keyword
	if strings.Contains(pScopeExp, "var ") {
		t.Errorf("pScopeExp should not contain 'var' keyword, got: %s", pScopeExp)
	}

	// Check that the output doesn't contain "const" keyword
	if strings.Contains(pScopeExp, "const ") {
		t.Errorf("pScopeExp should not contain 'const' keyword, got: %s", pScopeExp)
	}

	// Check that all statements end with semicolons
	// Split by semicolons and check that we have all expected declarations
	statements := strings.Split(pScopeExp, ";")
	expectedVars := []string{"content", "text", "title", "salutation", "path", "comp", "new_animal", "show_animals", "test"}

	foundVars := make(map[string]bool)
	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		// Check if statement has an equals sign
		parts := strings.Split(stmt, "=")
		if len(parts) >= 1 {
			varName := strings.TrimSpace(parts[0])
			foundVars[varName] = true
		}
	}

	for _, varName := range expectedVars {
		if !foundVars[varName] {
			t.Errorf("Expected to find variable '%s' in pScopeExp, got: %s", varName, pScopeExp)
		}
	}

	// Check that line breaks have been removed
	if strings.Contains(pScopeExp, "\n") {
		t.Errorf("pScopeExp should not contain line breaks, got: %s", pScopeExp)
	}

	// Check that tabs have been removed
	if strings.Contains(pScopeExp, "\t") {
		t.Errorf("pScopeExp should not contain tabs, got: %s", pScopeExp)
	}

	t.Logf("Generated p-scope: %s", pScopeExp)
}
