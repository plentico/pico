package pico

import (
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/dop251/goja"
)

func generateRandom() (string, error) {
	chars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	var bytes = make([]byte, 6)
	for i := range bytes {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			return "", err
		}
		bytes[i] = chars[num.Int64()]
	}
	return string(bytes), nil
}

func templateParts(path string) (string, string, string, string) {
	c, err := os.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}
	template := string(c)
	reFence := regexp.MustCompile(`(?s)---(.*?)---`)
	reScript := regexp.MustCompile(`(?s)<script>(.*?)</script>`)
	reStyle := regexp.MustCompile(`(?s)<style>(.*?)</style>`)
	fences := reFence.FindAllStringSubmatch(template, -1)
	scripts := reScript.FindAllStringSubmatch(template, -1)
	styles := reStyle.FindAllStringSubmatch(template, -1)
	if len(fences) > 1 {
		log.Fatal("Can only have one set of Fences (--- and ---) per template")
	}
	if len(scripts) > 1 {
		log.Fatal("Can only have one set of Script tags (<script></script>) per template")
	}
	if len(styles) > 1 {
		log.Fatal("Can only have one set of Style tags (<style></style>) per template")
	}
	markup := template
	fence := ""
	script := ""
	style := ""
	if len(fences) > 0 {
		wrapped_fence := fences[0][0]
		fence = fences[0][1]
		markup = strings.Replace(markup, wrapped_fence, "", 1)
	}
	if len(scripts) > 0 {
		wrapped_script := scripts[0][0]
		script = scripts[0][1]
		markup = strings.Replace(markup, wrapped_script, "", 1)
	}
	if len(styles) > 0 {
		wrapped_style := styles[0][0]
		style = styles[0][1]
		markup = strings.Replace(markup, wrapped_style, "", 1)
	}
	return markup, fence, script, style
}

func setProps(fence string, props map[string]any) (string, string) {
	pScopeExp := fence

	for name, value := range props {
		reProp := regexp.MustCompile(fmt.Sprintf(`prop (%s)(\s?=\s?(.*?))?;`, name))
		pScopeExp = reProp.ReplaceAllString(pScopeExp, "")
		fence = reProp.ReplaceAllString(fence, "let "+name+" = "+anyToString(value)+";")
	}

	rePropDefaults := regexp.MustCompile(`prop\s([a-zA-Z_$]*)(\s?=\s?(.*?))?;`)
	fence = rePropDefaults.ReplaceAllString(fence, "let $1$2;")
	pScopeExp = rePropDefaults.ReplaceAllString(pScopeExp, "$1$2;")

	// Ensure the fence ends with a semicolon (if it doesn't already)
	// This handles the case where the last statement omits the trailing semicolon
	// This is critical because JS allows omitting semicolons, but when we convert to
	// single-line, we need them to separate statements
	//
	// Apply to both fence (used for JS evaluation) and pScopeExp (used for p-scope attribute)
	fence = strings.TrimSpace(fence)
	if fence != "" && !strings.HasSuffix(fence, ";") {
		fence = fence + ";"
	}
	pScopeExp = strings.TrimSpace(pScopeExp)
	if pScopeExp != "" && !strings.HasSuffix(pScopeExp, ";") {
		pScopeExp = pScopeExp + ";"
	}

	// First, ensure all variable declarations end with semicolons
	// Add semicolons before let/var/const keywords if missing
	reAddSemicolon := regexp.MustCompile(`([^;\s])\s*((?:let|const|var)\s)`)
	fence = reAddSemicolon.ReplaceAllString(fence, "$1;$2")
	pScopeExp = reAddSemicolon.ReplaceAllString(pScopeExp, "$1;$2")

	// Strip let/const/var keywords
	// Handle simple pattern: (let|var|const) varName...
	reLocalVars := regexp.MustCompile(`(?:let|const|var)\s+`)
	pScopeExp = reLocalVars.ReplaceAllString(pScopeExp, "")

	pScopeExp = makeAttrStr(pScopeExp)
	return fence, pScopeExp
}

func makeAttrStr(str string) string {
	reComments := regexp.MustCompile(`//.*`)
	str = reComments.ReplaceAllString(str, "")

	str = strings.TrimSpace(str)
	// Replace line breaks and tabs with empty string, but preserve spacing around operators
	str = strings.ReplaceAll(str, "\t", "")
	str = strings.ReplaceAll(str, "\n", "")
	str = strings.ReplaceAll(str, "'", "\\'")
	str = strings.ReplaceAll(str, "\"", "'")

	return str
}

func evalAllBrackets(str string, fence string) string {
	for {
		startPos := strings.IndexRune(str, '{')
		endPos := strings.IndexRune(str, '}')
		if startPos == -1 || endPos == -1 {
			break
		}
		jsCode := str[startPos+1 : endPos]
		evaluated := fmt.Sprintf("%v", evalJS(jsCode, fence))
		str = str[0:startPos] + evaluated + str[endPos+1:]
	}
	return str
}

func evalJS(jsCode string, fence string) any {
	vm := goja.New()
	goja_value, err := vm.RunString(fence + jsCode)
	if err != nil {
		_, fenceErr := vm.RunString(fence)
		if fenceErr != nil {
			log.Printf("Frontmatter/Fence Error: %v", fenceErr)
		}
		// Log error to help diagnose fence/JS issues
		log.Printf("Error evaluating JS expression '%s': %v", jsCode, err)
		return ""
	}
	return goja_value.Export()
}

func isUpper(c byte) bool {
	return c >= 'A' && c <= 'Z'
}

