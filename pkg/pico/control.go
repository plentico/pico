package pico

import (
	"fmt"
	"log"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

// control represents a node in the template control tree (if/for/component/text)
type control struct {
	isIfStmt          bool
	ifCondition       string
	parsedIfCondition *ParsedIfCondition

	isElseIfStmt          bool
	elseIfCondition       string
	parsedElseIfCondition *ParsedIfCondition

	isElseStmt bool

	isForLoop          bool
	forVar             string
	forCollection      string
	forKeyword         string // "of" or "in"
	forIsDestructuring bool
	forDestructureVars []string // variables from destructuring pattern

	isTextNode  bool
	textContent string

	isComp    bool
	compName  string
	compProps CompProps

	isDynamicComp    bool
	dynamicCompPath  string
	dynamicCompProps CompProps

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
			parsedIfCondition := ParseIfCondition(ifCondition)

			newControl := control{
				isIfStmt:          true,
				ifCondition:       parsedIfCondition.BaseCondition,
				parsedIfCondition: &parsedIfCondition,
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

			// Find the closing } for the {for ...} statement
			// Skip over object literals for smarter brace counting
			// Look for the pattern: "} (of|in) " to know we're past the variable declaration
			endOpenForIndex := -1

			// Simple approach: find "of" or "in" keyword first, then find closing }
			forStart := startOpenForIndex + len("{for ")
			ofPos := strings.Index(markup[forStart:], " of ")
			inPos := strings.Index(markup[forStart:], " in ")

			keywordPos := -1
			if ofPos != -1 && (inPos == -1 || ofPos < inPos) {
				keywordPos = forStart + ofPos
			} else if inPos != -1 {
				keywordPos = forStart + inPos
			}

			if keywordPos != -1 {
				// Find the closing } for the {for} statement by counting braces
				// Start after the keyword + " of " or " in "
				searchStart := keywordPos + 4 // length of " of " or " in "
				braceCount := 1               // We're inside {for
				endOpenForIndex = -1

				for j := searchStart; j < len(markup); j++ {
					if markup[j] == '{' {
						braceCount++
					} else if markup[j] == '}' {
						braceCount--
						if braceCount == 0 {
							endOpenForIndex = j
							break
						}
					}
				}
			}

			if endOpenForIndex == -1 {
				return nil, fmt.Errorf("{for } loop missing closing \"}\" at index %d", startOpenForIndex)
			}

			forContent := markup[startOpenForIndex+len("{for ") : endOpenForIndex]

			// Try to match destructuring patterns first: [a, b] or {a, b}
			reDestructure := regexp.MustCompile(`(?:let|var|const)\s+(\[[^\]]+\]|\{[^}]+\})\s+(of|in)\s+(.*)`)
			matchesDestructure := reDestructure.FindStringSubmatch(forContent)

			var newControl control

			if len(matchesDestructure) >= 4 {
				// Handle destructuring
				pattern := matchesDestructure[1]
				keyword := matchesDestructure[2]
				collection := matchesDestructure[3]

				// Extract variable names from pattern
				var vars []string
				cleanPattern := strings.Trim(pattern, "[]{}")
				parts := strings.Split(cleanPattern, ",")
				for _, part := range parts {
					varName := strings.TrimSpace(part)
					if varName != "" {
						vars = append(vars, varName)
					}
				}

				newControl = control{
					isForLoop:          true,
					forVar:             pattern, // Store the full pattern
					forKeyword:         keyword,
					forCollection:      collection,
					forIsDestructuring: true,
					forDestructureVars: vars,
				}
			} else {
				// Try simple variable pattern
				re := regexp.MustCompile(`(?:let|var|const)\s+(\w+)\s+(of|in)\s+(.*)`)
				matches := re.FindStringSubmatch(forContent)
				if len(matches) < 4 {
					return nil, fmt.Errorf("{for } loop missing iterator / collection at index %d", startOpenForIndex)
				}

				newControl = control{
					isForLoop:     true,
					forVar:        matches[1],
					forKeyword:    matches[2],
					forCollection: matches[3],
				}
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
				controlStack = controlStack[:len(controlStack)-1]
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
				controlStack = controlStack[:len(controlStack)-1]
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
			relativeEndDynamicCompPathIndex := strings.IndexAny(markup[startDynamicCompPathIndex:], "'\"")
			endDynamicCompPathIndex := startDynamicCompPathIndex + relativeEndDynamicCompPathIndex
			dynamicCompPath := markup[startDynamicCompPathIndex:endDynamicCompPathIndex]
			dynamicCompProps := markup[endDynamicCompPathIndex+1 : endDynamicCompIndex]

			newControl := control{
				isDynamicComp:    true,
				dynamicCompPath:  strings.Trim(dynamicCompPath, "'\""),
				dynamicCompProps: getCompArgs(dynamicCompProps),
			}

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
				controlStack = controlStack[:len(controlStack)-1]
			}
			controlStack = controlStack[:len(controlStack)-1]
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
			controlStack = controlStack[:len(controlStack)-1]
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
				} else {
					controlTree = append(controlTree, newControl)
				}
			}
		}
	}

	return controlTree, nil
}

