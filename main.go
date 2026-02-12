package main

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/dop251/goja"
	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/css"
	"github.com/tdewolff/parse/v2/js"
)

type Component struct {
	Name string
	Path string
}

// Render renders the template with the given data
func Render(path string, props map[string]any, scopeStack []scopeStackItem, noPattr ...bool) (string, string, string, []scopeStackItem, string, string) {
	// Split template into parts
	markup, fence, script, style := templateParts(path)
	// Get list of imported components and remove imports from fence
	fence, components := getComponents(path, fence)
	// Set the prop to the value that's passed in
	fence, pScopeExp := setProps(fence, props)
	// Build AST with {if} and {for} controls + text nodes
	controlTree, err := buildControlTree(markup)
	if err != nil {
		fmt.Println(err)
	}
	// Default noPattr to false (use Pattr by default)
	usePattr := true
	if len(noPattr) > 0 && noPattr[0] {
		usePattr = false
	}
	markup, scopeStack = evalControlTree(controlTree, scopeStack, props, pScopeExp, fence, components, usePattr)

	return markup, script, style, scopeStack, pScopeExp, fence
}

// Starting point for top-level <html> document
func RenderRoot(path string, props map[string]any, noPattr ...bool) (string, string, string) {
	usePattr := true
	if len(noPattr) > 0 && noPattr[0] {
		usePattr = false
	}
	markup, script, style, scopeStack, pScopeExp, fence := Render(path, props, []scopeStackItem{}, !usePattr)
	// Create scoped classes and add to html
	markup, scopedElements := scopeHTML(markup, props, pScopeExp, fence)
	scopeStack = append(scopeStack, scopeStackItem{
		scopedElements: scopedElements,
		style:          style,
		script:         script,
	})
	// Add scoped classes to css
	style, script = evalScopeStack(scopeStack)

	return markup, script, style
}

func evalScopeStack(scopeStack []scopeStackItem) (string, string) {
	var styleBuilder strings.Builder
	var scriptBuilder strings.Builder

	for _, stackItem := range scopeStack {
		if stackItem.script != "" {
			// Add scoped classes to js
			scopedScript := scopeJS(stackItem.script, stackItem.scopedElements)
			scriptBuilder.WriteString(scopedScript)
		}
		// Process style with CSS parser
		if stackItem.style != "" {
			// Add scoped classes to CSS
			scopedStyle := scopeCSS(stackItem.style, stackItem.scopedElements)
			styleBuilder.WriteString(scopedStyle)
		}
	}

	return styleBuilder.String(), scriptBuilder.String()
}

func scopeCSS(style string, scopedElements []scopedElement) string {
	var out strings.Builder

	// Create new CSS Parser
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
					// CSS ID
					scopedClass := getScopedClass(string(val.Data), "id", scopedElements)
					if scopedClass != "" {
						out.WriteString(string(val.Data) + "." + scopedClass)
					} else {
						out.Write(val.Data)
					}
				} else if val.TokenType == css.IdentToken {
					if i > 0 && p.Values()[i-1].TokenType == css.DelimToken {
						// CSS Class
						scopedClass := getScopedClass(string(val.Data), "class", scopedElements)
						if scopedClass != "" {
							out.WriteString(string(val.Data) + "." + scopedClass)
						} else {
							out.Write(val.Data)
						}
					} else {
						scopedClass := getScopedClass(string(val.Data), "tag", scopedElements)
						// TODO: This not only captures tags / elements, but styles (e.g. red, bold, 2rem) too
						// The styles shouldn't return a scopedClass, but we should filter these intentionally
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

func formatJS(script string) string {
	ast, err := js.Parse(parse.NewInputString(script), js.Options{})
	if err != nil {
		panic(err)
	}
	return ast.JSString()
}

type scopedElement struct {
	tag         string
	id          string
	classes     []string
	scopedClass string
}

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
				// orphan end tag (ignore)
				continue
			}
			if stack[len(stack)-1].Data == token.Data {
				stack = stack[:len(stack)-1]
			}
			// else: mismatched (no auto-correction)

		case html.TextToken:
			token := z.Token()
			node := &html.Node{
				Type: html.TextNode,
				Data: string(token.Data),
			}
			if len(stack) > 0 {
				parent := stack[len(stack)-1]
				// append as child
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
			// DOCTYPE should be at the root level only
			nodes = append(nodes, node)
		}
	}
}

// Simple check for elements that lack closing tags and have no children nodes
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

