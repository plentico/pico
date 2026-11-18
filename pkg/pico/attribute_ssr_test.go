package pico

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestStaticComponentPAttrSVGUse(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, filepath.Join(dir, "components", "icons.pico"), "---\nprop name;\n---\n<svg><use href={`/icons.svg#${name}`} /></svg>")
	writeFixture(t, filepath.Join(dir, "page.pico"), "---\nimport Icons from \"./components/icons.pico\";\n---\n<Icons name={'arrow'} />")

	markup, _, _ := RenderRoot(filepath.Join(dir, "page.pico"), map[string]any{})

	if !strings.Contains(markup, `href="/icons.svg#arrow"`) {
		t.Fatalf("static SVG use attribute was not server-rendered: %s", markup)
	}
	if strings.Contains(markup, `p-scope="name = \'`) {
		t.Fatalf("static component scope contains escaped quotes that are invalid in Pattr: %s", markup)
	}
	if !strings.Contains(markup, `p-attr:href="`+"`"+`/icons.svg#${name}`+"`"+`"`) {
		t.Fatalf("static SVG use attribute lost its client binding: %s", markup)
	}
}
