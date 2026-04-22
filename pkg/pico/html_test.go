package pico

import (
	"os"
	"strings"
	"testing"
)

func TestAsteriskValueBinding(t *testing.T) {
	// Test *value creates p-model attribute and SSR value
	markup := `---
let myVar = "hello";
---
<input *value="{myVar}" />`

	tmpPath := "/tmp/test_asterisk_value.pico"
	if err := os.WriteFile(tmpPath, []byte(markup), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	rendered, _, _ := RenderRoot(tmpPath, map[string]any{})

	// Should have p-model attribute
	if !strings.Contains(rendered, `p-model="myVar"`) {
		t.Errorf("Expected p-model attribute, got:\n%s", rendered)
	}
	// Should have SSR value
	if !strings.Contains(rendered, `value="hello"`) {
		t.Errorf("Expected SSR value='hello', got:\n%s", rendered)
	}
	// Should NOT have *value attribute in output
	if strings.Contains(rendered, "*value") {
		t.Errorf("*value attribute should be removed, got:\n%s", rendered)
	}
}

func TestAsteriskCheckedBinding(t *testing.T) {
	// Test *checked creates p-model:checked attribute and SSR checked
	markup := `---
let isActive = true;
---
<input type="checkbox" *checked="{isActive}" />`

	tmpPath := "/tmp/test_asterisk_checked.pico"
	if err := os.WriteFile(tmpPath, []byte(markup), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	rendered, _, _ := RenderRoot(tmpPath, map[string]any{})

	// Should have p-model:checked attribute
	if !strings.Contains(rendered, `p-model:checked="isActive"`) {
		t.Errorf("Expected p-model:checked attribute, got:\n%s", rendered)
	}
	// Should have SSR checked attribute (empty val for boolean attrs)
	if !strings.Contains(rendered, `checked=""`) {
		t.Errorf("Expected checked attribute for true value, got:\n%s", rendered)
	}
	// Should NOT have *checked attribute in output
	if strings.Contains(rendered, "*checked") {
		t.Errorf("*checked attribute should be removed, got:\n%s", rendered)
	}
}

func TestAsteriskCheckedBindingFalse(t *testing.T) {
	// Test *checked with false value - should NOT add checked attribute
	markup := `---
let isActive = false;
---
<input type="checkbox" *checked="{isActive}" />`

	tmpPath := "/tmp/test_asterisk_checked_false.pico"
	if err := os.WriteFile(tmpPath, []byte(markup), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	rendered, _, _ := RenderRoot(tmpPath, map[string]any{})

	// Should have p-model:checked attribute
	if !strings.Contains(rendered, `p-model:checked="isActive"`) {
		t.Errorf("Expected p-model:checked attribute, got:\n%s", rendered)
	}
	// Should NOT have checked attribute when false
	if strings.Contains(rendered, `checked=""`) {
		t.Errorf("Should not have checked attribute for false value, got:\n%s", rendered)
	}
}

func TestAsteriskIndeterminateBinding(t *testing.T) {
	// Test *indeterminate creates p-model:indeterminate attribute
	// No SSR HTML attribute for indeterminate (DOM property only)
	markup := `---
let isIndeterminate = true;
---
<input type="checkbox" *indeterminate="{isIndeterminate}" />`

	tmpPath := "/tmp/test_asterisk_indeterminate.pico"
	if err := os.WriteFile(tmpPath, []byte(markup), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	rendered, _, _ := RenderRoot(tmpPath, map[string]any{})

	// Should have p-model:indeterminate attribute
	if !strings.Contains(rendered, `p-model:indeterminate="isIndeterminate"`) {
		t.Errorf("Expected p-model:indeterminate attribute, got:\n%s", rendered)
	}
	// Should NOT have indeterminate HTML attribute (DOM-only property)
	// Note: p-model:indeterminate is expected, but not a bare indeterminate= attr
	if strings.Contains(rendered, ` indeterminate=`) {
		t.Errorf("indeterminate should not be added as HTML attribute, got:\n%s", rendered)
	}
	// Should NOT have *indeterminate attribute in output
	if strings.Contains(rendered, "*indeterminate") {
		t.Errorf("*indeterminate attribute should be removed, got:\n%s", rendered)
	}
}

func TestRegularValueDoesNotCreatePModel(t *testing.T) {
	// Test that plain value="{expr}" does NOT create p-model anymore
	markup := `---
let myVar = "hello";
---
<input value="{myVar}" />`

	tmpPath := "/tmp/test_regular_value.pico"
	if err := os.WriteFile(tmpPath, []byte(markup), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	rendered, _, _ := RenderRoot(tmpPath, map[string]any{})

	// Should NOT have p-model attribute (asterisk is required now)
	if strings.Contains(rendered, `p-model="myVar"`) {
		t.Errorf("Plain value should NOT create p-model anymore, got:\n%s", rendered)
	}
	// Should still have evaluated SSR value
	if !strings.Contains(rendered, `value="hello"`) {
		t.Errorf("Expected SSR value='hello', got:\n%s", rendered)
	}
}

func TestAsteriskValueNoPattr(t *testing.T) {
	// Test *value without pattr still evaluates and renders correctly
	markup := `---
let myVar = "hello";
---
<input *value="{myVar}" />`

	tmpPath := "/tmp/test_asterisk_value_nopattr.pico"
	if err := os.WriteFile(tmpPath, []byte(markup), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	rendered, _, _ := RenderRoot(tmpPath, map[string]any{}, true) // noPattr=true

	// Should NOT have p-model attribute when pattr is disabled
	if strings.Contains(rendered, `p-model`) {
		t.Errorf("Should not have p-model when pattr disabled, got:\n%s", rendered)
	}
	// Should still have SSR value
	if !strings.Contains(rendered, `value="hello"`) {
		t.Errorf("Expected SSR value='hello' even without pattr, got:\n%s", rendered)
	}
}

func TestExtractPClassNames(t *testing.T) {
	tests := []struct {
		name      string
		pClassVal string
		want      []string
	}{
		{
			name:      "simple ternary with two classes",
			pClassVal: "show_animals ? 'expanded' : 'collapsed'",
			want:      []string{"expanded", "collapsed"},
		},
		{
			name:      "ternary with empty true class",
			pClassVal: "show_animals ? '' : 'collapsed'",
			want:      []string{"collapsed"},
		},
		{
			name:      "ternary with multiple classes in each",
			pClassVal: "show_animals ? 'expanded visible' : 'collapsed hidden'",
			want:      []string{"expanded", "visible", "collapsed", "hidden"},
		},
		{
			name:      "ternary with condition containing quotes",
			pClassVal: "name == 'test' ? 'expanded' : 'collapsed'",
			want:      []string{"expanded", "collapsed"},
		},
		{
			name:      "empty ternary",
			pClassVal: "",
			want:      []string{},
		},
		{
			name:      "ternary with only empty classes",
			pClassVal: "show_animals ? '' : ''",
			want:      []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractPClassNames(tt.pClassVal)
			if len(got) != len(tt.want) {
				t.Errorf("extractPClassNames() = %v, want %v", got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("extractPClassNames()[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestPClassCSSPreservation(t *testing.T) {
	// Test that CSS classes from p-class attributes are preserved during treeshaking
	scopedElements := []scopedElement{
		{
			tag:         "div",
			classes:     []string{"animals", "collapsed"}, // collapsed comes from p-class
			scopedClass: "p-abc123",
		},
	}

	css := `
		.animals { color: blue; }
		.collapsed { max-height: 0; }
		.unused { color: red; }
	`

	result := scopeCSS(css, scopedElements)

	// Check that .animals is preserved
	if !contains(result, ".animals") {
		t.Error("Expected .animals to be preserved in CSS")
	}

	// Check that .collapsed is preserved (from p-class)
	if !contains(result, ".collapsed") {
		t.Error("Expected .collapsed to be preserved in CSS (from p-class attribute)")
	}

	// Check that .unused is removed
	if contains(result, ".unused") {
		t.Error("Expected .unused to be removed from CSS")
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && string(s[0]) != "" && containsString(s, substr)
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