func scopeHTML(markup string, props map[string]any, pScopeExp string, fence string) (string, []scopedElement) {
	scopedElements := []scopedElement{}
	var markupBuilder strings.Builder

	nodes, err := parseNoFix(markup)
	if err != nil {
		fmt.Println("HTML Parsing Error:", err)
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

						// Insert at the very beginning of <head>
						if node.FirstChild != nil {
							node.InsertBefore(rootDataScript, node.FirstChild)
						} else {
							node.AppendChild(rootDataScript)
						}
					}
				}
			}
		}

		if node.Type == html.ElementNode && (len(props) > 0 || pScopeExp != "") {
			if node.Data != "html" {
				// Add args passed into the comp (which may be expressions)
				// Top-level HTML props should only be in p-root-data, not p-scope
				pScopeExp = flattenCompArgs(props) + pScopeExp
			}
			node.Attr = append(node.Attr, html.Attribute{Key: "p-scope", Val: pScopeExp})
			pID, _ := generateRandom()
			node.Attr = append(node.Attr, html.Attribute{Key: "p-id", Val: pID})
		}

		node, scopedElements = traverse(node, scopedElements, fence)

		if err := html.Render(&markupBuilder, node); err != nil {
			log.Fatal(err)
		}

	}

	return markupBuilder.String(), scopedElements
}

func traverse(node *html.Node, scopedElements []scopedElement, fence string) (*html.Node, []scopedElement) {
	var traverse func(*html.Node)
	traverse = func(node *html.Node) {
		if node.Type == html.TextNode {
			if strings.Contains(node.Data, "{") && strings.Contains(node.Data, "}") {
				if p := node.Parent; p != nil && p.Data == "script" {
					for _, attr := range p.Attr {
						if attr.Key == "type" && attr.Val == "application/json" {
							// Don't run on JSON
							return
						}
					}
				}
				attr := html.Attribute{
					Key: "p-text",
					Val: "`" + strings.ReplaceAll(strings.ReplaceAll(node.Data, "{", "${"), "\"", "'") + "`",
				}
				node.Parent.Attr = append(node.Parent.Attr, attr)
			}
			node.Data = evalAllBrackets(node.Data, fence)
		}
		if node.Type == html.ElementNode && node.DataAtom.String() != "" {
			tag := node.Data
			id := ""
			classes := []string{}
			scopedClass := getScopedClass(tag, "tag", scopedElements)

			if scopedClass == "" {
				// There wasn't an existing scoped class for the element, so create one
				randomStr, err := generateRandom()
				if err != nil {
					log.Fatal(err)
				}
				scopedClass = "plenti-" + randomStr
			}

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
					if attr.Key != "p-text" && attr.Key != "p-scope" && !strings.HasPrefix(attr.Key, "p-attr") {
						node.Attr = append(node.Attr, html.Attribute{
							Key: "p-attr:" + attr.Key,
							Val: "`" + strings.ReplaceAll(strings.ReplaceAll(attr.Val, "{", "${"), "\"", "'") + "`",
						})
						node.Attr[i].Val = evalAllBrackets(attr.Val, fence)
					}
				}
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
			traverse(child)
		}
	}
	traverse(node)

	return node, scopedElements
}

type visitor struct {
	scopedElements []scopedElement
}

func (*visitor) Exit(js.INode) {}

func (v *visitor) Enter(node js.INode) js.IVisitor {
	switch node := node.(type) {
	case *js.Var:
		if node.Decl.String() == "LexicalDecl" && !strings.Contains(node.String(), "_plenti_") {
			randomStr, _ := generateRandom()
			node.Data = append(node.Data, []byte("_plenti_"+randomStr)...)
		}
	case *js.BindingElement:
		if expr := node.Default; expr != nil {
			if callExpr, ok := expr.(*js.CallExpr); ok {
				// Check if it's a member expression (like document.querySelector)
				if memberExpr, ok := callExpr.X.(*js.DotExpr); ok {
					objName := string(memberExpr.X.String())
					propName := string(memberExpr.Y.Data)
					if objName == "document" && propName == "querySelector" {
						for i, arg := range callExpr.Args.List {
							argStrOrig := strings.Trim(arg.String(), "\"")
							argStr := argStrOrig
							target_type := "tag"
							if strings.HasPrefix(argStr, ".") {
								argStr = strings.TrimPrefix(argStr, ".")
								target_type = "class"
							}
							if strings.HasPrefix(argStr, "#") {
								argStr = strings.TrimPrefix(argStr, "#")
								target_type = "id"
							}
							scopedClass := getScopedClass(argStr, target_type, v.scopedElements)
							newData := []byte(`"` + argStrOrig + `"`)
							if !strings.Contains(argStrOrig, "plenti-") {
								newData = []byte(`"` + argStrOrig + "." + scopedClass + `"`)
							}
							callExpr.Args.List[i] = js.Arg{Value: &js.LiteralExpr{
								Data: newData,
							}}
						}
					}
				}
			}
		}
		//fmt.Println(node)
	case *js.Element:
		//fmt.Println(node.Value.String())
	default:
		//fmt.Println()
		//fmt.Println(node.String())
	}
	return v
}

func scopeJS(script string, scopedElements []scopedElement) string {
	ast, _ := js.Parse(parse.NewInputString(script), js.Options{})
	v := visitor{scopedElements: scopedElements}
	js.Walk(&v, ast)
	script = ast.JSString()
	return script
}

func getScopedClass(target string, target_type string, scopedElements []scopedElement) string {
	for _, elem := range scopedElements {
		if target_type == "tag" && elem.tag == target {
			return elem.scopedClass
		}
		if target_type == "id" && elem.id == target {
			return elem.scopedClass
		}
		if target_type == "class" {
			for _, class := range elem.classes {
				if class == target {
					return elem.scopedClass
				}
			}
		}
	}
	return ""
}

