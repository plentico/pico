package pico

import (
	"log"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"golang.org/x/net/html"
)

// compRegistryEntry is a dynamic component (<= ... />) that the client may
// need to (re)render at runtime. Entries are emitted as inert
// <template p-comp-src="name"> elements at the end of <body> so Pattr can
// inflate component markup without network requests.
type compRegistryEntry struct {
	name           string          // value the p-comp expression evaluates to (e.g. "./hero.pico")
	path           string          // resolved template file path
	compProps      CompProps       // unevaluated prop expressions from the first invocation
	evaluatedProps map[string]any  // evaluated props from the first invocation
	ssrRendered    bool            // whether the component was also rendered inline (SSR)
	ssrElements    []scopedElement // scoped elements accumulated from inline SSR renders
	scripts        []string        // deduped scripts collected from SSR + registry renders
}

// Package-level render state for dynamic components. Reset per page in RenderRoot.
var compRegistry = map[string]*compRegistryEntry{}
var compRegistryOrder []string

// buildingLoopTemplate is true while evalControlTree renders the content of a
// <template p-for> client template. Dynamic comps emit only their p-comp anchor
// there; the client inflates the matching content from the registry per item.
var buildingLoopTemplate bool

// registryRenderMode is true while rendering a component into the p-comp-src
// registry. Loop SSR items are suppressed (only <template p-for> is emitted)
// so the registry source carries no data baked in from a single invocation.
var registryRenderMode bool

// pageNoTreeshake disables CSS treeshaking for the whole page. It is set when a
// dynamic comp path can't be resolved at build time, because page styles may
// target component markup that only exists at runtime.
var pageNoTreeshake bool

func resetCompRegistry() {
	compRegistry = map[string]*compRegistryEntry{}
	compRegistryOrder = nil
	buildingLoopTemplate = false
	registryRenderMode = false
	pageNoTreeshake = false
}

// registerDynamicComp records a resolved dynamic comp for the client registry.
// The first registration wins for props; ssrRendered is sticky.
func registerDynamicComp(name string, path string, props CompProps, evaluatedProps map[string]any, ssrRendered bool) {
	if name == "" {
		return
	}
	if existing, ok := compRegistry[name]; ok {
		if ssrRendered {
			existing.ssrRendered = true
		}
		return
	}
	compRegistry[name] = &compRegistryEntry{
		name:           name,
		path:           path,
		compProps:      props,
		evaluatedProps: evaluatedProps,
		ssrRendered:    ssrRendered,
	}
	compRegistryOrder = append(compRegistryOrder, name)
}

var reCompPathExpr = regexp.MustCompile(`\{([^{}]*)\}`)

