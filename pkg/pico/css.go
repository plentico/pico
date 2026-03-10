package pico

import (
	"strings"

	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/css"
)

// Helper function to check if a selector is used in the component
func isSelectorUsed(selectorType string, selectorValue string, scopedElements []scopedElement) bool {
	for _, elem := range scopedElements {
		switch selectorType {
		case "tag":
			if elem.tag == selectorValue {
				return true
			}
		case "id":
			if elem.id == selectorValue {
				return true
			}
		case "class":
			for _, class := range elem.classes {
				if class == selectorValue {
					return true
				}
			}
		}
	}
	return false
}

// Helper function to check if a ruleset contains any global selectors
func hasGlobalSelector(values []css.Token) bool {
	for i, val := range values {
		if val.TokenType == css.DelimToken && string(val.Data) == "*" {
			// Check if the next token is a selector
			if i < len(values)-1 {
				nextToken := values[i+1]
				if nextToken.TokenType == css.IdentToken || nextToken.TokenType == css.HashToken {
					return true
				} else if nextToken.TokenType == css.DelimToken && string(nextToken.Data) == "." {
					return true
				}
			}
		}
	}
	return false
}

// outputScopedValues writes scoped selector tokens to the output builder
func outputScopedValues(values []css.Token, scopedElements []scopedElement, out *strings.Builder) {
	for i, val := range values {
		isGlobal := false
		skipOutput := false

		if val.TokenType == css.DelimToken && string(val.Data) == "*" {
			if i < len(values)-1 {
				nextToken := values[i+1]
				if nextToken.TokenType == css.IdentToken || nextToken.TokenType == css.HashToken {
					skipOutput = true
				} else if nextToken.TokenType == css.DelimToken && string(nextToken.Data) == "." {
					skipOutput = true
				}
			}
		}

		if i > 0 {
			prevToken := values[i-1]
			if prevToken.TokenType == css.DelimToken && string(prevToken.Data) == "*" {
				if val.TokenType == css.IdentToken || val.TokenType == css.HashToken {
					isGlobal = true
				}
			}
		}

		if i > 1 {
			twoTokensBack := values[i-2]
			oneTokenBack := values[i-1]
			if twoTokensBack.TokenType == css.DelimToken && string(twoTokensBack.Data) == "*" &&
				oneTokenBack.TokenType == css.DelimToken && string(oneTokenBack.Data) == "." &&
				val.TokenType == css.IdentToken {
				isGlobal = true
			}
		}

		if skipOutput {
			continue
		}

		if val.TokenType == css.HashToken {
			if !isGlobal {
				scopedClass := getScopedClass(string(val.Data), "id", scopedElements)
				if scopedClass != "" {
					out.WriteString(string(val.Data) + "." + scopedClass)
				} else {
					out.Write(val.Data)
				}
			} else {
				out.Write(val.Data)
			}
		} else if val.TokenType == css.IdentToken {
			if i > 0 && values[i-1].TokenType == css.DelimToken && string(values[i-1].Data) == "." {
				if !isGlobal {
					scopedClass := getScopedClass(string(val.Data), "class", scopedElements)
					if scopedClass != "" {
						out.WriteString(string(val.Data) + "." + scopedClass)
					} else {
						out.Write(val.Data)
					}
				} else {
					out.Write(val.Data)
				}
			} else {
				if !isGlobal {
					scopedClass := getScopedClass(string(val.Data), "tag", scopedElements)
					if scopedClass != "" {
						out.WriteString(string(val.Data) + "." + scopedClass)
					} else {
						out.Write(val.Data)
					}
				} else {
					out.Write(val.Data)
				}
			}
		} else {
			out.Write(val.Data)
		}
	}
}

// isSingleSimpleSelector returns true if the token list represents a single simple selector
// (no whitespace, no combinators - just one element/class/id possibly with chained classes)
func isSingleSimpleSelector(values []css.Token) bool {
	for _, val := range values {
		if val.TokenType == css.WhitespaceToken {
			return false
		}
		if val.TokenType == css.DelimToken {
			d := string(val.Data)
			if d == ">" || d == "+" || d == "~" {
				return false
			}
		}
	}
	return true
}

