package pico

import (
	"testing"
)

func TestParseIfCondition(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected ParsedIfCondition
	}{
		{
			name:  "simple condition",
			input: "age > 0",
			expected: ParsedIfCondition{
				BaseCondition: "age > 0",
				Modifiers:     nil,
			},
		},
		{
			name:  "condition with style modifier",
			input: "age > 0 | style('max-height: 100px', 'max-height: 0')",
			expected: ParsedIfCondition{
				BaseCondition: "age > 0",
				Modifiers: []IfModifier{
					{Type: "style", Args: []string{"max-height: 100px", "max-height: 0"}},
				},
			},
		},
		{
			name:  "condition with class modifier",
			input: "visible | class('expanded', 'collapsed')",
			expected: ParsedIfCondition{
				BaseCondition: "visible",
				Modifiers: []IfModifier{
					{Type: "class", Args: []string{"expanded", "collapsed"}},
				},
			},
		},
		{
			name:  "condition with attr modifier",
			input: "active | attr('data-state', 'on', 'off')",
			expected: ParsedIfCondition{
				BaseCondition: "active",
				Modifiers: []IfModifier{
					{Type: "attr", Args: []string{"data-state", "on", "off"}},
				},
			},
		},
		{
			name:  "condition with multiple modifiers",
			input: "age > 0 | style('max-height: 100px', 'max-height: 0') | class('expanded', 'collapsed') | attr('data-born', 'true', 'false')",
			expected: ParsedIfCondition{
				BaseCondition: "age > 0",
				Modifiers: []IfModifier{
					{Type: "style", Args: []string{"max-height: 100px", "max-height: 0"}},
					{Type: "class", Args: []string{"expanded", "collapsed"}},
					{Type: "attr", Args: []string{"data-born", "true", "false"}},
				},
			},
		},
		{
			name:  "condition with double quotes",
			input: `age > 0 | style("max-height: 100px", "max-height: 0")`,
			expected: ParsedIfCondition{
				BaseCondition: "age > 0",
				Modifiers: []IfModifier{
					{Type: "style", Args: []string{"max-height: 100px", "max-height: 0"}},
				},
			},
		},
		{
			name:  "complex condition with modifiers",
			input: "user.age >= 18 && user.active | style('opacity: 1', 'opacity: 0.5') | class('adult', 'minor')",
			expected: ParsedIfCondition{
				BaseCondition: "user.age >= 18 && user.active",
				Modifiers: []IfModifier{
					{Type: "style", Args: []string{"opacity: 1", "opacity: 0.5"}},
					{Type: "class", Args: []string{"adult", "minor"}},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseIfCondition(tt.input)

			if result.BaseCondition != tt.expected.BaseCondition {
				t.Errorf("BaseCondition: got %q, want %q", result.BaseCondition, tt.expected.BaseCondition)
			}

			if len(result.Modifiers) != len(tt.expected.Modifiers) {
				t.Errorf("Modifiers count: got %d, want %d", len(result.Modifiers), len(tt.expected.Modifiers))
				return
			}

			for i, mod := range result.Modifiers {
				if mod.Type != tt.expected.Modifiers[i].Type {
					t.Errorf("Modifier %d Type: got %q, want %q", i, mod.Type, tt.expected.Modifiers[i].Type)
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

func TestParsedIfConditionGetters(t *testing.T) {
	parsed := ParsedIfCondition{
		BaseCondition: "age > 0",
		Modifiers: []IfModifier{
			{Type: "style", Args: []string{"max-height: 100px", "max-height: 0"}},
			{Type: "class", Args: []string{"expanded", "collapsed"}},
			{Type: "attr", Args: []string{"data-born", "true", "false"}},
		},
	}

	// Test GetStyleModifier
	styleMod := parsed.GetStyleModifier()
	if styleMod == nil {
		t.Error("GetStyleModifier() returned nil")
	} else if styleMod.Type != "style" {
		t.Errorf("GetStyleModifier() Type: got %q, want %q", styleMod.Type, "style")
	}

	// Test GetClassModifier
	classMod := parsed.GetClassModifier()
	if classMod == nil {
		t.Error("GetClassModifier() returned nil")
	} else if classMod.Type != "class" {
		t.Errorf("GetClassModifier() Type: got %q, want %q", classMod.Type, "class")
	}

	// Test GetAttrModifier
	attrMod := parsed.GetAttrModifier()
	if attrMod == nil {
		t.Error("GetAttrModifier() returned nil")
	} else if attrMod.Type != "attr" {
		t.Errorf("GetAttrModifier() Type: got %q, want %q", attrMod.Type, "attr")
	}

	// Test HasModifiers
	if !parsed.HasModifiers() {
		t.Error("HasModifiers() returned false, expected true")
	}

	// Test with no modifiers
	parsedNoMods := ParsedIfCondition{
		BaseCondition: "age > 0",
		Modifiers:     nil,
	}
	if parsedNoMods.HasModifiers() {
		t.Error("HasModifiers() returned true for empty modifiers, expected false")
	}
	if parsedNoMods.GetStyleModifier() != nil {
		t.Error("GetStyleModifier() returned non-nil for empty modifiers")
	}
}
