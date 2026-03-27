// Package pico provides a template rendering engine with reactive UI support via Pattr.
// It supports server-side rendering (SSR) with automatic hydration attributes.
package pico

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/net/html"
)

// Component represents an imported component with its name and file path.
type Component struct {
	Name string
	Path string
}

// scopeStackItem holds scoped elements, style, and script for a component scope.
type scopeStackItem struct {
	scopedElements []scopedElement
	style          string
	script         string
}

// scopedElement represents an HTML element with scoping information for CSS isolation.
type scopedElement struct {
	tag         string
	id          string
	classes     []string
	scopedClass string
}

// Global counter for loop scope IDs (supports nested loops)
var loopScopeCounter int

// Render renders a template with the given props and returns markup, script, style,
// scope stack, p-scope expression, and fence. This is typically used for nested components.
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
		// Log error but continue
		_ = err
	}
	// Default noPattr to false (use Pattr by default)
	usePattr := true
	if len(noPattr) > 0 && noPattr[0] {
		usePattr = false
	}
	templateDir := filepath.Dir(path)
	markup, scopeStack = evalControlTree(controlTree, scopeStack, props, pScopeExp, fence, components, usePattr, templateDir)

	return markup, script, style, scopeStack, pScopeExp, fence
}

// RenderRoot is the starting point for rendering top-level <html> documents.
// It returns the final markup, script, and style strings ready for output.
func RenderRoot(path string, props map[string]any, noPattr ...bool) (string, string, string) {
	usePattr := true
	if len(noPattr) > 0 && noPattr[0] {
		usePattr = false
	}
	markup, script, style, scopeStack, pScopeExp, fence := Render(path, props, []scopeStackItem{}, !usePattr)
	// Create scoped classes and add to html
	markup, scopedElements := scopeHTML(markup, props, pScopeExp, fence, usePattr)
	scopeStack = append(scopeStack, scopeStackItem{
		scopedElements: scopedElements,
		style:          style,
		script:         script,
	})
	// Add scoped classes to css
	style, script = evalScopeStack(scopeStack)

	return markup, script, style
}

// RenderRootFromJSON renders a template using props loaded from a JSON file.
func RenderRootFromJSON(templatePath string, propsPath string, noPattr ...bool) (string, string, string, error) {
	data, err := os.ReadFile(propsPath)
	if err != nil {
		return "", "", "", err
	}

	var props map[string]any
	if err := json.Unmarshal(data, &props); err != nil {
		return "", "", "", err
	}

	markup, script, style := RenderRoot(templatePath, props, noPattr...)
	return markup, script, style, nil
}

