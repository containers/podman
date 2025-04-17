package iso9660

import (
	"fmt"
	"io/fs"
	"os"
)

/* The following types of Rock Ridge records are being handled in some way:
 * - [X] PX (RR 4.1.1: POSIX file attributes)
 * - [ ] PN (RR 4.1.2: POSIX device number)
 * - [ ] SL (RR 4.1.3: symbolic link)
 * - [x] NM (RR 4.1.4: alternate name)
 * - [ ] CL (RR 4.1.5.1: child link)
 * - [ ] PL (RR 4.1.5.2: parent link)
 * - [ ] RE (RR 4.1.5.3: relocated directory)
 * - [ ] TF (RR 4.1.6: time stamp(s) for a file)
 * - [ ] SF (RR 4.1.7: file data in sparse file format)
 */

const (
	RockRidgeIdentifier = "RRIP_1991A"
	RockRidgeVersion    = 1
)

type RockRidgeNameEntry struct {
	Flags byte
	Name  string
}

func suspHasRockRidge(se SystemUseEntrySlice) (bool, error) {
	extensions, err := se.GetExtensionRecords()
	if err != nil {
		return false, err
	}

	for _, entry := range extensions {
		if entry.Identifier == RockRidgeIdentifier && entry.Version == RockRidgeVersion {
			return true, nil
		}
	}

	return false, nil
}

func (s SystemUseEntrySlice) GetRockRidgeName() string {
	var name string

	for _, entry := range s {
		// There is a continuation flag in the record, but we determine continuation
		// by simply reading all NM entries.
		if entry.Type() == "NM" {
			nm := umarshalRockRidgeNameEntry(entry)
			name += nm.Name
		}
	}

	return name
}

func (s SystemUseEntrySlice) GetPosixAttr() (fs.FileMode, error) {
	for _, entry := range s {
		if entry.Type() == "PX" {
			// BUG(kdomanski): If there are multiple RR PX entries (which is forbidden by the spec), the reader will use the first one.
			return umarshalRockRidgeAttrEntry(entry)
		}
	}

	return 0, fmt.Errorf("mandatory entry PX not found")
}

func umarshalRockRidgeAttrEntry(e SystemUseEntry) (fs.FileMode, error) {
	rrMode, err := UnmarshalUint32LSBMSB(e.Data()[0:8])
	if err != nil {
		return 0, fmt.Errorf("unmarshall RR PX entry: %w", err)
	}

	S_IFLNK := (rrMode & 0170000) == 0120000
	S_IFDIR := (rrMode & 0170000) == 0040000

	mode := rrMode & uint32(fs.ModePerm) // UNIX permissions

	if S_IFLNK {
		mode |= uint32(os.ModeSymlink)
	}

	if S_IFDIR {
		mode |= uint32(os.ModeDir)
	}

	return fs.FileMode(mode), nil
}

func umarshalRockRidgeNameEntry(e SystemUseEntry) *RockRidgeNameEntry {
	return &RockRidgeNameEntry{
		Flags: e.Data()[0],
		Name:  string(e.Data()[1:]),
	}
}