// compAnchorExpr converts a dynamic comp path (e.g. "./{comp}.pico") into a JS
// expression the client can evaluate at runtime: a template literal with each
// {expr} turned into ${expr}.
func compAnchorExpr(path string) string {
	esc := strings.ReplaceAll(path, `\`, `\\`)
	esc = strings.ReplaceAll(esc, "`", "\\`")
	esc = strings.ReplaceAll(esc, "${", `\${`)
	esc = reCompPathExpr.ReplaceAllString(esc, `${$1}`)
	return "`" + esc + "`"
}

// compAnchorMarkup renders the inert <template p-comp> anchor for a dynamic comp.
func compAnchorMarkup(path string) string {
	return `<template p-comp="` + html.EscapeString(compAnchorExpr(path)) + `"></template>`
}

// stampCompNode marks every top-level element of SSR'd component markup with
// p-comp-node="<name>" so the client adopts it during hydration instead of
// re-inflating from the registry.
func stampCompNode(markup string, name string) string {
	nodes, err := parseNoFix(markup)
	if err != nil {
		return markup
	}
	var buf strings.Builder
	for _, node := range nodes {
		if node.Type == html.ElementNode {
			node.Attr = append(node.Attr, html.Attribute{Key: "p-comp-node", Val: name})
		}
		if err := html.Render(&buf, node); err != nil {
			log.Fatal(err)
		}
	}
	return buf.String()
}

// resolveDynamicCompPath evaluates a dynamic comp path expression against the
// current fence and reports the registry name (the evaluated path as the client
// will compute it), the build-time file path, and whether resolution succeeded
// (every {expression} evaluated and the file exists).
func resolveDynamicCompPath(pathExpr string, fence string, templateDir string) (string, string, bool) {
	name := evalAllBrackets(pathExpr, fence)
	resolvedPath := name
	if !filepath.IsAbs(resolvedPath) {
		resolvedPath = filepath.Join(templateDir, resolvedPath)
	}
	resolved := strings.TrimSpace(name) != "" && !strings.Contains(name, "{")
	if resolved {
		if info, err := os.Stat(resolvedPath); err != nil || info.IsDir() {
			resolved = false
		}
	}
	return name, resolvedPath, resolved
}

// registerCompCandidates handles dynamic comp paths that can't be resolved at
// build time: every .pico file matching the path's static prefix/suffix is
// registered so the client can resolve the component at runtime. The path's
// static parts determine the candidate directory and the registry key format,
// e.g. "./{comp}.pico" registers "./<name>.pico" for each ./<name>.pico file.
func registerCompCandidates(pathExpr string, templateDir string, props CompProps, evaluatedProps map[string]any) {
	// Page styles may target component markup that only exists at runtime.
	pageNoTreeshake = true

	staticPrefix := pathExpr
	staticSuffix := ""
	if openIdx := strings.Index(pathExpr, "{"); openIdx != -1 {
		staticPrefix = pathExpr[:openIdx]
		if closeIdx := strings.LastIndex(pathExpr, "}"); closeIdx > openIdx {
			staticSuffix = pathExpr[closeIdx+1:]
		}
	}

	dir := templateDir
	filePrefix := staticPrefix
	if strings.HasSuffix(staticPrefix, "/") {
		dir = filepath.Join(templateDir, staticPrefix)
		filePrefix = ""
	} else if staticPrefix != "" {
		dir = filepath.Join(templateDir, filepath.Dir(staticPrefix))
		filePrefix = filepath.Base(staticPrefix)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Printf("Dynamic comp path \"%s\" is not resolvable at build time and has no candidate directory (%s)", pathExpr, dir)
		return
	}

	registered := 0
	for _, entry := range entries {
		fileName := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(fileName, filePrefix) {
			continue
		}
		if staticSuffix != "" {
			if !strings.HasSuffix(fileName, staticSuffix) {
				continue
			}
		} else if !strings.HasSuffix(fileName, ".pico") {
			continue
		}
		candidatePath := filepath.Join(dir, fileName)
		// Never register the template that's currently being rendered.
		if filepath.Clean(candidatePath) == filepath.Clean(currentTemplatePath) {
			continue
		}
		key := staticPrefix + strings.TrimPrefix(fileName, filePrefix)
		registerDynamicComp(key, candidatePath, props, evaluatedProps, false)
		registered++
	}
	if registered > 0 {
		log.Printf("Dynamic comp path \"%s\" is not resolvable at build time; registered %d candidate(s) from %s for client-side rendering", pathExpr, registered, dir)
	}
}

// renderCompRegistry renders every registered dynamic component into an inert
// <template p-comp-src="name"> entry for the client. Components that were never
// SSR'd inline also contribute their style/script here (with CSS treeshaking
// disabled, since the client may render markup shapes that don't exist in the
// registry source, e.g. loop items).
func renderCompRegistry(scopeStack []scopeStackItem, usePattr bool) (string, []scopeStackItem) {
	var registryBuilder strings.Builder

	savedTemplatePath := currentTemplatePath
	defer func() { currentTemplatePath = savedTemplatePath }()

	// compRegistryOrder can grow while iterating: rendering an entry can
	// register nested dynamic comps, which are rendered in the same pass.
	for i := 0; i < len(compRegistryOrder); i++ {
		entry := compRegistry[compRegistryOrder[i]]

		wasRegistryMode := registryRenderMode
		registryRenderMode = true
		// Errors were already logged by the inline SSR render (or are expected
		// for best-effort candidates with mismatched props) — don't log twice.
		wasSuppressed := suppressJSErrors
		suppressJSErrors = true
		markup, script, style, newScopeStack, newPScopeExp, newFence := Render(entry.path, entry.evaluatedProps, []scopeStackItem{}, !usePattr)
		suppressJSErrors = wasSuppressed
		registryRenderMode = wasRegistryMode

		markup, scopedElements := scopeHTML(markup, entry.compProps, newPScopeExp, newFence, usePattr, entry.name)

		// Styles are emitted once per component and never treeshaken: inline SSR
		// markup, this registry source, and any client-side re-render all share
		// the same deterministic scoped classes (seeded by entry.name), and the
		// client may render markup shapes that don't exist in either build-time
		// copy (e.g. {if} branches that were false during SSR).
		elements := append(entry.ssrElements, scopedElements...)
		if script != "" && !slices.Contains(entry.scripts, script) {
			entry.scripts = append(entry.scripts, script)
		}
		scopeStack = append(scopeStack, scopeStackItem{
			scopedElements: elements,
			style:          style,
			script:         strings.Join(entry.scripts, "\n"),
			noTreeshake:    true,
		})
		for _, item := range newScopeStack {
			item.noTreeshake = true
			scopeStack = append(scopeStack, item)
		}

		registryBuilder.WriteString(`<template p-comp-src="` + html.EscapeString(entry.name) + `">`)
		registryBuilder.WriteString(markup)
		registryBuilder.WriteString(`</template>`)
	}

	return registryBuilder.String(), scopeStack
}

// injectCompRegistry inserts the p-comp-src registry before </body>.
func injectCompRegistry(markup string, registry string) string {
	if idx := strings.LastIndex(markup, "</body>"); idx != -1 {
		return markup[:idx] + registry + markup[idx:]
	}
	return markup + registry
}