func generateRandom() (string, error) {
	chars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	var bytes = make([]byte, 6)
	for i := range bytes {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			return "", err
		}
		bytes[i] = chars[num.Int64()]
	}
	return string(bytes), nil
}

func templateParts(path string) (string, string, string, string) {
	c, err := os.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}
	template := string(c)
	reFence := regexp.MustCompile(`(?s)---(.*?)---`)
	reScript := regexp.MustCompile(`(?s)<script>(.*?)</script>`)
	reStyle := regexp.MustCompile(`(?s)<style>(.*?)</style>`)
	fences := reFence.FindAllStringSubmatch(template, -1)
	scripts := reScript.FindAllStringSubmatch(template, -1)
	styles := reStyle.FindAllStringSubmatch(template, -1)
	if len(fences) > 1 {
		log.Fatal("Can only have one set of Fences (--- and ---) per template")
	}
	if len(scripts) > 1 {
		log.Fatal("Can only have one set of Script tags (<script></script>) per template")
	}
	if len(styles) > 1 {
		log.Fatal("Can only have one set of Style tags (<style></style>) per template")
	}
	markup := template
	fence := ""
	script := ""
	style := ""
	if len(fences) > 0 {
		wrapped_fence := fences[0][0]
		fence = fences[0][1]
		markup = strings.Replace(markup, wrapped_fence, "", 1)
	}
	if len(scripts) > 0 {
		wrapped_script := scripts[0][0]
		script = scripts[0][1]
		markup = strings.Replace(markup, wrapped_script, "", 1)
	}
	if len(styles) > 0 {
		wrapped_style := styles[0][0]
		style = styles[0][1]
		markup = strings.Replace(markup, wrapped_style, "", 1)
	}
	return markup, fence, script, style
}

// setProps converts fence to valid JS is used for evaluating and creates p-scope expressions string for modifying data
func setProps(fence string, props map[string]any) (string, string) {
	pScopeExp := fence

	for name, value := range props {
		// For props passed in, find corresponding prop declaration in fence
		reProp := regexp.MustCompile(fmt.Sprintf(`prop (%s)(\s?=\s?(.*?))?;`, name))
		// Remove prop since it will live in p-local-data and is not needed in p-scope
		pScopeExp = reProp.ReplaceAllString(pScopeExp, "")
		// Add the value passed in via Render()
		fence = reProp.ReplaceAllString(fence, "let "+name+" = "+anyToString(value)+";")
	}

	// Any prop not found above, will be caught here (unpassed props)
	rePropDefaults := regexp.MustCompile(`prop\s([a-zA-Z_$]*)(\s?=\s?(.*?))?;`)
	// Attempt to use default value (if it exists)
	fence = rePropDefaults.ReplaceAllString(fence, "let $1$2;")
	// Don't use "let" since p-scope is modification only of existing values
	pScopeExp = rePropDefaults.ReplaceAllString(pScopeExp, "$1$2;")

	// Find any local var assignments in fence_logic (passed props have already been removed above)
	reLocalVars := regexp.MustCompile(`(?:let|const|var)\s([a-zA-Z_$]*)(\s?=\s?(.*?))?;`)
	// Remove let/const/var keywords because p-scope only modifies existing values
	pScopeExp = reLocalVars.ReplaceAllString(pScopeExp, "$1$2;")

	pScopeExp = makeAttrStr(pScopeExp)
	return fence, pScopeExp
}

func makeAttrStr(str string) string {
	reComments := regexp.MustCompile(`//.*`)
	str = reComments.ReplaceAllString(str, "") // Remove comments before putting on single line

	str = strings.TrimSpace(str)              // Remove leading and trailing whitespace
	str = strings.ReplaceAll(str, "\n", "")   // Remove all tabs to put on single line
	str = strings.ReplaceAll(str, "'", "\\'") // escape single quotes
	str = strings.ReplaceAll(str, "\"", "'")  // change double quotes to single

	return str
}

func evalAllBrackets(str string, fence string) string {
	for {
		startPos := strings.IndexRune(str, '{')
		endPos := strings.IndexRune(str, '}')
		if startPos == -1 || endPos == -1 {
			break
		}
		jsCode := str[startPos+1 : endPos]
		evaluated := fmt.Sprintf("%v", evalJS(jsCode, fence)) // Like anyToString but doesn't wrap strings in quotes
		str = str[0:startPos] + evaluated + str[endPos+1:]
	}
	return str
}

func evalJS(jsCode string, fence string) any {
	// TODO: expensive to create VMs each time, pass in instead
	vm := goja.New()
	goja_value, err := vm.RunString(fence + jsCode)
	if err != nil {
		return ""
	}
	return goja_value.Export()
}

// Helper function to check if a character is uppercase
func isUpper(c byte) bool {
	return c >= 'A' && c <= 'Z'
}

