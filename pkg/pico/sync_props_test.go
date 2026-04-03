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

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}
