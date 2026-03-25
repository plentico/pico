package pico

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func parseNoFix(input string) ([]*html.Node, error) {
	var nodes []*html.Node
	z := html.NewTokenizer(strings.NewReader(input))

	var stack []*html.Node
	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			err := z.Err()
			if err == io.EOF {
				return nodes, nil
			}
			return nil, err

		case html.StartTagToken, html.SelfClosingTagToken:
			token := z.Token()
			node := &html.Node{
				Type:     html.ElementNode,
				DataAtom: token.DataAtom,
				Data:     token.Data,
				Attr:     token.Attr,
			}

			if len(stack) > 0 {
				parent := stack[len(stack)-1]
				if parent.LastChild == nil {
					parent.FirstChild = node
				} else {
					parent.LastChild.NextSibling = node
				}
				node.PrevSibling = parent.LastChild
				parent.LastChild = node
				node.Parent = parent
			} else {
				nodes = append(nodes, node)
			}

			if token.Type != html.SelfClosingTagToken && !isVoidElement(token.DataAtom) {
				stack = append(stack, node)
			}

		case html.EndTagToken:
			token := z.Token()
			if len(stack) == 0 {
				continue
			}
			if stack[len(stack)-1].Data == token.Data {
				stack = stack[:len(stack)-1]
			}

		case html.TextToken:
			token := z.Token()
			node := &html.Node{
				Type: html.TextNode,
				Data: string(token.Data),
			}
			if len(stack) > 0 {
				parent := stack[len(stack)-1]
				if parent.LastChild == nil {
					parent.FirstChild = node
				} else {
					parent.LastChild.NextSibling = node
				}
				node.PrevSibling = parent.LastChild
				parent.LastChild = node
				node.Parent = parent
			} else {
				nodes = append(nodes, node)
			}

		case html.CommentToken:
			token := z.Token()
			node := &html.Node{
				Type: html.CommentNode,
				Data: string(token.Data),
			}
			if len(stack) > 0 {
				parent := stack[len(stack)-1]
				if parent.LastChild == nil {
					parent.FirstChild = node
				} else {
					parent.LastChild.NextSibling = node
				}
				node.PrevSibling = parent.LastChild
				parent.LastChild = node
				node.Parent = parent
			} else {
				nodes = append(nodes, node)
			}

		case html.DoctypeToken:
			token := z.Token()
			node := &html.Node{
				Type: html.DoctypeNode,
				Data: string(token.Data),
			}
			nodes = append(nodes, node)
		}
	}
}

func isVoidElement(a atom.Atom) bool {
	switch a {
	case atom.Area, atom.Base, atom.Br, atom.Col, atom.Command,
		atom.Embed, atom.Hr, atom.Img, atom.Input, atom.Keygen,
		atom.Link, atom.Meta, atom.Param, atom.Source, atom.Track, atom.Wbr:
		return true
	default:
		return false
	}
}

// extractPClassNames parses a p-class ternary expression and extracts class names
// Format: condition ? 'trueClass' : 'falseClass'
// Only extracts classes from the true/false branches (after the ?)
func extractPClassNames(pClassVal string) []string {
	var classes []string

	// Find the position of ? to skip the condition part
	qIdx := strings.Index(pClassVal, "?")
	if qIdx == -1 {
		return classes
	}

	// Only look at the part after ?
	tBranchPart := pClassVal[qIdx+1:]

	// Regex to match quoted strings in the true/false branches
	re := regexp.MustCompile(`'([^']*)'`)
	matches := re.FindAllStringSubmatch(tBranchPart, -1)
	for _, match := range matches {
		if len(match) > 1 && match[1] != "" {
			// Split by space in case multiple classes are in one string
			for _, class := range strings.Fields(match[1]) {
				if class != "" {
					classes = append(classes, class)
				}
			}
		}
	}
	return classes
}

