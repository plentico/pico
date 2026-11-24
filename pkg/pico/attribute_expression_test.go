package pico

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTemplateLiteralAttribute(t *testing.T) {
	markup := "---\nlet icon = \"battery\";\n---\n<a href={`/icons.svg#${icon}`}>Icon</a>"
	rendered := renderLiteralPropsPage(t, markup, nil)

	if !strings.Contains(rendered, `href="/icons.svg#battery"`) {
		t.Fatalf("expected the evaluated SSR href, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "p-attr:href=\"`/icons.svg#${icon}`\"") {
		t.Fatalf("expected Pico to preserve the complete expression for Pattr, got:\n%s", rendered)
	}
}
func TestLoopComponentTemplateLiteralAttributes(t *testing.T) {
	d := t.TempDir()
	page := `---
import Icon from "./icon.pico";
let features = [{icon: "signal"}];
---
{for let feature in features}<Icon name={feature.icon} />{/for}`
	icon := `---
prop name;
let className = "";
---
<svg class="icon {className}"><use href={` + "`/icons.svg#${name}`" + `} /></svg>`
	if err := os.WriteFile(filepath.Join(d, "page.pico"), []byte(page), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, "icon.pico"), []byte(icon), 0644); err != nil {
		t.Fatal(err)
	}
	markup, _, _ := RenderRoot(filepath.Join(d, "page.pico"), nil)

	if !strings.Contains(markup, "p-class=\"`icon ${className}`\"") {
		t.Fatalf("nested component p-class was processed as a Pico expression:\n%s", markup)
	}
	if strings.Contains(markup, "p-attr:p-class") {
		t.Fatalf("Pattr directive was incorrectly converted to a bound attribute:\n%s", markup)
	}
	if !strings.Contains(markup, "p-attr:href=\"`/icons.svg#${name}`\"") {
		t.Fatalf("nested component bound href was not preserved:\n%s", markup)
	}
}

func TestExpressionBraceMatching(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		fence      string
		want       string
	}{
		{name: "identifier", expression: `{name}`, fence: `let name = "Lora";`, want: "Lora"},
		{name: "object literal", expression: `{({name: "Lora"}).name}`, want: "Lora"},
		{name: "braces in strings", expression: `{"}" + name}`, fence: `let name = "Lora";`, want: "}Lora"},
		{name: "template interpolation", expression: "{`/icons.svg#${feature.icon}`}", fence: `let feature = {icon: "voice"};`, want: "/icons.svg#voice"},
		{name: "multiple interpolations", expression: "{`${feature.kind}/${feature.icon}`}", fence: `let feature = {kind: "icon", icon: "voice"};`, want: "icon/voice"},
		{name: "nested template interpolation", expression: "{`icon-${`${feature.icon}`}`}", fence: `let feature = {icon: "voice"};`, want: "icon-voice"},
		{name: "object in interpolation", expression: "{`icon-${({name: feature.icon}).name}`}", fence: `let feature = {icon: "voice"};`, want: "icon-voice"},
		{name: "escaped backtick", expression: "{`icon\\`${feature.icon}`}", fence: `let feature = {icon: "voice"};`, want: "icon`voice"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := evalAllBrackets(tt.expression, tt.fence); got != tt.want {
				t.Errorf("evalAllBrackets() = %q, want %q", got, tt.want)
			}
		})
	}
}