// RenderRootFromJSONString renders a template using props from a JSON string.
func RenderRootFromJSONString(templatePath string, propsJSON string, noPattr ...bool) (string, string, string, error) {
	var props map[string]any
	if err := json.Unmarshal([]byte(propsJSON), &props); err != nil {
		return "", "", "", err
	}

	markup, script, style := RenderRoot(templatePath, props, noPattr...)
	return markup, script, style, nil
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

// addPShowAttribute adds p-show attribute to all top-level HTML elements
// and adds style="display: none;" if the condition evaluates to false during SSR
func addPShowAttribute(htmlStr string, showCondition string, fence string, usePreScope bool, usePattr bool) (string, error) {
	// Evaluate the condition during SSR
	conditionResult := evalJS(showCondition, fence)
	shouldShow := isBoolAndTrue(conditionResult)

	// Parse the HTML string
	nodes, err := parseNoFix(htmlStr)
	if err != nil {
		return "", err
	}

	var buf strings.Builder
	for _, node := range nodes {
		if node.Type == html.ElementNode {
			if usePattr {
				// Determine the attribute key based on whether this is a pre-scope or post-scope conditional
				attrKey := "p-show"
				if usePreScope {
					attrKey = "p-show:pre-scope"
				}

				// Check if p-show attribute already exists
				hasPShow := false
				for _, attr := range node.Attr {
					if attr.Key == "p-show" || attr.Key == "p-show:pre-scope" {
						hasPShow = true
						break
					}
				}
				// Add p-show if not present
				if !hasPShow {
					node.Attr = append(node.Attr, html.Attribute{Key: attrKey, Val: showCondition})
				}
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
			return "", err
		}
	}

	return buf.String(), nil
}

// addConditionalAttributes adds conditional attributes (style, class, attr) based on if-statement modifiers
// This allows for accordion expands and other conditional styling beyond just p-show
func addConditionalAttributes(htmlStr string, condition string, parsedIfCondition *ParsedIfCondition, fence string, usePreScope bool, usePattr bool) (string, error) {
	// Evaluate the condition during SSR
	conditionResult := evalJS(condition, fence)
	shouldShow := isBoolAndTrue(conditionResult)

	// Parse the HTML string
	nodes, err := parseNoFix(htmlStr)
	if err != nil {
		log.Printf("Warning: addConditionalAttributes failed to parse HTML (condition: %s): %v", condition, err)
		log.Printf("HTML content length: %d, first 100 chars: %s", len(htmlStr), htmlStr[:min(100, len(htmlStr))])
		return "", err
	}

	if len(nodes) == 0 {
		log.Printf("Warning: addConditionalAttributes got 0 nodes from HTML (condition: %s, HTML length: %d)", condition, len(htmlStr))
	}

	// Get modifiers
	styleMod := parsedIfCondition.GetStyleModifier()
	classMod := parsedIfCondition.GetClassModifier()
	attrMod := parsedIfCondition.GetAttrModifier()

	var buf strings.Builder
	for _, node := range nodes {
		if node.Type == html.ElementNode {
			if usePattr {
				// Add p-style if style modifier is present (using ternary syntax)
				if styleMod != nil && len(styleMod.Args) >= 2 {
					trueStyle := styleMod.Args[0]
					falseStyle := styleMod.Args[1]
					// Use ternary syntax: condition ? 'true-value' : 'false-value'
					pStyleVal := condition + " ? '" + trueStyle + "' : '" + falseStyle + "'"
					node.Attr = append(node.Attr, html.Attribute{Key: "p-style", Val: pStyleVal})
				}

				// Add p-class if class modifier is present (using ternary syntax)
				if classMod != nil && len(classMod.Args) >= 2 {
					trueClass := classMod.Args[0]
					falseClass := classMod.Args[1]
					// Use ternary syntax: condition ? 'true-value' : 'false-value'
					pClassVal := condition + " ? '" + trueClass + "' : '" + falseClass + "'"
					node.Attr = append(node.Attr, html.Attribute{Key: "p-class", Val: pClassVal})
				}

				// Add p-attr if attr modifier is present (using ternary syntax)
				if attrMod != nil && len(attrMod.Args) >= 3 {
					attrName := attrMod.Args[0]
					trueVal := attrMod.Args[1]
					falseVal := attrMod.Args[2]
					// Use ternary syntax for p-attr: condition ? 'attrName:trueVal' : 'attrName:falseVal'
					pAttrVal := condition + " ? '" + attrName + ":" + trueVal + "' : '" + attrName + ":" + falseVal + "'"
					node.Attr = append(node.Attr, html.Attribute{Key: "p-attr", Val: pAttrVal})
				}
			}

			// Apply SSR styles if condition is false
			if !shouldShow {
				// Check for style modifier first
				if styleMod != nil && len(styleMod.Args) >= 2 {
					falseStyle := styleMod.Args[1]
					applyStyleToNode(node, falseStyle)
				}
			} else {
				// Apply true styles if condition is true
				if styleMod != nil && len(styleMod.Args) >= 1 {
					trueStyle := styleMod.Args[0]
					applyStyleToNode(node, trueStyle)
				}
			}

			// Apply class modifier for SSR
			if classMod != nil && len(classMod.Args) >= 2 {
				if shouldShow {
					applyClassToNode(node, classMod.Args[0])
				} else {
					applyClassToNode(node, classMod.Args[1])
				}
			}

			// Apply attr modifier for SSR
			if attrMod != nil && len(attrMod.Args) >= 3 {
				attrName := attrMod.Args[0]
				if shouldShow {
					applyAttrToNode(node, attrName, attrMod.Args[1])
				} else {
					applyAttrToNode(node, attrName, attrMod.Args[2])
				}
			}
		}
		if err := html.Render(&buf, node); err != nil {
			return "", err
		}
	}

	return buf.String(), nil
}

// applyStyleToNode applies a style string to a node, merging with existing styles
func applyStyleToNode(node *html.Node, style string) {
	hasStyle := false
	for i, attr := range node.Attr {
		if attr.Key == "style" {
			hasStyle = true
			// Append to existing styles
			if !strings.HasSuffix(attr.Val, ";") && attr.Val != "" {
				node.Attr[i].Val = attr.Val + "; " + style + ";"
			} else {
				node.Attr[i].Val = attr.Val + style + ";"
			}
			break
		}
	}
	if !hasStyle {
		node.Attr = append(node.Attr, html.Attribute{Key: "style", Val: style + ";"})
	}
}

// applyClassToNode applies a class to a node, appending to existing classes
func applyClassToNode(node *html.Node, class string) {
	hasClass := false
	for i, attr := range node.Attr {
		if attr.Key == "class" {
			hasClass = true
			node.Attr[i].Val = attr.Val + " " + class
			break
		}
	}
	if !hasClass {
		node.Attr = append(node.Attr, html.Attribute{Key: "class", Val: class})
	}
}

// applyAttrToNode applies an attribute to a node, or updates existing
func applyAttrToNode(node *html.Node, key, val string) {
	hasAttr := false
	for i, attr := range node.Attr {
		if attr.Key == key {
			hasAttr = true
			node.Attr[i].Val = val
			break
		}
	}
	if !hasAttr {
		node.Attr = append(node.Attr, html.Attribute{Key: key, Val: val})
	}
}