type control struct {
	isIfStmt    bool
	ifCondition string

	isElseIfStmt    bool
	elseIfCondition string

	isElseStmt bool

	isForLoop     bool
	forVar        string
	forCollection string

	isTextNode  bool
	textContent string

	isComp    bool
	compName  string
	compProps map[string]any

	isDynamicComp    bool
	dynamicCompPath  string
	dynamicCompProps map[string]any

	children []control
}

func buildControlTree(markup string) ([]control, error) {
	var controlTree []control
	var controlStack []*control
	var openControl *control
	for i := 0; i < len(markup); {
		if strings.HasPrefix(markup[i:], "{if ") {
			startOpenIfIndex := i

			relativeEndOpenIfIndex := strings.Index(markup[startOpenIfIndex:], "}")
			if relativeEndOpenIfIndex == -1 {
				return nil, fmt.Errorf("{if ...} condition missing closing \"}\" at index %d", startOpenIfIndex)
			}
			endOpenIfIndex := startOpenIfIndex + relativeEndOpenIfIndex

			ifCondition := markup[startOpenIfIndex+len("{if ") : endOpenIfIndex]

			newControl := control{
				isIfStmt:    true,
				ifCondition: ifCondition,
			}

			if openControl != nil {
				openControl.children = append(openControl.children, newControl)
				controlStack = append(controlStack, &openControl.children[len(openControl.children)-1])
			} else {
				controlTree = append(controlTree, newControl)
				controlStack = append(controlStack, &controlTree[len(controlTree)-1])
			}
			openControl = controlStack[len(controlStack)-1]

			i = endOpenIfIndex + 1
		} else if strings.HasPrefix(markup[i:], "{for ") {
			startOpenForIndex := i
			relativeEndOpenForIndex := strings.Index(markup[startOpenForIndex:], "}")
			if relativeEndOpenForIndex == -1 {
				return nil, fmt.Errorf("{for } loop missing closing \"}\" at index %d", startOpenForIndex)
			}
			endOpenForIndex := startOpenForIndex + relativeEndOpenForIndex

			re := regexp.MustCompile(`for (?:let|var|const) (\w+) (?:of|in) (.*)`)
			matches := re.FindStringSubmatch(markup[startOpenForIndex:endOpenForIndex])
			if len(matches) < 2 {
				return nil, fmt.Errorf("{for } loop missing iterator / collection \"}\" at index %d", startOpenForIndex)
			}

			newControl := control{
				isForLoop:     true,
				forVar:        matches[1],
				forCollection: matches[2],
			}
			if openControl != nil {
				openControl.children = append(openControl.children, newControl)
				controlStack = append(controlStack, &openControl.children[len(openControl.children)-1])
			} else {
				controlTree = append(controlTree, newControl)
				controlStack = append(controlStack, &controlTree[len(controlTree)-1])
			}
			openControl = controlStack[len(controlStack)-1]

			i = endOpenForIndex + 1
		} else if strings.HasPrefix(markup[i:], "{else if ") {
			if openControl == nil {
				return nil, fmt.Errorf("{else if} at index %d missing opening {if}", i)
			}
			startElseIfIndex := i

			relativeEndElseIfIndex := strings.Index(markup[startElseIfIndex:], "}")
			if relativeEndElseIfIndex == -1 {
				return nil, fmt.Errorf("{else if} condition missing closing \"}\" at index %d", startElseIfIndex)
			}
			endElseIfIndex := startElseIfIndex + relativeEndElseIfIndex

			elseIfCondition := markup[startElseIfIndex+len("{else if ") : endElseIfIndex]

			if openControl.isElseIfStmt {
				controlStack = controlStack[:len(controlStack)-1] // Pop from stack
				openControl = controlStack[len(controlStack)-1]
			}

			openControl.children = append(openControl.children, control{
				isElseIfStmt:    true,
				elseIfCondition: elseIfCondition,
			})
			controlStack = append(controlStack, &openControl.children[len(openControl.children)-1])
			openControl = controlStack[len(controlStack)-1]

			i = endElseIfIndex + 1
		} else if strings.HasPrefix(markup[i:], "{else}") {
			if openControl == nil {
				return nil, fmt.Errorf("{else} at index %d missing opening {if}", i)
			}
			newControl := control{
				isElseStmt: true,
			}

			if openControl.isElseIfStmt {
				controlStack = controlStack[:len(controlStack)-1] // Pop from stack
				openControl = controlStack[len(controlStack)-1]
			}
			openControl.children = append(openControl.children, newControl)
			controlStack = append(controlStack, &openControl.children[len(openControl.children)-1])
			openControl = controlStack[len(controlStack)-1]

			i += len("{else}")
		} else if i+1 < len(markup) && markup[i] == '<' && isUpper(markup[i+1]) {
			startCompIndex := i
			relativeEndCompIndex := strings.Index(markup[startCompIndex:], "/>")
			if relativeEndCompIndex == -1 {
				return nil, fmt.Errorf("Component missing closing \"/>\" at index %d", startCompIndex)
			}
			endCompIndex := startCompIndex + relativeEndCompIndex

			startCompNameIndex := i + 1
			relativeEndCompNameIndex := strings.Index(markup[startCompNameIndex:], " ")
			endCompNameIndex := startCompNameIndex + relativeEndCompNameIndex

			compName := markup[startCompNameIndex:endCompNameIndex]
			compProps := markup[endCompNameIndex+1 : endCompIndex]

			newControl := control{
				isComp:    true,
				compName:  compName,
				compProps: getCompArgs(compProps),
			}

			// TODO: For now Comp won't have children (eventually add slot support)
			if openControl != nil {
				openControl.children = append(openControl.children, newControl)
			} else {
				controlTree = append(controlTree, newControl)
			}

			i = endCompIndex + len("/>")
		} else if strings.HasPrefix(markup[i:], "<=") {
			startDynamicCompIndex := i
			relativeEndDynamicCompIndex := strings.Index(markup[startDynamicCompIndex:], "/>")
			if relativeEndDynamicCompIndex == -1 {
				return nil, fmt.Errorf("<= dynamic comp missing closing \"/>\" at index %d", startDynamicCompIndex)
			}
			endDynamicCompIndex := startDynamicCompIndex + relativeEndDynamicCompIndex

			startDynamicCompPathIndex := startDynamicCompIndex + len("<='")
			// TODO: dynamic paths now need to be wrapped in either single or double quotes
			relativeEndDynamicCompPathIndex := strings.IndexAny(markup[startDynamicCompPathIndex:], "'\"")
			endDynamicCompPathIndex := startDynamicCompPathIndex + relativeEndDynamicCompPathIndex
			dynamicCompPath := markup[startDynamicCompPathIndex:endDynamicCompPathIndex]
			dynamicCompProps := markup[endDynamicCompPathIndex+1 : endDynamicCompIndex]

			newControl := control{
				isDynamicComp:    true,
				dynamicCompPath:  strings.Trim(dynamicCompPath, "'\""),
				dynamicCompProps: getCompArgs(dynamicCompProps),
			}

			// TODO: For now dynamicComp won't have children (eventually add slot support)
			if openControl != nil {
				openControl.children = append(openControl.children, newControl)
			} else {
				controlTree = append(controlTree, newControl)
			}

			i = endDynamicCompIndex + len("/>")
		} else if strings.HasPrefix(markup[i:], "{/if}") {
			if openControl == nil {
				return nil, fmt.Errorf("closing {/if} at index %d without opening {if}", i)
			}
			if openControl.isElseIfStmt || openControl.isElseStmt {
				controlStack = controlStack[:len(controlStack)-1] // Pop from stack
			}
			controlStack = controlStack[:len(controlStack)-1] // Pop from stack
			if len(controlStack) > 0 {
				openControl = controlStack[len(controlStack)-1]
			} else {
				openControl = nil
			}
			i += len("{/if}")
		} else if strings.HasPrefix(markup[i:], "{/for}") {
			if openControl == nil {
				return nil, fmt.Errorf("closing {/for} at index %d without opening {for}", i)
			}
			controlStack = controlStack[:len(controlStack)-1] // Pop from stack
			if len(controlStack) > 0 {
				openControl = controlStack[len(controlStack)-1]
			} else {
				openControl = nil
			}
			i += len("{/for}")
		} else {
			start := i
			for i < len(markup) &&
				!strings.HasPrefix(markup[i:], "<=") &&
				!(i+1 < len(markup) && markup[i] == '<' && isUpper(markup[i+1])) &&
				!strings.HasPrefix(markup[i:], "{if ") &&
				!strings.HasPrefix(markup[i:], "{for ") &&
				!strings.HasPrefix(markup[i:], "{else if ") &&
				!strings.HasPrefix(markup[i:], "{else}") &&
				!strings.HasPrefix(markup[i:], "{/if}") &&
				!strings.HasPrefix(markup[i:], "{/for}") {
				i++
			}
			if start < i {
				newControl := control{
					isTextNode:  true,
					textContent: markup[start:i],
				}
				if openControl != nil {
					openControl.children = append(openControl.children, newControl)
					// Note: Not adding text nodes to controlStack as they don't need closing
				} else {
					controlTree = append(controlTree, newControl)
					// Note: Not adding text nodes to controlStack as they don't need closing
				}
			}
		}
	}

	return controlTree, nil
}