func scopeHTML(markup string, props map[string]any, pScopeExp string, fence string, usePattr bool) (string, []scopedElement) {
	scopedElements := []scopedElement{}
	var markupBuilder strings.Builder

	nodes, err := parseNoFix(markup)
	if err != nil {
		return markup, scopedElements
	}

	for _, node := range nodes {
		if node.Type == html.ElementNode && node.Data == "html" {
			for c := node.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode && c.Data == "head" {
					if len(props) > 0 {
						rootData, _ := json.Marshal(props)
						rootDataScript := &html.Node{
							Type: html.ElementNode,
							Data: "script",
							Attr: []html.Attribute{
								{Key: "id", Val: "p-root-data"},
								{Key: "type", Val: "application/json"},
							},
						}
						rootDataScript.AppendChild(&html.Node{
							Type: html.TextNode,
							Data: string(rootData),
						})

						if c.FirstChild != nil {
							c.InsertBefore(rootDataScript, c.FirstChild)
						} else {
							c.AppendChild(rootDataScript)
						}
					}
				}
			}
		}

		if usePattr && node.Type == html.ElementNode && (len(props) > 0 || pScopeExp != "") {
			if node.Data != "html" {
				pScopeExp = flattenCompArgs(props) + pScopeExp
			}
			node.Attr = append(node.Attr, html.Attribute{Key: "p-scope", Val: pScopeExp})
		}

		node, scopedElements = traverse(node, scopedElements, fence, usePattr)

		if err := html.Render(&markupBuilder, node); err != nil {
			log.Fatal(err)
		}
	}

	return markupBuilder.String(), scopedElements
}

