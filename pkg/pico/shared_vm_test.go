package pico

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// sharedVMByDefault is useSharedVM before any test changes it.
var sharedVMByDefault = useSharedVM

func writeSharedVMSite(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, src := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return filepath.Join(dir, "page.pico")
}

// renderBothWays renders a page with and without the shared VM and returns
// the markup and the errors logged (without their times), for each.
func renderBothWays(t *testing.T, page string, props map[string]any, noPattr bool) (shared, fresh [2]string) {
	t.Helper()
	defer func() { useSharedVM = true }()
	render := func(on bool) [2]string {
		useSharedVM = on
		var logs bytes.Buffer
		log.SetOutput(&logs)
		defer log.SetOutput(os.Stderr)
		markup, script, _ := RenderRoot(page, props, noPattr)
		lines := regexp.MustCompile(`(?m)^\S+ \S+ `).ReplaceAllString(logs.String(), "")
		return [2]string{comparable(markup) + script, lines}
	}
	return render(true), render(false)
}

func visibleText(markup string) string {
	markup = regexp.MustCompile(`(?s)<template.*?</template>`).ReplaceAllString(markup, " ")
	markup = regexp.MustCompile(`<[^>]*>`).ReplaceAllString(markup, " ")
	return strings.Join(strings.Fields(markup), " ")
}

// A page whose expressions have no side effects renders the same with the
// shared VM as without, and logs the same errors (with the same positions),
// with Pattr on and off.
func TestSharedVMRendersTheSame(t *testing.T) {
	if !sharedVMByDefault {
		t.Fatal("useSharedVM should be on by default")
	}
	page := writeSharedVMSite(t, map[string]string{
		"card.pico":   "---\nprop item = {};\nprop n = 0;\nlet label = item.title + \" #\" + n;\n---\n<li class=\"{n % 2 ? 'odd' : 'even'}\">{label} {item.tags.join(\", \")}</li>\n",
		"note.pico":   "---\nprop text = \"\";\n---\n<p>{text.toUpperCase()}</p>\n",
		"broken.pico": "---\nlet a = \"A\";\nlet b = missing.x;\n---\n<span>[{a}]</span>\n",
		"page.pico": "---\nimport Card from \"./card.pico\";\nimport Broken from \"./broken.pico\";\nprop items;\nprop grid;\nprop people;\nprop stats;\nprop kind;\nfunction twice(x) { return x * 2; }\nconst total = items.length;\n---\n" +
			"<div>\n<ul>{for let [i, item] of items.entries()}<Card {item} n={i} />{/for}</ul>\n" +
			"<table>{for let row of grid}<tr>{for let cell of row}<td>{twice(cell)} {row.length}</td>{/for}</tr>{/for}</table>\n" +
			"{for let {name, role} of people}<b>{name}: {role}</b>{/for}\n" +
			"{for let [k, v] of Object.entries(stats)}<i>{k}={v}</i>{/for}\n" +
			"{if total > 5}<p>many</p>{else if total > 1}<p>some {total}</p>{else}<p>few</p>{/if}\n" +
			"<section><=\"./{kind}.pico\" text={kind} /></section>\n" +
			"<p>[{this === globalThis}] [{let z = 2; z}] [{missing.field}] [{items[9].title}]</p>\n" +
			"<Broken />\n</div>\n",
	})
	props := map[string]any{
		"items":  []any{map[string]any{"title": "One", "tags": []any{"a", "b"}}, map[string]any{"title": "Two", "tags": []any{}}, map[string]any{"title": "Three", "tags": []any{"c"}}},
		"grid":   []any{[]any{1, 2}, []any{3}},
		"people": []any{map[string]any{"name": "Sam", "role": "Editor"}},
		"stats":  map[string]any{"jobs": 12},
		"kind":   "note",
	}
	for _, noPattr := range []bool{true, false} {
		shared, fresh := renderBothWays(t, page, props, noPattr)
		if shared[0] != fresh[0] {
			t.Errorf("noPattr=%v: the page differs with the shared VM:\n%s\n---- without:\n%s", noPattr, shared[0], fresh[0])
		}
		if shared[1] != fresh[1] {
			t.Errorf("noPattr=%v: the errors logged differ:\n%s\n---- without:\n%s", noPattr, shared[1], fresh[1])
		}
		for _, want := range []string{"One #0 a, b", "Three #2 c", ">2 2<", ">6 1<", "Sam: Editor", "jobs=12", "some 3", "NOTE", "[true]"} {
			if !strings.Contains(shared[0], want) {
				t.Errorf("noPattr=%v: want %q in:\n%s", noPattr, want, shared[0])
			}
		}
		if !strings.Contains(fresh[1], "missing") {
			t.Errorf("noPattr=%v: want the errors logged:\n%s", noPattr, fresh[1])
		}
	}
}

