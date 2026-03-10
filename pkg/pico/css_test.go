package pico

import (
	"strings"
	"testing"
)

// ============ CSS Treeshaking Tests ============

func TestCSSTreeshaking(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		scopedElements []scopedElement
		expected       string
		description    string
	}{
		{
			name: "Keep used class selector",
			input: `.used-class {
	color: red;
}`,
			scopedElements: []scopedElement{
				{classes: []string{"used-class"}, scopedClass: "scope123"},
			},
			expected:    `.used-class.scope123{color:red;}`,
			description: "Ruleset with used class should be kept and scoped",
		},
		{
			name: "Remove unused class selector",
			input: `.unused-class {
	color: red;
}`,
			scopedElements: []scopedElement{
				{classes: []string{"other-class"}, scopedClass: "scope123"},
			},
			expected:    ``,
			description: "Ruleset with unused class should be removed",
		},
		{
			name: "Keep used tag selector",
			input: `div {
	color: blue;
}`,
			scopedElements: []scopedElement{
				{tag: "div", scopedClass: "scope456"},
			},
			expected:    `div.scope456{color:blue;}`,
			description: "Ruleset with used tag should be kept and scoped",
		},
		{
			name: "Remove unused tag selector",
			input: `span {
	color: blue;
}`,
			scopedElements: []scopedElement{
				{tag: "div", scopedClass: "scope456"},
			},
			expected:    ``,
			description: "Ruleset with unused tag should be removed",
		},
		{
			name: "Keep used ID selector",
			input: `#myid {
	color: green;
}`,
			scopedElements: []scopedElement{
				{id: "myid", scopedClass: "scope789"},
			},
			expected:    `#myid.scope789{color:green;}`,
			description: "Ruleset with used ID should be kept and scoped",
		},
		{
			name: "Remove unused ID selector",
			input: `#unused-id {
	color: green;
}`,
			scopedElements: []scopedElement{
				{id: "other-id", scopedClass: "scope789"},
			},
			expected:    ``,
			description: "Ruleset with unused ID should be removed",
		},
		{
			name: "Keep global selector even if unused",
			input: `*p {
	color: orange;
}`,
			scopedElements: []scopedElement{},
			expected:       `p{color:orange;}`,
			description:    "Global selectors should always be kept",
		},
		{
			name: "Keep universal selector",
			input: `* {
	box-sizing: border-box;
}`,
			scopedElements: []scopedElement{
				{tag: "div", scopedClass: "scope111"},
			},
			expected:    `*{box-sizing:border-box;}`,
			description: "Universal selector should always be kept",
		},
		{
			name: "Keep attribute selector",
			input: `[data-test] {
	color: purple;
}`,
			scopedElements: []scopedElement{},
			expected:       `[data-test]{color:purple;}`,
			description:    "Attribute selectors should always be kept",
		},
		{
			name: "Mixed used and unused selectors",
			input: `.used, .unused {
	color: red;
}`,
			scopedElements: []scopedElement{
				{classes: []string{"used"}, scopedClass: "scope123"},
			},
			expected:    `.used.scope123, .unused.scope123{color:red;}`,
			description: "Ruleset with at least one used selector should be kept",
		},
		{
			name: "Remove ruleset with only unused selectors",
			input: `.unused1, .unused2 {
	color: red;
}`,
			scopedElements: []scopedElement{
				{classes: []string{"other"}, scopedClass: "scope123"},
			},
			expected:    ``,
			description: "Ruleset with all unused selectors should be removed",
		},
		{
			name: "Descendant selector with used leaf only - should be removed",
			input: `.parent .child {
	color: pink;
}`,
			scopedElements: []scopedElement{
				{classes: []string{"child"}, scopedClass: "scope222"},
			},
			expected:    ``,
			description: "Descendant selector removed when parent is not in component",
		},
		{
			name: "Descendant selector with only parent used - should be removed",
			input: `.parent .child {
	color: pink;
}`,
			scopedElements: []scopedElement{
				{classes: []string{"parent"}, scopedClass: "scope111"},
			},
			expected:    ``,
			description: "Descendant selector removed when child is not in component",
		},
		{
			name: "Descendant selector with both parent and child used - should be kept",
			input: `.parent .child {
	color: teal;
}`,
			scopedElements: []scopedElement{
				{classes: []string{"parent"}, scopedClass: "scope111"},
				{classes: []string{"child"}, scopedClass: "scope222"},
			},
			expected:    `.parent.scope111 .child.scope222{color:teal;}`,
			description: "Ruleset kept when both parent and child exist in component",
		},
		{
			name: "Chained classes - only first used - should be removed",
			input: `.container.fake {
	color: cyan;
}`,
			scopedElements: []scopedElement{
				{classes: []string{"container"}, scopedClass: "scope111"},
			},
			expected:    ``,
			description: "Chained class selector removed when not all classes exist on same element",
		},
		{
			name: "Chained classes - both used on same element - should be kept",
			input: `.container.fake {
	color: cyan;
}`,
			scopedElements: []scopedElement{
				{classes: []string{"container", "fake"}, scopedClass: "scope111"},
			},
			expected:    `.container.scope111.fake.scope111{color:cyan;}`,
			description: "Chained class selector kept when all classes exist on same element",
		},
		{
			name: "Tag with class selector",
			input: `div.special {
	color: magenta;
}`,
			scopedElements: []scopedElement{
				{tag: "div", classes: []string{"special"}, scopedClass: "scope333"},
			},
			expected:    `div.scope333.special.scope333{color:magenta;}`,
			description: "Tag with class selector should be kept when both match",
		},
		{
			name: "Multiple declarations kept",
			input: `.used {
	color: red;
	font-size: 14px;
	margin: 10px;
}`,
			scopedElements: []scopedElement{
				{classes: []string{"used"}, scopedClass: "scope123"},
			},
			expected:    `.used.scope123{color:red;font-size:14px;margin:10px;}`,
			description: "Ruleset with multiple declarations should be fully kept",
		},
		{
			name: "Multiple unused rulesets removed",
			input: `.unused1 { color: red; }
.unused2 { color: blue; }
.used { color: green; }`,
			scopedElements: []scopedElement{
				{classes: []string{"used"}, scopedClass: "scope123"},
			},
			expected:    `.used.scope123{color:green;}`,
			description: "Multiple unused rulesets should be removed, used one kept",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := scopeCSS(tt.input, tt.scopedElements)
			// Normalize whitespace for comparison
			result = strings.ReplaceAll(result, "\n", "")
			result = strings.ReplaceAll(result, "\t", "")
			result = strings.ReplaceAll(result, " ", "")

			expected := strings.ReplaceAll(tt.expected, "\n", "")
			expected = strings.ReplaceAll(expected, "\t", "")
			expected = strings.ReplaceAll(expected, " ", "")

			if result != expected {
				t.Errorf("%s\nExpected: %s\nGot:      %s\nDescription: %s",
					tt.name, expected, result, tt.description)
			}
		})
	}
}

