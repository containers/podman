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
	"encoding/binary"
	"fmt"
	"os"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// ELFBinary abstracts a binary in ELF format
type ELFBinary struct {
	Header  *ELFHeader
	Options *Options
}

// ELFHeader abstracts the data we need from the elf header
type ELFHeader struct {
	WordFlag   uint8    // Flag: 32 or 64 bit binary
	_          uint8    // byte order
	_          uint8    // ELF version
	OSABI      uint8    // Binary Interface
	ABIVersion uint8    // ABI Version
	_          [7]uint8 // EI_PAD Zero padding
	EType      uint16   // Executable Type: Executable, relocatable, etc
	EMachine   uint16   // Machine architecture
}

// NewELFBinary opens a file and returns an ELF binary if it is one
func NewELFBinary(filePath string, opts *Options) (*ELFBinary, error) {
	header, err := GetELFHeader(filePath)
	if err != nil {
		return nil, errors.Wrap(err, "while trying to get ELF header from file")
	}
	if header == nil {
		logrus.Debug("file is not an ELF binary")
		return nil, nil
	}

	return &ELFBinary{
		Header: header,
	}, nil
}

// String returns the relevant info of the header as a string
func (eh *ELFHeader) String() string {
	return fmt.Sprintf("%s %dbit", eh.MachineType(), eh.WordLength())
}

// WordLength returns either 32 or 64 for 32bit or 64 bit architectures
func (eh *ELFHeader) WordLength() int {
	if eh.WordFlag == 1 {
		return 32
	}
	if eh.WordFlag == 2 {
		return 64
	}
	logrus.Warn("Cannot determine if ELF binary is 32 or 64 bit")
	return 0
}

// MachineType returns a string with the architecture moniker
func (eh *ELFHeader) MachineType() string {
	switch eh.EMachine {
	// 0x02	SPARC
	// 0x03	x86
	case 0x03:
		return I386
	// 0x06	Intel MCU
	// 0x07	Intel 80860
	// 0x08	MIPS
	// 0x09	IBM_System/370
	// 0x0A	MIPS RS3000 Little-endian
	// 0x14	PowerPC
	case 0x14:
		return PPC
	// 0x15	PowerPC (64-bit)
	case 0x15:
		return PPC64LE
	// 0x16	S390, including S390x
	case 0x16:
		return S390
	// 0x28	ARM (up to ARMv7/Aarch32)
	case 0x28:
		return ARM
	// 0x3E	amd64
	case 0x3e:
		return AMD64
	// 0xB7	ARM 64-bits (ARMv8/Aarch64)
	case 0xb7:
		return ARM64
	// 0xF3	RISC-V
	case 0xF3:
		return RISCV
	}
	logrus.Warn("Unknown machine type in elf binary")
	return "arch unknown"
}

// GetELFHeader returns the header if the binary is and EF binary
func GetELFHeader(path string) (*ELFHeader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, errors.Wrap(err, "opening binary for reading")
	}
	defer f.Close()

	// Read the first 20 bytes of the binary, just enough of the
	// header for us to get the info we need:
	reader := bufio.NewReader(f)
	hBytes, err := reader.Peek(6)
	if err != nil {
		return nil, errors.Wrap(err, "reading the binary header")
	}

	logrus.StandardLogger().Debugf("Header bytes: %+v", hBytes)

	// Check we're dealing with an elf binary:
	if string(hBytes[1:4]) != "ELF" {
		logrus.Debug("Binary is not an ELF executable")
		return nil, nil
	}

	// Check if binary byte order is big or little endian
	var endianness binary.ByteOrder
	if hBytes[5] == 2 {
		endianness = binary.BigEndian
	} else if hBytes[5] == 1 {
		endianness = binary.LittleEndian
	} else {
		return nil, errors.Wrap(err, "invalid endianness specified in elf binary")
	}

	header := &ELFHeader{}
	if _, err := f.Seek(4, 0); err != nil {
		return nil, errors.Wrap(err, "seeking past the ELF magic bytes")
	}
	if err := binary.Read(f, endianness, header); err != nil {
		return nil, errors.Wrap(err, "reading elf header from binary file")
	}
	return header, nil
}

// Arch return the GOOS label of the binary
func (elf *ELFBinary) Arch() string {
	return elf.Header.MachineType()
}

// OS returns the GOOS label for the operating system
func (elf *ELFBinary) OS() string {
	return LINUX
}
