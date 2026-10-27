package pico

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// writeEvalCacheSite writes a Plenti-shaped site: a layout with a head
// component and a nav over the menu, and a dynamic content template with a
// body, sections in a loop and nested loops.
func writeEvalCacheSite(t testing.TB) string {
	dir := t.TempDir()
	files := map[string]string{
		"global/html.pico":        "---\nimport Head from \"./head.pico\";\nimport Nav from \"../components/nav.pico\";\nprop env;\nprop content;\nprop menu;\n---\n\n<!DOCTYPE html>\n<html lang=\"en\">\n  <Head {env} {content} />\n  <body>\n    <Nav {menu} current={content.path} />\n    <main>\n      <='../content/{content.type}.pico' {content} />\n    </main>\n    <footer><p>{env.siteName} · {menu.length} pages</p></footer>\n  </body>\n</html>\n",
		"global/head.pico":        "---\nprop env;\nprop content;\nlet title = content.fields.title + \" | \" + env.siteName;\n---\n\n<head>\n  <title>{title}</title>\n  <meta name=\"description\" content=\"{content.fields.summary}\" />\n</head>\n",
		"components/nav.pico":     "---\nprop menu = [];\nprop current = \"\";\n---\n\n<nav>\n  <ul>\n    {for let item of menu}\n      <li class=\"{item.path === current ? 'active' : ''}\"><a href=\"{item.path}\">{item.title}</a></li>\n    {/for}\n  </ul>\n</nav>\n",
		"components/section.pico": "---\nprop section = {};\n---\n\n<section class=\"section-{section.kind}\">\n  <h2>{section.heading}</h2>\n  {if section.image}\n    <img src=\"{section.image.src}\" alt=\"{section.image.alt}\" />\n  {else}\n    <p>No image</p>\n  {/if}\n  <div>{section.body | html}</div>\n</section>\n",
		"content/pages.pico":      "---\nimport Section from \"../components/section.pico\";\nprop content;\nlet count = content.fields.sections.length;\n---\n\n<article>\n  <h1>{content.fields.title}</h1>\n  <div class=\"body\">{content.fields.body | html}</div>\n  {for let section of content.fields.sections}\n    <Section {section} />\n  {/for}\n  <p>{count} sections, {missing.field} after an error</p>\n  <table>\n    {for let row of content.fields.grid}\n      <tr>{for let cell of row}<td>{cell * 2}</td>{/for}</tr>\n    {/for}\n  </table>\n  <ul>\n    {for let {name, role} of content.fields.people}\n      <li>{name}: {role}</li>\n    {/for}\n    {for let [k, v] of Object.entries(content.fields.stats)}\n      <li>{k} = {v}</li>\n    {/for}\n  </ul>\n</article>\n",
	}
	for name, src := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return filepath.Join(dir, "global", "html.pico")
}

func evalCacheProps(menuItems, sections, bodyKB int) map[string]any {
	para := "<p>" + strings.Repeat("Lorem ipsum dolor sit amet, consectetur adipiscing elit. ", 18) + "</p>"
	body := strings.Repeat(para, bodyKB*1024/len(para)+1)
	var menu, secs []any
	for i := 0; i < menuItems; i++ {
		menu = append(menu, map[string]any{"path": fmt.Sprintf("/page-%d/", i), "title": fmt.Sprintf("Page %d", i)})
	}
	for i := 0; i < sections; i++ {
		s := map[string]any{"kind": "text", "heading": fmt.Sprintf("Section %d", i), "body": para + para}
		if i%2 == 0 {
			s["image"] = map[string]any{"src": fmt.Sprintf("/media/img-%d.jpg", i), "alt": "A photo"}
		}
		secs = append(secs, s)
	}
	return map[string]any{
		"env":  map[string]any{"siteName": "Example Co"},
		"menu": menu,
		"content": map[string]any{"path": "/page-3/", "type": "pages", "fields": map[string]any{
			"title": "Repairs", "summary": "Fix it", "body": body, "sections": secs,
			"grid":   []any{[]any{1, 2}, []any{3, 4, 5}},
			"people": []any{map[string]any{"name": "Sam", "role": "Editor"}, map[string]any{"name": "Ali", "role": "Writer"}},
			"stats":  map[string]any{"jobs": 12},
		}},
	}
}

