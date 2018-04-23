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

func GoToVarlinkType(t types.Type) string {
	switch u := t.(type) {
	case *types.Basic:
		if u.Info()&types.IsBoolean != 0 {
			return "bool"
		}
		if u.Info()&types.IsInteger != 0 {
			return "int"
		}
		if u.Info()&types.IsFloat != 0 {
			return "float"
		}
		if u.Info()&types.IsString != 0 {
			return "string"
		}
		return fmt.Sprintf("<<<%s>>>", t.String())

	case *types.Named:
		return u.Obj().Name()

	case *types.Map:
		return fmt.Sprintf("<<<%s>>>", u.String())

	case *types.Interface:
		return fmt.Sprintf("<<<%s>>>", u.String())

	case *types.Pointer:
		return fmt.Sprintf("?%s", GoToVarlinkType(u.Elem()))

	case *types.Array:
		return fmt.Sprintf("[]%s", GoToVarlinkType(u.Elem()))

	case *types.Slice:
		return fmt.Sprintf("[]%s", GoToVarlinkType(u.Elem()))

	default:
		return fmt.Sprintf("<<<%T %s>>>", t, u)
	}

	return t.String()
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
				fmt.Printf("type %s (\n", id.Name)
				fmt.Printf("\t%s: %s",
					f.Field(0).Name(), GoToVarlinkType(f.Field(0).Type()))
				for i := 1; i < f.NumFields(); i++ {
					fmt.Printf(",\n\t%s: %s",
						f.Field(i).Name(), GoToVarlinkType(f.Field(i).Type()))
				}
				fmt.Printf("\n)\n\n")
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
