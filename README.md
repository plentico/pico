# Pico

Pico is a pure-Go, component-based templating system. It features scoped CSS, JavaScript expressions, and control flow logic.

## Features

- **Component-Based Architecture**: Build reusable HTML components with props
- **Scoped CSS**: Automatically scopes styles to components to prevent conflicts
- **JavaScript Expressions**: Use JS expressions directly in templates via Goja runtime
- **Control Flow**: `{if}`, `{else if}`, `{else}`, and `{for}` loop constructs
- **Component Composition**: Import and nest components with props
- **Dynamic Components**: Render components dynamically using paths
- **Built-in CMS (Temporary)**: Simple editor for testing purposes only (will be removed)

## Installation

```bash
go mod download
go build -o pico
```

## Usage

```bash
go build
./pico render example/views/home.html example/props.json
./pico serve
```

Then visit `http://localhost:3000` in your browser.

## WIP Project Structure (for manual testing)

```
.
├── main.go              # Application entry point and rendering engine
├── go.mod               # Go module dependencies
├── views/               # Component templates
│   ├── home.html        # Root page component
│   ├── head.html        # HTML head component
│   ├── age.html         # Example component with conditionals
│   ├── age_button.html  # Button component
│   ├── todos.html       # Component fetching external data
│   ├── mycomp.html      # Simple component example
│   ├── double.html      # Helper component
│   ├── cms.js           # CMS interface JavaScript
│   └── cms.css          # CMS interface styles
└── public/              # Generated static output (auto-created)
```

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
let compPath = "./views/mycomp.html";
---

<="./views/mycomp.html" {prop} />
<='{compPath}' />
```

## CMS Interface (temporary only)

For now, Pico includes a simple built-in CMS for content editing. Eventually this will be removed and more robust CMS capabilities will be provided by the [Plenti](https://github.com/plentico/plenti) project.

The CMS:

- Automatically generates input fields from component props
- Allows real-time editing of data
- Can be toggled via a button

To enable the CMS, include the CMS scripts and container in your component:

```html
<head>
  <script defer src="/cms.js"></script>
  <link rel="stylesheet" href="/cms.css">
</head>
<body>
  <div id="plenti_cms"></div>
  <button id="toggle_plenti_cms">Toggle CMS</button>
</body>
```

## Dependencies

- [goja](https://github.com/dop251/goja) - JavaScript runtime in Go
- [parse](https://github.com/tdewolff/parse) - CSS/JS parsers
- [golang.org/x/net](https://golang.org/x/net) - HTML parser

## Building

The application compiles components to static files in the `public/` directory:

```
public/
├── index.html    # Rendered HTML
├── style.css     # Scoped styles
├── script.js     # Component scripts
├── cms.js        # CMS interface (copied)
└── cms.css       # CMS styles (copied)
```

## License

MIT License

## Related

- [Plenti](https://github.com/plentico/plenti) - An SSG/CMS that currently uses Svelte templates, to be replaced by Pico templates once project reaches maturity
- [Pattr](https://github.com/plentico/pattr) - An attribute-driven JS library that provides client-side reactivity
