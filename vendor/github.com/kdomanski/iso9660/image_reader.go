package iso9660

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// Image is a wrapper around an image file that allows reading its ISO9660 data
type Image struct {
	ra                io.ReaderAt
	volumeDescriptors []volumeDescriptor
}

// OpenImage returns an Image reader reating from a given file
func OpenImage(ra io.ReaderAt) (*Image, error) {
	i := &Image{ra: ra}

	if err := i.readVolumes(); err != nil {
		return nil, err
	}

	return i, nil
}

func (i *Image) readVolumes() error {
	buffer := make([]byte, sectorSize)
	// skip the 16 sectors of system area
	for sector := 16; ; sector++ {
		if _, err := i.ra.ReadAt(buffer, int64(sector)*int64(sectorSize)); err != nil {
			return err
		}

		var vd volumeDescriptor
		if err := vd.UnmarshalBinary(buffer); err != nil {
			return err
		}

		// NOTE: the instance of the root Directory Record that appears
		// in the Primary Volume Descriptor cannot contain a System Use
		// field. See the SUSP standard.

		i.volumeDescriptors = append(i.volumeDescriptors, vd)
		if vd.Header.Type == volumeTypeTerminator {
			break
		}
	}

	return nil
}

// RootDir returns the File structure corresponding to the root directory
// of the first primary volume
func (i *Image) RootDir() (*File, error) {
	for _, vd := range i.volumeDescriptors {
		if vd.Type() == volumeTypePrimary {
			return &File{de: vd.Primary.RootDirectoryEntry, ra: i.ra, children: nil, isRootDir: true}, nil
		}
	}
	return nil, os.ErrNotExist
}

// RootDir returns the label of the first Primary Volume
func (i *Image) Label() (string, error) {
	for _, vd := range i.volumeDescriptors {
		if vd.Type() == volumeTypePrimary {
			return string(vd.Primary.VolumeIdentifier), nil
		}
	}
	return "", os.ErrNotExist
}

// File is a os.FileInfo-compatible wrapper around an ISO9660 directory entry
type File struct {
	ra        io.ReaderAt
	de        *DirectoryEntry
	children  []*File
	isRootDir bool
	susp      *SUSPMetadata
}

var _ os.FileInfo = &File{}

func (f *File) hasRockRidge() bool {
	return f.susp != nil && f.susp.HasRockRidge
}

// IsDir returns true if the entry is a directory or false otherwise
func (f *File) IsDir() bool {
	if f.hasRockRidge() {
		if mode, err := f.de.SystemUseEntries.GetPosixAttr(); err == nil {
			return mode&os.ModeDir != 0
		}
	}

	return f.de.FileFlags&dirFlagDir != 0
}

// ModTime returns the entry's recording time
func (f *File) ModTime() time.Time {
	return time.Time(f.de.RecordingDateTime)
}

// Mode returns file mode when available.
// Otherwise it returns os.FileMode flag set with the os.ModeDir flag enabled in case of directories.
func (f *File) Mode() os.FileMode {
	if f.hasRockRidge() {
		if mode, err := f.de.SystemUseEntries.GetPosixAttr(); err == nil {
			return mode
		}
	}

	var mode os.FileMode
	if f.IsDir() {
		mode |= os.ModeDir
	}
	return mode
}

// Name returns the base name of the given entry
func (f *File) Name() string {
	if f.hasRockRidge() {
		if name := f.de.SystemUseEntries.GetRockRidgeName(); name != "" {
			return name
		}
	}

	if f.IsDir() {
		return f.de.Identifier
	}

	// drop the version part
	// assume only one ';'
	fileIdentifier := strings.Split(f.de.Identifier, ";")[0]

	// split into filename and extension
	// assume only only one '.'
	splitFileIdentifier := strings.Split(fileIdentifier, ".")

	// there's no dot in the name, thus no extension
	if len(splitFileIdentifier) == 1 {
		return splitFileIdentifier[0]
	}

	// extension is empty, return just the name without a dot
	if len(splitFileIdentifier[1]) == 0 {
		return splitFileIdentifier[0]
	}

	// return file with extension
	return fileIdentifier
}

