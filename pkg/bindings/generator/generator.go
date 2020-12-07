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

Created {{.Date}}
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
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	s := reflect.ValueOf(o)
	if reflect.Ptr == s.Kind() {
		s = s.Elem()
	}
	sType := s.Type()
	for i := 0; i < s.NumField(); i++ {
		fieldName := sType.Field(i).Name
		if !o.Changed(fieldName) {
			continue
		}
		f := s.Field(i)
		if reflect.Ptr == f.Kind() {
			f = f.Elem()
		}
		switch f.Kind() {
		case reflect.Bool:
			params.Set(fieldName, strconv.FormatBool(f.Bool()))
		case reflect.String:
			params.Set(fieldName, f.String())
		case reflect.Int, reflect.Int64:
			// f.Int() is always an int64
			params.Set(fieldName, strconv.FormatInt(f.Int(), 10))
		case reflect.Slice:
			typ := reflect.TypeOf(f.Interface()).Elem()
			slice := reflect.MakeSlice(reflect.SliceOf(typ), f.Len(), f.Cap())
			switch typ.Kind() {
			case reflect.String:
				s, ok := slice.Interface().([]string)
				if !ok {
					return nil, errors.New("failed to convert to string slice")
				}
				for _, val := range s {
					params.Add(fieldName, val)
				}
			default:
				return nil, errors.Errorf("unknown slice type %s", f.Kind().String())
			}
		case reflect.Map:
			lowerCaseKeys := make(map[string][]string)
			// I dont know if this code is needed anymore, TBD
			// for k, v := range filters {
			// 	lowerCaseKeys[strings.ToLower(k)] = v
			// }
			s, err := json.MarshalToString(lowerCaseKeys)
			if err != nil {
				return nil, err
			}

			params.Set(fieldName, s)
		default:
			return nil, errors.Errorf("unknown type %s", f.Kind().String())
		}
	}
	return params, nil
}
`

var fieldTmpl = `
// With{{.Name}}
func(o *{{.StructName}}) With{{.Name}}(value {{.Type}}) *{{.StructName}} {
	v := &value
	o.{{.Name}} = v
	return o
}
`

type fieldStruct struct {
	Name       string
	StructName string
	Type       string
}

func main() {
	var (
		closed       bool
		fieldStructs []fieldStruct
		structNode   ast.Node
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
	imports := []string{"\"reflect\""}
	for _, imp := range f.Imports {
		imports = append(imports, imp.Path.Value)
	}

	out, err := os.Create(strings.ToLower(inputStructName) + "_" + srcFile)
	if err != nil {
		panic(err)
	}
	defer func() {
		if !closed {
			out.Close()
		}
	}()
	bodyStruct := struct {
		PackageName string
		Imports     []string
		Date        string
		StructName  string
	}{
		PackageName: pkg,
		Imports:     imports,
		Date:        time.Now().String(),
		StructName:  inputStructName,
	}

	body := template.Must(template.New("body").Parse(bodyTmpl))
	fields := template.Must(template.New("fields").Parse(fieldTmpl))
	ast.Inspect(f, func(n ast.Node) bool {
		ref, refOK := n.(*ast.TypeSpec)
		if refOK {
			if ref.Name.Name == inputStructName {
				structNode = n
				x := ref.Type.(*ast.StructType)
				for _, field := range x.Fields.List {
					var (
						name string
					)
					typeExpr := field.Type
					start := typeExpr.Pos() - 1
					end := typeExpr.End() - 1
					fieldType := strings.Replace(string(b[start:end]), "*", "", 1)
					if len(field.Names) > 0 {
						name = field.Names[0].Name
						if len(name) < 1 {
							panic(errors.New("bad name"))
						}
					}
					fStruct := fieldStruct{
						Name:       name,
						StructName: inputStructName,
						Type:       fieldType,
					}
					fieldStructs = append(fieldStructs, fStruct)
				} // for

				// create the body
				if err := body.Execute(out, bodyStruct); err != nil {
					fmt.Println(err)
					os.Exit(1)
				}

				// create with func from the struct fields
				for _, fs := range fieldStructs {
					if err := fields.Execute(out, fs); err != nil {
						fmt.Println(err)
						os.Exit(1)
					}
				}

				// close out file
				if err := out.Close(); err != nil {
					fmt.Println(err)
					os.Exit(1)
				}
				closed = true

				// go fmt file
				gofmt := exec.Command("gofmt", "-w", "-s", out.Name())
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
