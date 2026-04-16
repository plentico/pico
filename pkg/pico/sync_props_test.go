package pico

import (
	"os"
	"strings"
	"testing"
)

func TestSyncPropsShorthand(t *testing.T) {
	// Test {*myVar} syntax for sync props
	compDecl := "{*count}"
	result := getCompArgs(compDecl)

	if len(result.Sync) != 1 {
		t.Errorf("Expected 1 sync prop, got %d", len(result.Sync))
	}

	if result.Sync["count"] != "count" {
		t.Errorf("Expected sync prop 'count' with value 'count', got: %v", result.Sync["count"])
	}

	if len(result.Regular) != 0 {
		t.Errorf("Expected 0 regular props, got %d", len(result.Regular))
	}
}

func TestSyncPropsWithValue(t *testing.T) {
	// Test *myVar={value} syntax
	compDecl := "*childCount={count}"
	result := getCompArgs(compDecl)

	if len(result.Sync) != 1 {
		t.Errorf("Expected 1 sync prop, got %d", len(result.Sync))
	}

	if result.Sync["childCount"] != "count" {
		t.Errorf("Expected sync prop 'childCount' with value 'count', got: %v", result.Sync["childCount"])
	}

	if len(result.Regular) != 0 {
		t.Errorf("Expected 0 regular props, got %d", len(result.Regular))
	}
}

func TestMixedRegularAndSyncProps(t *testing.T) {
	// Test <MyComp {*count} {otherVar} />
	compDecl := "{*count} {otherVar}"
	result := getCompArgs(compDecl)

	if len(result.Sync) != 1 {
		t.Errorf("Expected 1 sync prop, got %d", len(result.Sync))
	}

	if result.Sync["count"] != "count" {
		t.Errorf("Expected sync prop 'count', got: %v", result.Sync)
	}

	if len(result.Regular) != 1 {
		t.Errorf("Expected 1 regular prop, got %d", len(result.Regular))
	}

	if result.Regular["otherVar"] != "otherVar" {
		t.Errorf("Expected regular prop 'otherVar', got: %v", result.Regular)
	}
}

func TestMixedSyncPropsWithValues(t *testing.T) {
	// Test <MyComp *count={parentCount} title={myTitle} />
	compDecl := "*count={parentCount} title={myTitle}"
	result := getCompArgs(compDecl)

	if len(result.Sync) != 1 {
		t.Errorf("Expected 1 sync prop, got %d", len(result.Sync))
	}

	if result.Sync["count"] != "parentCount" {
		t.Errorf("Expected sync prop 'count' with value 'parentCount', got: %v", result.Sync["count"])
	}

	if len(result.Regular) != 1 {
		t.Errorf("Expected 1 regular prop, got %d", len(result.Regular))
	}

	if result.Regular["title"] != "myTitle" {
		t.Errorf("Expected regular prop 'title' with value 'myTitle', got: %v", result.Regular["title"])
	}
}

func TestSyncPropsRendering(t *testing.T) {
	// Create a test component with sync prop
	compMarkup := `---
prop childCount;
---
<section>
  <div>Count: <span>{childCount}</span></div>
  <button onclick="{childCount++}">+</button>
</section>`

	// Write temp component file
	compPath := "/tmp/test_sync_comp.pico"
	if err := writeFile(compPath, compMarkup); err != nil {
		t.Fatalf("Failed to write component file: %v", err)
	}

	// Create parent template using the component with sync prop
	// Use *childCount={count} syntax to map parent's 'count' to child's 'childCount'
	parentMarkup := `---
import TestComp from "` + compPath + `";
let count = 5;
---
<div>
  Parent: {count}
  <TestComp *childCount={count} />
</div>`

	parentPath := "/tmp/test_sync_parent.pico"
	if err := writeFile(parentPath, parentMarkup); err != nil {
		t.Fatalf("Failed to write parent file: %v", err)
	}

	// Render the parent template
	markup, _, _ := RenderRoot(parentPath, map[string]any{})

	// Check that p-scope:sync attribute is present
	if !strings.Contains(markup, "p-scope:sync") {
		t.Error("Expected p-scope:sync attribute in rendered markup")
	}

	// Check that sync prop is in p-scope:sync with correct mapping
	if !strings.Contains(markup, `p-scope:sync="childCount = count;`) && !strings.Contains(markup, `p-scope:sync='childCount = count;`) {
		t.Errorf("Expected p-scope:sync='childCount = count;', got:\n%s", markup)
	}

	// Verify the value was passed correctly (should show 5)
	if !strings.Contains(markup, "Count: <span") || !strings.Contains(markup, ">5</span>") {
		t.Errorf("Expected childCount to render as 5, got:\n%s", markup)
	}
}