// What the shared VM changes: a component's frontmatter runs once, so a
// value an expression changes can be seen by expressions evaluated after
// it; a loop variable may shadow a frontmatter name; and a frontmatter
// function doesn't see loop variables (one that uses `it` directly fails,
// and is evaluated as before). Loop variables still stay inside their loop.
func TestSharedVMDifferences(t *testing.T) {
	page := writeSharedVMSite(t, map[string]string{
		"page.pico": "---\nlet n = 0;\nlet item = \"outer\";\nfunction lab() { return typeof it === 'undefined' ? 'none' : it; }\nfunction direct() { return it * 10; }\n---\n<div><p>{n++} {n++}</p>{for let item of [1, 2]}<b>{item}</b>{/for}<p>{item} {typeof i}</p>{for let i of [1]}<i>{i}</i>{/for}<p>{typeof i}</p>{for let it of [3]}<u>{lab()} {direct()}</u>{/for}</div>\n",
	})
	shared, fresh := renderBothWays(t, page, map[string]any{}, true)
	if got, want := visibleText(fresh[0]), "0 0 outer undefined 1 undefined 3 30"; got != want {
		t.Errorf("without the shared VM: want %q, got %q", want, got)
	}
	if got, want := visibleText(shared[0]), "0 1 1 2 outer undefined 1 undefined none 30"; got != want {
		t.Errorf("with the shared VM: want %q, got %q", want, got)
	}
}

// A page keeps at most maxFenceVMs VMs, however many components it renders.
func TestSharedVMsAreBounded(t *testing.T) {
	var items []any
	for i := 0; i < 200; i++ {
		items = append(items, map[string]any{"title": fmt.Sprintf("Card %d", i), "tags": []any{}})
	}
	page := writeSharedVMSite(t, map[string]string{
		"card.pico": "---\nprop item = {};\n---\n<li>{item.title}</li>\n",
		"page.pico": "---\nimport Card from \"./card.pico\";\nprop items;\n---\n<ul>{for let item of items}<Card {item} />{/for}</ul>\n",
	})
	shared, fresh := renderBothWays(t, page, map[string]any{"items": items}, false)
	if shared[0] != fresh[0] {
		t.Errorf("the page differs with the shared VM")
	}
	RenderRoot(page, map[string]any{"items": items})
	if len(fenceVMs) > 64 {
		t.Errorf("want at most 64 VMs, got %d", len(fenceVMs))
	}
	if !strings.Contains(shared[0], "Card 199") {
		t.Errorf("want every card rendered")
	}
}

// Going over the limit on parsed programs drops only the parsed programs:
// the page's VMs and loop variables are kept, so shared state doesn't
// restart part way through a page.
func TestProgramLimitKeepsVMs(t *testing.T) {
	resetEvalCache()
	fence := "let n = 0;"
	loop := extendFence(fence, "x", "1")
	if _, ok := evalParsed("n++", loop); !ok {
		t.Fatal("want the expression evaluated")
	}
	for i := 0; i <= 20000; i++ {
		parsedProgram(fmt.Sprintf("%d", i))
	}
	if len(fenceVMs) != 1 || len(fenceExtends) != 1 {
		t.Fatalf("want the VM and loop variable kept, got %d VMs and %d loop variables", len(fenceVMs), len(fenceExtends))
	}
	if v, _ := evalParsed("n", loop); v != int64(1) {
		t.Errorf("want n = 1 after the limit, got %v", v)
	}
}

// Expressions that are statements (a block, declarations) are evaluated as
// before, not as a function's return value: `{a: 1}` is a block whose
// value is 1, not an object.
func TestSharedVMLeavesStatementsAlone(t *testing.T) {
	defer func() { useSharedVM = true }()
	fence := "let x = 1;"
	for _, code := range []string{"{a: 1}", "function f() { return 1 }", "let z = 2; z", "var q = 5; q", "class A {}"} {
		useSharedVM = false
		resetEvalCache()
		want, wantOK := evalParsed(code, fence)
		useSharedVM = true
		resetEvalCache()
		got, gotOK := evalParsed(code, fence)
		if fmt.Sprintf("%v %v", got, gotOK) != fmt.Sprintf("%v %v", want, wantOK) {
			t.Errorf("%q: want %v (%v), got %v (%v)", code, want, wantOK, got, gotOK)
		}
	}
}

// Things the arrow function doesn't change: `arguments` and unbalanced text
// are errors as before, and items with the same text are still separate
// objects.
func TestSharedVMEdgesRenderTheSame(t *testing.T) {
	page := writeSharedVMSite(t, map[string]string{
		"page.pico": "---\nprop xs;\nlet a = 1;\nlet b = 2;\n---\n" +
			"<div><p>[{arguments.length}] [{a) + (b}]</p>{for let it of xs}<b>{it === xs[0]}</b>{/for}{for let p of [[1]]}{for let q of [[1]]}<i>{p === q}</i>{/for}{/for}</div>\n",
	})
	props := map[string]any{"xs": []any{map[string]any{"n": 1}, map[string]any{"n": 1}}}
	for _, noPattr := range []bool{true, false} {
		shared, fresh := renderBothWays(t, page, props, noPattr)
		if shared[0] != fresh[0] || shared[1] != fresh[1] {
			t.Errorf("noPattr=%v: differs with the shared VM:\n%s\n%s\n---- without:\n%s\n%s", noPattr, shared[0], shared[1], fresh[0], fresh[1])
		}
	}
}
