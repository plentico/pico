package pico

import (
	"strings"

	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/css"
)

func scopeCSS(style string, scopedElements []scopedElement) string {
	var out strings.Builder

	p := css.NewParser(parse.NewInputString(style), false)
	for {
		gt, _, data := p.Next()
		if gt == css.ErrorGrammar {
			break
		} else if gt == css.AtRuleGrammar || gt == css.BeginAtRuleGrammar || gt == css.BeginRulesetGrammar || gt == css.DeclarationGrammar {
			out.Write(data)
			if gt == css.DeclarationGrammar {
				out.WriteString(":")
			}
			for i, val := range p.Values() {
				if val.TokenType == css.HashToken {
					scopedClass := getScopedClass(string(val.Data), "id", scopedElements)
					if scopedClass != "" {
						out.WriteString(string(val.Data) + "." + scopedClass)
					} else {
						out.Write(val.Data)
					}
				} else if val.TokenType == css.IdentToken {
					if i > 0 && p.Values()[i-1].TokenType == css.DelimToken {
						scopedClass := getScopedClass(string(val.Data), "class", scopedElements)
						if scopedClass != "" {
							out.WriteString(string(val.Data) + "." + scopedClass)
						} else {
							out.Write(val.Data)
						}
					} else {
						scopedClass := getScopedClass(string(val.Data), "tag", scopedElements)
						if scopedClass != "" {
							out.WriteString(string(val.Data) + "." + scopedClass)
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
			} else if gt == css.AtRuleGrammar || gt == css.DeclarationGrammar {
				out.WriteString(";")
			}
		} else {
			out.Write(data)
		}
	}

	return out.String()
}
