package iso9660

import (
	"bytes"
	"container/list"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"time"
)

const (
	primaryVolumeDirectoryIdentifierMaxLength = 31 // ECMA-119 7.6.3
	primaryVolumeFileIdentifierMaxLength      = 30 // ECMA-119 7.5
)

var (
	// ErrFileTooLarge is returned when trying to process a file of size greater
	// than 4GB, which due to the 32-bit address limitation is not possible
	// except with ISO 9660-Level 3
	ErrFileTooLarge = errors.New("file is exceeding the maximum file size of 4GB")
)

// ImageWriter is responsible for staging an image's contents
// and writing them to an image.
type ImageWriter struct {
	stagingDir string
}

// NewWriter creates a new ImageWrite and initializes its temporary staging dir.
// Cleanup should be called after the ImageWriter is no longer needed.
func NewWriter() (*ImageWriter, error) {
	tmp, err := os.MkdirTemp("", "")
	if err != nil {
		return nil, err
	}

	return &ImageWriter{stagingDir: tmp}, nil
}

// Cleanup deletes the underlying temporary staging directory of an ImageWriter.
// It can be called multiple times without issues.
func (iw *ImageWriter) Cleanup() error {
	if iw.stagingDir == "" {
		return nil
	}

	if err := os.RemoveAll(iw.stagingDir); err != nil {
		return err
	}

	iw.stagingDir = ""
	return nil
}

// AddFile adds a file to the ImageWriter's staging area.
// All path components are mangled to match basic ISO9660 filename requirements.
func (iw *ImageWriter) AddFile(data io.Reader, filePath string) error {
	directoryPath, fileName := manglePath(filePath)

	if err := os.MkdirAll(path.Join(iw.stagingDir, directoryPath), 0755); err != nil {
		return err
	}

	f, err := os.OpenFile(path.Join(iw.stagingDir, directoryPath, fileName), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, data)
	return err
}

func failIfSymlink(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}

	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%q is a symlink - these are not yet supported", path)
	}

	return nil
}

// AddLocalFile adds a file identified by its path to the ImageWriter's staging area.
func (iw *ImageWriter) AddLocalFile(origin, target string) error {
	if err := failIfSymlink(origin); err != nil {
		return err
	}

	directoryPath, fileName := manglePath(target)

	if err := os.MkdirAll(path.Join(iw.stagingDir, directoryPath), 0755); err != nil {
		return err
	}

	// try to hardlink file to staging area before copying.
	stagedFile := path.Join(iw.stagingDir, directoryPath, fileName)
	if err := os.Remove(stagedFile); err != nil && !os.IsNotExist(err) {
		return err
	}

	if err := os.Link(origin, stagedFile); err == nil {
		return nil
	}

	f, err := os.Open(origin)
	if err != nil {
		return err
	}

	defer f.Close()

	return iw.AddFile(f, target)
}

func ensureIsDirectory(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	fileinfo, err := f.Stat()
	if err != nil {
		return err
	}

	if !fileinfo.IsDir() {
		return fmt.Errorf("%q is not a directory", path)
	}

	return nil
}

// AddLocalDirectory adds a directory recursively to the ImageWriter's staging area.
func (iw *ImageWriter) AddLocalDirectory(origin, target string) error {
	if err := ensureIsDirectory(origin); err != nil {
		return err
	}

	walkfn := func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		relPath := path[len(origin):] // We need the path to be relative to the origin.
		return iw.AddLocalFile(path, filepath.Join(target, relPath))
	}

	return filepath.Walk(origin, walkfn)
}

func manglePath(input string) (string, string) {
	input = posixifyPath(input)

	nonEmptySegments := splitPath(input)

	dirSegments := nonEmptySegments[:len(nonEmptySegments)-1]
	name := nonEmptySegments[len(nonEmptySegments)-1]

	for i := 0; i < len(dirSegments); i++ {
		dirSegments[i] = mangleDirectoryName(dirSegments[i])
	}
	name = mangleFileName(name)

	return path.Join(dirSegments...), name
}

// Converts given path to Posix (replacing \ with /)
//
// @param {string} givenPath Path to convert
//
// @returns {string} Converted filepath
func posixifyPath(path string) string {
	if runtime.GOOS == "windows" {
		return strings.ReplaceAll(path, "\\", "/")
	}
	return path
}

func splitPath(input string) []string {
	rawSegments := strings.Split(input, "/")
	var nonEmptySegments []string
	for _, s := range rawSegments {
		if len(s) > 0 {
			nonEmptySegments = append(nonEmptySegments, s)
		}
	}
	return nonEmptySegments
}