func traverse(node *html.Node, scopedElements []scopedElement, fence string, usePattr bool) (*html.Node, []scopedElement) {
	var traverseFunc func(*html.Node)
	traverseFunc = func(node *html.Node) {
		if node.Type == html.TextNode {
			if strings.Contains(node.Data, "{") && strings.Contains(node.Data, "}") {
				if p := node.Parent; p != nil && p.Data == "script" {
					for _, attr := range p.Attr {
						if attr.Key == "type" && attr.Val == "application/json" {
							return
						}
					}
				}
				if usePattr {
					// Check if this is a simple expression or one with modifiers
					expr := node.Data
					// Extract the expression between braces
					startIdx := strings.Index(expr, "{")
					endIdx := strings.LastIndex(expr, "}")
					if startIdx != -1 && endIdx != -1 && endIdx > startIdx {
						innerExpr := expr[startIdx : endIdx+1]
						// Check if expression contains pipe (modifier syntax)
						if strings.Contains(innerExpr, "|") {
							// Process with modifiers
							attrKey, attrValue := ProcessExpression(innerExpr)
							attr := html.Attribute{
								Key: attrKey,
								Val: attrValue,
							}
							node.Parent.Attr = append(node.Parent.Attr, attr)
							// Evaluate for SSR
							parsed := ParseExpression(innerExpr[1 : len(innerExpr)-1])
							evaluated := fmt.Sprintf("%v", evalJS(parsed.Base, fence))
							isHTML := false
							var allowedTags []string
							for _, mod := range parsed.Modifiers {
								// Check if using html modifier and get allowed tags
								if mod.Name == "html" {
									isHTML = true
									allowedTags = mod.Args
									break
								}
								// Apply trim if specified
								if mod.Name == "trim" && len(mod.Args) > 0 {
									if maxLen, err := strconv.Atoi(mod.Args[0]); err == nil {
										evaluated = trimHTML(evaluated, maxLen)
									}
								}
							}
							// Sanitize HTML to only allow specified tags
							if isHTML && len(allowedTags) > 0 {
								evaluated = sanitizeHTML(evaluated, allowedTags)
							}
							if isHTML {
								// Parse evaluated HTML and insert as nodes
								htmlNodes, err := parseNoFix(evaluated)
								if err == nil && len(htmlNodes) > 0 {
									// Get text before and after the expression
									beforeText := expr[:startIdx]
									afterText := expr[endIdx+1:]
									// Clear current node data
									node.Data = ""
									// Insert before text if any
									if beforeText != "" {
										node.Data = beforeText
									}
									// Insert parsed HTML nodes
									for i, n := range htmlNodes {
										if i == 0 {
											// First node replaces current or is inserted after
											if beforeText == "" {
												// Replace current node's data with first text node content if it's text
												if n.Type == html.TextNode {
													node.Data = n.Data
												} else {
													// Insert element node as sibling
													if node.Parent != nil {
														node.Parent.InsertBefore(n, node.NextSibling)
													}
												}
											} else {
												// Add to existing text
												if n.Type == html.TextNode {
													node.Data += n.Data
												}
											}
										} else {
											// Insert subsequent nodes after current
											if node.Parent != nil {
												node.Parent.InsertBefore(n, node.NextSibling)
											}
										}
									}
									// Add after text to last inserted node or current
									if afterText != "" {
										// Find the last node we inserted
										lastNode := node
										for sibling := node.NextSibling; sibling != nil; sibling = sibling.NextSibling {
											lastNode = sibling
										}
										if lastNode.Type == html.TextNode {
											lastNode.Data += afterText
										} else {
											// Insert text node after
											textNode := &html.Node{
												Type: html.TextNode,
												Data: afterText,
											}
											if lastNode.Parent != nil {
												lastNode.Parent.InsertBefore(textNode, lastNode.NextSibling)
											}
										}
									}
								} else {
									// Fallback: treat as text
									node.Data = expr[:startIdx] + evaluated + expr[endIdx+1:]
								}
							} else {
								// Plain text - just set the data
								node.Data = expr[:startIdx] + evaluated + expr[endIdx+1:]
							}
						} else {
							// Original behavior for simple expressions
							attr := html.Attribute{
								Key: "p-text",
								Val: "`" + strings.ReplaceAll(strings.ReplaceAll(node.Data, "{", "${"), "\"", "'") + "`",
							}
							node.Parent.Attr = append(node.Parent.Attr, attr)
						}
					} else {
						// Original behavior
						attr := html.Attribute{
							Key: "p-text",
							Val: "`" + strings.ReplaceAll(strings.ReplaceAll(node.Data, "{", "${"), "\"", "'") + "`",
						}
						node.Parent.Attr = append(node.Parent.Attr, attr)
					}
				}
			}
			node.Data = evalAllBrackets(node.Data, fence)
		}
		if node.Type == html.ElementNode && node.DataAtom.String() != "" {
			tag := node.Data
			id := ""
			classes := []string{}
			scopedClass := getScopedClass(tag, "tag", scopedElements)

			if scopedClass == "" {
				randomStr, err := generateRandom()
				if err != nil {
					log.Fatal(err)
				}
				scopedClass = "p-" + randomStr
			}

			// Track attributes to remove
			attrsToRemove := []int{}

			for i, attr := range node.Attr {
				if attr.Key == "id" {
					id = attr.Val
				}
				if attr.Key == "class" {
					classes = strings.Split(attr.Val, " ")
					alreadyScoped := false
					for _, class := range classes {
						if strings.HasPrefix(class, "p-") {
							alreadyScoped = true
							scopedClass = class
						}
					}
					if !alreadyScoped {
						node.Attr[i].Val += " " + scopedClass
					}
				}
				// Extract classes from p-class attribute for CSS treeshaking
				if attr.Key == "p-class" {
					pClassNames := extractPClassNames(attr.Val)
					classes = append(classes, pClassNames...)
				}
				if strings.Contains(attr.Val, "{") && strings.Contains(attr.Val, "}") {
					if attr.Key != "p-text" && attr.Key != "p-scope" && !strings.HasPrefix(attr.Key, "p-attr") && !strings.HasPrefix(attr.Key, "p-on") && attr.Key != "p-model" {
						if strings.HasPrefix(attr.Key, "on") {
							eventName := attr.Key[2:]
							expr := processEventHandler(attr.Val)
							if usePattr {
								node.Attr = append(node.Attr, html.Attribute{
									Key: "p-on:" + eventName,
									Val: expr,
								})
							}
							// Mark this attribute for removal
							attrsToRemove = append(attrsToRemove, i)
						} else if attr.Key == "value" && node.Data == "input" {
							expr := strings.TrimSpace(attr.Val)
							if strings.HasPrefix(expr, "{") && strings.HasSuffix(expr, "}") {
								varName := expr[1 : len(expr)-1]
								if usePattr {
									node.Attr = append(node.Attr, html.Attribute{
										Key: "p-model",
										Val: varName,
									})
								}
							}
							node.Attr[i].Val = evalAllBrackets(attr.Val, fence)
						} else {
							if usePattr {
								node.Attr = append(node.Attr, html.Attribute{
									Key: "p-attr:" + attr.Key,
									Val: "`" + strings.ReplaceAll(strings.ReplaceAll(attr.Val, "{", "${"), "\"", "'") + "`",
								})
							}
							node.Attr[i].Val = evalAllBrackets(attr.Val, fence)
						}
					}
				}
			}

			// Remove marked attributes (iterate backwards to maintain indices)
			for i := len(attrsToRemove) - 1; i >= 0; i-- {
				idx := attrsToRemove[i]
				node.Attr = append(node.Attr[:idx], node.Attr[idx+1:]...)
			}

			if len(classes) == 0 {
				node.Attr = append(node.Attr, html.Attribute{Key: "class", Val: scopedClass})
			}

			scopedElements = append(scopedElements, scopedElement{
				tag:         tag,
				id:          id,
				classes:     classes,
				scopedClass: scopedClass,
			})
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			traverseFunc(child)
		}
	}
	traverseFunc(node)

	return node, scopedElements
}

