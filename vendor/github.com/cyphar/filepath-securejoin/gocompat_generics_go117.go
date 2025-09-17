//go:build linux && go1.17

package securejoin

import (
	"os"
	"sync"
)

func slices_DeleteFunc(input []string, del func(string) bool) []string {
	output := make([]string, len(input))
	outputIndex := 0
	for index := range input {
		part := input[index]
		if !del(part) {
			output[outputIndex] = part
			outputIndex++
		}
	}
	output = output[:outputIndex]
	return output
}

func slices_Contains(input []string, val string) bool {
	for index := range input {
		part := input[index]
		if part == val {
			return true
		}
	}
	return false
}

// Copied from the Go 1.24 stdlib implementation.
func sync_OnceValue(f func() bool) func() bool {
	var (
		once   sync.Once
		valid  bool
		p      interface{}
		result bool
	)
	g := func() {
		defer func() {
			p = recover()
			if !valid {
				panic(p)
			}
		}()
		result = f()
		f = nil
		valid = true
	}
	return func() bool {
		once.Do(g)
		if !valid {
			panic(p)
		}
		return result
	}
}

// Copied from the Go 1.24 stdlib implementation.
func sync_OnceValues(f func() (*os.File, error)) func() (*os.File, error) {
	var (
		once  sync.Once
		valid bool
		p     interface{}
		r1    *os.File
		r2    error
	)
	g := func() {
		defer func() {
			p = recover()
			if !valid {
				panic(p)
			}
		}()
		r1, r2 = f()
		f = nil
		valid = true
	}
	return func() (*os.File, error) {
		once.Do(g)
		if !valid {
			panic(p)
		}
		return r1, r2
	}
}
