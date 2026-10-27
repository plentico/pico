package pico

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/dop251/goja"
)

// Text with braces is evaluated expression by expression, each "{" with its
// own closing "}": nested braces and quoted braces belong to the
// expression, and a "}" or "{" on its own is text. (A "}" before a "{"
// crashed the build: slice bounds out of range.) With Pattr, the p-text
// template literal shows the same text when the browser runs it.
func TestNestedBraces(t *testing.T) {
	cases := []struct{ text, want string }{
		{"[{ ({a: 1}).a }] [{x}]", "[1] [X]"},
		{"a } b {x}", "a } b X"},
		{"[{\"}\"}] [{'{'}] [{x}]", "[}] [{] [X]"},
		{"[{xs.map(i => { return i * 2 }).join(\",\")}]", "[2,4]"},
		{"a { b {x}", "a { b X"},
		{"{x}{x}", "XX"},
	}
	for _, tc := range cases {
		dir := t.TempDir()
		src := "---\nlet x = \"X\";\nlet xs = [1, 2];\n---\n<p>" + tc.text + "</p>\n"
		if err := os.WriteFile(filepath.Join(dir, "page.pico"), []byte(src), 0644); err != nil {
			t.Fatal(err)
		}
		for _, noPattr := range []bool{true, false} {
			got, err := func() (out string, err error) {
				defer func() {
					if r := recover(); r != nil {
						err = fmt.Errorf("%v", r)
					}
				}()
				markup, _, _ := RenderRoot(filepath.Join(dir, "page.pico"), map[string]any{}, noPattr)
				return markup, nil
			}()
			if err != nil {
				t.Errorf("%q (noPattr=%v): the build crashed: %v", tc.text, noPattr, err)
				continue
			}
			text := strings.TrimSpace(regexp.MustCompile(`<[^>]*>`).ReplaceAllString(got, ""))
			if text != tc.want {
				t.Errorf("%q (noPattr=%v): want %q, got %q", tc.text, noPattr, tc.want, text)
			}
			if noPattr {
				continue
			}
			m := regexp.MustCompile(`p-text="([^"]*)"`).FindStringSubmatch(got)
			if m == nil {
				t.Errorf("%q: no p-text in:\n%s", tc.text, got)
				continue
			}
			v, err := goja.New().RunString("let x = 'X'; let xs = [1, 2];\n" + html.UnescapeString(m[1]))
			if err != nil || v.String() != html.UnescapeString(tc.want) {
				t.Errorf("%q: p-text %s gives %v (%v), want %q", tc.text, m[1], v, err, tc.want)
			}
		}
	}
}
