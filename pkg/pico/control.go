package pico

import (
	"fmt"
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

				ifFullCondition := buildFullCondition(ctrl.ifCondition)
				ifMarkup, newScopeStack := evalControlTree(ctrl.children, scopeStack, props, pScopeExp, fence, components, pattrEnabled, templateDir, ifFullCondition)

				// Use addConditionalAttributes if modifiers are present, otherwise use addPShowAttribute
				var ifMarkupWithAttrs string
				var err error
				if ctrl.parsedIfCondition != nil && ctrl.parsedIfCondition.HasModifiers() {
					ifMarkupWithAttrs, err = addConditionalAttributes(ifMarkup, ifFullCondition, ctrl.parsedIfCondition, fence, usePreScope, pattrEnabled)
				} else {
					ifMarkupWithAttrs, err = addPShowAttribute(ifMarkup, ifFullCondition, fence, usePreScope, pattrEnabled)
				}
				if err == nil {
					markupBuilder.WriteString(ifMarkupWithAttrs)
					scopeStack = newScopeStack
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

						elseIfMarkup, newScopeStack := evalControlTree(child.children, scopeStack, props, pScopeExp, fence, components, pattrEnabled, templateDir, fullCondition)
						elseIfMarkupWithPShow, err := addPShowAttribute(elseIfMarkup, fullCondition, fence, usePreScope, pattrEnabled)
						if err == nil {
							markupBuilder.WriteString(elseIfMarkupWithPShow)
							scopeStack = newScopeStack
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
						elseMarkupWithPShow, err := addPShowAttribute(elseMarkup, fullCondition, fence, usePreScope, pattrEnabled)
						if err == nil {
							markupBuilder.WriteString(elseMarkupWithPShow)
							scopeStack = newScopeStack
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
			iterableVal := evalJS(ctrl.forCollection, fence)
			items, ok := iterableVal.([]any)
			if ok {
				currentScopeId := loopScopeCounter
				loopScopeCounter++

				if pattrEnabled {
					pForExpr := ctrl.forVar + " of " + ctrl.forCollection
					var templateLoopFence string
					if len(items) > 0 {
						templateLoopFence = fence + "\nlet " + ctrl.forVar + " = " + anyToString(items[0]) + ";"
					} else {
						templateLoopFence = fence + "\nlet " + ctrl.forVar + " = null;"
					}
					templateNewProps := make(map[string]any)
					for k, v := range props {
						templateNewProps[k] = v
					}
					if len(items) > 0 {
						templateNewProps[ctrl.forVar] = items[0]
					}
					templateMarkup, _ := evalControlTree(ctrl.children, scopeStack, templateNewProps, pScopeExp, templateLoopFence, components, pattrEnabled, templateDir)
					templateMarkup = processLoopTemplate(templateMarkup, templateLoopFence, pattrEnabled)
					markupBuilder.WriteString("<template p-for=\"" + html.EscapeString(pForExpr) + "\">")
					markupBuilder.WriteString(templateMarkup)
					markupBuilder.WriteString("</template>")
				}

				for idx, item := range items {
					newProps := make(map[string]any)
					for k, v := range props {
						newProps[k] = v
					}
					newProps[ctrl.forVar] = item
					loopFence := fence + "\nlet " + ctrl.forVar + " = " + anyToString(item) + ";"
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
			for prop_name, prop_value := range ctrl.compProps {
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
			for prop_name, prop_value := range ctrl.dynamicCompProps {
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