// containsComponent checks if any child control contains a component
func containsComponent(ctrl *control) bool {
	if ctrl.isComp || ctrl.isDynamicComp {
		return true
	}
	for _, child := range ctrl.children {
		if containsComponent(&child) {
			return true
		}
	}
	return false
}

func evalControlTree(controlTree []control, scopeStack []scopeStackItem, props map[string]any, pScopeExp string, fence string, components []Component, usePattr bool, templateDir string, parentConditions ...string) (string, []scopeStackItem) {
	var markupBuilder strings.Builder

	pattrEnabled := usePattr

	for _, ctrl := range controlTree {
		if ctrl.isTextNode {
			markupBuilder.WriteString(ctrl.textContent)
		} else if ctrl.isIfStmt {
			if pattrEnabled {
				collectIfConditions := []string{}

				usePreScope := false
				for _, child := range ctrl.children {
					if containsComponent(&child) {
						usePreScope = true
						break
					}
				}

				buildFullCondition := func(currentCondition string) string {
					if len(parentConditions) > 0 {
						parentCondition := strings.Join(parentConditions, " && ")
						return "(" + parentCondition + ") && (" + currentCondition + ")"
					}
					return currentCondition
				}

				// Check if parent if statement has modifiers
				parentHasModifiers := ctrl.parsedIfCondition != nil && ctrl.parsedIfCondition.HasModifiers()

				ifFullCondition := buildFullCondition(ctrl.ifCondition)
				// Suppress expected JS errors in dead branches during SSR.
				// When pattrEnabled is true, we recurse into all children to build the
				// full template structure for hydration, but errors in branches whose
				// condition is false are expected noise (e.g., .entries() on a non-array
				// inside a guarded {if} block).
				wasSuppressed := suppressJSErrors
				if !isBoolAndTrue(evalJS(ifFullCondition, fence)) {
					suppressJSErrors = true
				}
				ifMarkup, newScopeStack := evalControlTree(ctrl.children, scopeStack, props, pScopeExp, fence, components, pattrEnabled, templateDir, ifFullCondition)
				suppressJSErrors = wasSuppressed

				// Debug: Log if markup is empty when modifiers are present
				if parentHasModifiers && strings.TrimSpace(ifMarkup) == "" {
					log.Printf("Warning: if statement with modifiers produced empty content (condition: %s, children: %d)", ctrl.ifCondition, len(ctrl.children))
				}

				// Use addConditionalAttributes if modifiers are present, otherwise use addPShowAttribute
				var ifMarkupWithAttrs string
				var err error
				if parentHasModifiers {
					ifMarkupWithAttrs, err = addConditionalAttributes(ifMarkup, ifFullCondition, ctrl.parsedIfCondition, fence, usePreScope, pattrEnabled)
				} else {
					ifMarkupWithAttrs, err = addPShowAttribute(ifMarkup, ifFullCondition, fence, usePreScope, pattrEnabled)
				}
				if err == nil {
					markupBuilder.WriteString(ifMarkupWithAttrs)
					scopeStack = newScopeStack
				} else {
					log.Printf("Warning: Failed to process if statement (condition: %s): %v", ifFullCondition, err)
				}
				collectIfConditions = append(collectIfConditions, ctrl.ifCondition)

				for _, child := range ctrl.children {
					if child.isElseIfStmt {
						negatedConditions := []string{}
						for _, cond := range collectIfConditions {
							negatedConditions = append(negatedConditions, "!("+cond+")")
						}
						currentCondition := strings.Join(negatedConditions, " && ") + " && " + child.elseIfCondition
						fullCondition := buildFullCondition(currentCondition)

						// Suppress expected JS errors in dead else-if branches during SSR.
						wasSuppressed = suppressJSErrors
						if !isBoolAndTrue(evalJS(fullCondition, fence)) {
							suppressJSErrors = true
						}
						elseIfMarkup, newScopeStack := evalControlTree(child.children, scopeStack, props, pScopeExp, fence, components, pattrEnabled, templateDir, fullCondition)
						suppressJSErrors = wasSuppressed
						// Use same modifier handling as parent if
						var elseIfMarkupWithAttrs string
						var err error
						if parentHasModifiers {
							elseIfMarkupWithAttrs, err = addConditionalAttributes(elseIfMarkup, fullCondition, ctrl.parsedIfCondition, fence, usePreScope, pattrEnabled)
						} else {
							elseIfMarkupWithAttrs, err = addPShowAttribute(elseIfMarkup, fullCondition, fence, usePreScope, pattrEnabled)
						}
						if err == nil {
							markupBuilder.WriteString(elseIfMarkupWithAttrs)
							scopeStack = newScopeStack
						} else {
							log.Printf("Warning: Failed to process else-if statement (condition: %s): %v", fullCondition, err)
						}
						collectIfConditions = append(collectIfConditions, child.elseIfCondition)
					}
				}

				for _, child := range ctrl.children {
					if child.isElseStmt {
						negatedConditions := []string{}
						for _, cond := range collectIfConditions {
							negatedConditions = append(negatedConditions, "!("+cond+")")
						}
						currentCondition := strings.Join(negatedConditions, " && ")
						fullCondition := buildFullCondition(currentCondition)

						elseMarkup, newScopeStack := evalControlTree(child.children, scopeStack, props, pScopeExp, fence, components, pattrEnabled, templateDir, fullCondition)
						// Use same modifier handling as parent if
						var elseMarkupWithAttrs string
						var err error
						if parentHasModifiers {
							elseMarkupWithAttrs, err = addConditionalAttributes(elseMarkup, fullCondition, ctrl.parsedIfCondition, fence, usePreScope, pattrEnabled)
						} else {
							elseMarkupWithAttrs, err = addPShowAttribute(elseMarkup, fullCondition, fence, usePreScope, pattrEnabled)
						}
						if err == nil {
							markupBuilder.WriteString(elseMarkupWithAttrs)
							scopeStack = newScopeStack
						} else {
							log.Printf("Warning: Failed to process else statement (condition: %s): %v", fullCondition, err)
						}
					}
				}
			} else {
				if isBoolAndTrue(evalJS(ctrl.ifCondition, fence)) {
					markup, newScopeStack := evalControlTree(ctrl.children, scopeStack, props, pScopeExp, fence, components, pattrEnabled, templateDir)
					markupBuilder.WriteString(markup)
					scopeStack = newScopeStack
				} else {
					evaluated := false
					for _, child := range ctrl.children {
						if child.isElseIfStmt && isBoolAndTrue(evalJS(child.elseIfCondition, fence)) {
							markup, newScopeStack := evalControlTree(child.children, scopeStack, props, pScopeExp, fence, components, pattrEnabled, templateDir)
							markupBuilder.WriteString(markup)
							scopeStack = newScopeStack
							evaluated = true
							break
						}
					}
					if !evaluated {
						for _, child := range ctrl.children {
							if child.isElseStmt {
								markup, newScopeStack := evalControlTree(child.children, scopeStack, props, pScopeExp, fence, components, pattrEnabled, templateDir)
								markupBuilder.WriteString(markup)
								scopeStack = newScopeStack
								break
							}
						}
					}
				}
			}
		} else if ctrl.isForLoop {
			// If the collection starts with {, wrap it in parentheses for proper object literal parsing
			collection := ctrl.forCollection
			if strings.HasPrefix(strings.TrimSpace(collection), "{") && !strings.HasPrefix(strings.TrimSpace(collection), "{[") {
				collection = "(" + collection + ")"
			}

			// Wrap .entries() and other iterator methods with Array.from() for SSR
			if strings.Contains(collection, ".entries()") || strings.Contains(collection, ".keys()") || strings.Contains(collection, ".values()") {
				collection = "Array.from(" + collection + ")"
			}

			iterableVal := evalJS(collection, fence)

			// Check if it's an array (for "of") or object (for "in")
			items, isArray := iterableVal.([]any)
			objMap, isObject := iterableVal.(map[string]any)

			// Convert object keys to array if using "in"
			var iterationItems []any
			if ctrl.forKeyword == "in" && isObject {
				// Get object keys
				for key := range objMap {
					iterationItems = append(iterationItems, key)
				}
			} else if isArray {
				iterationItems = items
			}

			if isArray || isObject {
				currentScopeId := loopScopeCounter
				loopScopeCounter++

				if pattrEnabled {
					pForExpr := ctrl.forVar + " " + ctrl.forKeyword + " " + ctrl.forCollection
					var templateLoopFence string
					if len(iterationItems) > 0 {
						templateLoopFence = fence + "\nlet " + ctrl.forVar + " = " + anyToString(iterationItems[0]) + ";"
					} else {
						templateLoopFence = fence + "\nlet " + ctrl.forVar + " = null;"
					}
					templateNewProps := make(map[string]any)
					for k, v := range props {
						templateNewProps[k] = v
					}
					if len(iterationItems) > 0 {
						templateNewProps[ctrl.forVar] = iterationItems[0]
					}
					templateMarkup, _ := evalControlTree(ctrl.children, scopeStack, templateNewProps, pScopeExp, templateLoopFence, components, pattrEnabled, templateDir)
					templateMarkup = processLoopTemplate(templateMarkup, templateLoopFence, pattrEnabled)
					markupBuilder.WriteString("<template p-for=\"" + html.EscapeString(pForExpr) + "\">")
					markupBuilder.WriteString(templateMarkup)
					markupBuilder.WriteString("</template>")
				}

				for idx, item := range iterationItems {
					newProps := make(map[string]any)
					for k, v := range props {
						newProps[k] = v
					}

					var loopFence string

					if ctrl.forIsDestructuring {
						// Handle destructuring
						if strings.HasPrefix(ctrl.forVar, "[") {
							// Array destructuring: [a, b]
							itemArray, ok := item.([]any)
							if ok {
								for i, varName := range ctrl.forDestructureVars {
									if i < len(itemArray) {
										newProps[varName] = itemArray[i]
									}
								}
							}
							// Build fence with destructured variables - just the destructuring statement
							loopFence = fence + "\nlet " + ctrl.forVar + " = " + anyToString(item) + ";"
						} else if strings.HasPrefix(ctrl.forVar, "{") {
							// Object destructuring: {a, b}
							// Used when iterating over arrays of objects: for (let {name, age} of people)
							itemMap, ok := item.(map[string]any)
							if ok {
								for _, varName := range ctrl.forDestructureVars {
									if val, exists := itemMap[varName]; exists {
										newProps[varName] = val
									}
								}
							}
							// Build fence with destructured variables
							loopFence = fence + "\nlet " + ctrl.forVar + " = " + anyToString(item) + ";"
						}
					} else {
						// Normal (non-destructuring) case
						newProps[ctrl.forVar] = item
						loopFence = fence + "\nlet " + ctrl.forVar + " = " + anyToString(item) + ";"
					}

					markup, newScopeStack := evalControlTree(ctrl.children, scopeStack, newProps, pScopeExp, loopFence, components, pattrEnabled, templateDir)
					if pattrEnabled {
						markup = processLoopIteration(markup, loopFence, ctrl.forVar, item, currentScopeId, idx, pattrEnabled)
					} else {
						markup = evalAllBrackets(markup, loopFence)
					}
					markupBuilder.WriteString(markup)
					scopeStack = newScopeStack
				}
			}
		} else if ctrl.isComp {
			newProps := make(map[string]any)
			// Evaluate both regular and sync props
			for prop_name, prop_value := range ctrl.compProps.Regular {
				newProps[prop_name] = evalJS(fmt.Sprintf(`%s`, prop_value), fence)
			}
			for prop_name, prop_value := range ctrl.compProps.Sync {
				newProps[prop_name] = evalJS(fmt.Sprintf(`%s`, prop_value), fence)
			}
			var compPath string
			for _, comp := range components {
				if comp.Name == ctrl.compName {
					compPath = comp.Path
				}
			}
			markup, script, style, newScopeStack, newPScopeExp, newFence := Render(compPath, newProps, scopeStack, !pattrEnabled)
			markup, scopedElements := scopeHTML(markup, ctrl.compProps, newPScopeExp, newFence, pattrEnabled)
			newScopeStack = append(newScopeStack, scopeStackItem{
				scopedElements: scopedElements,
				style:          style,
				script:         script,
			})
			scopeStack = newScopeStack
			markupBuilder.WriteString(markup)
		} else if ctrl.isDynamicComp {
			newProps := make(map[string]any)
			// Evaluate both regular and sync props
			for prop_name, prop_value := range ctrl.dynamicCompProps.Regular {
				newProps[prop_name] = evalJS(fmt.Sprintf(`%s`, prop_value), fence)
			}
			for prop_name, prop_value := range ctrl.dynamicCompProps.Sync {
				newProps[prop_name] = evalJS(fmt.Sprintf(`%s`, prop_value), fence)
			}
			evaluatedCompPath := evalAllBrackets(ctrl.dynamicCompPath, fence)
			// Resolve dynamic component path relative to current template's directory
			if !filepath.IsAbs(evaluatedCompPath) {
				evaluatedCompPath = filepath.Join(templateDir, evaluatedCompPath)
			}
			markup, script, style, newScopeStack, newPScopeExp, newFence := Render(evaluatedCompPath, newProps, scopeStack, !pattrEnabled)
			markup, scopedElements := scopeHTML(markup, ctrl.dynamicCompProps, newPScopeExp, newFence, pattrEnabled)
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
