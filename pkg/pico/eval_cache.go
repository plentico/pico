package pico

import (
	"regexp"
	"strings"

	"github.com/dop251/goja"
)

// Every expression is evaluated after its component's frontmatter (the
// fence), which holds the page's props as JavaScript literals. Parsing that
// fence again for each expression was most of a page's build time, so each
// fence and expression is parsed once per page and the parsed programs are
// re-run in a fresh VM: each expression still sees a fresh scope.

// useEvalCache turns the cache off (for comparing with the uncached path).
var useEvalCache = true

// evalPrograms holds the parsed fences and expressions for the page being
// rendered (nil for source that didn't parse).
var evalPrograms = map[string]*goja.Program{}

// fenceExtends maps a loop item's fence (a fence plus one line declaring the
// loop variable) to the length of the fence it extends, so only that line is
// parsed for each item.
var fenceExtends = map[string]int{}

// readonlyEvalVMs reuses a fence's initialized VM for expressions that only
// read variables, object properties, or array elements. This prevents a loop
// with several field interpolations from recomputing its frontmatter (and
// reimporting external props) for every field.
var readonlyEvalVMs = map[string]*goja.Runtime{}
var reReadonlyExpression = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*(?:(?:\.[A-Za-z_$][A-Za-z0-9_$]*)|(?:\[[0-9]+\]))*$`)
var reLoopVariable = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*$`)
var reUnsafeExpressionWord = regexp.MustCompile(`\b(?:delete|new)\b`)
var comparisonOperators = strings.NewReplacer("===", "", "!==", "", "==", "", "!=", "", "<=", "", ">=", "")

// isReadonlyExpression accepts expressions that can read data and compute a
// value but cannot call methods, assign variables, or update state. Keeping
// this conservative lets the VM be reused without changing expression behavior.
func isReadonlyExpression(expression string) bool {
	expression = strings.TrimSpace(expression)
	if expression == "" || strings.ContainsAny(expression, "(){};") ||
		strings.Contains(expression, "++") || strings.Contains(expression, "--") ||
		reUnsafeExpressionWord.MatchString(expression) {
		return false
	}
	return !strings.Contains(comparisonOperators.Replace(expression), "=")
}

type loopBinding struct {
	base  string
	name  string
	value any
}

var loopBindings = map[string]loopBinding{}

// loopDecl is one loop variable (a name or a destructuring pattern) and the
// source of its value, as declared after a fence by extendLoopFence.
type loopDecl struct {
	pattern string
	value   string
}

var loopDecls = map[string]loopDecl{}

// loopShadows caches, per loop fence, whether its loop variables clash with
// names the frontmatter (or an outer loop) already declares.
var loopShadows = map[string]bool{}
var loopBaseEvalVMs = map[string]*goja.Runtime{}

// resetEvalCache drops the parsed programs; it's called for each page.
func resetEvalCache() {
	evalPrograms = map[string]*goja.Program{}
	fenceExtends = map[string]int{}
	readonlyEvalVMs = map[string]*goja.Runtime{}
	loopBindings = map[string]loopBinding{}
	loopDecls = map[string]loopDecl{}
	loopShadows = map[string]bool{}
	loopBaseEvalVMs = map[string]*goja.Runtime{}
	externalPropBindings = map[string]map[string]any{}
}

// extendFence returns fence followed by decl (a loop variable's declaration).
func extendFence(fence, decl string) string {
	extended := fence + decl
	fenceExtends[extended] = len(fence)
	externalPropBindings[extended] = externalPropBindings[fence]
	return extended
}

// extendLoopFence records a simple loop binding so readonly expressions can
// reuse the base frontmatter VM instead of rebuilding it for every item.
func extendLoopFence(fence, name string, value any) string {
	valueSrc := anyToString(value)
	extended := extendFence(fence, "\nlet "+name+" = "+valueSrc+";")
	loopDecls[extended] = loopDecl{pattern: name, value: valueSrc}
	if reLoopVariable.MatchString(name) {
		loopBindings[extended] = loopBinding{base: fence, name: name, value: value}
	}
	return extended
}

