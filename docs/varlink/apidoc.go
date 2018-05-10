package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strings"

	"github.com/varlink/go/varlink/idl"
)

func readFileToString(path string) (string, error) {
	content, err := ioutil.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func exit(err error) {
	fmt.Println(err.Error())
	os.Exit(1)
}

func typeToString(input *idl.Type) string {
	switch input.Kind {
	case idl.TypeString:
		return "string"
	case idl.TypeBool:
		return "bool"
	case idl.TypeFloat:
		return "float"
	case idl.TypeArray:
		result := input.ElementType.Alias
		if result == "" {
			return fmt.Sprintf("[]%s", typeToString(input.ElementType))
		}
		return result
	case idl.TypeAlias:
		return input.Alias
	case idl.TypeMap:
		return "map[string]"
	case idl.TypeInt:
		return "int"
	}
	return ""
}

func typeToLink(input string) string {
	switch input {
	case "string":
		return "https://godoc.org/builtin#string"
	case "int":
		return "https://godoc.org/builtin#int"
	case "bool":
		return "https://godoc.org/builtin#bool"
	case "float":
		return "https://golang.org/src/builtin/builtin.go#L58"
	default:
		return fmt.Sprintf("#%s", input)
	}
}

type funcArgs struct {
	paramName string
	paramKind string
}
type funcDescriber struct {
	Name         string
	inputParams  []funcArgs
	returnParams []funcArgs
	doc          string
}

type funcSorter []funcDescriber

func (a funcSorter) Len() int           { return len(a) }
func (a funcSorter) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a funcSorter) Less(i, j int) bool { return a[i].Name < a[j].Name }

type typeAttrs struct {
	Name     string
	AttrType string
}
type typeDescriber struct {
	Name  string
	doc   string
	Attrs []typeAttrs
}

type typeSorter []typeDescriber

func (a typeSorter) Len() int           { return len(a) }
func (a typeSorter) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a typeSorter) Less(i, j int) bool { return a[i].Name < a[j].Name }

type err struct {
	Name string
	doc  string
}

type errorSorter []err

func (a errorSorter) Len() int           { return len(a) }
func (a errorSorter) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a errorSorter) Less(i, j int) bool { return a[i].Name < a[j].Name }

// collects defined types in the idl
func getTypes(tidl *idl.IDL) []typeDescriber {
	var types []typeDescriber
	for _, x := range tidl.Aliases {
		i := typeDescriber{
			Name: x.Name,
			doc:  x.Doc,
		}
		ta := []typeAttrs{}
		for _, y := range x.Type.Fields {
			result := typeToString(y.Type)
			ta = append(ta, typeAttrs{Name: y.Name, AttrType: result})
		}
		i.Attrs = ta
		types = append(types, i)
	}
	return types
}

// collects defined methods in the idl
func getMethods(midl *idl.IDL) []funcDescriber {
	var methods []funcDescriber
	for _, t := range midl.Methods {
		m := funcDescriber{
			Name: t.Name,
			doc:  t.Doc,
		}
		fa := []funcArgs{}
		fo := []funcArgs{}

		for _, i := range t.In.Fields {
			fa = append(fa, funcArgs{paramName: i.Name, paramKind: typeToString(i.Type)})

		}
		for _, f := range t.Out.Fields {
			fo = append(fo, funcArgs{paramName: f.Name, paramKind: typeToString(f.Type)})
		}
		m.inputParams = fa
		m.returnParams = fo
		methods = append(methods, m)
	}
	return methods
}

// collects defined errors in the idl
func getErrors(midl *idl.IDL) []err {
	var errors []err
	for _, e := range midl.Errors {
		myError := err{
			Name: e.Name,
			doc:  e.Doc,
		}
		errors = append(errors, myError)
	}
	return errors
}

