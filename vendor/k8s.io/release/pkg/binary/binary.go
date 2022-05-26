/*
Copyright 2021 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package binary

import (
	"bufio"
	"io"
	"os"
	"unicode"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate
const (
	// GOOS labels
	LINUX  = "linux"
	DARWIN = "darwin"
	WIN    = "windows"

	// GOARCH Architecture labels
	I386    = "386"
	AMD64   = "amd64"
	ARM     = "arm"
	ARM64   = "arm64"
	PPC     = "ppc"
	PPC64LE = "ppc64le"
	S390    = "s390x"
	RISCV   = "riscv"
)

// Binary is the base type of the package. It abstracts a binary executable
type Binary struct {
	options *Options
	binaryImplementation
}

// Options to control the binary checker
type Options struct {
	Path string
}

// DefaultOptions set of options
var DefaultOptions = &Options{}

// New creates a new binary instance.
func New(filePath string) (bin *Binary, err error) {
	// Get the right implementation for the specified file
	return NewWithOptions(filePath, DefaultOptions)
}

// NewWithOptions creates a new binary with the specified options
func NewWithOptions(filePath string, opts *Options) (bin *Binary, err error) {
	bin = &Binary{
		options: opts,
	}
	// Get the right implementation for the specified file
	impl, err := getArchImplementation(filePath, opts)
	if err != nil {
		return nil, errors.Wrap(err, "getting arch implementation")
	}
	bin.options.Path = filePath
	bin.SetImplementation(impl)
	return bin, nil
}

// getArchImplementation returns the implementation that corresponds
// to the specified binary
func getArchImplementation(filePath string, opts *Options) (impl binaryImplementation, err error) {
	// Check if we're dealing with a Linux binary
	elf, err := NewELFBinary(filePath, opts)
	if err != nil {
		return nil, errors.Wrap(err, "checking if file is an ELF binary")
	}
	if elf != nil {
		return elf, nil
	}

	// Check if its a darwin binary
	macho, err := NewMachOBinary(filePath, opts)
	if err != nil {
		return nil, errors.Wrap(err, "checking if file is a Mach-O binary")
	}
	if macho != nil {
		return macho, nil
	}

	// Finally we check to see if it's a windows binary
	pe, err := NewPEBinary(filePath, opts)
	if err != nil {
		return nil, errors.Wrap(err, "checking if file is a windows PE binary")
	}
	if pe != nil {
		return pe, nil
	}

	logrus.Warnf("File is not a known executable: %s", filePath)
	return nil, errors.New("file is not an executable or is an unknown format")
}

//counterfeiter:generate . binaryImplementation
type binaryImplementation interface {
	// GetArch Returns a string with the GOARCH of the binary
	Arch() string

	// GetOS Returns a string with the GOOS of the binary
	OS() string
}

// SetImplementation sets the implementation to handle this sort of executable
func (b *Binary) SetImplementation(impl binaryImplementation) {
	b.binaryImplementation = impl
}

// Arch returns a string with the GOARCH label of the file
func (b *Binary) Arch() string {
	return b.binaryImplementation.Arch()
}

// OS returns a string with the GOOS label of the binary file
func (b *Binary) OS() string {
	return b.binaryImplementation.OS()
}

// ContainsStrings searches the printable strings un a binary file
func (b *Binary) ContainsStrings(s ...string) (match bool, err error) {
	// We cannot search for 0 items:
	if len(s) == 0 {
		return match, errors.New("cannot search binary, no search terms defined")
	}

	// Open the binary
	f, err := os.Open(b.options.Path)
	if err != nil {
		return match, errors.Wrap(err, "opening binary to search")
	}
	defer f.Close()
	terms := map[string]bool{}
	for _, s := range s {
		terms[s] = true
	}

	in := bufio.NewReader(f)
	runes := []rune{}

	for {
		// Read each rune from the binary file
		r, _, err := in.ReadRune()
		if err != nil {
			if err != io.EOF {
				return match, errors.Wrap(err, "while reading binary data")
			}
			return false, nil
		}
		// If the char is not printable, we assume the string ended
		// and we can check if the collected runes form one of our terms:
		if !unicode.IsPrint(r) {
			delete(terms, string(runes))
			runes = []rune{}
			if len(terms) == 0 {
				return true, nil
			}
			continue
		}
		runes = append(runes, r)
	}
}