// shadowLoopVars handles a loop variable with the same name as a variable
// the frontmatter (or an outer loop) declares. Declaring it after the fence
// would be a "has already been declared" error, so instead the expression is
// evaluated after the frontmatter alone, inside one arrow function per loop
// level whose parameter is the loop variable:
//
//	((item) => (expr))(value)
//
// The loop variable then shadows the frontmatter's within the expression,
// as it would in a JavaScript for loop. It reports false when fence has no
// such clash, and the expression is evaluated as before.
func shadowLoopVars(jsCode, fence string) (string, string, bool) {
	if _, ok := loopDecls[fence]; !ok || strings.TrimSpace(jsCode) == "" {
		return "", "", false
	}
	decls := []loopDecl{}
	base := fence
	for {
		decl, ok := loopDecls[base]
		if !ok {
			break
		}
		decls = append(decls, decl)
		base = base[:fenceExtends[base]]
	}
	shadows, ok := loopShadows[fence]
	if !ok {
		_, err := goja.Compile("", fence, false)
		shadows = err != nil && strings.Contains(err.Error(), "has already been declared")
		if shadows {
			if _, baseErr := goja.Compile("", base, false); baseErr != nil {
				shadows = false // the frontmatter itself is broken: report that as before
			}
		}
		loopShadows[fence] = shadows
	}
	if !shadows {
		return "", "", false
	}
	expr := jsCode
	for _, decl := range decls { // innermost loop first
		expr = "((" + decl.pattern + ") => (" + expr + "\n))(" + decl.value + ")"
	}
	if _, err := goja.Compile("", expr, false); err != nil {
		return "", "", false // not a single expression, such as a statement
	}
	return expr, base, true
}

func evalLoopReadonly(jsCode, fence string) (any, bool) {
	binding, ok := loopBindings[fence]
	if !ok || !isReadonlyExpression(jsCode) {
		return nil, false
	}
	vm := loopBaseEvalVMs[binding.base]
	if vm == nil {
		vm = goja.New()
		for name, value := range externalPropBindings[binding.base] {
			if err := vm.Set(name, value); err != nil {
				return nil, false
			}
		}
		progs, ok := fencePrograms(binding.base)
		if !ok {
			return nil, false
		}
		for _, p := range progs {
			if _, err := vm.RunProgram(p); err != nil {
				return nil, false
			}
		}
		loopBaseEvalVMs[binding.base] = vm
	}
	if err := vm.Set(binding.name, binding.value); err != nil {
		return nil, false
	}
	expr, ok := parsedProgram(jsCode)
	if !ok {
		return nil, false
	}
	v, err := vm.RunProgram(expr)
	if err != nil {
		return nil, false
	}
	return v.Export(), true
}

func parsedProgram(src string) (*goja.Program, bool) {
	if p, ok := evalPrograms[src]; ok {
		return p, p != nil
	}
	p, err := goja.Compile("", src, false)
	if err != nil {
		p = nil
	}
	if len(evalPrograms) > 20000 { // (bound the memory one page can use)
		resetEvalCache()
	}
	evalPrograms[src] = p
	return p, p != nil
}

// fencePrograms returns the parsed programs that run fence, in order.
func fencePrograms(fence string) ([]*goja.Program, bool) {
	if n, ok := fenceExtends[fence]; ok {
		base, ok := fencePrograms(fence[:n])
		if !ok {
			return nil, false
		}
		decl, ok := parsedProgram(fence[n:])
		if !ok {
			return nil, false
		}
		return append(base[:len(base):len(base)], decl), true
	}
	if strings.TrimSpace(fence) == "" {
		return nil, true
	}
	p, ok := parsedProgram(fence)
	if !ok {
		return nil, false
	}
	return []*goja.Program{p}, true
}

// evalParsed evaluates jsCode after fence from parsed programs. It reports
// false whenever anything goes wrong, and evalJS then evaluates the source
// as before (with its error messages), so the result is always the same.
func evalParsed(jsCode, fence string) (any, bool) {
	if !useEvalCache {
		return nil, false
	}
	// The programs run one after another as if they were one script, which
	// needs each to end at a statement boundary.
	if t := strings.TrimSpace(fence); t != "" && !strings.HasSuffix(t, ";") {
		return nil, false
	}
	if strings.TrimSpace(jsCode) == "" {
		return nil, false
	}
	if value, ok := evalLoopReadonly(jsCode, fence); ok {
		return value, true
	}
	progs, ok := fencePrograms(fence)
	if !ok {
		return nil, false
	}
	expr, ok := parsedProgram(jsCode)
	if !ok {
		return nil, false
	}
	readonly := len(externalPropBindings[fence]) > 0 && isReadonlyExpression(jsCode)
	if readonly {
		if vm := readonlyEvalVMs[fence]; vm != nil {
			v, err := vm.RunProgram(expr)
			if err != nil {
				return nil, false
			}
			return v.Export(), true
		}
	}
	vm := goja.New()
	for name, value := range externalPropBindings[fence] {
		if err := vm.Set(name, value); err != nil {
			return nil, false
		}
	}
	for _, p := range progs {
		if _, err := vm.RunProgram(p); err != nil {
			return nil, false
		}
	}
	v, err := vm.RunProgram(expr)
	if err != nil {
		return nil, false
	}
	if readonly {
		readonlyEvalVMs[fence] = vm
	}
	return v.Export(), true
}
