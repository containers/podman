package main

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"log"
	"os"
)

func IsBasicGoType(t types.Type, flag types.BasicInfo) bool {
	switch u := t.(type) {
	case *types.Basic:
		if u.Info()&flag != 0 {
			return true
		}
		return false
	case *types.Named:
		return IsBasicGoType(u.Underlying(), flag)
	}
	return false
}

func GoToVarlinkType(t types.Type) string {
	if IsBasicGoType(t, types.IsBoolean) {
		return "bool"
	}

	if IsBasicGoType(t, types.IsInteger) {
		return "int"
	}

	if IsBasicGoType(t, types.IsFloat) {
		return "float"
	}

	if IsBasicGoType(t, types.IsString) {
		return "string"
	}

	switch u := t.(type) {
	case *types.Basic:
		return fmt.Sprintf("<<<%s>>>", t.String())

	case *types.Named:
		return u.Obj().Name()

	case *types.Map:
		if IsBasicGoType(u.Key(), types.IsString) {
			return fmt.Sprintf("[string]%s", GoToVarlinkType(u.Elem()))
		} else {
			return fmt.Sprintf("<<<%s>>>", u.String())
		}

	case *types.Interface:
		if u.Empty() {
			return "()"
		}
		return fmt.Sprintf("<<<%s>>>", u.String())

	case *types.Pointer:
		return fmt.Sprintf("?%s", GoToVarlinkType(u.Elem()))

	case *types.Array:
		return fmt.Sprintf("[]%s", GoToVarlinkType(u.Elem()))

	case *types.Slice:
		return fmt.Sprintf("[]%s", GoToVarlinkType(u.Elem()))

	case *types.Struct:
		if u.NumFields() > 0 {
			s := ""
			for i := 0; i < u.NumFields(); i++ {
				if i > 0 {
					s += ",\n"
				}
				s += fmt.Sprintf("\t%s: %s",
					u.Field(i).Name(), GoToVarlinkType(u.Field(i).Type()))
			}

			return fmt.Sprintf("(\n%s\n)", s)
		}
		return "()"

	default:
		return fmt.Sprintf("<<<%T %s>>>", t, u)
	}
}

func PrintDefsUses(name string, fset *token.FileSet, files []*ast.File) error {
	conf := types.Config{
		Importer:    importer.Default(),
		FakeImportC: true,
	}

	info := &types.Info{
		Defs: make(map[*ast.Ident]types.Object),
	}

	_, err := conf.Check(name, fset, files, info)
	if err != nil {
		return err // type error
	}

	seen := map[string]interface{}{}

	for id, obj := range info.Defs {
		if obj == nil {
			continue
		}

		if _, ok := seen[id.Name]; ok {
			continue
		}

		/*
			if !obj.Exported() || obj.Pkg().Name() != name {
				continue
			}
		*/
		switch f := obj.Type().Underlying().(type) {
		case *types.Struct:
			if f.NumFields() > 0 {
				fmt.Printf("type %s %s\n\n", id.Name, GoToVarlinkType(f))
			}
		}
		seen[id.Name] = nil
	}

	return nil
}

func main() {

	path := os.Args[1]
	fs := token.NewFileSet()

	if stat, err := os.Stat(path); err == nil && stat.IsDir() {
		pkgs, err := parser.ParseDir(fs, path, nil, 0)
		if err != nil {
			fmt.Printf("parsing dir '%s': %s", path, err)
		}
		for name, pkg := range pkgs {
			log.Println("Found package:", name)

			fset := make([]*ast.File, len(pkg.Files), len(pkg.Files))
			idx := 0
			for _, value := range pkg.Files {
				fset[idx] = value
				idx++
			}

			if err := PrintDefsUses(name, fs, fset); err != nil {
				log.Print(err) // type error
			}
		}
	} else {

		fset, err := parser.ParseFile(fs, path, nil, 0)

		if err != nil {
			fmt.Printf("parsing file '%s': %s", path, err)
		}
		name := fset.Name.String()
		if err := PrintDefsUses(name, fs, []*ast.File{fset}); err != nil {
			log.Print(err) // type error
		}
	}
}
