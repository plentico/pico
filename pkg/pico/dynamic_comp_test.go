package pico

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func writeFixture(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeCompFixtures creates a components/ directory with hero and quote comps.
func writeCompFixtures(t *testing.T, dir string) {
	t.Helper()
	writeFixture(t, filepath.Join(dir, "components", "hero.pico"), `---
prop fields = {};
---

<section class="hero">
	<h2>{fields.title}</h2>
	{for let item of fields.items}
		<span class="hero-item">{item}</span>
	{/for}
</section>

<style>
	.hero {
		color: red;
	}
	.hero-item {
		color: pink;
	}
</style>
`)
	writeFixture(t, filepath.Join(dir, "components", "quote.pico"), `---
prop fields = {};
---

<blockquote class="quote">{fields.text}</blockquote>

<style>
	.quote {
		color: blue;
	}
</style>
`)
}

func TestDynamicCompResolved(t *testing.T) {
	dir := t.TempDir()
	writeCompFixtures(t, dir)
	writeFixture(t, filepath.Join(dir, "page.pico"), `---
let comp = "hero";
let heroFields = {title: "Welcome"};
---

<!DOCTYPE html>
<html>
<head></head>
<body>
	<main>
		<="./components/{comp}.pico" fields={heroFields} />
	</main>
</body>
</html>
`)

	markup, _, _ := RenderRoot(filepath.Join(dir, "page.pico"), map[string]any{})

	// Inert anchor with the unevaluated (client-resolvable) expression,
	// immediately followed by the SSR'd component markup.
	anchor := `<template p-comp="` + "`./components/${comp}.pico`" + `"`
	if !strings.Contains(markup, anchor) {
		t.Errorf("Expected p-comp anchor %s, got: %s", anchor, markup)
	}

	// SSR content is stamped with p-comp-node for hydration adoption.
	if !strings.Contains(markup, `p-comp-node="./components/hero.pico"`) {
		t.Errorf("Expected p-comp-node on SSR content, got: %s", markup)
	}

	// Registry entry is emitted at the end of <body>.
	regIdx := strings.Index(markup, `<template p-comp-src="./components/hero.pico">`)
	bodyEnd := strings.Index(markup, "</body>")
	if regIdx == -1 || bodyEnd == -1 || regIdx > bodyEnd {
		t.Errorf("Expected registry entry before </body>, got: %s", markup)
	}
	registry := markup[regIdx:bodyEnd]
	// Registry source carries live bindings, not just baked text.
	if !strings.Contains(registry, "p-text=\"`${fields.title}`\"") {
		t.Errorf("Expected p-text binding in registry source, got: %s", registry)
	}
	// Prop expressions stay unevaluated so client scopes resolve them.
	if !strings.Contains(registry, `p-scope="fields = heroFields;"`) {
		t.Errorf("Expected unevaluated p-scope props in registry source, got: %s", registry)
	}
	// Registry source is not itself stamped as SSR comp content.
	if strings.Contains(registry, "p-comp-node") {
		t.Errorf("Registry source should not carry p-comp-node, got: %s", registry)
	}
}

func TestDynamicCompInLoop(t *testing.T) {
	loopScopeCounter = 0
	dir := t.TempDir()
	writeCompFixtures(t, dir)
	writeFixture(t, filepath.Join(dir, "page.pico"), `---
let comps = [
	{name: "hero", fields: {title: "Welcome", items: ["a", "b"]}},
	{name: "quote", fields: {text: "Nice"}}
];
---

<!DOCTYPE html>
<html>
<head></head>
<body>
	<main>
		{for let comp of comps}
			<="./components/{comp.name}.pico" fields={comp.fields} />
		{/for}
	</main>
</body>
</html>
`)

	markup, _, _ := RenderRoot(filepath.Join(dir, "page.pico"), map[string]any{})

	// The client loop template contains only the anchor (content is inflated
	// from the registry on re-render, never baked from the first item).
	forStart := strings.Index(markup, `<template p-for="comp of comps"`)
	if forStart == -1 {
		t.Fatalf("Expected p-for template, got: %s", markup)
	}
	openEnd := strings.Index(markup[forStart:], ">")
	rest := markup[forStart+openEnd+1:]
	firstClose := strings.Index(rest, "</template>") // anchor close
	secondClose := strings.Index(rest[firstClose+1:], "</template>") + firstClose + 1
	tplContent := rest[:secondClose]
	if !strings.Contains(tplContent, `<template p-comp="`+"`./components/${comp.name}.pico`") {
		t.Errorf("Expected p-comp anchor inside p-for template, got: %s", tplContent)
	}
	if strings.Contains(tplContent, "<section") || strings.Contains(tplContent, "Welcome") {
		t.Errorf("p-for template should not contain baked comp markup, got: %s", tplContent)
	}

	// Each SSR iteration has its own anchor + p-comp-node content, both keyed.
	if !strings.Contains(markup, `p-comp-node="./components/hero.pico"`) {
		t.Errorf("Expected hero SSR content, got: %s", markup)
	}
	if !strings.Contains(markup, `p-comp-node="./components/quote.pico"`) {
		t.Errorf("Expected quote SSR content, got: %s", markup)
	}
	if !strings.Contains(markup, `p-for-key="s0:0"`) || !strings.Contains(markup, `p-for-key="s0:1"`) {
		t.Errorf("Expected keyed SSR iterations, got: %s", markup)
	}
	// The in-loop anchors carry the item's p-for-key too (grouping continuity).
	keyedAnchor := `<template p-comp="` + "`./components/${comp.name}.pico`" + `" p-scope="comp = {fields: {items: [&#39;a&#39;, &#39;b&#39;], title: &#39;Welcome&#39;}, name: &#39;hero&#39;};" p-for-key="s0:0"`
	if !strings.Contains(markup, keyedAnchor) {
		t.Errorf("Expected keyed in-loop anchor %s, got: %s", keyedAnchor, markup)
	}

	// One registry entry per resolved key, at the end of <body>.
	heroReg := strings.Index(markup, `<template p-comp-src="./components/hero.pico">`)
	quoteReg := strings.Index(markup, `<template p-comp-src="./components/quote.pico">`)
	bodyEnd := strings.Index(markup, "</body>")
	if heroReg == -1 || quoteReg == -1 || heroReg > bodyEnd || quoteReg > bodyEnd {
		t.Errorf("Expected both registry entries before </body>, got: %s", markup)
	}
	registry := markup[heroReg:bodyEnd]
	// Registry sources carry no baked loop items/keys from any invocation.
	if strings.Contains(registry, "p-for-key") {
		t.Errorf("Registry sources should not contain baked p-for-key, got: %s", registry)
	}
	// ...but the comp's own client template is preserved for client rendering.
	if !strings.Contains(registry, `<template p-for="item of fields.items"`) {
		t.Errorf("Expected inner p-for template in hero registry source, got: %s", registry)
	}
}

func TestDynamicCompUnresolvable(t *testing.T) {
	dir := t.TempDir()
	writeCompFixtures(t, dir)
	writeFixture(t, filepath.Join(dir, "page.pico"), `---
let comp;
---

<!DOCTYPE html>
<html>
<head></head>
<body>
	<main>
		<="./components/{comp}.pico" />
	</main>
</body>
</html>

<style>
	main section {
		color: green;
	}
</style>
`)

	// Must not crash (previously log.Fatal'd on the missing file).
	markup, _, style := RenderRoot(filepath.Join(dir, "page.pico"), map[string]any{})

	// Anchor only, no inline content (nothing to SSR).
	if !strings.Contains(markup, `<template p-comp="`+"`./components/${comp}.pico`"+`"`) {
		t.Errorf("Expected p-comp anchor, got: %s", markup)
	}
	if strings.Contains(markup, "p-comp-node") {
		t.Errorf("Unresolvable comp should have no SSR content, got: %s", markup)
	}

	// Every candidate matching the path's static prefix is registered.
	if !strings.Contains(markup, `<template p-comp-src="./components/hero.pico">`) ||
		!strings.Contains(markup, `<template p-comp-src="./components/quote.pico">`) {
		t.Errorf("Expected globbed registry candidates, got: %s", markup)
	}

	// CSS treeshaking is disabled: the page style targets runtime-only markup,
	// and registry comp styles may match client-rendered loop items.
	if !strings.Contains(style, "green") {
		t.Errorf("Expected page style to survive treeshaking, got: %s", style)
	}
	if !strings.Contains(style, "pink") {
		t.Errorf("Expected registry comp style to survive treeshaking, got: %s", style)
	}
}

func TestDynamicCompUnresolvableSkipsCurrentTemplate(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, filepath.Join(dir, "friend.pico"), `---
prop fields = {};
---

<p class="friend">{fields.text}</p>
`)
	writeFixture(t, filepath.Join(dir, "page.pico"), `---
let comp;
---

<!DOCTYPE html>
<html>
<head></head>
<body>
	<main>
		<="./{comp}.pico" />
	</main>
</body>
</html>
`)

	markup, _, _ := RenderRoot(filepath.Join(dir, "page.pico"), map[string]any{})

	// The sibling comp is registered...
	if !strings.Contains(markup, `<template p-comp-src="./friend.pico">`) {
		t.Errorf("Expected sibling candidate in registry, got: %s", markup)
	}
	// ...but the template being rendered never registers itself (recursion guard).
	if strings.Contains(markup, `<template p-comp-src="./page.pico">`) {
		t.Errorf("Current template should not be registered as a candidate, got: %s", markup)
	}
}