// Parsing each frontmatter once gives the same page as parsing it for every
// expression, with Pattr on and off.
func TestEvalCacheRendersTheSame(t *testing.T) {
	tpl := writeEvalCacheSite(t)
	props := evalCacheProps(12, 4, 2)
	defer func() { useEvalCache = true }()
	for _, noPattr := range []bool{false, true} {
		useEvalCache = false
		want, wantScript, _ := RenderRoot(tpl, props, noPattr)
		useEvalCache = true
		got, gotScript, _ := RenderRoot(tpl, props, noPattr)
		if comparable(got) != comparable(want) || gotScript != wantScript {
			t.Errorf("noPattr=%v: the page differs with the cache:\n%s\n---- without:\n%s", noPattr, got, want)
		}
		for _, s := range []string{"Repairs | Example Co", "Section 3", "No image", ">10</td>", "Ali: Writer", "jobs = 12", "4 sections,  after an error", `class="active`} {
			if !strings.Contains(got, s) {
				t.Errorf("noPattr=%v: want %q in:\n%s", noPattr, s, got)
			}
		}
	}
}

// comparable takes out what differs between any two renders: scoped class
// names (random), loop keys (numbered across renders) and the order of
// p-scope statements (map order).
func comparable(markup string) string {
	markup = regexp.MustCompile(`p-[A-Za-z0-9]{6}\b`).ReplaceAllString(markup, "p-X")
	markup = regexp.MustCompile(`p-for-key="s\d+:`).ReplaceAllString(markup, `p-for-key="s:`)
	return regexp.MustCompile(`p-scope="[^"]*"`).ReplaceAllStringFunc(markup, func(attr string) string {
		statements := strings.Split(strings.TrimSuffix(attr[len(`p-scope="`):len(attr)-1], ";"), ";")
		for i := range statements {
			statements[i] = strings.TrimSpace(statements[i])
		}
		sort.Strings(statements)
		return `p-scope="` + strings.Join(statements, "; ") + `"`
	})
}

// A loop item is parsed on its own, never with its whole frontmatter: one
// program per item (its declaration, or with a shared VM its value), so 50
// more menu items are 50 more programs.
func TestEvalCacheParsesLoopItemsAlone(t *testing.T) {
	tpl := writeEvalCacheSite(t)
	defer func() { useSharedVM = true }()
	for _, shared := range []bool{true, false} {
		useSharedVM = shared
		programs := func(menu int) int {
			RenderRoot(tpl, evalCacheProps(menu, 4, 2))
			for src := range evalPrograms {
				if strings.Contains(src, "let menu = ") && strings.Contains(src, "let item = ") {
					t.Fatalf("shared=%v: a loop item was parsed with its frontmatter:\n%.300s", shared, src)
				}
			}
			return len(evalPrograms)
		}
		if got := programs(100) - programs(50); got != 50 {
			t.Errorf("shared=%v: 50 more items gave %d more programs, want 50", shared, got)
		}
	}
}

func benchmarkRender(b *testing.B, menuItems, sections, bodyKB int, cache bool) {
	tpl := writeEvalCacheSite(b)
	props := evalCacheProps(menuItems, sections, bodyKB)
	suppressJSErrors = true
	useEvalCache = cache
	defer func() { suppressJSErrors = false; useEvalCache = true }()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RenderRoot(tpl, props)
	}
}

// go test ./pkg/pico -run '^$' -bench Render
func BenchmarkRenderLargePage(b *testing.B)         { benchmarkRender(b, 150, 20, 30, true) }
func BenchmarkRenderLargePageUncached(b *testing.B) { benchmarkRender(b, 150, 20, 30, false) }
func BenchmarkRenderSmallPage(b *testing.B)         { benchmarkRender(b, 10, 3, 2, true) }
func BenchmarkRenderSmallPageUncached(b *testing.B) { benchmarkRender(b, 10, 3, 2, false) }
