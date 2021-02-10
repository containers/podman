package main

import (
	"errors"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
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
	r := reflect.ValueOf(o)
	value := reflect.Indirect(r).FieldByName(fieldName)
	return !value.IsNil()
}

// ToParams
func (o *{{.StructName}}) ToParams() (url.Values, error) {
	params := url.Values{}

	if o == nil {
		return params, nil
	}

	{{range $field := .Fields}}
	if o.{{$field.Name}} != nil {
		{{$field.Stmt}}
	}
	{{end}}

	return params, nil
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
	Stmt       string
}

func main() {
	var (
		closed       bool
		fieldStructs []fieldStruct
	)
	srcFile := os.Getenv("GOFILE")
	pkg := os.Getenv("GOPACKAGE")
	inputStructName := os.Args[1]

	fmt.Println("srcFile: ", srcFile)
	fmt.Println("pkg: ", pkg)
	fmt.Println("inputStructName: ", inputStructName)

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
	var imports = make([]string, 0, len(f.Imports))
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

	srcImporter := importer.ForCompiler(fset, "source", nil)
	conf := types.Config{Importer: srcImporter, IgnoreFuncBodies: true}
	info := &types.Info{
		Defs:  make(map[*ast.Ident]types.Object),
		Uses:  make(map[*ast.Ident]types.Object),
		Types: make(map[ast.Expr]types.TypeAndValue),
	}
	_, err = conf.Check("", fset, []*ast.File{f}, info)
	if err != nil {
		panic(err)
	}

	typeExpr2TypeName := func(typeExpr ast.Expr) string {
		start := typeExpr.Pos() - 1
		end := typeExpr.End() - 1
		return strings.Replace(string(b[start:end]), "*", "", 1)
	}

	ast.Inspect(f, func(n ast.Node) bool {
		ref, refOK := n.(*ast.TypeSpec)
		if refOK {
			if ref.Name.Name == inputStructName {
				x := ref.Type.(*ast.StructType)
				for _, field := range x.Fields.List {
					var (
						name, zeroName, typedName, typedValue, stmt string
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
					stmt = "panic(\"*** GENERATOR DOESN'T IMPLEMENT THIS YET ***\")"
					//sub := "*"
					typeExpr := field.Type
					switch field.Type.(type) {
					case *ast.MapType, *ast.StructType, *ast.ArrayType:
						typedName = "o." + name
						typedValue = "value"

						switch field.Type.(type) {
						case *ast.ArrayType:

							elt := info.Types[typeExpr.(*ast.ArrayType).Elt]
							typ := elt.Type.Underlying()

							var valExpr string
							switch typ := typ.(type) {
							case *types.Basic:
								switch {
								case typ.Kind() == types.String:
									valExpr = "val"
								case (typ.Info() & types.IsInteger) != 0:
									valExpr = "strconv.FormatInt(int64(val), 10)"
								case (typ.Info() & types.IsUnsigned) != 0:
									valExpr = "strconv.FormatUint(uint64(val), 10)"
								case typ.Kind() == types.Bool:
									valExpr = "strconv.FormatBool(val)"
								}
							default:
							}
							if valExpr != "" {
								fmtStr := `for _, val := range %s { params.Add("%s", %s) }`
								stmt = fmt.Sprintf(fmtStr, typedName, strings.ToLower(name), valExpr)
							}
						case *ast.MapType:
							keyType := info.TypeOf(typeExpr.(*ast.MapType).Key).Underlying()
							valType := info.TypeOf(typeExpr.(*ast.MapType).Value).Underlying()

							// key must be string
							keyTypeOK := false
							if t, ok := keyType.(*types.Basic); ok {
								if t.Kind() == types.String {
									keyTypeOK = true
								}
							}

							// value must be slice of strings
							valTypeOK := false
							if t, ok := valType.(*types.Slice); ok {
								if t, ok := t.Elem().(*types.Basic); ok {
									if t.Kind() == types.String {
										valTypeOK = true
									}
								}
							}

							// to be done -- assert that the map is in fact of type map[string][]string
							fmtstr := `lower := make(map[string][]string, len(%s))
	for key, val := range %s {
		lower[strings.ToLower(key)] = val
	}
	s, err := jsoniter.ConfigCompatibleWithStandardLibrary.MarshalToString(lower)
	if err != nil {
		return nil, err
	}
	params.Set("%s", s)`
							if keyTypeOK && valTypeOK {
								stmt = fmt.Sprintf(fmtstr, typedName, typedName, strings.ToLower(name))
							}

						}

					default:
						typedName = "*o." + name
						typedValue = "&value"

						t := info.Types[typeExpr]

						ptrType, ok := t.Type.(*types.Pointer)
						if !ok {
							panic("must be a pointer type")
						}

						typ := ptrType.Elem().Underlying()

						switch typ := typ.(type) {
						case *types.Basic:
							switch {
							case typ.Kind() == types.String:
								stmt = fmt.Sprintf("params.Set(\"%s\", %s)", strings.ToLower(name), typedName)
							case (typ.Info() & types.IsInteger) != 0:
								stmt = fmt.Sprintf("params.Set(\"%s\", strconv.FormatInt(int64(%s), 10))", strings.ToLower(name), typedName)
							case (typ.Info() & types.IsUnsigned) != 0:
								stmt = fmt.Sprintf("params.Set(\"%s\", strconv.FormatUint(uint64(%s), 10))", strings.ToLower(name), typedName)
							case typ.Kind() == types.Bool:
								stmt = fmt.Sprintf("params.Set(\"%s\", strconv.FormatBool(%s))", strings.ToLower(name), typedName)
							}
						default:
						}
					}

					fieldType := typeExpr2TypeName(typeExpr)
					fStruct := fieldStruct{
						Name:       name,
						StructName: inputStructName,
						Type:       fieldType,
						TypedName:  typedName,
						TypedValue: typedValue,
						ZeroName:   zeroName,
						Stmt:       stmt,
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