func TestDynamicCompMissingFileNoBraces(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, filepath.Join(dir, "page.pico"), `---
---

<!DOCTYPE html>
<html>
<head></head>
<body>
	<main>
		<="./nope.pico" />
	</main>
</body>
</html>
`)

	markup, _, _ := RenderRoot(filepath.Join(dir, "page.pico"), map[string]any{})

	if !strings.Contains(markup, `<template p-comp="`+"`./nope.pico`"+`"`) {
		t.Errorf("Expected p-comp anchor, got: %s", markup)
	}
	if strings.Contains(markup, "p-comp-src") {
		t.Errorf("No registry entries expected for brace-less missing file, got: %s", markup)
	}
}

func TestDynamicCompNoPattrUnchanged(t *testing.T) {
	dir := t.TempDir()
	writeCompFixtures(t, dir)
	writeFixture(t, filepath.Join(dir, "page.pico"), `---
let comp = "hero";
---

<!DOCTYPE html>
<html>
<head></head>
<body>
	<main>
		<="./components/{comp}.pico" />
	</main>
</body>
</html>
`)

	// Legacy SSR-only mode: no anchors, no registry, content rendered inline.
	markup, _, _ := RenderRoot(filepath.Join(dir, "page.pico"), map[string]any{}, true)

	if strings.Contains(markup, "p-comp") {
		t.Errorf("Expected no p-comp markup with noPattr, got: %s", markup)
	}
	if !strings.Contains(markup, `<section class="hero`) {
		t.Errorf("Expected inline SSR content with noPattr, got: %s", markup)
	}
}