func getComponents(path, fence string) (string, []Component) {
	parentCompDir := filepath.Dir(path)
	components := []Component{}
	reImport := regexp.MustCompile(`import\s+([A-Za-z_][A-Za-z_0-9]*)\s+from\s*"([^"]+)";`)
	for _, line := range strings.Split(fence, "\n") {
		match := reImport.FindStringSubmatch(line)
		if len(match) > 1 {
			compName := match[1]
			compPath := match[2]
			// Resolve component path relative to the current template's directory
			if !filepath.IsAbs(compPath) {
				compPath = filepath.Join(parentCompDir, compPath)
			}
			components = append(components, Component{
				Name: compName,
				Path: compPath,
			})
			fence = reImport.ReplaceAllString(fence, "")
		}
	}
	return fence, components
}

func getCompArgs(comp_decl string) CompProps {
	comp_args := strings.SplitAfter(comp_decl, "}")
	compProps := CompProps{
		Regular: map[string]any{},
		Sync:    map[string]any{},
	}

	for _, comp_arg := range comp_args {
		comp_arg = strings.TrimSpace(comp_arg)

		// Handle {*myVar} syntax for sync props
		if strings.HasPrefix(comp_arg, "{*") && strings.HasSuffix(comp_arg, "}") {
			prop_name := strings.Trim(strings.TrimPrefix(comp_arg, "{"), "*}")
			compProps.Sync[prop_name] = prop_name
			continue
		}

		// Handle {myVar} syntax for regular props
		if strings.HasPrefix(comp_arg, "{") && strings.HasSuffix(comp_arg, "}") {
			prop_name := strings.Trim(comp_arg, "{}")
			compProps.Regular[prop_name] = prop_name
			continue
		}

		// Handle *myVar={value} syntax for sync props
		if strings.HasPrefix(comp_arg, "*") && strings.Contains(comp_arg, "={") && strings.HasSuffix(comp_arg, "}") {
			nameEndPos := strings.IndexRune(comp_arg, '=')
			prop_name := strings.TrimPrefix(comp_arg[0:nameEndPos], "*")

			valueStartPos := strings.IndexRune(comp_arg, '{')
			valueEndPos := strings.IndexRune(comp_arg, '}')

			compProps.Sync[prop_name] = comp_arg[valueStartPos+1 : valueEndPos]
			continue
		}

		// Handle myVar={value} syntax for regular props
		if strings.Contains(comp_arg, "={") && strings.HasSuffix(comp_arg, "}") {
			nameEndPos := strings.IndexRune(comp_arg, '=')
			prop_name := comp_arg[0:nameEndPos]

			valueStartPos := strings.IndexRune(comp_arg, '{')
			valueEndPos := strings.IndexRune(comp_arg, '}')

			compProps.Regular[prop_name] = comp_arg[valueStartPos+1 : valueEndPos]
		}
	}
	return compProps
}

func formatArray(value any) string {
	val := reflect.ValueOf(value)
	var elements []string
	for i := 0; i < val.Len(); i++ {
		elem := val.Index(i).Interface()
		elements = append(elements, anyToString(elem))
	}
	return "[" + strings.Join(elements, ", ") + "]"
}

func formatObject(value any) string {
	val := reflect.ValueOf(value)
	if val.Kind() != reflect.Map {
		return ""
	}

	keys := val.MapKeys()
	keyInterfaces := make([]interface{}, len(keys))
	for i, key := range keys {
		keyInterfaces[i] = key.Interface()
	}

	sort.Slice(keyInterfaces, func(i, j int) bool {
		return fmt.Sprintf("%v", keyInterfaces[i]) < fmt.Sprintf("%v", keyInterfaces[j])
	})

	var pairs []string
	for _, key := range keyInterfaces {
		value := val.MapIndex(reflect.ValueOf(key))
		pairs = append(pairs, fmt.Sprintf("%v: %v", key, anyToString(value.Interface())))
	}

	return "{" + strings.Join(pairs, ", ") + "}"
}

func formatElement(value any) string {
	switch v := value.(type) {
	case string:
		return strconv.Quote(v)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	default:
		return "unknown type"
	}
}

func anyToString(value any) string {
	val := reflect.ValueOf(value)
	switch val.Kind() {
	case reflect.Array, reflect.Slice:
		return formatArray(value)
	case reflect.Map:
		return formatObject(value)
	default:
		return formatElement(value)
	}
}

func flattenCompArgs(m map[string]any) string {
	parts := make([]string, 0, len(m))
	for k, v := range m {
		if k != v {
			parts = append(parts, fmt.Sprintf("%s = %s;", k, v))
		}
	}
	return makeAttrStr(strings.Join(parts, " "))
}

// flattenSyncCompArgs converts sync props to assignments, always including the assignment
// even when k == v (e.g., count = count) as this is required for 2-way binding
func flattenSyncCompArgs(m map[string]any) string {
	parts := make([]string, 0, len(m))
	for k, v := range m {
		parts = append(parts, fmt.Sprintf("%s = %s;", k, v))
	}
	return makeAttrStr(strings.Join(parts, " "))
}

func isBoolAndTrue(value any) bool {
	if b, ok := value.(bool); ok && b {
		return true
	}
	return false
}

func processEventHandler(attrVal string) string {
	expr := strings.TrimSpace(attrVal)
	if strings.HasPrefix(expr, "{") && strings.HasSuffix(expr, "}") {
		return expr[1 : len(expr)-1]
	}
	re := regexp.MustCompile(`\{(.*?)\}`)
	return re.ReplaceAllString(expr, "${1}")
}
