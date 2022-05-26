package mparser

import (
	"io/ioutil"
	"log"
	"path/filepath"

	"github.com/gomarkdown/markdown/ast"
	"github.com/gomarkdown/markdown/parser"
)

var UnsafeInclude parser.Flags = 1 << 3

// Hook will call both TitleHook and ReferenceHook.
func Hook(data []byte) (ast.Node, []byte, int) {
	n, b, i := TitleHook(data)
	if n != nil {
		return n, b, i
	}

	return ReferenceHook(data)
}

// ReadInclude is the hook to read includes.
// Its supports the following options for address.
//
// 4,5 - line numbers separated by commas
// N, - line numbers, end not specified, read until the end.
// /start/,/end/ - regexp separated by commas
// optional a prefix="" string.
func (i Initial) ReadInclude(from, file string, address []byte) []byte {
	path := i.path(from, file)

	if i.Flags&UnsafeInclude == 0 {
		if ok := i.pathAllowed(path); !ok {
			log.Printf("Failure to read: %q: path is not on or below %q", path, i.i)
			return nil
		}
	}

	data, err := ioutil.ReadFile(path)
	if err != nil {
		log.Printf("Failure to read: %q (from %q)", err, filepath.Join(from, "*"))
		return nil
	}

	data, err = parseAddress(address, data)
	if err != nil {
		log.Printf("Failure to parse address for %q: %q (from %q)", path, err, filepath.Join(from, "*"))
		return nil
	}
	if len(data) == 0 {
		return data
	}
	if data[len(data)-1] != '\n' {
		data = append(data, '\n')
	}
	return data
}
