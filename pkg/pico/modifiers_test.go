package pico

import (
	"testing"
)

func TestParseExpression(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected ParsedExpression
	}{
		{
			name:  "simple variable",
			input: "text",
			expected: ParsedExpression{
				Base:      "text",
				Modifiers: nil,
			},
		},
		{
			name:  "trim modifier",
			input: "text | trim(20)",
			expected: ParsedExpression{
				Base: "text",
				Modifiers: []Modifier{
					{Name: "trim", Args: []string{"20"}},
				},
			},
		},
		{
			name:  "html modifier no args",
			input: "text | html()",
			expected: ParsedExpression{
				Base: "text",
				Modifiers: []Modifier{
					{Name: "html", Args: nil},
				},
			},
		},
		{
			name:  "html modifier with args",
			input: "text | html('a', 'div')",
			expected: ParsedExpression{
				Base: "text",
				Modifiers: []Modifier{
					{Name: "html", Args: []string{"a", "div"}},
				},
			},
		},
		{
			name:  "trim and html modifiers",
			input: "text | trim(20) | html('a', 'div')",
			expected: ParsedExpression{
				Base: "text",
				Modifiers: []Modifier{
					{Name: "trim", Args: []string{"20"}},
					{Name: "html", Args: []string{"a", "div"}},
				},
			},
		},
		{
			name:  "html modifier with double quotes",
			input: `text | html("a", "div")`,
			expected: ParsedExpression{
				Base: "text",
				Modifiers: []Modifier{
					{Name: "html", Args: []string{"a", "div"}},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseExpression(tt.input)

			if result.Base != tt.expected.Base {
				t.Errorf("Base: got %q, want %q", result.Base, tt.expected.Base)
			}

			if len(result.Modifiers) != len(tt.expected.Modifiers) {
				t.Errorf("Modifiers count: got %d, want %d", len(result.Modifiers), len(tt.expected.Modifiers))
				return
			}

			for i, mod := range result.Modifiers {
				if mod.Name != tt.expected.Modifiers[i].Name {
					t.Errorf("Modifier %d Name: got %q, want %q", i, mod.Name, tt.expected.Modifiers[i].Name)
				}
				if len(mod.Args) != len(tt.expected.Modifiers[i].Args) {
					t.Errorf("Modifier %d Args count: got %d, want %d", i, len(mod.Args), len(tt.expected.Modifiers[i].Args))
					continue
				}
				for j, arg := range mod.Args {
					if arg != tt.expected.Modifiers[i].Args[j] {
						t.Errorf("Modifier %d Arg %d: got %q, want %q", i, j, arg, tt.expected.Modifiers[i].Args[j])
					}
				}
			}
		})
	}
}

func TestBuildPattrAttribute(t *testing.T) {
	tests := []struct {
		name          string
		parsed        ParsedExpression
		expectedKey   string
		expectedValue string
	}{
		{
			name: "simple text",
			parsed: ParsedExpression{
				Base: "text",
			},
			expectedKey:   "p-text",
			expectedValue: "text",
		},
		{
			name: "text with trim",
			parsed: ParsedExpression{
				Base: "text",
				Modifiers: []Modifier{
					{Name: "trim", Args: []string{"20"}},
				},
			},
			expectedKey:   "p-text:trim.20",
			expectedValue: "text",
		},
		{
			name: "html no args",
			parsed: ParsedExpression{
				Base: "text",
				Modifiers: []Modifier{
					{Name: "html", Args: nil},
				},
			},
			expectedKey:   "p-html",
			expectedValue: "text",
		},
		{
			name: "html with args",
			parsed: ParsedExpression{
				Base: "text",
				Modifiers: []Modifier{
					{Name: "html", Args: []string{"a", "div"}},
				},
			},
			expectedKey:   "p-html:allow.a.div",
			expectedValue: "text",
		},
		{
			name: "html with trim",
			parsed: ParsedExpression{
				Base: "text",
				Modifiers: []Modifier{
					{Name: "trim", Args: []string{"20"}},
					{Name: "html", Args: []string{"a", "div"}},
				},
			},
			expectedKey:   "p-html:trim.20:allow.a.div",
			expectedValue: "text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, value := BuildPattrAttribute(tt.parsed)

			if key != tt.expectedKey {
				t.Errorf("Key: got %q, want %q", key, tt.expectedKey)
			}
			if value != tt.expectedValue {
				t.Errorf("Value: got %q, want %q", value, tt.expectedValue)
			}
		})
	}
}