func TestCSSTreeshakingEdgeCases(t *testing.T) {
	tests := []struct {
		name             string
		input            string
		scopedElements   []scopedElement
		shouldContain    string
		shouldNotContain string
		description      string
	}{
		{
			name: "Pseudo-selector should not affect matching",
			input: `.used:hover {
	color: red;
}`,
			scopedElements: []scopedElement{
				{classes: []string{"used"}, scopedClass: "scope123"},
			},
			shouldContain:    `.used.scope123:hover`,
			shouldNotContain: "",
			description:      "Pseudo-selectors should be handled correctly",
		},
		{
			name: "Child combinator - only child used - should be removed",
			input: `.parent > .child {
	color: red;
}`,
			scopedElements: []scopedElement{
				{classes: []string{"child"}, scopedClass: "scope222"},
			},
			shouldContain:    ``,
			shouldNotContain: ".child",
			description:      "Child combinator removed when parent not in component",
		},
		{
			name: "Child combinator - both used - should be kept",
			input: `.parent > .child {
	color: red;
}`,
			scopedElements: []scopedElement{
				{classes: []string{"parent"}, scopedClass: "scope111"},
				{classes: []string{"child"}, scopedClass: "scope222"},
			},
			shouldContain:    `.parent.scope111>.child.scope222`,
			shouldNotContain: "",
			description:      "Child combinator kept when both parent and child exist",
		},
		{
			name: "Adjacent sibling - only second used - should be removed",
			input: `.first + .second {
	color: red;
}`,
			scopedElements: []scopedElement{
				{classes: []string{"second"}, scopedClass: "scope222"},
			},
			shouldContain:    ``,
			shouldNotContain: ".second",
			description:      "Adjacent sibling removed when first element not in component",
		},
		{
			name: "General sibling - only second used - should be removed",
			input: `.first ~ .second {
	color: red;
}`,
			scopedElements: []scopedElement{
				{classes: []string{"second"}, scopedClass: "scope222"},
			},
			shouldContain:    ``,
			shouldNotContain: ".second",
			description:      "General sibling removed when first element not in component",
		},
		{
			name: "At-rule should be preserved",
			input: `@media screen {
	.used { color: red; }
}`,
			scopedElements: []scopedElement{
				{classes: []string{"used"}, scopedClass: "scope123"},
			},
			shouldContain:    `@media screen`,
			shouldNotContain: "",
			description:      "At-rules should be preserved",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := scopeCSS(tt.input, tt.scopedElements)

			if tt.shouldContain != "" && !strings.Contains(result, tt.shouldContain) {
				t.Errorf("%s\nExpected to contain: %s\nGot: %s\nDescription: %s",
					tt.name, tt.shouldContain, result, tt.description)
			}

			if tt.shouldNotContain != "" && strings.Contains(result, tt.shouldNotContain) {
				t.Errorf("%s\nExpected NOT to contain: %s\nGot: %s\nDescription: %s",
					tt.name, tt.shouldNotContain, result, tt.description)
			}
		})
	}
}

