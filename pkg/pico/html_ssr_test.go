package pico

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestHTMLModifierSSRPreservesMixedContentOrder(t *testing.T) {
	tests := []struct {
		name    string
		markup  string
		content string
		want    string
	}{
		{
			name:    "text before and after elements",
			markup:  "<h1>Meet Lora 01 {heading | html} in the wild.</h1>",
			content: "Pro.<br><em>Offline by nature.</em>",
			want:    ">Meet Lora 01 Pro.<br/><em>Offline by nature.</em> in the wild.</h1>",
		},
		{
			name:    "multiple elements without surrounding text",
			markup:  "<h1>{heading | html}</h1>",
			content: "Lora 01 Pro.<br><em>Offline by nature.</em>",
			want:    ">Lora 01 Pro.<br/><em>Offline by nature.</em></h1>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "page.pico")
			page := "---\nprop heading;\n---\n" + tt.markup
			if err := os.WriteFile(path, []byte(page), 0644); err != nil {
				t.Fatal(err)
			}

			markup, _, _ := RenderRoot(path, map[string]any{"heading": tt.content})
			// Scoped classes vary by render; they don't affect child ordering.
			markup = regexp.MustCompile(` class="p-[^"]+"`).ReplaceAllString(markup, "")
			if !strings.Contains(markup, tt.want) {
				t.Fatalf("SSR mixed content is out of order\nwant fragment: %s\ngot: %s", tt.want, markup)
			}
		})
	}
}
