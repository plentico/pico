package pico

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/dop251/goja"
)

// renderLiteralPropsPage renders page with props from a temporary folder.
func renderLiteralPropsPage(t *testing.T, page string, props map[string]any) string {
	t.Helper()
	d := t.TempDir()
	if err := os.WriteFile(filepath.Join(d, "page.pico"), []byte(page), 0644); err != nil {
		t.Fatal(err)
	}
	markup, _, _ := RenderRoot(filepath.Join(d, "page.pico"), props)
	return markup
}

// A page's props are inserted into the frontmatter literally: nothing in
// content is read as part of the frontmatter, so it can't garble text or
// turn into code at build time.
func TestPropsAreInsertedLiterally(t *testing.T) {
	// "$5" isn't a reference to the regular expression's match.
	m := renderLiteralPropsPage(t, "---\nprop msg;\n---\n<div><p>{msg}</p></div>\n",
		map[string]any{"msg": "Only $5 or $price today"})
	if !strings.Contains(m, ">Only $5 or $price today</p>") {
		t.Errorf("want the text as it is:\n%s", m)
	}

	// $3 is the prop's default value: expanded inside the content, it closed
	// the string and the rest of the content ran as code. (The default ";"
	// is also cut at its own semicolon, a separate bug, so this frontmatter
	// doesn't parse after the fix either: the value is checked as text.)
	fence, _ := setProps("prop delim = \";\";\nlet x = 1;", map[string]any{"delim": "$3; globalThis.pwned = 1; //"})
	if !strings.Contains(fence, `let delim = "$3; globalThis.pwned = 1; //";`) {
		t.Errorf("want the value as one string in the frontmatter:\n%s", fence)
	}

	// An object's keys are content too: unquoted, a key ran as code.
	key := "a: (globalThis.pwned = 1), b"
	fence, _ = setProps("prop obj;", map[string]any{"obj": map[string]any{key: 2}})
	vm := goja.New()
	got, err := vm.RunString(fence + "\nobj[" + strconv.Quote(key) + "]")
	if pwned := vm.Get("pwned"); pwned != nil && !goja.IsUndefined(pwned) {
		t.Errorf("content ran as code:\n%s", fence)
	} else if err != nil || got.Export() != int64(2) {
		t.Errorf("want the key as it is (%v):\n%s", err, fence)
	}

	// A value reaches JavaScript as it is (Go's \a and \U escapes aren't
	// JavaScript's).
	text := "a\a\vb\U000E0001c</script>\u2028"
	fence, _ = setProps("prop s;", map[string]any{"s": text})
	if got, err := goja.New().RunString(fence + "\ns"); err != nil || got.Export() != text {
		t.Errorf("want %q, got %v (%v):\n%s", text, got, err, fence)
	}

	// A key that isn't an identifier isn't a prop: matched literally, it
	// spelled out template text and swallowed it.
	fence, pScope := setProps("prop a;\nlet open = false;\nprop b;", map[string]any{"a;\nlet open = false;\nprop b": "x"})
	if !strings.Contains(fence, "let open = false;") || strings.Contains(fence, `"x"`) || !strings.Contains(pScope, "open = false") {
		t.Errorf("want the template's own text kept:\n%s\n%s", fence, pScope)
	}

	// Words in content aren't frontmatter: "let" and "const" aren't
	// declarations to add semicolons before, and "prop x;" isn't a prop.
	m = renderLiteralPropsPage(t, "---\nprop a;\nprop b;\n---\n<div><p>{a}</p><p>{b}</p></div>\n",
		map[string]any{"a": "Please let me know; a prop x; ok", "b": "Call const tim"})
	if !strings.Contains(m, ">Please let me know; a prop x; ok</p>") || !strings.Contains(m, ">Call const tim</p>") {
		t.Errorf("want the text as it is:\n%s", m)
	}
	for i := 0; i < 20; i++ { // (props are inserted in map order)
		fence, _ = setProps("prop a;\nprop b;", map[string]any{"a": "prop b;", "b": "x"})
		if !strings.Contains(fence, `let a = "prop b;";`) || !strings.Contains(fence, `let b = "x";`) {
			t.Errorf("one prop's value was read as another prop:\n%s", fence)
			break
		}
	}

	// A prop name from content is matched literally (a "(" in it crashed the
	// build).
	m = renderLiteralPropsPage(t, "---\nprop msg;\n---\n<div><p>{msg}</p></div>\n",
		map[string]any{"msg": "ok", "a(b": 1})
	if !strings.Contains(m, ">ok</p>") {
		t.Errorf("want the page to render:\n%s", m)
	}
}
