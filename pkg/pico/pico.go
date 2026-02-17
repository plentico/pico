// Package pico provides a template rendering engine with reactive UI support via Pattr.
// It supports server-side rendering (SSR) with automatic hydration attributes.
package pico

import (
	"encoding/json"
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
func addPShowAttribute(htmlStr string, showCondition string, fence string, usePreScope bool) (string, error) {
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
