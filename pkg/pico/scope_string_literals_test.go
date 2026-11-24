package pico

import (
	"html"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/dop251/goja"
)

func TestScopePreservesQuotedStringLiterals(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{name: "single quotes", value: "'pro'"},
		{name: "double quotes", value: `"pro"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "page.pico")
			page := "---\nlet current = " + tt.value + ";\n---\n<main>{current}</main>"
			if err := os.WriteFile(path, []byte(page), 0644); err != nil {
				t.Fatal(err)
			}

			markup, _, _ := RenderRoot(path, map[string]any{})
			match := regexp.MustCompile(`p-scope="([^"]*)"`).FindStringSubmatch(markup)
			if len(match) != 2 {
				t.Fatalf("expected a root scope, got:\n%s", markup)
			}

			scope := html.UnescapeString(match[1])
			result, err := goja.New().RunString(scope + "\ncurrent")
			if err != nil {
				t.Fatalf("scope is not valid JavaScript: %v\nscope: %s", err, scope)
			}
			if got := result.String(); got != "pro" {
				t.Fatalf("current = %q, want pro\nscope: %s", got, scope)
			}
		})
	}
}