// ============ Global CSS Feature Tests ============

func TestGlobalCSSSelectors(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		scopedElements []scopedElement
		expected       string
		description    string
	}{
		{
			name: "Global tag selector with *p",
			input: `*p {
	color: red;
}`,
			scopedElements: []scopedElement{
				{tag: "p", scopedClass: "scope123"},
			},
			expected:    `p{color:red;}`,
			description: "*p should NOT be scoped (global)",
		},
		{
			name: "Global class selector with *.myclass",
			input: `*.myclass {
	color: blue;
}`,
			scopedElements: []scopedElement{
				{classes: []string{"myclass"}, scopedClass: "scope456"},
			},
			expected:    `.myclass{color:blue;}`,
			description: "*.myclass should NOT be scoped (global)",
		},
		{
			name: "Global id selector with *#myid",
			input: `*#myid {
	color: green;
}`,
			scopedElements: []scopedElement{
				{id: "myid", scopedClass: "scope789"},
			},
			expected:    `#myid{color:green;}`,
			description: "*#myid should NOT be scoped (global)",
		},
		{
			name: "Regular tag selector without *",
			input: `p {
	color: red;
}`,
			scopedElements: []scopedElement{
				{tag: "p", scopedClass: "scope123"},
			},
			expected:    `p.scope123{color:red;}`,
			description: "p should be scoped normally",
		},
		{
			name: "Regular class selector without *",
			input: `.myclass {
	color: blue;
}`,
			scopedElements: []scopedElement{
				{classes: []string{"myclass"}, scopedClass: "scope456"},
			},
			expected:    `.myclass.scope456{color:blue;}`,
			description: ".myclass should be scoped normally",
		},
		{
			name: "Universal selector with space - div * p",
			input: `div * p {
	color: orange;
}`,
			scopedElements: []scopedElement{
				{tag: "div", scopedClass: "scope111"},
				{tag: "p", scopedClass: "scope222"},
			},
			expected:    `div.scope111 * p.scope222{color:orange;}`,
			description: "* with spaces should remain as universal selector, div and p should be scoped",
		},
		{
			name: "Mixed - scoped parent with global child",
			input: `div *p {
	color: purple;
}`,
			scopedElements: []scopedElement{
				{tag: "div", scopedClass: "scope111"},
				{tag: "p", scopedClass: "scope222"},
			},
			expected:    `div.scope111 p{color:purple;}`,
			description: "div should be scoped, *p should be global (not scoped)",
		},
		{
			name: "Multiple global selectors",
			input: `*p, *div {
	margin: 0;
}`,
			scopedElements: []scopedElement{
				{tag: "p", scopedClass: "scope123"},
				{tag: "div", scopedClass: "scope456"},
			},
			expected:    `p, div{margin:0;}`,
			description: "Both *p and *div should be global (not scoped)",
		},
		{
			name: "Global class in descendant selector",
			input: `div *.global-class {
	padding: 10px;
}`,
			scopedElements: []scopedElement{
				{tag: "div", scopedClass: "scope111"},
				{classes: []string{"global-class"}, scopedClass: "scope222"},
			},
			expected:    `div.scope111 .global-class{padding:10px;}`,
			description: "div should be scoped, *.global-class should be global",
		},
		{
			name: "Regular universal selector alone",
			input: `* {
	box-sizing: border-box;
}`,
			scopedElements: []scopedElement{},
			expected:       `*{box-sizing:border-box;}`,
			description:    "Standalone * should remain as universal selector",
		},
		{
			name: "Complex selector with global element",
			input: `div.container *p.paragraph {
	font-size: 16px;
}`,
			scopedElements: []scopedElement{
				{tag: "div", classes: []string{"container"}, scopedClass: "scope111"},
				{tag: "p", classes: []string{"paragraph"}, scopedClass: "scope222"},
			},
			expected:    `div.scope111.container.scope111 p.paragraph.scope222{font-size:16px;}`,
			description: "div.container should be scoped, *p should be global but .paragraph should still be scoped",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := scopeCSS(tt.input, tt.scopedElements)
			// Normalize whitespace for comparison
			result = strings.ReplaceAll(result, "\n", "")
			result = strings.ReplaceAll(result, "\t", "")
			result = strings.ReplaceAll(result, " ", "")

			expected := strings.ReplaceAll(tt.expected, "\n", "")
			expected = strings.ReplaceAll(expected, "\t", "")
			expected = strings.ReplaceAll(expected, " ", "")

			if result != expected {
				t.Errorf("%s\nExpected: %s\nGot:      %s\nDescription: %s",
					tt.name, expected, result, tt.description)
			}
		})
	}
}