// generates the index for the top of the markdown page
func generateIndex(methods []funcDescriber, types []typeDescriber, errors []err, b bytes.Buffer) bytes.Buffer {
	// Sort the methods, types, and errors by alphabetical order
	sort.Sort(funcSorter(methods))
	sort.Sort(typeSorter(types))
	sort.Sort(errorSorter(errors))

	for _, method := range methods {
		var inArgs []string
		var outArgs []string
		for _, inArg := range method.inputParams {
			inArgs = append(inArgs, fmt.Sprintf("%s: %s", inArg.paramName, inArg.paramKind))

		}
		for _, outArg := range method.returnParams {
			outArgs = append(outArgs, fmt.Sprintf("%s", outArg.paramKind))

		}
		b.WriteString(fmt.Sprintf("\n[func %s(%s) %s](#%s)\n", method.Name, strings.Join(inArgs, ", "), strings.Join(outArgs, ", "), method.Name))
	}
	for _, t := range types {
		b.WriteString(fmt.Sprintf("[type %s](#%s)\n\n", t.Name, t.Name))
	}
	for _, e := range errors {
		b.WriteString(fmt.Sprintf("[error %s](#%s)\n\n", e.Name, e.Name))
	}
	return b
}

// performs the output for defined methods
func generateFuncDescriptions(methods []funcDescriber, b bytes.Buffer) bytes.Buffer {
	for _, method := range methods {
		b.WriteString(fmt.Sprintf("### <a name=\"%s\"></a>func %s\n", method.Name, method.Name))
		var inArgs []string
		var outArgs []string
		for _, inArg := range method.inputParams {
			inArgs = append(inArgs, fmt.Sprintf("%s: [%s](%s)", inArg.paramName, inArg.paramKind, typeToLink(inArg.paramKind)))
		}
		for _, outArg := range method.returnParams {
			outArgs = append(outArgs, fmt.Sprintf("[%s](%s)", outArg.paramKind, typeToLink(outArg.paramKind)))
		}
		b.WriteString(fmt.Sprintf("<div style=\"background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;\">\n\nmethod %s(%s) %s</div>", method.Name, strings.Join(inArgs, ", "), strings.Join(outArgs, ", ")))
		b.WriteString("\n")
		b.WriteString(method.doc)
		b.WriteString("\n")
	}
	return b
}

// performs the output for defined types/structs
func generateTypeDescriptions(types []typeDescriber, b bytes.Buffer) bytes.Buffer {
	for _, t := range types {
		b.WriteString(fmt.Sprintf("### <a name=\"%s\"></a>type %s\n", t.Name, t.Name))
		b.WriteString(fmt.Sprintf("\n%s\n", t.doc))
		for _, i := range t.Attrs {
			b.WriteString(fmt.Sprintf("\n%s [%s](%s)\n", i.Name, i.AttrType, typeToLink(i.AttrType)))
		}
	}
	return b
}

// performs the output for defined errors
func generateErrorDescriptions(errors []err, b bytes.Buffer) bytes.Buffer {
	for _, e := range errors {
		b.WriteString(fmt.Sprintf("### <a name=\"%s\"></a>type %s\n", e.Name, e.Name))
		b.WriteString(fmt.Sprintf("\n%s\n", e.doc))
	}
	return b
}

func main() {
	args := os.Args
	if len(args) < 2 {
		exit(fmt.Errorf("you must provide an input and output path"))
	}
	varlinkFile := args[1]
	mdFile := args[2]

	varlinkInput, err := readFileToString(varlinkFile)
	if err != nil {
		exit(err)
	}
	varlinkInput = strings.TrimRight(varlinkInput, "\n")

	// Run the idl parser
	midl, err := idl.New(varlinkInput)
	if err != nil {
		exit(err)
	}
	// Collect up the info from the idl
	methods := getMethods(midl)
	types := getTypes(midl)
	errors := getErrors(midl)

	out := bytes.Buffer{}
	out.WriteString(fmt.Sprintf("# %s\n", midl.Name))
	out.WriteString(fmt.Sprintf("%s\n", midl.Doc))
	out.WriteString("## Index\n")
	out = generateIndex(methods, types, errors, out)
	out.WriteString("## Methods\n")
	out = generateFuncDescriptions(methods, out)
	out.WriteString("## Types\n")
	out = generateTypeDescriptions(types, out)
	out.WriteString("## Errors\n")
	out = generateErrorDescriptions(errors, out)
	ioutil.WriteFile(mdFile, out.Bytes(), 0755)
}
