// Package iso9660 implements reading and creating basic ISO9660 images.
package iso9660

import (
	"encoding"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// ISO 9660 Overview
// https://archive.fo/xs9ac

const (
	sectorSize         uint32 = 2048
	systemAreaSize            = sectorSize * 16
	standardIdentifier        = "CD001"
	udfIdentifier             = "BEA01"

	volumeTypeBoot          byte = 0
	volumeTypePrimary       byte = 1
	volumeTypeSupplementary byte = 2
	volumeTypePartition     byte = 3
	volumeTypeTerminator    byte = 255

	volumeDescriptorBodySize = sectorSize - 7

	aCharacters = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_!\"%&'()*+,-./:;<=>?"
	dCharacters = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_"
	// ECMA-119 7.4.2.2 defines d1-characters as
	// "subject to agreement between the originator and the recipient of the volume".
	d1Characters = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_!\"%&'()*+,-./:;<=>?"
)

const (
	dirFlagHidden = 1 << iota
	dirFlagDir
	dirFlagAssociated
	dirFlagRecord
	dirFlagProtection
	_
	_
	dirFlagMultiExtent
)

var standardIdentifierBytes = [5]byte{'C', 'D', '0', '0', '1'}

var ErrUDFNotSupported = errors.New("UDF volumes are not supported")

// volumeDescriptorHeader represents the data in bytes 0-6
// of a Volume Descriptor as defined in ECMA-119 8.1
type volumeDescriptorHeader struct {
	Type       byte
	Identifier [5]byte
	Version    byte
}

var _ encoding.BinaryUnmarshaler = &volumeDescriptorHeader{}
var _ encoding.BinaryMarshaler = &volumeDescriptorHeader{}

// UnmarshalBinary decodes a volumeDescriptorHeader from binary form
func (vdh *volumeDescriptorHeader) UnmarshalBinary(data []byte) error {
	if len(data) < 7 {
		return io.ErrUnexpectedEOF
	}

	vdh.Type = data[0]
	copy(vdh.Identifier[:], data[1:6])
	vdh.Version = data[6]
	return nil
}

func (vdh volumeDescriptorHeader) MarshalBinary() ([]byte, error) {
	data := make([]byte, 7)
	data[0] = vdh.Type
	data[6] = vdh.Version
	copy(data[1:6], vdh.Identifier[:])
	return data, nil
}

// BootVolumeDescriptorBody represents the data in bytes 7-2047
// of a Boot Record as defined in ECMA-119 8.2
type BootVolumeDescriptorBody struct {
	BootSystemIdentifier string
	BootIdentifier       string
	BootSystemUse        [1977]byte
}

var _ encoding.BinaryUnmarshaler = &BootVolumeDescriptorBody{}

// PrimaryVolumeDescriptorBody represents the data in bytes 7-2047
// of a Primary Volume Descriptor as defined in ECMA-119 8.4
type PrimaryVolumeDescriptorBody struct {
	SystemIdentifier              string
	VolumeIdentifier              string
	VolumeSpaceSize               int32
	VolumeSetSize                 int16
	VolumeSequenceNumber          int16
	LogicalBlockSize              int16
	PathTableSize                 int32
	TypeLPathTableLoc             int32
	OptTypeLPathTableLoc          int32
	TypeMPathTableLoc             int32
	OptTypeMPathTableLoc          int32
	RootDirectoryEntry            *DirectoryEntry
	VolumeSetIdentifier           string
	PublisherIdentifier           string
	DataPreparerIdentifier        string
	ApplicationIdentifier         string
	CopyrightFileIdentifier       string
	AbstractFileIdentifier        string
	BibliographicFileIdentifier   string
	VolumeCreationDateAndTime     VolumeDescriptorTimestamp
	VolumeModificationDateAndTime VolumeDescriptorTimestamp
	VolumeExpirationDateAndTime   VolumeDescriptorTimestamp
	VolumeEffectiveDateAndTime    VolumeDescriptorTimestamp
	FileStructureVersion          byte
	ApplicationUsed               [512]byte
}

var _ encoding.BinaryUnmarshaler = &PrimaryVolumeDescriptorBody{}
var _ encoding.BinaryMarshaler = PrimaryVolumeDescriptorBody{}

// DirectoryEntry contains data from a Directory Descriptor
// as described by ECMA-119 9.1
type DirectoryEntry struct {
	ExtendedAtributeRecordLength byte
	ExtentLocation               int32
	ExtentLength                 uint32
	RecordingDateTime            RecordingTimestamp
	FileFlags                    byte
	FileUnitSize                 byte
	InterleaveGap                byte
	VolumeSequenceNumber         int16
	Identifier                   string
	SystemUse                    []byte
	SystemUseEntries             SystemUseEntrySlice
}

var _ encoding.BinaryUnmarshaler = &DirectoryEntry{}
var _ encoding.BinaryMarshaler = &DirectoryEntry{}

// UnmarshalBinary decodes a DirectoryEntry from binary form
func (de *DirectoryEntry) UnmarshalBinary(data []byte) error {
	length := data[0]
	if length == 0 {
		return io.EOF
	}

	var err error

	de.ExtendedAtributeRecordLength = data[1]

	if de.ExtentLocation, err = UnmarshalInt32LSBMSB(data[2:10]); err != nil {
		return err
	}

	if de.ExtentLength, err = UnmarshalUint32LSBMSB(data[10:18]); err != nil {
		return err
	}

	if err = de.RecordingDateTime.UnmarshalBinary(data[18:25]); err != nil {
		return err
	}

	de.FileFlags = data[25]
	de.FileUnitSize = data[26]
	de.InterleaveGap = data[27]

	if de.VolumeSequenceNumber, err = UnmarshalInt16LSBMSB(data[28:32]); err != nil {
		return err
	}

	identifierLen := data[32]
	de.Identifier = string(data[33 : 33+identifierLen])

	// add padding if identifier length was even]
	idPaddingLen := (identifierLen + 1) % 2
	de.SystemUse = data[33+identifierLen+idPaddingLen : length]

	return nil
}

// MarshalBinary encodes a DirectoryEntry to binary form
func (de *DirectoryEntry) MarshalBinary() ([]byte, error) {
	identifierLen := len(de.Identifier)
	idPaddingLen := (identifierLen + 1) % 2
	totalLen := 33 + identifierLen + idPaddingLen + len(de.SystemUse)
	if totalLen > 255 {
		return nil, fmt.Errorf("identifier %q is too long", de.Identifier)
	}

	data := make([]byte, totalLen)

	data[0] = byte(totalLen)
	data[1] = de.ExtendedAtributeRecordLength

	WriteInt32LSBMSB(data[2:10], de.ExtentLocation)
	WriteInt32LSBMSB(data[10:18], int32(de.ExtentLength))
	de.RecordingDateTime.MarshalBinary(data[18:25])
	data[25] = de.FileFlags
	data[26] = de.FileUnitSize
	data[27] = de.InterleaveGap
	WriteInt16LSBMSB(data[28:32], de.VolumeSequenceNumber)
	data[32] = byte(identifierLen)
	copy(data[33:33+identifierLen], []byte(de.Identifier))

	copy(data[33+identifierLen+idPaddingLen:totalLen], de.SystemUse)

	return data, nil
}

// Clone creates a copy of the DirectoryEntry
func (de *DirectoryEntry) Clone() DirectoryEntry {
	newDE := DirectoryEntry{
		ExtendedAtributeRecordLength: de.ExtendedAtributeRecordLength,
		ExtentLocation:               de.ExtentLocation,
		ExtentLength:                 de.ExtentLength,
		RecordingDateTime:            de.RecordingDateTime,
		FileFlags:                    de.FileFlags,
		FileUnitSize:                 de.FileUnitSize,
		InterleaveGap:                de.InterleaveGap,
		VolumeSequenceNumber:         de.VolumeSequenceNumber,
		Identifier:                   de.Identifier,
		SystemUse:                    make([]byte, len(de.SystemUse)),
	}
	copy(newDE.SystemUse, de.SystemUse)
	return newDE
}

// UnmarshalBinary decodes a PrimaryVolumeDescriptorBody from binary form as defined in ECMA-119 8.4
func (pvd *PrimaryVolumeDescriptorBody) UnmarshalBinary(data []byte) error {
	if len(data) < 2048 {
		return io.ErrUnexpectedEOF
	}

	var err error

	pvd.SystemIdentifier = strings.TrimRight(string(data[8:40]), " ")
	pvd.VolumeIdentifier = strings.TrimRight(string(data[40:72]), " ")

	if pvd.VolumeSpaceSize, err = UnmarshalInt32LSBMSB(data[80:88]); err != nil {
		return err
	}

	if pvd.VolumeSetSize, err = UnmarshalInt16LSBMSB(data[120:124]); err != nil {
		return err
	}

	if pvd.VolumeSequenceNumber, err = UnmarshalInt16LSBMSB(data[124:128]); err != nil {
		return err
	}

	if pvd.LogicalBlockSize, err = UnmarshalInt16LSBMSB(data[128:132]); err != nil {
		return err
	}

	if pvd.PathTableSize, err = UnmarshalInt32LSBMSB(data[132:140]); err != nil {
		return err
	}

	pvd.TypeLPathTableLoc = int32(binary.LittleEndian.Uint32(data[140:144]))
	pvd.OptTypeLPathTableLoc = int32(binary.LittleEndian.Uint32(data[144:148]))
	pvd.TypeMPathTableLoc = int32(binary.BigEndian.Uint32(data[148:152]))
	pvd.OptTypeMPathTableLoc = int32(binary.BigEndian.Uint32(data[152:156]))

	if pvd.RootDirectoryEntry == nil {
		pvd.RootDirectoryEntry = &DirectoryEntry{}
	}
	if err = pvd.RootDirectoryEntry.UnmarshalBinary(data[156:190]); err != nil {
		return err
	}

	pvd.VolumeSetIdentifier = strings.TrimRight(string(data[190:318]), " ")
	pvd.PublisherIdentifier = strings.TrimRight(string(data[318:446]), " ")
	pvd.DataPreparerIdentifier = strings.TrimRight(string(data[446:574]), " ")
	pvd.ApplicationIdentifier = strings.TrimRight(string(data[574:702]), " ")
	pvd.CopyrightFileIdentifier = strings.TrimRight(string(data[702:740]), " ")
	pvd.AbstractFileIdentifier = strings.TrimRight(string(data[740:776]), " ")
	pvd.BibliographicFileIdentifier = strings.TrimRight(string(data[776:813]), " ")

	if pvd.VolumeCreationDateAndTime.UnmarshalBinary(data[813:830]) != nil {
		return err
	}

	if pvd.VolumeModificationDateAndTime.UnmarshalBinary(data[830:847]) != nil {
		return err
	}

	if pvd.VolumeExpirationDateAndTime.UnmarshalBinary(data[847:864]) != nil {
		return err
	}

	if pvd.VolumeEffectiveDateAndTime.UnmarshalBinary(data[864:881]) != nil {
		return err
	}

	pvd.FileStructureVersion = data[881]
	copy(pvd.ApplicationUsed[:], data[883:1395])

	return nil
}

// MarshalBinary encodes the PrimaryVolumeDescriptorBody to its binary form
func (pvd PrimaryVolumeDescriptorBody) MarshalBinary() ([]byte, error) {
	output := make([]byte, sectorSize)

	d := MarshalString(pvd.SystemIdentifier, 32)
	copy(output[8:40], d)

	d = MarshalString(pvd.VolumeIdentifier, 32)
	copy(output[40:72], d)

	WriteInt32LSBMSB(output[80:88], pvd.VolumeSpaceSize)
	WriteInt16LSBMSB(output[120:124], pvd.VolumeSetSize)
	WriteInt16LSBMSB(output[124:128], pvd.VolumeSequenceNumber)
	WriteInt16LSBMSB(output[128:132], pvd.LogicalBlockSize)
	WriteInt32LSBMSB(output[132:140], pvd.PathTableSize)

	binary.LittleEndian.PutUint32(output[140:144], uint32(pvd.TypeLPathTableLoc))
	binary.LittleEndian.PutUint32(output[144:148], uint32(pvd.OptTypeLPathTableLoc))
	binary.BigEndian.PutUint32(output[148:152], uint32(pvd.TypeMPathTableLoc))
	binary.BigEndian.PutUint32(output[152:156], uint32(pvd.OptTypeMPathTableLoc))

	binaryRDE, err := pvd.RootDirectoryEntry.MarshalBinary()
	if err != nil {
		return nil, err
	}
	copy(output[156:190], binaryRDE)

	copy(output[190:318], MarshalString(pvd.VolumeSetIdentifier, 128))
	copy(output[318:446], MarshalString(pvd.PublisherIdentifier, 128))
	copy(output[446:574], MarshalString(pvd.DataPreparerIdentifier, 128))
	copy(output[574:702], MarshalString(pvd.ApplicationIdentifier, 128))
	copy(output[702:740], MarshalString(pvd.CopyrightFileIdentifier, 38))
	copy(output[740:776], MarshalString(pvd.AbstractFileIdentifier, 36))
	copy(output[776:813], MarshalString(pvd.BibliographicFileIdentifier, 37))

	d, err = pvd.VolumeCreationDateAndTime.MarshalBinary()
	if err != nil {
		return nil, err
	}
	copy(output[813:830], d)

	d, err = pvd.VolumeModificationDateAndTime.MarshalBinary()
	if err != nil {
		return nil, err
	}
	copy(output[830:847], d)

	d, err = pvd.VolumeExpirationDateAndTime.MarshalBinary()
	if err != nil {
		return nil, err
	}
	copy(output[847:864], d)

	d, err = pvd.VolumeEffectiveDateAndTime.MarshalBinary()
	if err != nil {
		return nil, err
	}
	copy(output[864:881], d)

	output[881] = pvd.FileStructureVersion
	output[882] = 0
	copy(output[883:1395], pvd.ApplicationUsed[:])
	for i := 1395; i < 2048; i++ {
		output[i] = 0
	}

	return output, nil
}

// UnmarshalBinary decodes a BootVolumeDescriptorBody from binary form
func (bvd *BootVolumeDescriptorBody) UnmarshalBinary(data []byte) error {
	bvd.BootSystemIdentifier = strings.TrimRight(string(data[7:39]), " ")
	bvd.BootIdentifier = strings.TrimRight(string(data[39:71]), " ")
	if n := copy(bvd.BootSystemUse[:], data[71:2048]); n != 1977 {
		return fmt.Errorf("BootVolumeDescriptorBody.UnmarshalBinary: copied %d bytes", n)
	}
	return nil
}

type volumeDescriptor struct {
	Header  volumeDescriptorHeader
	Boot    *BootVolumeDescriptorBody
	Primary *PrimaryVolumeDescriptorBody
}

var _ encoding.BinaryUnmarshaler = &volumeDescriptor{}
var _ encoding.BinaryMarshaler = &volumeDescriptor{}

func (vd volumeDescriptor) Type() byte {
	return vd.Header.Type
}

// UnmarshalBinary decodes a volumeDescriptor from binary form
func (vd *volumeDescriptor) UnmarshalBinary(data []byte) error {
	if uint32(len(data)) < sectorSize {
		return io.ErrUnexpectedEOF
	}

	if err := vd.Header.UnmarshalBinary(data); err != nil {
		// this should never fail, since volumeDescriptorHeader.UnmarshalBinary( ) only checks data size too
		return err
	}

	id := string(vd.Header.Identifier[:])
	if id != standardIdentifier {
		if id == udfIdentifier {
			return ErrUDFNotSupported
		}
		return fmt.Errorf("volume descriptor %q != %q", id, standardIdentifier)
	}

	switch vd.Header.Type {
	case volumeTypeBoot:
		vd.Boot = &BootVolumeDescriptorBody{}
		return vd.Boot.UnmarshalBinary(data)
	case volumeTypePartition:
		return errors.New("partition volumes are not yet supported")
	case volumeTypePrimary, volumeTypeSupplementary:
		vd.Primary = &PrimaryVolumeDescriptorBody{}
		return vd.Primary.UnmarshalBinary(data)
	case volumeTypeTerminator:
		return nil
	}

	return fmt.Errorf("unknown volume type 0x%X", vd.Header.Type)
}

// UnmarshalBinary decodes a volumeDescriptor from binary form
func (vd volumeDescriptor) MarshalBinary() ([]byte, error) {
	var output []byte
	var err error

	switch vd.Header.Type {
	case volumeTypeBoot:
		return nil, errors.New("boot volumes are not yet supported")
	case volumeTypePartition:
		return nil, errors.New("partition volumes are not yet supported")
	case volumeTypePrimary, volumeTypeSupplementary:
		if output, err = vd.Primary.MarshalBinary(); err != nil {
			return nil, err
		}
	case volumeTypeTerminator:
		output = make([]byte, sectorSize)
	}

	data, err := vd.Header.MarshalBinary()
	if err != nil {
		return nil, err
	}

	copy(output[0:7], data)

	return output, nil
}

// VolumeDescriptorTimestamp represents a time and date format
// that can be encoded according to ECMA-119 8.4.26.1
type VolumeDescriptorTimestamp struct {
	Year      int
	Month     int
	Day       int
	Hour      int
	Minute    int
	Second    int
	Hundredth int
	Offset    int
}

var _ encoding.BinaryMarshaler = &VolumeDescriptorTimestamp{}
var _ encoding.BinaryUnmarshaler = &VolumeDescriptorTimestamp{}

// MarshalBinary encodes the timestamp into a binary form
func (ts *VolumeDescriptorTimestamp) MarshalBinary() ([]byte, error) {
	formatted := fmt.Sprintf("%04d%02d%02d%02d%02d%02d%02d", ts.Year, ts.Month, ts.Day, ts.Hour, ts.Minute, ts.Second, ts.Hundredth)
	formattedBytes := append([]byte(formatted), byte(ts.Offset))
	if len(formattedBytes) != 17 {
		return nil, fmt.Errorf("VolumeDescriptorTimestamp.MarshalBinary: the formatted timestamp is %d bytes long", len(formatted))
	}
	return formattedBytes, nil
}

// UnmarshalBinary decodes a VolumeDescriptorTimestamp from binary form
func (ts *VolumeDescriptorTimestamp) UnmarshalBinary(data []byte) error {
	if len(data) < 17 {
		return io.ErrUnexpectedEOF
	}

	year, err := strconv.Atoi(strings.TrimSpace(string(data[0:4])))
	if err != nil {
		return err
	}

	month, err := strconv.Atoi(strings.TrimSpace(string(data[4:6])))
	if err != nil {
		return err
	}

	day, err := strconv.Atoi(strings.TrimSpace(string(data[6:8])))
	if err != nil {
		return err
	}

	hour, err := strconv.Atoi(strings.TrimSpace(string(data[8:10])))
	if err != nil {
		return err
	}

	min, err := strconv.Atoi(strings.TrimSpace(string(data[10:12])))
	if err != nil {
		return err
	}

	sec, err := strconv.Atoi(strings.TrimSpace(string(data[12:14])))
	if err != nil {
		return err
	}

	hundredth, err := strconv.Atoi(strings.TrimSpace(string(data[14:16])))
	if err != nil {
		return err
	}

	*ts = VolumeDescriptorTimestamp{
		Year:      year,
		Month:     month,
		Day:       day,
		Hour:      hour,
		Minute:    min,
		Second:    sec,
		Hundredth: hundredth,
		Offset:    int(data[16]),
	}

	return nil
}

// RecordingTimestamp represents a time and date format
// that can be encoded according to ECMA-119 9.1.5
type RecordingTimestamp time.Time

var _ encoding.BinaryUnmarshaler = &RecordingTimestamp{}

// UnmarshalBinary decodes a RecordingTimestamp from binary form
func (ts *RecordingTimestamp) UnmarshalBinary(data []byte) error {
	if len(data) < 7 {
		return io.ErrUnexpectedEOF
	}

	year := 1900 + int(data[0])
	month := int(data[1])
	day := int(data[2])
	hour := int(data[3])
	min := int(data[4])
	sec := int(data[5])
	tzOffset := int(data[6])
	secondsInAQuarter := 60 * 15

	tz := time.FixedZone("", tzOffset*secondsInAQuarter)
	*ts = RecordingTimestamp(time.Date(year, time.Month(month), day, hour, min, sec, 0, tz))
	return nil
}

// MarshalBinary encodes the RecordingTimestamp in its binary form to a buffer
// of the length of 7 or more bytes
func (ts RecordingTimestamp) MarshalBinary(dst []byte) {
	_ = dst[6] // early bounds check to guarantee safety of writes below
	t := time.Time(ts)
	year, month, day := t.Date()
	hour, min, sec := t.Clock()
	_, secOffset := t.Zone()
	secondsInAQuarter := 60 * 15
	offsetInQuarters := secOffset / secondsInAQuarter
	dst[0] = byte(year - 1900)
	dst[1] = byte(month)
	dst[2] = byte(day)
	dst[3] = byte(hour)
	dst[4] = byte(min)
	dst[5] = byte(sec)
	dst[6] = byte(offsetInQuarters)
}

// VolumeDescriptorTimestampFromTime converts time.Time to VolumeDescriptorTimestamp
func VolumeDescriptorTimestampFromTime(t time.Time) VolumeDescriptorTimestamp {
	t = t.UTC()
	year, month, day := t.Date()
	hour, minute, second := t.Clock()
	hundredth := t.Nanosecond() / 10000000
	return VolumeDescriptorTimestamp{
		Year:      year,
		Month:     int(month),
		Day:       day,
		Hour:      hour,
		Minute:    minute,
		Second:    second,
		Hundredth: hundredth,
		Offset:    0, // we converted to UTC
	}
}
