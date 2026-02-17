// Pico CLI - A template rendering engine with reactive UI support
package main

import (
	"flag"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/plentico/pico/pkg/pico"
)

func main() {
	// Define subcommands
	renderCmd := flag.NewFlagSet("render", flag.ExitOnError)
	serveCmd := flag.NewFlagSet("serve", flag.ExitOnError)

	// Render command flags
	propsFile := renderCmd.String("props", "", "Path to JSON file containing props")
	propsJSON := renderCmd.String("props-json", "", "JSON string containing props")
	outputDir := renderCmd.String("output", "./public", "Output directory for rendered files")
	staticDir := renderCmd.String("static", "", "Static files directory to copy (default: auto-detect ./static relative to template)")
	noPattr := renderCmd.Bool("no-pattr", false, "Disable Pattr hydration attributes")

	// Serve command flags
	serveDir := serveCmd.String("dir", "", "Directory to serve (default: ../pico-tests/public or ./public)")
	port := serveCmd.String("port", "3000", "Port to serve on")

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "render":
		renderCmd.Parse(os.Args[2:])
		args := renderCmd.Args()
		if len(args) < 1 {
			fmt.Println("Error: template path required")
			fmt.Println("Usage: pico render [options] <template>")
			os.Exit(1)
		}
		templatePath := args[0]
		// Allow props file as second positional argument if -props flag not used
		propsPath := *propsFile
		if propsPath == "" && len(args) >= 2 {
			propsPath = args[1]
		}
		runRender(templatePath, propsPath, *propsJSON, *outputDir, *staticDir, *noPattr)

	case "serve":
		serveCmd.Parse(os.Args[2:])
		dir := *serveDir
		if dir == "" {
			// Default to pico-tests/public if it exists, otherwise ./public
			if _, err := os.Stat("../pico-tests/public"); err == nil {
				dir = "../pico-tests/public"
			} else {
				dir = "./public"
			}
		}
		runServe(dir, *port)

	case "test":
		testDir := "../pico-tests"
		if len(os.Args) >= 3 {
			testDir = os.Args[2]
		}
		runTests(testDir)

	case "version":
		fmt.Println("pico version 0.1.0")

	case "help", "-h", "--help":
		printUsage()

	default:
		// If no subcommand, treat first arg as template path for quick render
		if len(os.Args) >= 2 && !startsWithDash(os.Args[1]) {
			templatePath := os.Args[1]
			propsPath := ""
			if len(os.Args) >= 3 && !startsWithDash(os.Args[2]) {
				propsPath = os.Args[2]
			}
			runRender(templatePath, propsPath, "", "./public", "", false)
		} else {
			fmt.Printf("Unknown command: %s\n", os.Args[1])
			printUsage()
			os.Exit(1)
		}
	}
}

func startsWithDash(s string) bool {
	return len(s) > 0 && s[0] == '-'
}

func printUsage() {
	fmt.Println(`Pico - A template rendering engine with reactive UI support

Usage:
  pico <command> [options]

Commands:
  render <template> [props.json]  Render a template to HTML/CSS/JS
  serve                           Start a local development server
  test [dir]                      Run e2e tests from pico-tests repo
  version                         Print version information
  help                            Show this help message

Quick Usage:
  pico <template> [props.json]    Shorthand for 'pico render'

Test Site (download from https://github.com/plentico/pico-tests):
  git clone https://github.com/plentico/pico-tests ../pico-tests

Render Options:
  -props <file>       Path to JSON file containing props
  -props-json <json>  JSON string containing props  
  -output <dir>       Output directory (default: ./public)
  -static <dir>       Static files directory to copy (auto-detects ./static)
  -no-pattr           Disable Pattr hydration attributes

Serve Options:
  -dir <dir>          Directory to serve (default: ../pico-tests/public)
  -port <port>        Port to serve on (default: 3000)

Test Options:
  [dir]               Path to pico-tests repo (default: ../pico-tests)

Examples:
  pico render ../pico-tests/site/views/home.html ../pico-tests/site/props.json
  pico render -output ../pico-tests/public ../pico-tests/site/views/home.html ../pico-tests/site/props.json
  pico serve                      # serves ../pico-tests/public
  pico serve -port 8080
  pico test                       # runs e2e tests from ../pico-tests

Library Usage (Go):
  import "github.com/plentico/pico/pkg/pico"
  
  markup, script, style := pico.RenderRoot("template.html", props)
  markup, script, style, _ := pico.RenderRootFromJSON("template.html", "props.json")
`)
}

