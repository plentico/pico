# Global CSS Feature

This feature allows you to prevent scoping from being added to specific CSS selectors using the `*` prefix.

## Usage

Place an asterisk (`*`) directly before a selector (element, class, or ID) to mark it as global and prevent automatic scoping:

### Examples

#### Global Tag Selector
```css
/* Without global marker - selector gets scoped */
p {
    color: red;
}
/* Becomes: p.scope123 { color: red; } */

/* With global marker - selector stays global */
*p {
    color: red;
}
/* Becomes: p { color: red; } */
```

#### Global Class Selector
```css
/* With global marker */
*.global-class {
    color: blue;
}
/* Becomes: .global-class { color: blue; } */
```

#### Global ID Selector
```css
/* With global marker */
*#myid {
    color: green;
}
/* Becomes: #myid { color: green; } */
```

#### Mixed Scoped and Global Selectors
```css
/* Parent is scoped, child is global */
div *p {
    color: purple;
}
/* Becomes: div.scope111 p { color: purple; } */
```

#### Multiple Global Selectors
```css
*p, *div {
    margin: 0;
}
/* Becomes: p, div { margin: 0; } */
```

## Important Notes

### Universal Selector vs Global Marker

The `*` only functions as a global marker when it directly precedes a selector with **no spaces** in between:

```css
/* This is a GLOBAL marker (no space between * and p) */
*p {
    color: red;
}

/* This is a UNIVERSAL selector (spaces around *) */
div * p {
    color: red;
}
/* The universal selector * is preserved */
```

### Valid Patterns

- `*p` - Global tag selector
- `*.class` - Global class selector  
- `*#id` - Global ID selector
- `div *p` - Scoped div with global p
- `*p, *div` - Multiple global selectors

### Behavior

This feature follows the `:global()` syntax convention used by Svelte and Vue, but provides a shorthand notation. It only applies when `*` directly precedes an element, class, or ID selector without spaces, allowing native CSS usage of the universal selector with spaces.

## Reference

See [GitHub Issue #17](https://github.com/plentico/pico/issues/17) for the original feature request.