type scopeStackItem struct {
	scopedElements []scopedElement
	style          string
	script         string
}

// addPScopeAttribute adds p-scope="" to all top-level HTML elements
func addPScopeAttribute(htmlStr string, dataStr string) (string, error) {
	// Parse the HTML string
	doc, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return "", fmt.Errorf("failed to parse HTML: %w", err)
	}

	// Find the body node (html.Parse wraps fragments in <html><body>)
	var body *html.Node
	var findBody func(*html.Node)
	findBody = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "body" {
			body = n
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findBody(c)
		}
	}
	findBody(doc)

	if body == nil {
		return "", fmt.Errorf("no body node found")
	}

	// Add p-scope="" to all top-level elements in the body
	for c := body.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode {
			// Check if p-scope attribute already exists
			hasPScope := false
			for _, attr := range c.Attr {
				if attr.Key == "p-scope" {
					hasPScope = true
					break
				}
			}
			// Add p-scope="" if not present
			if !hasPScope {
				c.Attr = append(c.Attr, html.Attribute{Key: "p-scope", Val: dataStr})
			}
		}
	}

	// Render the modified HTML
	var buf bytes.Buffer
	for c := body.FirstChild; c != nil; c = c.NextSibling {
		if err := html.Render(&buf, c); err != nil {
			return "", fmt.Errorf("failed to render HTML: %w", err)
		}
	}

	return buf.String(), nil
}

