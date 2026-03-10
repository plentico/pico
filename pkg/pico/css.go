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

// Helper function to check if a ruleset should be kept (has used selectors or global selectors)
func shouldKeepRuleset(values []css.Token, scopedElements []scopedElement) bool {
	// If it has global selectors, always keep it
	if hasGlobalSelector(values) {
		return true
	}

	// Check if ANY selector in the comma-separated list is used
	// If at least one selector is found, we keep the entire rule and scope all selectors
	for i, val := range values {
		if val.TokenType == css.HashToken {
			// ID selector - the Data includes the # character
			idValue := string(val.Data)
			if len(idValue) > 0 && idValue[0] == '#' {
				idValue = idValue[1:]
			}
			if isSelectorUsed("id", idValue, scopedElements) {
				return true
			}
		} else if val.TokenType == css.IdentToken {
			// Could be tag or class selector
			isClassSelector := false
			if i > 0 && values[i-1].TokenType == css.DelimToken && string(values[i-1].Data) == "." {
				isClassSelector = true
			}

			// Check if this looks like a pseudo-selector or other CSS keyword
			valStr := string(val.Data)
			isPseudoOrKeyword := false
			if i > 0 && values[i-1].TokenType == css.ColonToken {
				isPseudoOrKeyword = true
			}

			// Skip CSS keywords and pseudo-selectors
			if isPseudoOrKeyword {
				continue
			}

			if isClassSelector {
				if isSelectorUsed("class", valStr, scopedElements) {
					return true
				}
			} else {
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
		} else if gt == css.BeginRulesetGrammar {
			// Check if this ruleset should be kept BEFORE we start processing it
			values := p.Values()
			if !shouldKeepRuleset(values, scopedElements) {
				skipCurrentRuleset = true
				rulesetDepth = 1
				// Skip the entire ruleset
				continue
			}
			// If we get here, we should keep this ruleset, so fall through to normal processing
		}

		if gt == css.AtRuleGrammar || gt == css.BeginAtRuleGrammar || gt == css.QualifiedRuleGrammar || gt == css.BeginRulesetGrammar || gt == css.DeclarationGrammar {
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
