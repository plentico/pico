package pico

import (
	"strings"
	"testing"
)

func TestExternalPropRendersSSRWithoutEmbeddingHydrationValue(t *testing.T) {
	markup := renderLiteralPropsPage(t, `---
prop content;
prop allContent;
let first = allContent[0];
---
<html><head></head><body><h1>{first.fields.title}</h1></body></html>`, map[string]any{
		"content": map[string]any{"type": "pages"},
		"allContent": NewExternalProp([]any{
			map[string]any{"fields": map[string]any{"title": "Hello"}},
		}, "/core/allContent.json"),
	})

	if !strings.Contains(markup, ">Hello</h1>") {
		t.Fatalf("external prop was not available during SSR:\n%s", markup)
	}
	if !strings.Contains(markup, `p-external-props="{&#34;allContent&#34;:&#34;/core/allContent.json&#34;}"`) {
		t.Fatalf("external data source was not emitted:\n%s", markup)
	}
	if strings.Contains(markup, `"allContent":[{`) || strings.Contains(markup, `"title":"Hello"`) {
		t.Fatalf("external prop value was embedded in hydration data:\n%s", markup)
	}
}
