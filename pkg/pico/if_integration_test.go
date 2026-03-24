package pico

import (
	"strings"
	"testing"
)

func TestIfStatementModifiers(t *testing.T) {
	tests := []struct {
		name         string
		markup       string
		fence        string
		expectedHTML []string // Substrings that should be in the output
		notExpected  []string // Substrings that should NOT be in the output
	}{
		{
			name:   "if with style modifier - true condition",
			markup: "{if age > 0 | style('max-height: 100px', 'max-height: 0')}<div>Content</div>{/if}",
			fence:  "let age = 25;",
			expectedHTML: []string{
				`p-style=`, // Just check attribute exists
				`max-height: 100px`,
				`style="max-height: 100px;"`,
			},
			notExpected: []string{
				`p-show`,
				`style="display: none;"`,
			},
		},
		{
			name:   "if with style modifier - false condition",
			markup: "{if age > 0 | style('max-height: 100px', 'max-height: 0')}<div>Content</div>{/if}",
			fence:  "let age = -5;",
			expectedHTML: []string{
				`p-style=`,
				`max-height: 0`,
				`style="max-height: 0;"`,
			},
			notExpected: []string{
				`p-show`,
				`style="display: none;"`,
			},
		},
		{
			name:   "if with class modifier - true condition",
			markup: "{if visible | class('expanded', 'collapsed')}<div>Content</div>{/if}",
			fence:  "let visible = true;",
			expectedHTML: []string{
				`p-class=`,
				`expanded`,
				`class="`,
			},
			notExpected: []string{
				`p-show`,
			},
		},
		{
			name:   "if with class modifier - false condition",
			markup: "{if visible | class('expanded', 'collapsed')}<div>Content</div>{/if}",
			fence:  "let visible = false;",
			expectedHTML: []string{
				`p-class=`,
				`collapsed`,
				`class="`,
			},
			notExpected: []string{
				`p-show`,
			},
		},
		{
			name:   "if with attr modifier - true condition",
			markup: "{if active | attr('data-state', 'on', 'off')}<div>Content</div>{/if}",
			fence:  "let active = true;",
			expectedHTML: []string{
				`p-attr=`,
				`data-state=`,
				`"on"`,
			},
			notExpected: []string{
				`p-show`,
			},
		},
		{
			name:   "if with attr modifier - false condition",
			markup: "{if active | attr('data-state', 'on', 'off')}<div>Content</div>{/if}",
			fence:  "let active = false;",
			expectedHTML: []string{
				`p-attr=`,
				`data-state=`,
				`"off"`,
			},
			notExpected: []string{
				`p-show`,
			},
		},
		{
			name:   "if with multiple modifiers",
			markup: "{if age > 0 | style('opacity: 1', 'opacity: 0') | class('visible', 'hidden') | attr('data-born', 'true', 'false')}<div>Content</div>{/if}",
			fence:  "let age = 25;",
			expectedHTML: []string{
				`p-style=`,
				`p-class=`,
				`p-attr=`,
				`opacity: 1`,
				`visible`,
				`data-born=`,
				`"true"`,
			},
			notExpected: []string{
				`p-show`,
			},
		},
		{
			name:   "simple if without modifiers - should have p-show",
			markup: "{if age > 0}<div>Content</div>{/if}",
			fence:  "let age = 25;",
			expectedHTML: []string{
				`p-show=`,
			},
			notExpected: []string{
				`p-style`,
				`p-class`,
				`p-attr`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse the markup
			controlTree, err := buildControlTree(tt.markup)
			if err != nil {
				t.Fatalf("buildControlTree failed: %v", err)
			}

			// Evaluate the control tree
			props := map[string]any{}
			markup, _ := evalControlTree(controlTree, []scopeStackItem{}, props, "", tt.fence, []Component{}, true, ".")

			// Check expected substrings
			for _, expected := range tt.expectedHTML {
				if !strings.Contains(markup, expected) {
					t.Errorf("Expected output to contain %q, but got:\n%s", expected, markup)
				}
			}

			// Check not expected substrings
			for _, notExpected := range tt.notExpected {
				if strings.Contains(markup, notExpected) {
					t.Errorf("Expected output NOT to contain %q, but got:\n%s", notExpected, markup)
				}
			}
		})
	}
}