// Size returns the size in bytes of the extent occupied by the file or directory
func (f *File) Size() int64 {
	return int64(f.de.ExtentLength)
}

// Sys returns nil
func (f *File) Sys() interface{} {
	return nil
}

// GetAllChildren returns the children entries in case of a directory
// or an error in case of a file. It includes the "." and ".." entries.
func (f *File) GetAllChildren() ([]*File, error) {
	if !f.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", f.Name())
	}

	if f.children != nil {
		return f.children, nil
	}

	baseOffset := uint32(f.de.ExtentLocation) * sectorSize

	buffer := make([]byte, sectorSize)
	for bytesProcessed := uint32(0); bytesProcessed < uint32(f.de.ExtentLength); bytesProcessed += sectorSize {
		if _, err := f.ra.ReadAt(buffer, int64(baseOffset+bytesProcessed)); err != nil {
			return nil, nil
		}

		for i := uint32(0); i < sectorSize; {
			entryLength := uint32(buffer[i])
			if entryLength == 0 {
				break
			}

			if i+entryLength > sectorSize {
				return nil, fmt.Errorf("reading directory entries: DE outside of sector boundries")
			}

			newDE := &DirectoryEntry{}
			if err := newDE.UnmarshalBinary(buffer[i : i+entryLength]); err != nil {
				return nil, err
			}

			// Is this a root directory '.' record?
			if f.isRootDir && newDE.Identifier == string([]byte{0}) {
				newDE.SystemUseEntries, _ = splitSystemUseEntries(newDE.SystemUse, f.ra)

				// get the SP record
				if len(newDE.SystemUseEntries) > 0 && newDE.SystemUseEntries[0].Type() == "SP" {
					sprecord, err := SPRecordDecode(newDE.SystemUseEntries[0])
					if err != nil {
						return nil, fmt.Errorf("invalid SP record: %w", err)
					}

					hasRockRidge, err := suspHasRockRidge(newDE.SystemUseEntries)
					if err != nil {
						return nil, fmt.Errorf("failed to check for Rock Ridge extension: %w", err)
					}

					// save SUSP offset from the SP record
					f.susp = &SUSPMetadata{
						Offset:       sprecord.BytesSkipped,
						HasRockRidge: hasRockRidge,
					}
				}
			} else {
				// are we on a volume with SUSP?
				if f.susp != nil {
					// Ignore error if some of the SUSP data is malformed. Just take the valid part.
					offsetSystemUse := newDE.SystemUse[f.susp.Offset:]
					newDE.SystemUseEntries, _ = splitSystemUseEntries(offsetSystemUse, f.ra)
				}
			}

			i += entryLength

			newFile := &File{ra: f.ra,
				de:       newDE,
				children: nil,
				susp:     f.susp.Clone(),
			}

			f.children = append(f.children, newFile)
		}
	}

	return f.children, nil
}

// GetChildren returns the children entries in case of a directory
// or an error in case of a file. It does NOT include the "." and ".." entries.
func (f *File) GetChildren() ([]*File, error) {
	children, err := f.GetAllChildren()
	if err != nil {
		return nil, err
	}

	filteredChildren := make([]*File, 0, len(children)-2)
	for _, child := range children {
		if child.de.Identifier == string([]byte{0}) || child.de.Identifier == string([]byte{1}) {
			continue
		}

		filteredChildren = append(filteredChildren, child)
	}

	return filteredChildren, nil
}

// GetDotEntry returns the "." entry of a directory
// or an error in case of a file.
func (f *File) GetDotEntry() (*File, error) {
	children, err := f.GetAllChildren()
	if err != nil {
		return nil, err
	}

	for _, child := range children {
		if child.de.Identifier == string([]byte{0}) {
			return child, nil
		}
	}

	return nil, nil
}

// Reader returns a reader that allows to read the file's data.
// If File is a directory, it returns nil.
func (f *File) Reader() io.Reader {
	if f.IsDir() {
		return nil
	}

	baseOffset := int64(f.de.ExtentLocation) * int64(sectorSize)
	return io.NewSectionReader(f.ra, baseOffset, int64(f.de.ExtentLength))
}
