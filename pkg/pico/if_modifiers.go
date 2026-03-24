package pico

import (
	"strings"
)

// IfModifier represents a modifier for if-statements (style, class, attr)
type IfModifier struct {
	Type string   // "style", "class", "attr"
	Args []string // Arguments for the modifier
}

// ParsedIfCondition represents a parsed if-condition with its base condition and modifiers
type ParsedIfCondition struct {
	BaseCondition string
	Modifiers     []IfModifier
}

// ParseIfCondition parses an if-condition with optional modifiers like:
// "age > 0 | style('max-height: 100px', 'max-height: 0') | class('expanded', 'collapsed')"
func ParseIfCondition(condition string) ParsedIfCondition {
	condition = strings.TrimSpace(condition)

	// Split by pipe to get parts
	parts := splitByPipe(condition)
	if len(parts) == 0 {
		return ParsedIfCondition{BaseCondition: condition}
	}

	// First part is the base condition
	result := ParsedIfCondition{
		BaseCondition: strings.TrimSpace(parts[0]),
	}

	// Remaining parts are modifiers
	for i := 1; i < len(parts); i++ {
		modifier := parseIfModifier(strings.TrimSpace(parts[i]))
		if modifier.Type != "" {
			result.Modifiers = append(result.Modifiers, modifier)
		}
	}

	return result
}

// parseIfModifier parses a single if-modifier like "style('max-height: 100px', 'max-height: 0')"
func parseIfModifier(s string) IfModifier {
	s = strings.TrimSpace(s)

	// Match modifier type and arguments
	// Pattern: type(args) or type()
	// Find the opening parenthesis
	parenIdx := strings.Index(s, "(")
	if parenIdx == -1 {
		return IfModifier{Type: s}
	}

	modType := strings.TrimSpace(s[:parenIdx])

	// Extract arguments
	if !strings.HasSuffix(s, ")") {
		return IfModifier{Type: modType}
	}

	argsStr := s[parenIdx+1 : len(s)-1]
	args := parseArgs(argsStr)

	return IfModifier{
		Type: modType,
		Args: args,
	}
}

// GetStyleModifier returns the style modifier if present
func (p ParsedIfCondition) GetStyleModifier() *IfModifier {
	for _, mod := range p.Modifiers {
		if mod.Type == "style" {
			return &mod
		}
	}
	return nil
}

// GetClassModifier returns the class modifier if present
func (p ParsedIfCondition) GetClassModifier() *IfModifier {
	for _, mod := range p.Modifiers {
		if mod.Type == "class" {
			return &mod
		}
	}
	return nil
}

// GetAttrModifier returns the attr modifier if present
func (p ParsedIfCondition) GetAttrModifier() *IfModifier {
	for _, mod := range p.Modifiers {
		if mod.Type == "attr" {
			return &mod
		}
	}
	return nil
}

// HasModifiers returns true if there are any modifiers
func (p ParsedIfCondition) HasModifiers() bool {
	return len(p.Modifiers) > 0
}