// addPShowAttribute adds p-show attribute to all top-level HTML elements
// and adds style="display: none;" if the condition evaluates to false during SSR
func addPShowAttribute(htmlStr string, showCondition string, fence string) (string, error) {
	// Evaluate the condition during SSR
	conditionResult := evalJS(showCondition, fence)
	shouldShow := isBoolAndTrue(conditionResult)

	// Parse the HTML string
	nodes, err := parseNoFix(htmlStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse HTML: %w", err)
	}

	var buf bytes.Buffer
	for _, node := range nodes {
		if node.Type == html.ElementNode {
			// Check if p-show attribute already exists
			hasPShow := false
			for _, attr := range node.Attr {
				if attr.Key == "p-show" {
					hasPShow = true
					break
				}
			}
			// Add p-show if not present
			if !hasPShow {
				node.Attr = append(node.Attr, html.Attribute{Key: "p-show", Val: showCondition})
			}

			// If condition is false during SSR, add display: none to prevent flash
			if !shouldShow {
				// Check if style attribute already exists
				hasStyle := false
				for i, attr := range node.Attr {
					if attr.Key == "style" {
						hasStyle = true
						// Append display: none to existing styles
						node.Attr[i].Val = attr.Val + "; display: none;"
						break
					}
				}
				if !hasStyle {
					node.Attr = append(node.Attr, html.Attribute{Key: "style", Val: "display: none;"})
				}
			}
		}
		if err := html.Render(&buf, node); err != nil {
			return "", fmt.Errorf("failed to render HTML: %w", err)
		}
	}

	return buf.String(), nil
}

