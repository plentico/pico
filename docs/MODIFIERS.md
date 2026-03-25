# Pico Modifiers

Pico supports modifiers for both expressions and if-statements. Modifiers allow you to transform values and add conditional behavior.

## Expression Modifiers

Expression modifiers are used within curly braces `{expression | modifier}` to transform the output.

### Available Modifiers

#### `trim(n)` - Trims content to n characters

Truncates the expression result to a maximum length while preserving HTML structure.

```svelte
<span>{longText | trim(100)}</span>
```

#### `html(tag1, tag2, ...)` - Allows specific HTML tags

Sanitizes the output to only allow specified HTML tags.

```svelte
<span>{userContent | html("em", "strong", "a")}</span>
```

#### Chaining Modifiers

Multiple modifiers can be chained using the pipe operator:

```svelte
<span>{content | trim(200) | html("em", "strong")}</span>
```

## If-Statement Modifiers

If-statement modifiers allow you to conditionally apply styles, classes, or attributes based on the if-condition. This is useful for creating accordion expands, collapsible sections, and other conditional UI patterns.

### Available Modifiers

#### `style(trueStyle, falseStyle)` - Conditional inline styles

Applies different inline styles based on whether the condition is true or false.

```svelte
{if showContent | style("max-height: 500px", "max-height: 0")}
  <div class="collapsible">Content here</div>
{/if}
```

**Note:** Multiple CSS properties can be separated by semicolons:
```svelte
{if visible | style("opacity: 1; transform: scale(1)", "opacity: 0; transform: scale(0.9)")}
  <div>Fade and scale animation</div>
{/if}
```

#### `class(trueClass, falseClass)` - Conditional CSS classes

Adds different CSS classes based on the condition.

```svelte
{if isExpanded | class("expanded", "collapsed")}
  <div class="panel">Panel content</div>
{/if}
```

You can use an empty string as an argument to toggle a class on true or false only:

```svelte
{if showAnimals | class("", "collapsed")}
  <div class="animals">Animal list</div>
{/if}
```

Static classes defined on an element will persist, and modified classes will be added / removed:

```svelte
{if expanded | class("expanded", "collapsed")}
  <div class="my-class">My list</div>
{/if}
<!--
  Will be:
    <div class="my-class expanded">My list</div>
  Or:
    <div class="my-class collapsed">My list</div>
-->
```

**Multiple classes** can be specified using spaces:
```svelte
{if active | class("visible expanded", "hidden collapsed")}
  <div>Content</div>
{/if}
```

#### `attr(name, trueValue, falseValue)` - Conditional attributes

Sets different attribute values based on the condition.

```svelte
{if isActive | attr("data-state", "active", "inactive")}
  <button>Toggle</button>
{/if}
```

### Combining Modifiers

Multiple if-statement modifiers can be combined:

```svelte
{if age > 0
  | style("max-height: 100px", "max-height: 0")
  | class("expanded", "collapsed")
  | attr("data-born", "true", "false")
}
  <div>You're born</div>
{/if}
```

### How It Works

When you use if-statement modifiers:

1. **SSR (Server-Side Rendering):** The appropriate value (true or false) is applied to the HTML element's inline style/class/attribute based on the condition evaluation during rendering.

2. **Pattr (Client-Side):** A `p-style`, `p-class`, or `p-attr` attribute is added with a ternary expression. Pattr uses this to dynamically update the element when the condition changes.

Example output:
```html
<div p-class="showAnimals ? '' : 'collapsed'" class="animals collapsed">...</div>
```

### CSS Treeshaking

Classes used in `p-class` modifiers are automatically preserved during CSS treeshaking, even if they don't appear in the initial HTML. This ensures your conditional styles are available when Pattr toggles them client-side.

```css
/* This CSS will be preserved even if .collapsed isn't in the initial HTML */
.collapsed { max-height: 0; }
.expanded { max-height: 500px; }
```
