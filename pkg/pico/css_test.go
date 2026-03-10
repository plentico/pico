package pico

import (
	"strings"
	"testing"
)

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