// See ECMA-119 7.5
func mangleFileName(input string) string {
	// https://github.com/torvalds/linux/blob/v5.6/fs/isofs/dir.c#L29
	input = strings.ToLower(input)
	split := strings.Split(input, ".")

	version := "1"
	var filename, extension string
	if len(split) == 1 {
		filename = split[0]
	} else {
		filename = strings.Join(split[:len(split)-1], "_")
		extension = split[len(split)-1]
	}

	// enough characters for the `.ignition` extension
	extension = mangleD1String(extension, 8)

	maxRemainingFilenameLength := primaryVolumeFileIdentifierMaxLength - (1 + len(version))
	if len(extension) > 0 {
		maxRemainingFilenameLength -= (1 + len(extension))
	}

	filename = mangleD1String(filename, maxRemainingFilenameLength)

	if len(extension) > 0 {
		return filename + "." + extension + ";" + version
	}

	return filename + ";" + version
}

// See ECMA-119 7.6
func mangleDirectoryName(input string) string {
	return mangleD1String(input, primaryVolumeDirectoryIdentifierMaxLength)
}

func mangleD1String(input string, maxCharacters int) string {
	// https://github.com/torvalds/linux/blob/v5.6/fs/isofs/dir.c#L29
	input = strings.ToLower(input)

	var mangledString string
	for i := 0; i < len(input) && i < maxCharacters; i++ {
		r := rune(input[i])
		if strings.ContainsRune(d1Characters, r) {
			mangledString += string(r)
		} else {
			mangledString += "_"
		}
	}

	return mangledString
}

// calculateDirChildrenSectors calculates the total mashalled size of all DirectoryEntries
// within a directory. The size of each entry depends of the length of the filename.
func calculateDirChildrenSectors(path string) (uint32, error) {
	contents, err := os.ReadDir(path)
	if err != nil {
		return 0, err
	}

	var sectors uint32
	var currentSectorOccupied uint32 = 68 // the 0x00 and 0x01 entries

	for _, c := range contents {
		identifierLen := len(c.Name())
		idPaddingLen := (identifierLen + 1) % 2
		entryLength := uint32(33 + identifierLen + idPaddingLen)

		if currentSectorOccupied+entryLength > sectorSize {
			sectors++
			currentSectorOccupied = entryLength
		} else {
			currentSectorOccupied += entryLength
		}
	}

	if currentSectorOccupied > 0 {
		sectors++
	}

	return sectors, nil
}

func fileLengthToSectors(l uint32) uint32 {
	if (l % sectorSize) == 0 {
		return l / sectorSize
	}

	return (l / sectorSize) + 1
}

type writeContext struct {
	stagingDir        string
	timestamp         RecordingTimestamp
	freeSectorPointer uint32
}

func (wc *writeContext) allocateSectors(n uint32) uint32 {
	return atomic.AddUint32(&wc.freeSectorPointer, n) - n
}

func (wc *writeContext) createDEForRoot() (*DirectoryEntry, error) {
	extentLengthInSectors, err := calculateDirChildrenSectors(wc.stagingDir)
	if err != nil {
		return nil, err
	}

	extentLocation := wc.allocateSectors(extentLengthInSectors)
	de := &DirectoryEntry{
		ExtendedAtributeRecordLength: 0,
		ExtentLocation:               int32(extentLocation),
		ExtentLength:                 uint32(extentLengthInSectors * sectorSize),
		RecordingDateTime:            wc.timestamp,
		FileFlags:                    dirFlagDir,
		FileUnitSize:                 0, // 0 for non-interleaved write
		InterleaveGap:                0, // not interleaved
		VolumeSequenceNumber:         1, // we only have one volume
		Identifier:                   string([]byte{0}),
		SystemUse:                    []byte{},
	}
	return de, nil
}

type itemToWrite struct {
	isDirectory     bool
	dirPath         string
	ownEntry        *DirectoryEntry
	parentEntery    *DirectoryEntry
	childrenEntries []*DirectoryEntry
	targetSector    uint32
}

