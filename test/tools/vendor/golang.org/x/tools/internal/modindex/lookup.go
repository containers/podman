// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package modindex

import (
	"slices"
	"strconv"
	"strings"
)

type Candidate struct {
	PkgName    string
	Name       string
	Dir        string
	ImportPath string
	Type       LexType
	// information for Funcs
	Results int16   // how many results
	Sig     []Field // arg names and types
}

type Field struct {
	Arg, Type string
}

type LexType int8

const (
	Const LexType = iota
	Var
	Type
	Func
)

// Lookup finds all the symbols in the index with the given PkgName and name.
// If prefix is true, it finds all of these with name as a prefix.
func (ix *Index) Lookup(pkg, name string, prefix bool) []Candidate {
	loc, ok := slices.BinarySearchFunc(ix.Entries, pkg, func(e Entry, pkg string) int {
		return strings.Compare(e.PkgName, pkg)
	})
	if !ok {
		return nil // didn't find the package
	}
	var ans []Candidate
	// loc is the first entry for this package name, but there may be severeal
	for i := loc; i < len(ix.Entries); i++ {
		e := ix.Entries[i]
		if e.PkgName != pkg {
			break // end of sorted package names
		}
		nloc, ok := slices.BinarySearchFunc(e.Names, name, func(s string, name string) int {
			if strings.HasPrefix(s, name) {
				return 0
			}
			if s < name {
				return -1
			}
			return 1
		})
		if !ok {
			continue // didn't find the name, nor any symbols with name as a prefix
		}
		for j := nloc; j < len(e.Names); j++ {
			nstr := e.Names[j]
			// benchmarks show this makes a difference when there are a lot of Possibilities
			flds := fastSplit(nstr)
			if !(flds[0] == name || prefix && strings.HasPrefix(flds[0], name)) {
				// past range of matching Names
				break
			}
			if len(flds) < 2 {
				continue // should never happen
			}
			px := Candidate{
				PkgName:    pkg,
				Name:       flds[0],
				Dir:        string(e.Dir),
				ImportPath: e.ImportPath,
				Type:       asLexType(flds[1][0]),
			}
			if flds[1] == "F" {
				n, err := strconv.Atoi(flds[2])
				if err != nil {
					continue // should never happen
				}
				px.Results = int16(n)
				if len(flds) >= 4 {
					sig := strings.Split(flds[3], " ")
					for i := 0; i < len(sig); i++ {
						// $ cannot otherwise occur. removing the spaces
						// almost works, but for chan struct{}, e.g.
						sig[i] = strings.Replace(sig[i], "$", " ", -1)
					}
					px.Sig = toFields(sig)
				}
			}
			ans = append(ans, px)
		}
	}
	return ans
}

func toFields(sig []string) []Field {
	ans := make([]Field, len(sig)/2)
	for i := 0; i < len(ans); i++ {
		ans[i] = Field{Arg: sig[2*i], Type: sig[2*i+1]}
	}
	return ans
}

// benchmarks show this is measurably better than strings.Split
func fastSplit(x string) []string {
	ans := make([]string, 0, 4)
	nxt := 0
	start := 0
	for i := 0; i < len(x); i++ {
		if x[i] != ' ' {
			continue
		}
		ans = append(ans, x[start:i])
		nxt++
		start = i + 1
		if nxt >= 3 {
			break
		}
	}
	ans = append(ans, x[start:])
	return ans
}

func asLexType(c byte) LexType {
	switch c {
	case 'C':
		return Const
	case 'V':
		return Var
	case 'T':
		return Type
	case 'F':
		return Func
	}
	return -1
}
