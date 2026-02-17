# Pico

Pico is a pure-Go, component-based templating system. It features scoped CSS, JavaScript expressions, and control flow logic.

## Features

- **Component-Based Architecture**: Build reusable HTML components with props
- **Scoped CSS**: Automatically scopes styles to components to prevent conflicts
- **JavaScript Expressions**: Use JS expressions directly in templates via Goja runtime
- **Control Flow**: `{if}`, `{else if}`, `{else}`, and `{for}` loop constructs
- **Component Composition**: Import and nest components with props
- **Dynamic Components**: Render components dynamically using paths
- **Pattr Hydration**: Outputs attributes for client-side reactivity with [Pattr](https://github.com/plentico/pattr)

## Installation

```bash
go build
```

## Quick Start

```bash
# Clone the test site for examples and e2e tests
git clone https://github.com/plentico/pico-tests ../pico-tests

# Build pico
go build

# Render the test site
./pico render -output ../pico-tests/public ../pico-tests/site/views/home.html ../pico-tests/site/props.json

# Serve it
./pico serve

# Run e2e tests
./pico test
```

Then visit `http://localhost:3000` in your browser.

## CLI Commands

```
pico <command> [options]

Commands:
  render <template> [props.json]  Render a template to HTML/CSS/JS
  serve                           Start a local development server
  test [dir]                      Run e2e tests from pico-tests repo
  version                         Print version information
  help                            Show this help message

Render Options:
  -props <file>       Path to JSON file containing props
  -props-json <json>  JSON string containing props  
  -output <dir>       Output directory (default: ./public)
  -static <dir>       Static files directory to copy (auto-detects ./static)
  -no-pattr           Disable Pattr hydration attributes

Serve Options:
  -dir <dir>          Directory to serve (default: ../pico-tests/public)
  -port <port>        Port to serve on (default: 3000)

Examples:
  pico render ../pico-tests/site/views/home.html ../pico-tests/site/props.json
  pico render -output ../pico-tests/public ../pico-tests/site/views/home.html ../pico-tests/site/props.json
  pico serve                      # serves ../pico-tests/public
  pico serve -port 8080
  pico test                       # runs e2e tests from ../pico-tests
```

## Library Usage

Pico can be used as a Go library:

```go
import "github.com/plentico/pico/pkg/pico"

// Render with props map
markup, script, style := pico.RenderRoot("template.html", props)

// Render with JSON file
markup, script, style, err := pico.RenderRootFromJSON("template.html", "props.json")

// Render with JSON string
markup, script, style, err := pico.RenderRootFromJSONString("template.html", `{"name": "World"}`)
```

## Project Structure

```
pico/
├── main.go              # CLI entry point
├── pkg/pico/            # Library package
│   ├── pico.go          # Main exports (RenderRoot, etc.)
│   ├── control.go       # Control flow (if/for/components)
│   ├── html.go          # HTML parsing and scoping
│   ├── css.go           # CSS scoping
│   ├── js.go            # JS scoping
│   └── util.go          # Utilities
├── go.mod
└── README.md
```

For example templates and e2e tests, see [pico-tests](https://github.com/plentico/pico-tests).

## Component Syntax

Components are HTML files with four optional sections:

### 1. Frontmatter (Fence) - `---`

Define imports, props, and local variables:

```html
---
import Child from "./child.html";
import Header from "./header.html";

prop name;           // Required prop
prop age = 25;       // Prop with default value

let greeting = "Hello " + name;  // Local variable
---
```

This is all evaluated "server-side" during the build.

### 2. Template

HTML markup with expressions and control flow:

```html
<Header {title} />

<h1>{greeting}</h1>

{if age > 18}
  <p>Adult</p>
{else if age > 12}
  <p>Teenager</p>
{else}
  <p>Child</p>
{/if}

{for let item of items}
  <div>{item.name}: {item.value}</div>
{/for}

<Child {name} age={age + 5} />
```

Control structures and variables will be evaluated initially during the build, but attributes will be added that allow [Pattr](https://github.com/plentico/pattr) to provide interactivity in the browser.

### 3. Style - `<style>`

Scoped CSS that only applies to this component:

```html
<style>
  h1 {
    color: blue;
  }
  .container {
    padding: 1rem;
  }
</style>
```

### 4. Script - `<script>`

Component-specific JavaScript with scoped element selectors:

```html
<script>
  let btn = document.querySelector("button");
  btn.addEventListener("click", () => {
    console.log("Clicked!");
  });
</script>
```

This is only evaluated "client-side" on the deployed site.

## Template Features

### Props

Props are declared in the frontmatter and can be passed from parent components:

```html
---
prop name;
prop age = 25;  // With default
---

<!-- In parent -->
<Child name="John" age={30} />
<Child name="Jane" />  <!-- Uses default age -->
```

### Expressions

Use curly braces `{}` for JavaScript expressions:

```html
<p>Name: {name}</p>
<p>Next year: {age + 1}</p>
<p>Upper: {name.toUpperCase()}</p>
```

### Conditionals

```html
{if user.isAdmin}
  <AdminPanel />
{else if user.isLoggedIn}
  <UserPanel />
{else}
  <LoginPrompt />
{/if}
```

### Loops

```html
{for let item of items}
  <div class="item-{item.id}">{item.name}</div>
{/for}
```

### Components

Import and use components:

```html
---
import Button from "./button.html";
import Card from "./card.html";
---

<Card title="Welcome">
  <Button onclick="{handleClick}">Click me</Button>
</Card>
```

### Dynamic Components

Render components dynamically by path:

```html
---
let compPath = "./mycomp.html";
---

<="./mycomp.html" {prop} />
<='{compPath}' />
```

## Dependencies

- [goja](https://github.com/dop251/goja) - JavaScript runtime in Go
- [parse](https://github.com/tdewolff/parse) - CSS/JS parsers
- [golang.org/x/net](https://golang.org/x/net) - HTML parser

## Output

The CLI compiles components to static files:

```
public/
├── index.html    # Rendered HTML with Pattr attributes
├── style.css     # Scoped styles
└── script.js     # Component scripts
```

## License

MIT License

## Related

- [pico-tests](https://github.com/plentico/pico-tests) - Example site and e2e tests for Pico
- [Plenti](https://github.com/plentico/plenti) - An SSG/CMS that currently uses Svelte templates, to be replaced by Pico templates once project reaches maturity
- [Pattr](https://github.com/plentico/pattr) - An attribute-driven JS library that provides client-side reactivity