// scanDirectory reads the directory's contents and adds them to the queue, as well as stores all their DirectoryEntries in the item,
// because we'll need them to write this item's descriptor.
func (wc *writeContext) scanDirectory(item *itemToWrite, dirPath string, ownEntry *DirectoryEntry, parentEntery *DirectoryEntry, targetSector uint32) (*list.List, error) {
	contents, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	itemsToWrite := list.New()

	for _, c := range contents {
		var (
			fileFlags             byte
			extentLengthInSectors uint32
			extentLength          uint32
		)
		if c.IsDir() {
			extentLengthInSectors, err = calculateDirChildrenSectors(path.Join(dirPath, c.Name()))
			if err != nil {
				return nil, err
			}
			fileFlags = dirFlagDir
			extentLength = extentLengthInSectors * sectorSize
		} else {
			fileinfo, err := c.Info()
			if err != nil {
				return nil, err
			}
			if fileinfo.Size() > int64(math.MaxUint32) {
				return nil, ErrFileTooLarge
			}
			extentLength = uint32(fileinfo.Size())
			extentLengthInSectors = fileLengthToSectors(extentLength)

			fileFlags = 0
		}

		extentLocation := wc.allocateSectors(extentLengthInSectors)
		de := &DirectoryEntry{
			ExtendedAtributeRecordLength: 0,
			ExtentLocation:               int32(extentLocation),
			ExtentLength:                 uint32(extentLength),
			RecordingDateTime:            wc.timestamp,
			FileFlags:                    fileFlags,
			FileUnitSize:                 0, // 0 for non-interleaved write
			InterleaveGap:                0, // not interleaved
			VolumeSequenceNumber:         1, // we only have one volume
			Identifier:                   c.Name(),
			SystemUse:                    []byte{},
		}

		// Add this child's descriptor to the currently scanned directory's list of children,
		// so that later we can use it for writing the current item.
		if item.childrenEntries == nil {
			item.childrenEntries = []*DirectoryEntry{de}
		} else {
			item.childrenEntries = append(item.childrenEntries, de)
		}

		// queue this child for processing
		itemsToWrite.PushBack(itemToWrite{
			isDirectory:  c.IsDir(),
			dirPath:      path.Join(dirPath, c.Name()),
			ownEntry:     de,
			parentEntery: ownEntry,
			targetSector: uint32(de.ExtentLocation),
		})
	}

	return itemsToWrite, nil
}

// processDirectory writes a given directory item to the destination sectors
func processDirectory(w io.Writer, children []*DirectoryEntry, ownEntry *DirectoryEntry, parentEntry *DirectoryEntry) error {
	var currentOffset uint32

	currentDE := ownEntry.Clone()
	currentDE.Identifier = string([]byte{0})
	parentDE := parentEntry.Clone()
	parentDE.Identifier = string([]byte{1})

	currentDEData, err := currentDE.MarshalBinary()
	if err != nil {
		return err
	}
	parentDEData, err := parentDE.MarshalBinary()
	if err != nil {
		return err
	}

	n, err := w.Write(currentDEData)
	if err != nil {
		return err
	}
	currentOffset += uint32(n)
	n, err = w.Write(parentDEData)
	if err != nil {
		return err
	}
	currentOffset += uint32(n)

	for _, childDescriptor := range children {
		data, err := childDescriptor.MarshalBinary()
		if err != nil {
			return err
		}

		remainingSectorSpace := sectorSize - (currentOffset % sectorSize)
		if remainingSectorSpace < uint32(len(data)) {
			// ECMA-119 6.8.1.1 If the body of the next descriptor won't fit into the sector,
			// we fill the rest of space with zeros and skip to the next sector.
			zeros := bytes.Repeat([]byte{0}, int(remainingSectorSpace))
			_, err = w.Write(zeros)
			if err != nil {
				return err
			}

			// skip to the next sector
			currentOffset = 0
		}

		n, err = w.Write(data)
		if err != nil {
			return err
		}
		currentOffset += uint32(n)
	}

	// fill with zeros to the end of the sector
	remainingSectorSpace := sectorSize - (currentOffset % sectorSize)
	if remainingSectorSpace != 0 {
		zeros := bytes.Repeat([]byte{0}, int(remainingSectorSpace))
		_, err = w.Write(zeros)
		if err != nil {
			return err
		}
	}

	return nil
}

func processFile(w io.Writer, dirPath string) error {
	f, err := os.Open(dirPath)
	if err != nil {
		return err
	}
	defer f.Close()

	fileinfo, err := f.Stat()
	if err != nil {
		return err
	}

	if fileinfo.Size() > int64(math.MaxUint32) {
		return ErrFileTooLarge
	}

	buffer := make([]byte, sectorSize)

	for bytesLeft := uint32(fileinfo.Size()); bytesLeft > 0; {
		var toRead uint32
		if bytesLeft < sectorSize {
			toRead = bytesLeft
		} else {
			toRead = sectorSize
		}

		if _, err = io.ReadAtLeast(f, buffer, int(toRead)); err != nil {
			return err
		}

		if _, err = w.Write(buffer); err != nil {
			return err
		}

		bytesLeft -= toRead
	}
	// We already write a whole sector-sized buffer, so there's need to fill with zeroes.

	return nil
}

