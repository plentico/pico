package pico

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEvalUsesTemplateScope(t *testing.T) {
	dir := t.TempDir()
	template := filepath.Join(dir, "page.pico")
	source := `---
prop allContent;
let posts = allContent.filter(node => node.type === "posts");
let totalPages = Math.ceil(posts.length / 2);
---
<div>not rendered</div>`
	if err := os.WriteFile(template, []byte(source), 0644); err != nil {
		t.Fatal(err)
	}
	props := map[string]any{"allContent": []any{
		map[string]any{"type": "posts"},
		map[string]any{"type": "pages"},
		map[string]any{"type": "posts"},
		map[string]any{"type": "posts"},
	}}
	if got := Eval(template, props, "totalPages"); got != int64(2) {
		t.Fatalf("expected 2 pages, got %#v", got)
	}
}