func evalControlTree(controlTree []control, scopeStack []scopeStackItem, props map[string]any, pScopeExp string, fence string, components []Component, usePattr bool, parentConditions ...string) (string, []scopeStackItem) {
	var markupBuilder strings.Builder

	pattrEnabled := usePattr

	for _, ctrl := range controlTree {
		if ctrl.isTextNode {
			markupBuilder.WriteString(ctrl.textContent)
		} else if ctrl.isIfStmt {
			if pattrEnabled {
				// Pattr mode: output all branches with p-show attributes
				collectIfConditions := []string{}

				// Build the full condition including parent conditions
				buildFullCondition := func(currentCondition string) string {
					if len(parentConditions) > 0 {
						parentCondition := strings.Join(parentConditions, " && ")
						return "(" + parentCondition + ") && (" + currentCondition + ")"
					}
					return currentCondition
				}

				// Render if branch with its condition
				ifFullCondition := buildFullCondition(ctrl.ifCondition)
				ifMarkup, newScopeStack := evalControlTree(ctrl.children, scopeStack, props, pScopeExp, fence, components, pattrEnabled, ifFullCondition)
				ifMarkupWithPShow, err := addPShowAttribute(ifMarkup, ifFullCondition, fence)
				if err == nil {
					markupBuilder.WriteString(ifMarkupWithPShow)
					scopeStack = newScopeStack
				}
				collectIfConditions = append(collectIfConditions, ctrl.ifCondition)

				// Process else-if statements
				for _, child := range ctrl.children {
					if child.isElseIfStmt {
						// Build condition: !(prev1) && !(prev2) && ... && currentCondition
						negatedConditions := []string{}
						for _, cond := range collectIfConditions {
							negatedConditions = append(negatedConditions, "!("+cond+")")
						}
						// Add the current else-if condition
						currentCondition := strings.Join(negatedConditions, " && ") + " && " + child.elseIfCondition
						fullCondition := buildFullCondition(currentCondition)

						elseIfMarkup, newScopeStack := evalControlTree(child.children, scopeStack, props, pScopeExp, fence, components, pattrEnabled, fullCondition)
						elseIfMarkupWithPShow, err := addPShowAttribute(elseIfMarkup, fullCondition, fence)
						if err == nil {
							markupBuilder.WriteString(elseIfMarkupWithPShow)
							scopeStack = newScopeStack
						}
						collectIfConditions = append(collectIfConditions, child.elseIfCondition)
					}
				}

				// Process else statement with all negated conditions
				for _, child := range ctrl.children {
					if child.isElseStmt {
						// Build negated condition: !(cond1) && !(cond2) && ...
						negatedConditions := []string{}
						for _, cond := range collectIfConditions {
							negatedConditions = append(negatedConditions, "!("+cond+")")
						}
						currentCondition := strings.Join(negatedConditions, " && ")
						fullCondition := buildFullCondition(currentCondition)

						elseMarkup, newScopeStack := evalControlTree(child.children, scopeStack, props, pScopeExp, fence, components, pattrEnabled, fullCondition)
						elseMarkupWithPShow, err := addPShowAttribute(elseMarkup, fullCondition, fence)
						if err == nil {
							markupBuilder.WriteString(elseMarkupWithPShow)
							scopeStack = newScopeStack
						}
					}
				}
			} else {
				// No Pattr mode: traditional conditional rendering (evaluate and output only matching branch)
				if isBoolAndTrue(evalJS(ctrl.ifCondition, fence)) {
					markup, newScopeStack := evalControlTree(ctrl.children, scopeStack, props, pScopeExp, fence, components, pattrEnabled)
					markupBuilder.WriteString(markup)
					scopeStack = newScopeStack
				} else {
					evaluated := false
					// Process else-if statements
					for _, child := range ctrl.children {
						if child.isElseIfStmt && isBoolAndTrue(evalJS(child.elseIfCondition, fence)) {
							markup, newScopeStack := evalControlTree(child.children, scopeStack, props, pScopeExp, fence, components, pattrEnabled)
							markupBuilder.WriteString(markup)
							scopeStack = newScopeStack
							evaluated = true
							break
						}
					}
					// Process else statement if no else-if was true
					if !evaluated {
						for _, child := range ctrl.children {
							if child.isElseStmt {
								markup, newScopeStack := evalControlTree(child.children, scopeStack, props, pScopeExp, fence, components, pattrEnabled)
								markupBuilder.WriteString(markup)
								scopeStack = newScopeStack
								break
							}
						}
					}
				}
			}
		} else if ctrl.isForLoop {
			iterableVal := evalJS(ctrl.forCollection, fence)
			items, ok := iterableVal.([]any)
			if ok {
				for _, item := range items {
					newProps := make(map[string]any)
					for k, v := range props {
						newProps[k] = v
					}
					newProps[ctrl.forVar] = item
					markup, newScopeStack := evalControlTree(ctrl.children, scopeStack, newProps, pScopeExp, fence, components, pattrEnabled)
					//dataStr := "{" + ctrl.forVar + ": " + makeAttrStr(anyToString(item)) + "}"
					//markup, _ = addPScopeAttribute(markup, dataStr)
					markupBuilder.WriteString(markup)
					scopeStack = newScopeStack
				}
			}
		} else if ctrl.isComp {
			newProps := make(map[string]any)
			for prop_name, prop_value := range ctrl.compProps {
				// Evaluate the passed in props within the context of the parent comp
				newProps[prop_name] = evalJS(fmt.Sprintf(`%s`, prop_value), fence)
			}
			var compPath string
			for _, comp := range components {
				if comp.Name == ctrl.compName {
					compPath = comp.Path
				}
			}
			markup, script, style, newScopeStack, newPScopeExp, newFence := Render(compPath, newProps, scopeStack, !pattrEnabled)
			// Create scoped classes and add to html
			markup, scopedElements := scopeHTML(markup, ctrl.compProps, newPScopeExp, newFence)
			// Add scoped classes to css
			newScopeStack = append(newScopeStack, scopeStackItem{
				scopedElements: scopedElements,
				style:          style,
				script:         script,
			})
			scopeStack = newScopeStack
			markupBuilder.WriteString(markup)
		} else if ctrl.isDynamicComp {
			newProps := make(map[string]any)
			for prop_name, prop_value := range ctrl.dynamicCompProps {
				// Evaluate the passed in props within the context of the parent comp
				newProps[prop_name] = evalJS(fmt.Sprintf(`%s`, prop_value), fence)
			}
			evaluatedCompPath := evalAllBrackets(ctrl.dynamicCompPath, fence)
			markup, script, style, newScopeStack, newPScopeExp, newFence := Render(evaluatedCompPath, newProps, scopeStack, !pattrEnabled)
			// Create scoped classes and add to html
			markup, scopedElements := scopeHTML(markup, ctrl.dynamicCompProps, newPScopeExp, newFence)
			// Add scoped classes to css
			newScopeStack = append(newScopeStack, scopeStackItem{
				scopedElements: scopedElements,
				style:          style,
				script:         script,
			})
			scopeStack = newScopeStack
			markupBuilder.WriteString(markup)
		}
	}

	return markupBuilder.String(), scopeStack
}

func getComponents(path, fence string) (string, []Component) {
	parentCompDir := filepath.Dir(path)
	components := []Component{}
	reImport := regexp.MustCompile(`import\s+([A-Za-z_][A-Za-z_0-9]*)\s+from\s*"([^"]+)";`)
	for _, line := range strings.Split(fence, "\n") {
		match := reImport.FindStringSubmatch(line)
		if len(match) > 1 {
			compName := match[1]
			compPath := match[2]
			if filepath.IsAbs(compPath) {
				compPath = "." + filepath.Clean("/"+compPath)
			} else {
				compPath = filepath.Join(parentCompDir, filepath.Clean("/"+compPath))
			}
			components = append(components, Component{
				Name: compName,
				Path: compPath,
			})
			fence = reImport.ReplaceAllString(fence, "") // Remove current import so script can run in goja
		}
	}
	return fence, components
}