func TestDynamicCompSSRStyleClassParity(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, filepath.Join(dir, "thing.pico"), `---
prop label = "x";
---

<h1 class="title">{label}</h1>

<style>
	.title {
		color: green;
	}
</style>
`)
	writeFixture(t, filepath.Join(dir, "page.pico"), `---
let comp = "thing";
---

<!DOCTYPE html>
<html>
<head></head>
<body>
	<main>
		<="./{comp}.pico" label={"hello"} />
	</main>
</body>
</html>
`)

	markup, _, style := RenderRoot(filepath.Join(dir, "page.pico"), map[string]any{})

	// Scoped class stamped on the SSR'd comp root.
	reSSR := regexp.MustCompile(`<h1[^>]*class="title (p-[A-Za-z0-9]+)"[^>]*p-comp-node="\./thing\.pico"`)
	m := reSSR.FindStringSubmatch(markup)
	if m == nil {
		t.Fatalf("Expected SSR comp root with scoped class, got: %s", markup)
	}
	scopedClass := m[1]

	// The registry source reuses the same scoped class, so markup the client
	// (re)inflates from it is styled by the same CSS rules as the SSR copy.
	regIdx := strings.Index(markup, `<template p-comp-src="./thing.pico">`)
	if regIdx == -1 {
		t.Fatalf("Expected registry entry, got: %s", markup)
	}
	registry := markup[regIdx:]
	if !strings.Contains(registry, `class="title `+scopedClass+`"`) {
		t.Errorf("Registry source should reuse SSR scoped class %s, got: %s", scopedClass, registry)
	}

	// Exactly one CSS rule, matching the shared scoped class (emitted once by
	// the registry render, not per SSR instance).
	rule := ".title." + scopedClass + "{color:green;}"
	if count := strings.Count(style, rule); count != 1 {
		t.Errorf("Expected exactly one %q rule, got %d in: %s", rule, count, style)
	}
}
