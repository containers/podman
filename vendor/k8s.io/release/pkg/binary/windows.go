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
	"encoding/binary"
	"fmt"
	"os"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// PEFileHeader captures the header information of the executable
type PEFileHeader struct {
	Machine              uint16
	NumberOfSections     uint16
	TimeDateStamp        uint32
	PointerToSymbolTable uint32
	NumberOfSymbols      uint32
	SizeOfOptionalHeader uint16
	Characteristics      uint16
}

// PEHeader captures the header information of the executable
type PEHeader struct {
	Machine uint16
	Magic   uint16
}

// PEOptionalHeader we only care about the magic number to determine the binary wordsize
type PEOptionalHeader struct {
	Magic uint16
}

// PEBinary is a struct that abstracts a Windows Portable Executable
type PEBinary struct {
	Header  *PEHeader
	Options *Options
}

// NewPEBinary Returns a binary implementation for a Windows executable
func NewPEBinary(filePath string, opts *Options) (bin *PEBinary, err error) {
	header, err := GetPEHeader(filePath)
	if err != nil {
		return nil, errors.Wrap(err, "reading header from binary")
	}
	if header == nil {
		logrus.Infof("file is not a PE executable")
		return nil, nil
	}
	return &PEBinary{
		Header:  header,
		Options: opts,
	}, nil
}

// Return the header information as a string
func (peh *PEHeader) String() string {
	return fmt.Sprintf("%s %dbit", peh.MachineType(), peh.WordLength())
}

// MachineType returns the moniker of the binary architecture
func (peh *PEHeader) MachineType() string {
	//nolint:gocritic
	switch peh.Machine {
	// IMAGE_FILE_MACHINE_AMD64     = 0x8664
	case 0x8664:
		return AMD64
	// IMAGE_FILE_MACHINE_ARM       = 0x1c0
	case 0x1c0:
		return ARM
	// IMAGE_FILE_MACHINE_ARMNT     = 0x1c4
	// IMAGE_FILE_MACHINE_ARM64     = 0xaa64
	case 0xaa64:
		return ARM64
	// IMAGE_FILE_MACHINE_EBC       = 0xebc
	// IMAGE_FILE_MACHINE_I386      = 0x14c
	case 0x14c:
		return I386
	// IMAGE_FILE_MACHINE_IA64      = 0x200
	// IMAGE_FILE_MACHINE_M32R      = 0x9041
	// IMAGE_FILE_MACHINE_MIPS16    = 0x266
	// IMAGE_FILE_MACHINE_MIPSFPU   = 0x366
	// IMAGE_FILE_MACHINE_MIPSFPU16 = 0x466
	// IMAGE_FILE_MACHINE_POWERPC   = 0x1f0
	// IMAGE_FILE_MACHINE_POWERPCFP = 0x1f1
	// IMAGE_FILE_MACHINE_R4000     = 0x166
	// IMAGE_FILE_MACHINE_SH3       = 0x1a2
	// IMAGE_FILE_MACHINE_SH3DSP    = 0x1a3
	// IMAGE_FILE_MACHINE_SH4       = 0x1a6
	// IMAGE_FILE_MACHINE_SH5       = 0x1a8
	// IMAGE_FILE_MACHINE_THUMB     = 0x1c2
	// IMAGE_FILE_MACHINE_WCEMIPSV2 = 0x169
	case 0x1f0:
		return PPC
	}

	logrus.Warn("Could not determine architecture type")
	return ""
}

// WordLength Returns an integer indicating if it's a 64 or 32 bit binary
func (peh *PEHeader) WordLength() int {
	// We infer the wordlength from the machine type
	// https://en.wikibooks.org/wiki/X86_Disassembly/Windows_Executable_Files#PE_Optional_Header
	switch peh.Magic {
	case 0x10b:
		return 32
	case 0x20b:
		return 64
	default:
		logrus.Warn("Unable to interpret Magic byte to determine word length")
		return 0
	}
}

// GetPEHeader returns a portable executable header from the specified file
func GetPEHeader(path string) (*PEHeader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, errors.Wrap(err, "opening binary for reading")
	}
	defer f.Close()

	// Get the DOS header to determine the file header offset
	var dosheader [96]byte
	if _, err := f.ReadAt(dosheader[0:], 0); err != nil {
		return nil, err
	}
	var base int64
	if dosheader[0] == 'M' && dosheader[1] == 'Z' {
		// "At offset 60 (0x3C) from the beginning of the DOS header is a pointer to
		// the Portable Executable (PE) File header":
		signoff := int64(binary.LittleEndian.Uint32(dosheader[0x3c:]))
		var sign [4]byte
		if _, err := f.ReadAt(sign[:], signoff); err != nil {
			return nil, errors.Wrap(err, "reading the PE file header location")
		}
		if !(sign[0] == 'P' && sign[1] == 'E' && sign[2] == 0 && sign[3] == 0) {
			return nil, errors.New("Invalid PE COFF file signature")
		}
		base = signoff + 4
	} else {
		// If the DOS header signature is not found, then discard the file as a valid
		// windows executable
		logrus.Debug("File is not a valid windows PE executable")
		return nil, nil
	}

	if _, err := f.Seek(base, 0); err != nil {
		return nil, errors.Wrap(err, "seeking to start of the PE file header location")
	}

	// Read the full header, will be discarded later
	header := &PEFileHeader{}
	if err := binary.Read(f, binary.LittleEndian, header); err != nil {
		return nil, err
	}

	// Now from the full file header we got, we jump to the "optional" header
	if _, err = f.Seek(base+int64(binary.Size(header)), 0); err != nil {
		return nil, errors.Wrap(err, "Unable to seek to start of PE Optional header")
	}

	// Read the file optional header
	oheader := &PEOptionalHeader{}
	if err := binary.Read(f, binary.LittleEndian, oheader); err != nil {
		return nil, errors.Wrap(err, "reading optional PE header from binary")
	}

	return &PEHeader{
		Machine: header.Machine,
		Magic:   oheader.Magic,
	}, nil
}

// Arch return the architecture
func (pe *PEBinary) Arch() string {
	return pe.Header.MachineType()
}

// OS returns the operating system of the binary
func (pe *PEBinary) OS() string {
	return WIN
}
