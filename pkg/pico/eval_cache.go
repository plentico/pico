package pico

import (
	"strconv"
	"strings"

	"github.com/dop251/goja"
)

// Every expression is evaluated after its component's frontmatter (the
// fence), which holds the page's props as JavaScript literals.
//
// Each fence, expression and loop item is parsed once per page. With
// useSharedVM (the default), each distinct fence is also run once, in a VM
// kept for the page, and its expressions are evaluated there as functions
// of their loop variables. Otherwise the parsed programs are re-run in a
// fresh VM for each expression.
//
// Expressions should have no side effects: with a shared VM, what one
// expression changes can be seen by others evaluated after it, and the
// order Pico evaluates them in isn't defined. A frontmatter's functions
// don't see loop variables (pass them as arguments); an expression that
// uses one that way is evaluated as before.

// useEvalCache turns the cache off (for comparing with the uncached path).
var useEvalCache = true

// useSharedVM runs each fence once per page instead of once per expression.
var useSharedVM = true

// evalPrograms holds the parsed fences, expressions and loop items for the
// page being rendered (nil for source that didn't parse).
var evalPrograms = map[string]*goja.Program{}

// loopVar is a loop variable declared after a fence: `let pattern = value;`.
type loopVar struct {
	base           int // the length of the fence it extends
	pattern, value string
}

// fenceExtends maps a loop item's fence (a fence plus one line declaring the
// loop variable) to that declaration, so only it is parsed for each item.
var fenceExtends = map[string]loopVar{}

// fenceVM is the VM a fence has run in, with the expressions compiled in it.
type fenceVM struct {
	vm    *goja.Runtime // nil when the fence failed to run
	funcs map[string]goja.Callable
	used  int
}

// fenceVMs holds the shared VMs for the page, at most maxFenceVMs of them
// (the least recently used is dropped).
var fenceVMs = map[string]*fenceVM{}

const maxFenceVMs = 64

var vmClock int

// resetEvalCache drops the parsed programs and VMs; it's called for each page.
func resetEvalCache() {
	evalPrograms = map[string]*goja.Program{}
	fenceExtends = map[string]loopVar{}
	fenceVMs = map[string]*fenceVM{}
}

// extendFence returns fence followed by a loop variable's declaration,
// `let pattern = value;`.
func extendFence(fence, pattern, value string) string {
	extended := fence + "\nlet " + pattern + " = " + value + ";"
	fenceExtends[extended] = loopVar{len(fence), pattern, value}
	return extended
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
		evalPrograms = map[string]*goja.Program{}
	}
	evalPrograms[src] = p
	return p, p != nil
}

// fencePrograms returns the parsed programs that run fence, in order.
func fencePrograms(fence string) ([]*goja.Program, bool) {
	if lv, ok := fenceExtends[fence]; ok {
		base, ok := fencePrograms(fence[:lv.base])
		if !ok {
			return nil, false
		}
		decl, ok := parsedProgram(fence[lv.base:])
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
	if useSharedVM {
		if v, ok := evalShared(jsCode, fence); ok {
			return v, true
		}
	}
	progs, ok := fencePrograms(fence)
	if !ok {
		return nil, false
	}
	expr, ok := parsedProgram(jsCode)
	if !ok {
		return nil, false
	}
	vm := goja.New()
	for _, p := range progs {
		if _, err := vm.RunProgram(p); err != nil {
			return nil, false
		}
	}
	v, err := vm.RunProgram(expr)
	if err != nil {
		return nil, false
	}
	return v.Export(), true
}

// fenceRoot splits a fence into the fence it starts from and the loop
// variables declared after it, outermost first.
func fenceRoot(fence string) (string, []loopVar) {
	var vars []loopVar
	for {
		lv, ok := fenceExtends[fence]
		if !ok {
			return fence, vars
		}
		vars = append([]loopVar{lv}, vars...)
		fence = fence[:lv.base]
	}
}

// sharedVM returns the page's VM for a fence, running the fence in a new
// one the first time; nil when the fence fails to run.
func sharedVM(fence string) *fenceVM {
	vmClock++
	if fv, ok := fenceVMs[fence]; ok {
		fv.used = vmClock
		if fv.vm == nil {
			return nil
		}
		return fv
	}
	if len(fenceVMs) >= maxFenceVMs {
		oldest, at := "", vmClock
		for f, fv := range fenceVMs {
			if fv.used < at {
				oldest, at = f, fv.used
			}
		}
		delete(fenceVMs, oldest)
	}
	fv := &fenceVM{funcs: map[string]goja.Callable{}, used: vmClock}
	fenceVMs[fence] = fv
	vm := goja.New()
	if strings.TrimSpace(fence) != "" {
		p, ok := parsedProgram(fence)
		if !ok {
			return nil
		}
		if _, err := vm.RunProgram(p); err != nil {
			return nil
		}
	}
	fv.vm = vm
	return fv
}

// loopArg names the arguments carrying loop items' values.
const loopArg = "__picoLoopItem"

// statementStarts are the starts of expressions that are statements
// (`{let a = 1; a}`, a block, a declaration). They're evaluated as before.
var statementStarts = []string{"{", "let ", "const ", "var ", "function", "class "}

// evalShared evaluates jsCode in the page's VM for its fence, as an arrow
// function of its loop variables: `((v0, v1) => { let p0 = v0; let p1 = v1;
// return (jsCode); })`, compiled once per expression and called with each
// item's values (each parsed once, and evaluated for each call, so no two
// items share an object). It reports false when anything fails.
func evalShared(jsCode, fence string) (any, bool) {
	trimmed := strings.TrimSpace(jsCode)
	for _, s := range statementStarts {
		if strings.HasPrefix(trimmed, s) {
			return nil, false
		}
	}
	// It must be a whole expression or statement on its own, as it is when
	// evaluated after its fence ("a) + (b" isn't).
	if _, ok := parsedProgram(jsCode); !ok {
		return nil, false
	}
	root, vars := fenceRoot(fence)
	fv := sharedVM(root)
	if fv == nil {
		return nil, false
	}
	var key strings.Builder
	for _, v := range vars {
		key.WriteString(v.pattern)
		key.WriteByte(0)
	}
	key.WriteString(jsCode)
	fn, seen := fv.funcs[key.String()]
	if !seen {
		var src strings.Builder
		src.WriteString("((")
		for i := range vars {
			if i > 0 {
				src.WriteString(", ")
			}
			src.WriteString(loopArg + strconv.Itoa(i))
		}
		src.WriteString(") => {\n")
		for i, v := range vars {
			src.WriteString("let " + v.pattern + " = " + loopArg + strconv.Itoa(i) + ";\n")
		}
		src.WriteString("return (\n" + jsCode + "\n);\n})")
		if p, ok := parsedProgram(src.String()); ok {
			if f, err := fv.vm.RunProgram(p); err == nil {
				fn, _ = goja.AssertFunction(f)
			}
		}
		fv.funcs[key.String()] = fn
	}
	if fn == nil {
		return nil, false
	}
	args := make([]goja.Value, len(vars))
	for i, v := range vars {
		p, parsed := parsedProgram("(\n" + v.value + "\n)")
		if !parsed {
			return nil, false
		}
		value, err := fv.vm.RunProgram(p)
		if err != nil {
			return nil, false
		}
		args[i] = value
	}
	result, err := fn(goja.Undefined(), args...)
	if err != nil {
		return nil, false
	}
	return result.Export(), true
}
