package pico

import (
	"regexp"
	"strings"
)

// Modifier represents a single modifier with its name and arguments
type Modifier struct {
	Name string
	Args []string
}

// ParsedExpression represents a parsed expression with its base variable and modifiers
type ParsedExpression struct {
	Base      string
	Modifiers []Modifier
}

// ParseExpression parses an expression like "text | trim(20) | html('a', 'div')"
// Returns the base expression and a list of modifiers
func ParseExpression(expr string) ParsedExpression {
	expr = strings.TrimSpace(expr)

	// Split by pipe to get parts
	parts := splitByPipe(expr)
	if len(parts) == 0 {
		return ParsedExpression{Base: expr}
	}

	// First part is the base expression
	result := ParsedExpression{
		Base: strings.TrimSpace(parts[0]),
	}

	// Remaining parts are modifiers
	for i := 1; i < len(parts); i++ {
		modifier := parseModifier(strings.TrimSpace(parts[i]))
		if modifier.Name != "" {
			result.Modifiers = append(result.Modifiers, modifier)
		}
	}

	return result
}

// splitByPipe splits a string by pipe character, but not inside quotes or parentheses
func splitByPipe(s string) []string {
	var parts []string
	var current strings.Builder
	inQuotes := false
	quoteChar := rune(0)
	parenDepth := 0

	for _, ch := range s {
		switch ch {
		case '"', '\'':
			if !inQuotes {
				inQuotes = true
				quoteChar = ch
			} else if ch == quoteChar {
				inQuotes = false
				quoteChar = 0
			}
			current.WriteRune(ch)
		case '(':
			if !inQuotes {
				parenDepth++
			}
			current.WriteRune(ch)
		case ')':
			if !inQuotes {
				parenDepth--
			}
			current.WriteRune(ch)
		case '|':
			if !inQuotes && parenDepth == 0 {
				parts = append(parts, current.String())
				current.Reset()
			} else {
				current.WriteRune(ch)
			}
		default:
			current.WriteRune(ch)
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

// parseModifier parses a single modifier like "trim(20)" or "html('a', 'div')"
func parseModifier(s string) Modifier {
	s = strings.TrimSpace(s)

	// Match modifier name and optional arguments
	// Pattern: name(args) or name()
	re := regexp.MustCompile(`^(\w+)\s*(?:\((.*)\))?$`)
	matches := re.FindStringSubmatch(s)

	if len(matches) == 0 {
		return Modifier{Name: s}
	}

	modifier := Modifier{
		Name: matches[1],
	}

	// Parse arguments if present
	if len(matches) > 2 && matches[2] != "" {
		modifier.Args = parseArgs(matches[2])
	}

	return modifier
}

// parseArgs parses comma-separated arguments, handling quoted strings
func parseArgs(s string) []string {
	var args []string
	var current strings.Builder
	inQuotes := false
	quoteChar := rune(0)

	for _, ch := range s {
		switch ch {
		case '"', '\'':
			if !inQuotes {
				inQuotes = true
				quoteChar = ch
			} else if ch == quoteChar {
				inQuotes = false
				quoteChar = 0
			} else {
				current.WriteRune(ch)
			}
		case ',':
			if !inQuotes {
				arg := strings.TrimSpace(current.String())
				// Remove quotes if present
				arg = strings.Trim(arg, `"'`)
				args = append(args, arg)
				current.Reset()
			} else {
				current.WriteRune(ch)
			}
		default:
			current.WriteRune(ch)
		}
	}

	if current.Len() > 0 {
		arg := strings.TrimSpace(current.String())
		arg = strings.Trim(arg, `"'`)
		args = append(args, arg)
	}

	return args
}

// BuildPattrAttribute builds the p-text or p-html attribute from a parsed expression
// Returns the attribute key and value
func BuildPattrAttribute(parsed ParsedExpression) (string, string) {
	baseValue := parsed.Base

	// Determine if we should use p-html or p-text
	useHTML := false
	var htmlArgs []string

	// Build modifier suffixes
	var modifiers []string

	for _, mod := range parsed.Modifiers {
		switch mod.Name {
		case "html":
			useHTML = true
			htmlArgs = mod.Args
		case "trim":
			if len(mod.Args) > 0 {
				modifiers = append(modifiers, "trim."+mod.Args[0])
			} else {
				modifiers = append(modifiers, "trim")
			}
		}
	}

	// Build attribute key
	attrKey := "p-text"
	if useHTML {
		attrKey = "p-html"
	}

	// Add modifiers to key
	for _, mod := range modifiers {
		attrKey += ":" + mod
	}

	// Add html allow list if present
	if useHTML && len(htmlArgs) > 0 {
		attrKey += ":allow"
		for _, arg := range htmlArgs {
			attrKey += "." + arg
		}
	}

	return attrKey, baseValue
}

// ProcessExpression processes a full expression with braces like "{text | trim(20) | html('a')}"
// Returns the replacement string for the template
func ProcessExpression(expr string) (string, string) {
	// Remove braces
	inner := strings.TrimSpace(expr)
	if strings.HasPrefix(inner, "{") && strings.HasSuffix(inner, "}") {
		inner = inner[1 : len(inner)-1]
	}

	parsed := ParseExpression(inner)
	attrKey, attrValue := BuildPattrAttribute(parsed)

	return attrKey, attrValue
}
