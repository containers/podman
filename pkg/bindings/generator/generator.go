package main

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"text/template"
	"time"
)

var bodyTmpl = `package {{.PackageName}}

import (
{{range $import := .Imports}}	{{$import}}
{{end}}
)

/*
This file is generated automatically by go generate.  Do not edit.
*/

// Changed
func (o *{{.StructName}}) Changed(fieldName string) bool {
	return util.Changed(o, fieldName)
}

// ToParams
func (o *{{.StructName}}) ToParams() (url.Values, error) {
	return util.ToParams(o)
}

{{range $field := .Fields}}
// With{{.Name}}
func(o *{{$field.StructName}}) With{{$field.Name}}(value {{$field.Type}}) *{{$field.StructName}} {
	v := {{$field.TypedValue}}
	o.{{$field.Name}} = v
	return o
}

// Get{{.Name}}
func(o *{{$field.StructName}}) Get{{$field.Name}}() {{$field.Type}} {
	var {{$field.ZeroName}} {{$field.Type}}
	if o.{{$field.Name}} == nil {
		return {{$field.ZeroName}}
	}
	return {{$field.TypedName}}
}
{{end}}
`

type fieldStruct struct {
	Name       string
	StructName string
	Type       string
	TypedName  string
	TypedValue string
	ZeroName   string
}

func main() {
	var (
		closed       bool
		fieldStructs []fieldStruct
	)
	srcFile := os.Getenv("GOFILE")
	pkg := os.Getenv("GOPACKAGE")
	inputStructName := os.Args[1]
	b, err := ioutil.ReadFile(srcFile)
	if err != nil {
		panic(err)
	}
	fset := token.NewFileSet() // positions are relative to fset
	f, err := parser.ParseFile(fset, "", b, parser.ParseComments)
	if err != nil {
		panic(err)
	}
	// always add reflect
	imports := []string{"\"reflect\"", "\"github.com/containers/podman/v3/pkg/bindings/internal/util\""}
	for _, imp := range f.Imports {
		imports = append(imports, imp.Path.Value)
	}

	out, err := os.Create(strings.TrimRight(srcFile, ".go") + "_" + strings.Replace(strings.ToLower(inputStructName), "options", "_options", 1) + ".go")
	if err != nil {
		panic(err)
	}
	defer func() {
		if !closed {
			out.Close()
		}
	}()

	ast.Inspect(f, func(n ast.Node) bool {
		ref, refOK := n.(*ast.TypeSpec)
		if refOK {
			if ref.Name.Name == inputStructName {
				x := ref.Type.(*ast.StructType)
				for _, field := range x.Fields.List {
					var (
						name, zeroName, typedName, typedValue string
					)
					if len(field.Names) > 0 {
						name = field.Names[0].Name
						if len(name) < 1 {
							panic(errors.New("bad name"))
						}
					}
					for k, v := range name {
						zeroName = strings.ToLower(string(v)) + name[k+1:]
						break
					}
					//sub := "*"
					typeExpr := field.Type
					switch field.Type.(type) {
					case *ast.MapType, *ast.StructType, *ast.ArrayType:
						typedName = "o." + name
						typedValue = "value"
					default:
						typedName = "*o." + name
						typedValue = "&value"
					}
					start := typeExpr.Pos() - 1
					end := typeExpr.End() - 1
					fieldType := strings.Replace(string(b[start:end]), "*", "", 1)
					fStruct := fieldStruct{
						Name:       name,
						StructName: inputStructName,
						Type:       fieldType,
						TypedName:  typedName,
						TypedValue: typedValue,
						ZeroName:   zeroName,
					}
					fieldStructs = append(fieldStructs, fStruct)
				} // for

				bodyStruct := struct {
					PackageName string
					Imports     []string
					Date        string
					StructName  string
					Fields      []fieldStruct
				}{
					PackageName: pkg,
					Imports:     imports,
					Date:        time.Now().String(),
					StructName:  inputStructName,
					Fields:      fieldStructs,
				}

				body := template.Must(template.New("body").Parse(bodyTmpl))

				// create the body
				if err := body.Execute(out, bodyStruct); err != nil {
					fmt.Println(err)
					os.Exit(1)
				}

				// close out file
				if err := out.Close(); err != nil {
					fmt.Println(err)
					os.Exit(1)
				}
				closed = true

				// go fmt file
				gofmt := exec.Command("go", "fmt", out.Name())
				gofmt.Stderr = os.Stdout
				if err := gofmt.Run(); err != nil {
					fmt.Println(err)
					os.Exit(1)
				}

				// go import file
				goimport := exec.Command("goimports", "-w", out.Name())
				goimport.Stderr = os.Stdout
				if err := goimport.Run(); err != nil {
					fmt.Println(err)
					os.Exit(1)
				}
			}
		}
		return true
	})
}
