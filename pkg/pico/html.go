package pico

import (
	"encoding/json"
	"io"
	"log"
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
			pID, _ := generateRandom()
			node.Attr = append(node.Attr, html.Attribute{Key: "p-id", Val: pID})
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
					attr := html.Attribute{
						Key: "p-text",
						Val: "`" + strings.ReplaceAll(strings.ReplaceAll(node.Data, "{", "${"), "\"", "'") + "`",
					}
					node.Parent.Attr = append(node.Parent.Attr, attr)
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
				scopedClass = "plenti-" + randomStr
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
						if strings.HasPrefix(class, "plenti-") {
							alreadyScoped = true
							scopedClass = class
						}
					}
					if !alreadyScoped {
						node.Attr[i].Val += " " + scopedClass
					}
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
			pID, _ := generateRandom()
			node.Attr = append(node.Attr, html.Attribute{Key: "p-id", Val: pID})
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
