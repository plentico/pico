package pico

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestComponentPropsAreEvaluatedInParentScope(t *testing.T) {
	dir := t.TempDir()
	componentPath := filepath.Join(dir, "age_button.pico")
	pagePath := filepath.Join(dir, "page.pico")

	component := `---
prop age;

age = age + 1;
age = age * 2;
---
<section class="age-button">
  <h3>Age is: {age}</h3>
  <button onclick="{age = age + 1}">+</button>
  <button onclick="{age = age - 1}">-</button>
</section>`
	page := `---
import AgeButton from "./age_button.pico";
let age = 2;
---
<AgeButton {age} />
<AgeButton age={age + 1} />`

	if err := os.WriteFile(componentPath, []byte(component), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pagePath, []byte(page), 0644); err != nil {
		t.Fatal(err)
	}

	markup, _, _ := RenderRoot(pagePath, nil)
	if !strings.Contains(markup, "Age is: 6") || !strings.Contains(markup, "Age is: 8") {
		t.Fatalf("expected shorthand prop to render 6 and expression prop to render 8, got:\n%s", markup)
	}

	// The incoming expression and the child's first mutation are intentionally
	// identical assignments. Both must run during hydration to produce 8.
	if !strings.Contains(markup, `p-scope="age = age;age = age + 1;age = age * 2;"`) ||
		!strings.Contains(markup, `p-scope="age = age + 1;age = age + 1;age = age * 2;"`) {
		t.Fatalf("expected each hydration scope to initialize its incoming prop before mutating it, got:\n%s", markup)
	}
}