func runRender(templatePath, propsFile, propsJSON, outputDir, staticDir string, noPattr bool) {
	var markup, script, style string
	var err error

	if propsJSON != "" {
		markup, script, style, err = pico.RenderRootFromJSONString(templatePath, propsJSON, noPattr)
		if err != nil {
			fmt.Printf("Error rendering template: %v\n", err)
			os.Exit(1)
		}
	} else if propsFile != "" {
		markup, script, style, err = pico.RenderRootFromJSON(templatePath, propsFile, noPattr)
		if err != nil {
			fmt.Printf("Error rendering template: %v\n", err)
			os.Exit(1)
		}
	} else {
		// Empty props
		markup, script, style = pico.RenderRoot(templatePath, map[string]any{}, noPattr)
	}

	// Create output directory
	if err := os.MkdirAll(outputDir, os.ModePerm); err != nil {
		fmt.Printf("Error creating output directory: %v\n", err)
		os.Exit(1)
	}

	// Write output files
	if err := os.WriteFile(filepath.Join(outputDir, "index.html"), []byte(markup), fs.ModePerm); err != nil {
		fmt.Printf("Error writing index.html: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(filepath.Join(outputDir, "script.js"), []byte(script), fs.ModePerm); err != nil {
		fmt.Printf("Error writing script.js: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(filepath.Join(outputDir, "style.css"), []byte(style), fs.ModePerm); err != nil {
		fmt.Printf("Error writing style.css: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Rendered to %s/\n", outputDir)
	fmt.Println("  - index.html")
	fmt.Println("  - script.js")
	fmt.Println("  - style.css")

	// Copy static files
	if staticDir == "" {
		// Auto-detect: look for static folder relative to template's parent directory
		templateParent := filepath.Dir(filepath.Dir(templatePath))
		if templateParent == "." {
			templateParent = filepath.Dir(templatePath)
		}
		potentialStatic := filepath.Join(templateParent, "static")
		if info, err := os.Stat(potentialStatic); err == nil && info.IsDir() {
			staticDir = potentialStatic
		}
	}

	if staticDir != "" {
		if info, err := os.Stat(staticDir); err == nil && info.IsDir() {
			if err := copyDir(staticDir, outputDir); err != nil {
				fmt.Printf("Warning: could not copy static files: %v\n", err)
			} else {
				fmt.Printf("  - static files from %s/\n", staticDir)
			}
		}
	}
}

// copyDir recursively copies a directory tree
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path from source
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		// Skip the root directory itself
		if relPath == "." {
			return nil
		}

		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		// Copy file
		srcFile, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dstPath, srcFile, info.Mode())
	})
}

func runTests(testDir string) {
	// Check if test directory exists
	if _, err := os.Stat(testDir); os.IsNotExist(err) {
		fmt.Printf("Test directory not found: %s\n", testDir)
		fmt.Println("Clone the pico-tests repo:")
		fmt.Println("  git clone https://github.com/plentico/pico-tests ../pico-tests")
		os.Exit(1)
	}

	fmt.Printf("Running tests from %s...\n", testDir)

	// Run go test in the e2e directory
	cmd := exec.Command("go", "test", "./e2e/...", "-v")
	cmd.Dir = testDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		fmt.Printf("Error running tests: %v\n", err)
		os.Exit(1)
	}
}

func runServe(dir, port string) {
	fmt.Printf("Starting server at http://localhost:%s\n", port)
	fmt.Printf("Serving files from: %s\n", dir)
	fmt.Println("Press Ctrl+C to stop")

	http.Handle("/", http.FileServer(http.Dir(dir)))
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		fmt.Printf("Server error: %v\n", err)
		os.Exit(1)
	}
}