func TestGlobalCSSEdgeCases(t *testing.T) {
	tests := []struct {
		name             string
		input            string
		scopedElements   []scopedElement
		shouldContain    string
		shouldNotContain string
		description      string
	}{
		{
			name: "Asterisk in attribute selector should be preserved",
			input: `[class*="test"] {
	color: red;
}`,
			scopedElements:   []scopedElement{},
			shouldContain:    `[class*="test"]`,
			shouldNotContain: "",
			description:      "* in attribute selector should not be treated as global marker",
		},
		{
			name: "Multiple spaces around * should preserve universal selector",
			input: `div  *  p {
	color: blue;
}`,
			scopedElements: []scopedElement{
				{tag: "div", scopedClass: "scope111"},
				{tag: "p", scopedClass: "scope222"},
			},
			shouldContain:    "*",
			shouldNotContain: "",
			description:      "* with spaces on both sides should remain as universal selector",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := scopeCSS(tt.input, tt.scopedElements)

			if tt.shouldContain != "" && !strings.Contains(result, tt.shouldContain) {
				t.Errorf("%s\nExpected to contain: %s\nGot: %s\nDescription: %s",
					tt.name, tt.shouldContain, result, tt.description)
			}

			if tt.shouldNotContain != "" && strings.Contains(result, tt.shouldNotContain) {
				t.Errorf("%s\nExpected NOT to contain: %s\nGot: %s\nDescription: %s",
					tt.name, tt.shouldNotContain, result, tt.description)
			}
		})
	}
}