// Helper function to check if a ruleset should be kept (has used selectors or global selectors)
// values contains ALL tokens from ALL comma-separated selectors concatenated
func shouldKeepRuleset(values []css.Token, scopedElements []scopedElement) bool {
	// If it has global selectors, always keep it
	if hasGlobalSelector(values) {
		return true
	}

	// Keep standalone universal selector * or selectors containing attribute selectors [...]
	for i, val := range values {
		// Standalone * universal selector
		if val.TokenType == css.DelimToken && string(val.Data) == "*" {
			// Check it's not a global marker (next token is not a selector)
			isGlobalMarker := false
			if i < len(values)-1 {
				nextToken := values[i+1]
				if nextToken.TokenType == css.IdentToken || nextToken.TokenType == css.HashToken {
					isGlobalMarker = true
				} else if nextToken.TokenType == css.DelimToken && string(nextToken.Data) == "." {
					isGlobalMarker = true
				}
			}
			if !isGlobalMarker {
				return true // Keep universal selector
			}
		}
		// Attribute selector [...]
		if val.TokenType == css.LeftBracketToken {
			return true // Keep attribute selectors
		}
	}

	// Split the combined values back into individual selectors
	// (they were concatenated from QualifiedRuleGrammar + BeginRulesetGrammar calls)
	// We check each selector independently
	for i, val := range values {
		if val.TokenType == css.HashToken {
			// ID selector - only keep if it's a simple selector (no combinators/whitespace before it)
			idValue := string(val.Data)
			if len(idValue) > 0 && idValue[0] == '#' {
				idValue = idValue[1:]
			}
			if isSelectorUsed("id", idValue, scopedElements) {
				return true
			}
		} else if val.TokenType == css.IdentToken {
			isClassSelector := false
			if i > 0 && values[i-1].TokenType == css.DelimToken && string(values[i-1].Data) == "." {
				isClassSelector = true
			}

			// Skip pseudo-selectors
			if i > 0 && values[i-1].TokenType == css.ColonToken {
				continue
			}

			valStr := string(val.Data)

			if isClassSelector {
				// Only count this class if it's the FIRST class in the selector
				// (i.e., not preceded by whitespace or combinator - meaning it's the root selector)
				// For .container.fake, we should NOT match just .container
				// For .container, we SHOULD match .container
				// Check if there's a whitespace or combinator before the dot
				isRootClass := true
				if i >= 2 {
					// Look back past the dot to see what's before it
					prevPrevToken := values[i-2]
					if prevPrevToken.TokenType == css.WhitespaceToken {
						// This is a descendant selector like ".parent .child" - not a root class
						isRootClass = false
					} else if prevPrevToken.TokenType == css.DelimToken {
						d := string(prevPrevToken.Data)
						if d == ">" || d == "+" || d == "~" {
							isRootClass = false
						} else if d == "." {
							// This is a chained class like .container.fake
							// Only keep if ALL chained classes exist together
							// For simplicity, don't match chained classes individually
							isRootClass = false
						}
					} else if prevPrevToken.TokenType == css.IdentToken {
						// This is a chained class like .container.fake (ident before dot)
						isRootClass = false
					}
				}

				if isRootClass && isSelectorUsed("class", valStr, scopedElements) {
					return true
				}
			} else {
				// Tag selector - only keep if it's a simple/root selector
				if isSelectorUsed("tag", valStr, scopedElements) {
					return true
				}
			}
		}
	}
	return false
}