func getCompArgs(comp_decl string) map[string]any {
	comp_args := strings.SplitAfter(comp_decl, "}")
	comp_props := map[string]any{}
	for _, comp_arg := range comp_args {
		comp_arg = strings.TrimSpace(comp_arg)
		if strings.HasPrefix(comp_arg, "{") && strings.HasSuffix(comp_arg, "}") {
			prop_name := strings.Trim(comp_arg, "{}")
			comp_props[prop_name] = prop_name
		}
		if strings.Contains(comp_arg, "={") && strings.HasSuffix(comp_arg, "}") {
			nameEndPos := strings.IndexRune(comp_arg, '=')
			prop_name := comp_arg[0:nameEndPos]

			valueStartPos := strings.IndexRune(comp_arg, '{')
			valueEndPos := strings.IndexRune(comp_arg, '}')

			comp_props[prop_name] = comp_arg[valueStartPos+1 : valueEndPos]
		}
	}
	return comp_props
}

func formatArray(value any) string {
	val := reflect.ValueOf(value)
	var elements []string
	for i := 0; i < val.Len(); i++ {
		elem := val.Index(i).Interface()
		elements = append(elements, anyToString(elem)) // Recursively format each element
	}
	return "[" + strings.Join(elements, ", ") + "]"
}

func formatObject(value any) string {
	val := reflect.ValueOf(value)
	if val.Kind() != reflect.Map {
		return ""
	}

	// Get the map keys
	keys := val.MapKeys()

	// Convert keys to a slice of interfaces
	keyInterfaces := make([]interface{}, len(keys))
	for i, key := range keys {
		keyInterfaces[i] = key.Interface()
	}

	// Sort the keys (assuming they are strings)
	sort.Slice(keyInterfaces, func(i, j int) bool {
		return fmt.Sprintf("%v", keyInterfaces[i]) < fmt.Sprintf("%v", keyInterfaces[j])
	})

	// Format the map entries
	var pairs []string
	for _, key := range keyInterfaces {
		value := val.MapIndex(reflect.ValueOf(key))
		pairs = append(pairs, fmt.Sprintf("%v: %v", key, anyToString(value.Interface())))
	}

	return "{" + strings.Join(pairs, ", ") + "}"
}

func formatElement(value any) string {
	switch v := value.(type) {
	case string:
		return strconv.Quote(v)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	default:
		return "unknown type"
	}
}

func anyToString(value any) string {
	val := reflect.ValueOf(value)
	switch val.Kind() {
	case reflect.Array, reflect.Slice:
		return formatArray(value)
	case reflect.Map:
		return formatObject(value)
	default:
		return formatElement(value)
	}
}

func flattenCompArgs(m map[string]any) string {
	parts := make([]string, 0, len(m))
	for k, v := range m {
		if k != v {
			parts = append(parts, fmt.Sprintf("%s = %s;", k, v))
		}
	}
	return makeAttrStr(strings.Join(parts, " "))
}

func isBoolAndTrue(value any) bool {
	if b, ok := value.(bool); ok && b {
		return true
	}
	return false
}

func copyFile(sourcePath, destPath string) {
	// Open the source file
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		panic(err)
	}
	defer sourceFile.Close()

	// Create the destination file
	destinationFile, err := os.Create(destPath)
	if err != nil {
		panic(err)
	}
	defer destinationFile.Close()

	// Copy the contents from the source file to the destination file
	_, err = io.Copy(destinationFile, sourceFile)
	if err != nil {
		panic(err)
	}
}

func main() {
	// Render the template with data
	props := map[string]any{"name": "Ja", "age": 2, "animals": []string{"cat", "dog", "pig"}}
	//props := map[string]any{"name": "Ja", "age": 2, "animals": []string{"cat", "dog", "pig"}, "test": "sup"}
	markup, script, style := RenderRoot("views/home.html", props)
	os.MkdirAll("./public", os.ModePerm)
	os.WriteFile("./public/script.js", []byte(script), fs.ModePerm)
	os.WriteFile("./public/style.css", []byte(style), fs.ModePerm)
	os.WriteFile("./public/index.html", []byte(markup), fs.ModePerm)
	/*
		// Test render speed
		start := time.Now()
		for i := 1; i <= 500; i++ {
			markup, script, style, _ := Render("views/home.html", props)
			os.WriteFile(fmt.Sprintf("./public/script%d.js", i), []byte(script), 0644)
			os.WriteFile(fmt.Sprintf("./public/style%d.css", i), []byte(style), 0644)
			os.WriteFile(fmt.Sprintf("./public/index%d.html", i), []byte(markup), 0644)
		}
		elapsed := time.Since(start)
		fmt.Printf("Execution time: %s\n", elapsed)
	*/
	copyFile("./views/cms.js", "./public/cms.js")
	copyFile("./views/cms.css", "./public/cms.css")

	http.Handle("/", http.FileServer(http.Dir("./public")))
	fmt.Println("visit site at: http://localhost:3000")
	http.ListenAndServe(":3000", nil)
}
