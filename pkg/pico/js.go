package pico

import (
	"strings"

	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/js"
)

type jsVisitor struct {
	scopedElements []scopedElement
}

func (*jsVisitor) Exit(js.INode) {}

func (v *jsVisitor) Enter(node js.INode) js.IVisitor {
	switch node := node.(type) {
	case *js.Var:
		if node.Decl.String() == "LexicalDecl" && !strings.Contains(node.String(), "_plenti_") {
			randomStr, _ := generateRandom()
			node.Data = append(node.Data, []byte("_plenti_"+randomStr)...)
		}
	case *js.BindingElement:
		if expr := node.Default; expr != nil {
			if callExpr, ok := expr.(*js.CallExpr); ok {
				if memberExpr, ok := callExpr.X.(*js.DotExpr); ok {
					objName := string(memberExpr.X.String())
					propName := string(memberExpr.Y.Data)
					if objName == "document" && propName == "querySelector" {
						for i, arg := range callExpr.Args.List {
							argStrOrig := strings.Trim(arg.String(), "\"")
							argStr := argStrOrig
							targetType := "tag"
							if strings.HasPrefix(argStr, ".") {
								argStr = strings.TrimPrefix(argStr, ".")
								targetType = "class"
							}
							if strings.HasPrefix(argStr, "#") {
								argStr = strings.TrimPrefix(argStr, "#")
								targetType = "id"
							}
							scopedClass := getScopedClass(argStr, targetType, v.scopedElements)
							newData := []byte(`"` + argStrOrig + `"`)
							if !strings.Contains(argStrOrig, "p-") {
								newData = []byte(`"` + argStrOrig + "." + scopedClass + `"`)
							}
							callExpr.Args.List[i] = js.Arg{Value: &js.LiteralExpr{
								Data: newData,
							}}
						}
					}
				}
			}
		}
	}
	return v
}

func scopeJS(script string, scopedElements []scopedElement) string {
	ast, _ := js.Parse(parse.NewInputString(script), js.Options{})
	v := jsVisitor{scopedElements: scopedElements}
	js.Walk(&v, ast)
	script = ast.JSString()
	return script
}