func processLoopIteration(markup string, loopFence string, forVar string, item any, scopeId int, index int, usePattr bool) string {
	nodes, err := parseNoFix(markup)
	if err != nil {
		return markup
	}

	pScopeVal := forVar + " = " + makeAttrStr(anyToString(item)) + ";"

	var buf strings.Builder
	for _, node := range nodes {
		if usePattr && node.Type == html.ElementNode {
			node.Attr = append(node.Attr, html.Attribute{Key: "p-scope", Val: pScopeVal})
			node.Attr = append(node.Attr, html.Attribute{Key: "p-for-key", Val: "s" + strconv.Itoa(scopeId) + ":" + strconv.Itoa(index)})
		}

		processLoopNode(node, loopFence, usePattr)

		if err := html.Render(&buf, node); err != nil {
			log.Fatal(err)
		}
	}
	return buf.String()
}

func processLoopNode(node *html.Node, loopFence string, usePattr bool) {
	if node.Type == html.TextNode {
		if strings.Contains(node.Data, "{") && strings.Contains(node.Data, "}") {
			if p := node.Parent; p != nil && p.Data == "script" {
				for _, attr := range p.Attr {
					if attr.Key == "type" && attr.Val == "application/json" {
						return
					}
				}
			}
			if usePattr {
				attr := html.Attribute{
					Key: "p-text",
					Val: "`" + strings.ReplaceAll(strings.ReplaceAll(node.Data, "{", "${"), "\"", "'") + "`",
				}
				node.Parent.Attr = append(node.Parent.Attr, attr)
			}
		}
		node.Data = evalAllBrackets(node.Data, loopFence)
	}
	if node.Type == html.ElementNode {
		// Track attributes to remove
		attrsToRemove := []int{}

		for i, attr := range node.Attr {
			if strings.Contains(attr.Val, "{") && strings.Contains(attr.Val, "}") {
				if attr.Key != "p-text" && attr.Key != "p-scope" && !strings.HasPrefix(attr.Key, "p-attr") && !strings.HasPrefix(attr.Key, "p-on") && attr.Key != "p-model" {
					if strings.HasPrefix(attr.Key, "on") {
						eventName := attr.Key[2:]
						expr := processEventHandler(attr.Val)
						if usePattr {
							node.Attr = append(node.Attr, html.Attribute{
								Key: "p-on:" + eventName,
								Val: expr,
							})
						}
						// Mark this attribute for removal
						attrsToRemove = append(attrsToRemove, i)
					} else if attr.Key == "value" && node.Data == "input" {
						expr := strings.TrimSpace(attr.Val)
						if strings.HasPrefix(expr, "{") && strings.HasSuffix(expr, "}") {
							varName := expr[1 : len(expr)-1]
							if usePattr {
								node.Attr = append(node.Attr, html.Attribute{
									Key: "p-model",
									Val: varName,
								})
							}
						}
						node.Attr[i].Val = evalAllBrackets(attr.Val, loopFence)
					} else {
						if usePattr {
							node.Attr = append(node.Attr, html.Attribute{
								Key: "p-attr:" + attr.Key,
								Val: "`" + strings.ReplaceAll(strings.ReplaceAll(attr.Val, "{", "${"), "\"", "'") + "`",
							})
						}
						node.Attr[i].Val = evalAllBrackets(attr.Val, loopFence)
					}
				}
			}
		}

		// Remove marked attributes (iterate backwards to maintain indices)
		for i := len(attrsToRemove) - 1; i >= 0; i-- {
			idx := attrsToRemove[i]
			node.Attr = append(node.Attr[:idx], node.Attr[idx+1:]...)
		}
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		processLoopNode(child, loopFence, usePattr)
	}
}

func processLoopTemplate(markup string, loopFence string, usePattr bool) string {
	nodes, err := parseNoFix(markup)
	if err != nil {
		return markup
	}

	var buf strings.Builder
	for _, node := range nodes {
		processLoopNode(node, loopFence, usePattr)
		if err := html.Render(&buf, node); err != nil {
			log.Fatal(err)
		}
	}
	return buf.String()
}

// stripHTMLTags removes HTML tags from a string and returns plain text
func stripHTMLTags(html string) string {
	var result strings.Builder
	inTag := false
	for _, ch := range html {
		if ch == '<' {
			inTag = true
			continue
		}
		if ch == '>' {
			inTag = false
			continue
		}
		if !inTag {
			result.WriteRune(ch)
		}
	}
	return result.String()
}