func scopeCSS(style string, scopedElements []scopedElement) string {
	var out strings.Builder

	p := css.NewParser(parse.NewInputString(style), false)

	// Track if we're in a ruleset that should be skipped
	skipCurrentRuleset := false
	rulesetDepth := 0

	// Buffer for comma-separated selectors - we need to accumulate all selectors
	// before deciding whether to keep the ruleset
	var pendingSelectors [][]css.Token

	for {
		gt, _, data := p.Next()
		if gt == css.ErrorGrammar {
			break
		} else if skipCurrentRuleset {
			// We're inside a ruleset that should be skipped
			if gt == css.BeginRulesetGrammar || gt == css.BeginAtRuleGrammar {
				rulesetDepth++
			} else if gt == css.EndRulesetGrammar || gt == css.EndAtRuleGrammar {
				rulesetDepth--
				if rulesetDepth == 0 {
					skipCurrentRuleset = false
				}
			}
			continue
		} else if gt == css.QualifiedRuleGrammar {
			// Buffer this selector - we'll decide whether to output after seeing all selectors
			// QualifiedRuleGrammar fires for each selector EXCEPT the last in a comma-separated list
			values := p.Values()
			tokensCopy := make([]css.Token, len(values))
			copy(tokensCopy, values)
			pendingSelectors = append(pendingSelectors, tokensCopy)
			continue
		} else if gt == css.BeginRulesetGrammar {
			// The parser gives us:
			// - QualifiedRuleGrammar: each selector EXCEPT the last (for comma-separated lists)
			// - BeginRulesetGrammar: the LAST (or only) selector
			// data is always "" for both; tokens are in p.Values()
			lastValues := p.Values()

			// Combine all selector tokens for the keep check
			var allValues []css.Token
			for _, sel := range pendingSelectors {
				allValues = append(allValues, sel...)
			}
			allValues = append(allValues, lastValues...)

			if !shouldKeepRuleset(allValues, scopedElements) {
				skipCurrentRuleset = true
				rulesetDepth = 1
				pendingSelectors = nil
				continue
			}

			// Output all buffered (non-last) selectors with scoping
			hasPending := len(pendingSelectors) > 0
			for si, sel := range pendingSelectors {
				if si > 0 {
					out.WriteString(", ")
				}
				outputScopedValues(sel, scopedElements, &out)
			}
			pendingSelectors = nil

			// Output the last selector (from BeginRulesetGrammar)
			if hasPending {
				out.WriteString(", ")
			}
			outputScopedValues(lastValues, scopedElements, &out)
			out.WriteString("{")
			continue
		}

		if gt == css.AtRuleGrammar || gt == css.BeginAtRuleGrammar || gt == css.DeclarationGrammar {
			out.Write(data)
			if gt == css.DeclarationGrammar {
				out.WriteString(":")
			}

			values := p.Values()
			for i, val := range values {
				// Check if this token should be globally scoped (not scoped)
				// A token is global if it's directly preceded by * delimiter
				isGlobal := false
				skipOutput := false

				if val.TokenType == css.DelimToken && string(val.Data) == "*" {
					// Check if the next token is a selector (no whitespace in between)
					if i < len(values)-1 {
						nextToken := values[i+1]
						// If next token is IdentToken (tag selector like *p) or HashToken (id like *#myid), this is a global marker
						if nextToken.TokenType == css.IdentToken || nextToken.TokenType == css.HashToken {
							// This is a global marker, don't output it
							skipOutput = true
						} else if nextToken.TokenType == css.DelimToken && string(nextToken.Data) == "." {
							// This is *.class pattern, also a global marker
							skipOutput = true
						}
					}
				}

				// Check if previous token was a global marker
				if i > 0 {
					prevToken := values[i-1]
					if prevToken.TokenType == css.DelimToken && string(prevToken.Data) == "*" {
						// Check if we're a selector token directly after *
						if val.TokenType == css.IdentToken || val.TokenType == css.HashToken {
							isGlobal = true
						}
					}
				}

				// Check if we're 2 tokens after a global marker (for *.class pattern)
				if i > 1 {
					twoTokensBack := values[i-2]
					oneTokenBack := values[i-1]
					if twoTokensBack.TokenType == css.DelimToken && string(twoTokensBack.Data) == "*" &&
						oneTokenBack.TokenType == css.DelimToken && string(oneTokenBack.Data) == "." &&
						val.TokenType == css.IdentToken {
						// This is the class name in *.class pattern
						isGlobal = true
					}
				}

				if skipOutput {
					continue
				}

				if val.TokenType == css.HashToken {
					// ID selector (#id)
					if !isGlobal {
						scopedClass := getScopedClass(string(val.Data), "id", scopedElements)
						if scopedClass != "" {
							out.WriteString(string(val.Data) + "." + scopedClass)
						} else {
							out.Write(val.Data)
						}
					} else {
						out.Write(val.Data)
					}
				} else if val.TokenType == css.IdentToken {
					// Could be tag selector or class name after .
					if i > 0 && values[i-1].TokenType == css.DelimToken && string(values[i-1].Data) == "." {
						// This is a class selector
						if !isGlobal {
							scopedClass := getScopedClass(string(val.Data), "class", scopedElements)
							if scopedClass != "" {
								out.WriteString(string(val.Data) + "." + scopedClass)
							} else {
								out.Write(val.Data)
							}
						} else {
							out.Write(val.Data)
						}
					} else {
						// This is a tag selector
						if !isGlobal {
							scopedClass := getScopedClass(string(val.Data), "tag", scopedElements)
							if scopedClass != "" {
								out.WriteString(string(val.Data) + "." + scopedClass)
							} else {
								out.Write(val.Data)
							}
						} else {
							out.Write(val.Data)
						}
					}
				} else {
					out.Write(val.Data)
				}
			}

			if gt == css.BeginAtRuleGrammar || gt == css.BeginRulesetGrammar {
				out.WriteString("{")
			} else if gt == css.QualifiedRuleGrammar {
				out.WriteString(", ")
			} else if gt == css.AtRuleGrammar || gt == css.DeclarationGrammar {
				out.WriteString(";")
			}
		} else {
			out.Write(data)
		}
	}

	return out.String()
}
