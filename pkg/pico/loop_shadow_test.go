package pico

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A loop variable may have the same name as a variable the frontmatter or
// an outer loop declares. Inside the loop it shadows that variable, as in a
// JavaScript for loop; outside the loop the frontmatter's value is kept.
func TestLoopVariableShadowsFrontmatter(t *testing.T) {
	tests := []struct {
		name     string
		template string
		want     []string
	}{
		{
			name:     "const with a method call",
			template: "---\nconst item = 'fm';\nconst items = ['a', 'b'];\n---\n<ul>{for const item of items}<li>{item.toUpperCase()}</li>{/for}</ul><p>{item}</p>",
			want:     []string{">A</li>", ">B</li>", ">fm</p>"},
		},
		{
			name:     "let passed to a frontmatter function",
			template: "---\nlet post = {title: 'x'};\nlet posts = [{title: 'a'}, {title: 'b'}];\nfunction up(p) { return p.title.toUpperCase(); }\n---\n<ul>{for let post of posts}<li>{up(post)}</li>{/for}</ul><p>{post.title}</p>",
			want:     []string{">A</li>", ">B</li>", ">x</p>"},
		},
		{
			name:     "array destructuring",
			template: "---\nlet i = 9;\nlet items = ['a', 'b'];\n---\n<ul>{for let [i, x] of items.entries()}<li>{i + x}</li>{/for}</ul>",
			want:     []string{">0a</li>", ">1b</li>"},
		},
		{
			name:     "object destructuring",
			template: "---\nlet name = 'fm';\nlet people = [{name: 'ann'}, {name: 'bo'}];\n---\n<ul>{for let {name} of people}<li>{name.length}</li>{/for}</ul>",
			want:     []string{">3</li>", ">2</li>"},
		},
		{
			name:     "nested loops with the same name",
			template: "---\nlet rows = [[1, 2], [3]];\n---\n{for let row of rows}<ul>{for let row of [row.length]}<li>{row * 10}</li>{/for}</ul>{/for}",
			want:     []string{">20</li>", ">10</li>"},
		},
		{
			name:     "if inside the loop",
			template: "---\nconst item = 0;\nconst items = [1, 2];\n---\n{for const item of items}{if item > 1}<b>{item}</b>{/if}{/for}",
			want:     []string{"<b", ">2</b>"},
		},
	}
	for _, tt := range tests {
		for _, noPattr := range []bool{true, false} {
			dir := t.TempDir()
			path := filepath.Join(dir, "page.pico")
			if err := os.WriteFile(path, []byte(tt.template), 0644); err != nil {
				t.Fatal(err)
			}
			var logs bytes.Buffer
			log.SetOutput(&logs)
			html, _, _ := RenderRoot(path, map[string]any{}, noPattr)
			log.SetOutput(os.Stderr)
			for _, want := range tt.want {
				if !strings.Contains(html, want) {
					t.Errorf("%s (noPattr=%v): want %q in\n%s", tt.name, noPattr, want, html)
				}
			}
			if strings.Contains(logs.String(), "already been declared") {
				t.Errorf("%s (noPattr=%v): logged %s", tt.name, noPattr, logs.String())
			}
		}
	}
}
