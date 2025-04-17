package iso9660

import (
	"encoding/binary"
	"fmt"
	"io"
	"strings"
)

// MarshalString encodes the given string as a byte array padded to the given length
func MarshalString(s string, padToLength int) []byte {
	if len(s) > padToLength {
		s = s[:padToLength]
	}
	missingPadding := padToLength - len(s)
	s = s + strings.Repeat(" ", missingPadding)
	return []byte(s)
}

// UnmarshalInt32LSBMSB decodes a 32-bit integer in both byte orders, as defined in ECMA-119 7.3.3
func UnmarshalInt32LSBMSB(data []byte) (int32, error) {
	if len(data) < 8 {
		return 0, io.ErrUnexpectedEOF
	}

	lsb := int32(binary.LittleEndian.Uint32(data[0:4]))
	msb := int32(binary.BigEndian.Uint32(data[4:8]))

	if lsb != msb {
		return 0, fmt.Errorf("little-endian and big-endian value mismatch: %d != %d", lsb, msb)
	}

	return lsb, nil
}

// UnmarshalUint32LSBMSB is the same as UnmarshalInt32LSBMSB but returns an unsigned integer
func UnmarshalUint32LSBMSB(data []byte) (uint32, error) {
	n, err := UnmarshalInt32LSBMSB(data)
	return uint32(n), err
}

// UnmarshalInt16LSBMSB decodes a 16-bit integer in both byte orders, as defined in ECMA-119 7.3.3
func UnmarshalInt16LSBMSB(data []byte) (int16, error) {
	if len(data) < 4 {
		return 0, io.ErrUnexpectedEOF
	}

	lsb := int16(binary.LittleEndian.Uint16(data[0:2]))
	msb := int16(binary.BigEndian.Uint16(data[2:4]))

	if lsb != msb {
		return 0, fmt.Errorf("little-endian and big-endian value mismatch: %d != %d", lsb, msb)
	}

	return lsb, nil
}

// WriteInt32LSBMSB writes a 32-bit integer in both byte orders, as defined in ECMA-119 7.3.3
func WriteInt32LSBMSB(dst []byte, value int32) {
	_ = dst[7] // early bounds check to guarantee safety of writes below
	binary.LittleEndian.PutUint32(dst[0:4], uint32(value))
	binary.BigEndian.PutUint32(dst[4:8], uint32(value))
}

// WriteInt16LSBMSB writes a 16-bit integer in both byte orders, as defined in ECMA-119 7.2.3
func WriteInt16LSBMSB(dst []byte, value int16) {
	_ = dst[3] // early bounds check to guarantee safety of writes below
	binary.LittleEndian.PutUint16(dst[0:2], uint16(value))
	binary.BigEndian.PutUint16(dst[2:4], uint16(value))
}