// traverseStagingDir creates a new queue of items to write by traversing the staging directory
func (wc *writeContext) traverseStagingDir(rootItem itemToWrite) (*list.List, error) {
	itemsToWrite := list.New()
	itemsToWrite.PushBack(rootItem)

	for item := itemsToWrite.Front(); item != nil; item = item.Next() {
		it := item.Value.(itemToWrite)

		if it.isDirectory {
			newItems, err := wc.scanDirectory(&it, it.dirPath, it.ownEntry, it.parentEntery, it.targetSector)
			if err != nil {
				relativePath := it.dirPath[len(wc.stagingDir):]
				return nil, fmt.Errorf("processing %s: %s", relativePath, err)
			}
			itemsToWrite.PushBackList(newItems)
		}

		item.Value = it
	}

	return itemsToWrite, nil
}

func writeAll(w io.Writer, itemsToWrite *list.List) error {
	for item := itemsToWrite.Front(); item != nil; item = item.Next() {
		it := item.Value.(itemToWrite)
		var err error
		if it.isDirectory {
			err = processDirectory(w, it.childrenEntries, it.ownEntry, it.parentEntery)
		} else {
			err = processFile(w, it.dirPath)
		}

		if err != nil {
			return err
		}
	}

	return nil
}

// WriteTo writes the image to the given WriterAt
func (iw *ImageWriter) WriteTo(w io.Writer, volumeIdentifier string) error {
	now := time.Now()

	wc := writeContext{
		stagingDir:        iw.stagingDir,
		timestamp:         RecordingTimestamp{},
		freeSectorPointer: 18, // system area (16) + 2 volume descriptors
	}

	rootDE, err := wc.createDEForRoot()
	if err != nil {
		return fmt.Errorf("creating root directory descriptor: %s", err)
	}

	rootItem := itemToWrite{
		isDirectory:  true,
		dirPath:      wc.stagingDir,
		ownEntry:     rootDE,
		parentEntery: rootDE,
		targetSector: uint32(rootDE.ExtentLocation),
	}

	itemsToWrite, err := wc.traverseStagingDir(rootItem)
	if err != nil {
		return fmt.Errorf("tranversing staging directory: %s", err)
	}

	pvd := volumeDescriptor{
		Header: volumeDescriptorHeader{
			Type:       volumeTypePrimary,
			Identifier: standardIdentifierBytes,
			Version:    1,
		},
		Primary: &PrimaryVolumeDescriptorBody{
			SystemIdentifier:              runtime.GOOS,
			VolumeIdentifier:              volumeIdentifier,
			VolumeSpaceSize:               int32(wc.freeSectorPointer),
			VolumeSetSize:                 1,
			VolumeSequenceNumber:          1,
			LogicalBlockSize:              int16(sectorSize),
			PathTableSize:                 0,
			TypeLPathTableLoc:             0,
			OptTypeLPathTableLoc:          0,
			TypeMPathTableLoc:             0,
			OptTypeMPathTableLoc:          0,
			RootDirectoryEntry:            rootDE,
			VolumeSetIdentifier:           "",
			PublisherIdentifier:           "",
			DataPreparerIdentifier:        "",
			ApplicationIdentifier:         "github.com/kdomanski/iso9660",
			CopyrightFileIdentifier:       "",
			AbstractFileIdentifier:        "",
			BibliographicFileIdentifier:   "",
			VolumeCreationDateAndTime:     VolumeDescriptorTimestampFromTime(now),
			VolumeModificationDateAndTime: VolumeDescriptorTimestampFromTime(now),
			VolumeExpirationDateAndTime:   VolumeDescriptorTimestamp{},
			VolumeEffectiveDateAndTime:    VolumeDescriptorTimestampFromTime(now),
			FileStructureVersion:          1,
			ApplicationUsed:               [512]byte{},
		},
	}

	terminator := volumeDescriptor{
		Header: volumeDescriptorHeader{
			Type:       volumeTypeTerminator,
			Identifier: standardIdentifierBytes,
			Version:    1,
		},
	}

	// write 16 sectors of zeroes
	zeroSector := bytes.Repeat([]byte{0}, int(sectorSize))
	for i := uint32(0); i < 16; i++ {
		if _, err = w.Write(zeroSector); err != nil {
			return err
		}
	}

	buffer, err := pvd.MarshalBinary()
	if err != nil {
		return err
	}
	if _, err = w.Write(buffer); err != nil {
		return err
	}

	if buffer, err = terminator.MarshalBinary(); err != nil {
		return err
	}
	if _, err = w.Write(buffer); err != nil {
		return err
	}

	if err = writeAll(w, itemsToWrite); err != nil {
		return fmt.Errorf("writing files: %s", err)
	}

	return nil
}
