package pico

import (
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

// resetEvalCache drops the parsed programs; it's called for each page.
func resetEvalCache() {
	evalPrograms = map[string]*goja.Program{}
	fenceExtends = map[string]int{}
}

// extendFence returns fence followed by decl (a loop variable's declaration).
func extendFence(fence, decl string) string {
	extended := fence + decl
	fenceExtends[extended] = len(fence)
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
