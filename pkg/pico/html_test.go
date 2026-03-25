package pico

import (
	"testing"
)

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
