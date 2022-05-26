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

const (
	MachO32Magic   uint32 = 0xfeedface // 32 bit, big endian
	MachO64Magic   uint32 = 0xfeedfacf // 64 bit, big endian
	MachO32LIMagic uint32 = 0xcefaedfe // 32 bit, little endian
	MachO64LIMagic uint32 = 0xcffaedfe // 64 bit, little endian
	MachOFat       uint32 = 0xcafebabe // Universal Binary
)

// MachOHeader is a structure to capture the data we need from the binary header
type MachOHeader struct {
	Magic  uint32
	CPU    uint32
	SubCPU uint32
}

// MachOBinary is an abstraction for a Mach-O executable
type MachOBinary struct {
	Header  *MachOHeader
	Options *Options
}

// NewMachOBinary returns a Mach-O binary if the specified file is one
func NewMachOBinary(filePath string, opts *Options) (*MachOBinary, error) {
	header, err := GetMachOHeader(filePath)
	if err != nil {
		return nil, errors.Wrap(err, "trying to read a Mach-O header from file")
	}
	if header == nil {
		logrus.Debug("File is not a Mach-O binary")
		return nil, nil
	}
	return &MachOBinary{
		Header:  header,
		Options: opts,
	}, nil
}

// String returns the header information as a string
func (machoh *MachOHeader) String() string {
	return fmt.Sprintf("%s %dbit", machoh.MachineType(), machoh.WordLength())
}

// WordLength returns an integer indicating if this is a 32 or 64bit binary
func (machoh *MachOHeader) WordLength() int {
	switch machoh.Magic {
	case MachO32Magic:
		return 32
	case MachO32LIMagic:
		return 32
	case MachO64Magic:
		return 64
	case MachO64LIMagic:
		return 64
	case MachOFat:
		return 0
	}
	return 0
}

// MachineType returns the architecture as a GOARCH label
func (machoh *MachOHeader) MachineType() string {
	// Interpret the header byte defining the CPU arch. Defined here:
	// https://opensource.apple.com/source/cctools/cctools-836/include/mach/machine.h

	// Universal Binaries can support many architectures in the same file.
	if machoh.Magic == MachOFat {
		return "FAT"
	}

	// Todo: Perhaps if only one arch is in the file, we could return it here

	// CPU_ARCH_ABI64 / 0x1000000
	switch machoh.CPU {
	// CPU_TYPE_I386 / ((cpu_type_t) 7)
	case 7:
		return I386
	// CPU_TYPE_X86_64 / ((cpu_type_t) (CPU_TYPE_I386 | CPU_ARCH_ABI64))
	case 16777223:
		return AMD64
	// CPU_TYPE_POWERPC / ((cpu_type_t) 18)
	case 18:
		return PPC
	// CPU_TYPE_POWERPC64 / ((cpu_type_t)(CPU_TYPE_POWERPC | CPU_ARCH_ABI64))
	case 16777234:
		return PPC64LE
	// CPU_TYPE_ARM / ((cpu_type_t) 12)
	case 12:
		return ARM
	// ARM 64-bits (ARMv8/Aarch64) (not in source)
	case 16777228:
		return ARM64
	}

	logrus.Warnf("Unable to interpret machine type from mach-o header value %d", machoh.CPU)
	return ""
}

// GetMachOHeader returns a struct with the executable header information
func GetMachOHeader(path string) (*MachOHeader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, errors.Wrap(err, "opening binary for reading")
	}
	defer f.Close()

	reader := bufio.NewReader(f)
	hBytes, err := reader.Peek(4)
	if err != nil {
		return nil, errors.Wrap(err, "reading the binary header")
	}

	var endianness binary.ByteOrder
	magic := binary.BigEndian.Uint32(hBytes)
	switch magic {
	case MachO32Magic:
		logrus.Info("Mach-O 32bit")
		endianness = binary.BigEndian
	case MachO64Magic:
		logrus.Info("Mach-O 64bit")
		endianness = binary.BigEndian
	case MachO32LIMagic:
		logrus.Info("Mach-O 32bit Little Endian")
		endianness = binary.LittleEndian
	case MachO64LIMagic:
		logrus.Info("Mach-O 64bit Little Endian")
		endianness = binary.LittleEndian
	case MachOFat:
		logrus.Info("Mach-O Universal Binary")
		endianness = binary.BigEndian
	default:
		logrus.Debug("File is not a Mach-O binary")
		return nil, nil
	}

	header := &MachOHeader{}
	if _, err := f.Seek(0, 0); err != nil {
		return nil, errors.Wrap(err, "seeking to the start of the file")
	}
	if err := binary.Read(f, endianness, header); err != nil {
		return nil, errors.Wrap(err, "reading Mach-O header from binary file")
	}
	return header, nil
}

// Arch returns a string with the GOARCH label of the file
func (macho *MachOBinary) Arch() string {
	return macho.Header.MachineType()
}

// OS returns a string with the GOOS label of the binary file
func (macho *MachOBinary) OS() string {
	return DARWIN
}
