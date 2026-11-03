package pico

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExternalPropRendersSSRWithoutEmbeddingHydrationValue(t *testing.T) {
	markup := renderLiteralPropsPage(t, `---
prop content;
prop allContent;
let first = allContent[0];
---
<html><head></head><body><h1>{first.fields.title}</h1><ul>{for let node of allContent}<li>{node.fields.title}</li>{/for}</ul></body></html>`, map[string]any{
		"content": map[string]any{"type": "pages"},
		"allContent": NewExternalProp([]any{
			map[string]any{"fields": map[string]any{"title": "Hello"}},
		}, "/core/allContent.json"),
	})

	if !strings.Contains(markup, ">Hello</h1>") || !strings.Contains(markup, "<li") || !strings.Contains(markup, ">Hello</li>") {
		t.Fatalf("external prop was not available during SSR or in loops:\n%s", markup)
	}
	if !strings.Contains(markup, `p-external-props="{&#34;allContent&#34;:&#34;/core/allContent.json&#34;}"`) {
		t.Fatalf("external data source was not emitted:\n%s", markup)
	}
	if strings.Contains(markup, `"allContent":[{`) || strings.Contains(markup, `"title":"Hello"`) {
		t.Fatalf("external prop value was embedded in hydration data:\n%s", markup)
	}
}

func TestExternalPropStaysExternalAcrossDynamicComponents(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "html.pico")
	child := filepath.Join(dir, "pages.pico")
	if err := os.WriteFile(root, []byte(`---
prop content;
prop allContent;
---
<html><head></head><body><='./{content.type}.pico' {content} {allContent} /></body></html>`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(child, []byte(`---
prop allContent;
---
<ul>{for let node of allContent}<li>{node.fields.title}</li>{/for}</ul>`), 0644); err != nil {
		t.Fatal(err)
	}
	markup, _, _ := RenderRoot(root, map[string]any{
		"content": map[string]any{"type": "pages"},
		"allContent": NewExternalProp([]any{
			map[string]any{"fields": map[string]any{"title": "Hello"}},
		}, "/core/allContent.json"),
	})
	if !strings.Contains(markup, ">Hello</li>") {
		t.Fatalf("forwarded external prop was not rendered:\n%s", markup)
	}
	if strings.Contains(markup, `"allContent":[{`) || strings.Contains(markup, `"title":"Hello"`) {
		t.Fatalf("forwarded external prop was embedded in the page:\n%s", markup)
	}
}