// trimHTML trims HTML content to maxLen visible characters while preserving HTML structure
func trimHTML(html string, maxLen int) string {
	if maxLen <= 0 {
		return html
	}

	type token struct {
		isTag bool
		data  string
	}

	// Parse into tokens
	var tokens []token
	var current strings.Builder
	inTag := false

	for _, ch := range html {
		if ch == '<' {
			if current.Len() > 0 {
				tokens = append(tokens, token{isTag: inTag, data: current.String()})
				current.Reset()
			}
			inTag = true
			current.WriteRune(ch)
		} else if ch == '>' {
			current.WriteRune(ch)
			tokens = append(tokens, token{isTag: true, data: current.String()})
			current.Reset()
			inTag = false
		} else {
			current.WriteRune(ch)
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, token{isTag: inTag, data: current.String()})
	}

	// Build output with trim
	var result strings.Builder
	visibleCount := 0
	trimmed := false
	openTags := []string{}

	for i, tok := range tokens {
		if tok.isTag {
			// Check if it's an opening or closing tag
			data := strings.TrimSpace(tok.data)
			if strings.HasPrefix(data, "</") {
				// Closing tag
				if !trimmed {
					result.WriteString(tok.data)
					// Pop from open tags
					if len(openTags) > 0 {
						openTags = openTags[:len(openTags)-1]
					}
				}
			} else if strings.HasSuffix(data, "/>") || strings.HasSuffix(data, "/ >") {
				// Self-closing tag
				if !trimmed {
					result.WriteString(tok.data)
				}
			} else {
				// Opening tag
				if !trimmed {
					result.WriteString(tok.data)
					// Extract tag name
					tagName := extractTagName(data)
					if tagName != "" {
						openTags = append(openTags, tagName)
					}
				}
			}
		} else {
			// Text content
			if trimmed {
				continue
			}

			remaining := maxLen - visibleCount
			if remaining <= 0 {
				trimmed = true
				result.WriteString("...")
				break
			}

			textRunes := []rune(tok.data)
			if len(textRunes) <= remaining {
				result.WriteString(tok.data)
				visibleCount += len(textRunes)
			} else {
				result.WriteString(string(textRunes[:remaining]))
				result.WriteString("...")
				visibleCount = maxLen
				trimmed = true
				break
			}
		}

		// Check if we need to close remaining tags
		if trimmed && i == len(tokens)-1 {
			break
		}
	}

	// Close any open tags in reverse order
	for i := len(openTags) - 1; i >= 0; i-- {
		result.WriteString("</" + openTags[i] + ">")
	}

	return result.String()
}

// sanitizeHTML removes all HTML tags except those in the allowed list
func sanitizeHTML(input string, allowedTags []string) string {
	if len(allowedTags) == 0 {
		return stripHTMLTags(input)
	}

	// Create a map for faster lookup
	allowed := make(map[string]bool)
	for _, tag := range allowedTags {
		allowed[strings.ToLower(tag)] = true
	}

	var result strings.Builder
	var currentTag strings.Builder
	inTag := false

	for _, ch := range input {
		if ch == '<' {
			if inTag {
				// Malformed HTML - treat as text
				result.WriteRune('<')
				result.WriteString(currentTag.String())
				currentTag.Reset()
			}
			inTag = true
			currentTag.WriteRune(ch)
		} else if ch == '>' {
			currentTag.WriteRune(ch)
			if inTag {
				tagContent := currentTag.String()
				// Check if this tag is allowed
				tagName := extractTagName(tagContent)

				// Allow the tag if it's in the allowed list
				if tagName != "" && allowed[strings.ToLower(tagName)] {
					result.WriteString(tagContent)
				}
				// Otherwise, skip the tag entirely

				currentTag.Reset()
				inTag = false
			}
		} else {
			if inTag {
				currentTag.WriteRune(ch)
			} else {
				result.WriteRune(ch)
			}
		}
	}

	// Handle any unclosed tag at the end
	if inTag && currentTag.Len() > 0 {
		result.WriteString(currentTag.String())
	}

	return result.String()
}

// extractTagName extracts the tag name from an HTML tag
func extractTagName(tag string) string {
	// Remove < and >
	tag = strings.TrimPrefix(tag, "<")
	tag = strings.TrimSuffix(tag, ">")
	tag = strings.TrimSpace(tag)

	// Get first word (tag name)
	parts := strings.Fields(tag)
	if len(parts) == 0 {
		return ""
	}

	name := parts[0]
	// Remove any attributes
	if idx := strings.IndexAny(name, " \t\n\r"); idx != -1 {
		name = name[:idx]
	}

	return name
}

func getScopedClass(target string, targetType string, scopedElements []scopedElement) string {
	for _, elem := range scopedElements {
		if targetType == "tag" && elem.tag == target {
			return elem.scopedClass
		}
		if targetType == "id" && elem.id == target {
			return elem.scopedClass
		}
		if targetType == "class" {
			for _, class := range elem.classes {
				if class == target {
					return elem.scopedClass
				}
			}
		}
	}
	return ""
}