// TestRegularShorthandPropsInPScope verifies that shorthand props like {content} are
// included in the child component's p-scope attribute.
// Previously flattenCompArgs skipped k==v entries, so {content} was silently dropped.
func TestRegularShorthandPropsInPScope(t *testing.T) {
	compMarkup := `---
prop content;
---
<section>
  <div>{content.name}</div>
</section>`

	compPath := "/tmp/test_regular_shorthand_comp.pico"
	if err := writeFile(compPath, compMarkup); err != nil {
		t.Fatalf("Failed to write component file: %v", err)
	}

	parentMarkup := `---
import TestComp from "` + compPath + `";
let content = {name: "Alice"};
---
<div>
  <TestComp {content} />
</div>`

	parentPath := "/tmp/test_regular_shorthand_parent.pico"
	if err := writeFile(parentPath, parentMarkup); err != nil {
		t.Fatalf("Failed to write parent file: %v", err)
	}

	markup, _, _ := RenderRoot(parentPath, map[string]any{})

	// The child component's root element should have p-scope containing 'content = content;'
	if !strings.Contains(markup, "p-scope") {
		t.Fatalf("Expected p-scope attribute in rendered markup, got:\n%s", markup)
	}
	if !strings.Contains(markup, "content = content;") {
		t.Errorf("Expected 'content = content;' in p-scope for shorthand regular prop, got:\n%s", markup)
	}
	// Should NOT be in p-scope:sync since it's a regular (one-way) prop
	if strings.Contains(markup, "p-scope:sync") {
		t.Errorf("Regular shorthand props should not be in p-scope:sync, got:\n%s", markup)
	}
}

// TestMixedShorthandRegularAndSyncPropsRendering verifies that
// <Comp {content} {*localVars} /> puts content in p-scope and localVars in p-scope:sync.
func TestMixedShorthandRegularAndSyncPropsRendering(t *testing.T) {
	compMarkup := `---
prop content;
prop localVars;
let showMenu = false;
---
<section>
  <div>{content.name}</div>
</section>`

	compPath := "/tmp/test_mixed_shorthand_comp.pico"
	if err := writeFile(compPath, compMarkup); err != nil {
		t.Fatalf("Failed to write component file: %v", err)
	}

	parentMarkup := `---
import TestComp from "` + compPath + `";
let content = {name: "Bob"};
let localVars = {title: "Test"};
---
<div>
  <TestComp {content} {*localVars} />
</div>`

	parentPath := "/tmp/test_mixed_shorthand_parent.pico"
	if err := writeFile(parentPath, parentMarkup); err != nil {
		t.Fatalf("Failed to write parent file: %v", err)
	}

	markup, _, _ := RenderRoot(parentPath, map[string]any{})

	// Regular shorthand prop 'content' should appear in p-scope
	if !strings.Contains(markup, "content = content;") {
		t.Errorf("Expected 'content = content;' in p-scope, got:\n%s", markup)
	}
	// Sync shorthand prop 'localVars' should appear in p-scope:sync
	if !strings.Contains(markup, "p-scope:sync") {
		t.Errorf("Expected p-scope:sync attribute for sync prop, got:\n%s", markup)
	}
	if !strings.Contains(markup, "localVars = localVars;") {
		t.Errorf("Expected 'localVars = localVars;' in p-scope:sync, got:\n%s", markup)
	}
}

// TestEvalJSNoSpuriousFenceError verifies that evalJS uses a fresh goja VM when
// checking the fence after an expression evaluation failure.
// Previously the same VM was reused, causing a false "Identifier already declared"
// SyntaxError for any let/const variable that appeared in both runs.
func TestEvalJSNoSpuriousFenceError(t *testing.T) {
	// Build a fence with a let declaration
	fence := `let content = {age: 2, name: "Ja"};` + "\nlet [key, value] = [\"name\", \"Ja\"];"

	// This expression will fail (string "Ja" has no .entries()) but the fence is valid.
	// Before the fix, calling vm.RunString(fence) on the same vm that already ran
	// fence+jsCode would report "Identifier 'content' already declared".
	result := evalJS(`Array.from(content[key].entries())`, fence)

	// evalJS returns "" on failure
	if result != "" {
		t.Errorf("Expected empty string from failed eval, got: %v", result)
	}
	// Verify fence-alone evaluation works correctly in isolation
	fenceResult := evalJS("content.age", fence)
	if fenceResult != int64(2) {
		t.Errorf("Fence should still be usable after a failed expression, expected 2, got: %v", fenceResult)
	}
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}
