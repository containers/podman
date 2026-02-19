package sftp

// ssh_FXP_ATTRS support
// see https://filezilla-project.org/specs/draft-ietf-secsh-filexfer-02.txt#section-5

import (
	"os"
	"time"
)

const (
	sshFileXferAttrSize        = 0x00000001
	sshFileXferAttrUIDGID      = 0x00000002
	sshFileXferAttrPermissions = 0x00000004
	sshFileXferAttrACmodTime   = 0x00000008
	sshFileXferAttrExtended    = 0x80000000

	sshFileXferAttrAll = sshFileXferAttrSize | sshFileXferAttrUIDGID | sshFileXferAttrPermissions |
		sshFileXferAttrACmodTime | sshFileXferAttrExtended
)

// fileInfo is an artificial type designed to satisfy os.FileInfo.
type fileInfo struct {
	name string
	stat *FileStat
}

// Name returns the base name of the file.
func (fi *fileInfo) Name() string { return fi.name }

// Size returns the length in bytes for regular files; system-dependent for others.
func (fi *fileInfo) Size() int64 { return int64(fi.stat.Size) }

// Mode returns file mode bits.
func (fi *fileInfo) Mode() os.FileMode { return toFileMode(fi.stat.Mode) }

// ModTime returns the last modification time of the file.
func (fi *fileInfo) ModTime() time.Time { return time.Unix(int64(fi.stat.Mtime), 0) }

// IsDir returns true if the file is a directory.
func (fi *fileInfo) IsDir() bool { return fi.Mode().IsDir() }

func (fi *fileInfo) Sys() interface{} { return fi.stat }

// FileStat holds the original unmarshalled values from a call to READDIR or
// *STAT. It is exported for the purposes of accessing the raw values via
// os.FileInfo.Sys(). It is also used server side to store the unmarshalled
// values for SetStat.
type FileStat struct {
	Size     uint64
	Mode     uint32
	Mtime    uint32
	Atime    uint32
	UID      uint32
	GID      uint32
	Extended []StatExtended
}

// StatExtended contains additional, extended information for a FileStat.
type StatExtended struct {
	ExtType string
	ExtData string
}

func fileInfoFromStat(stat *FileStat, name string) os.FileInfo {
	return &fileInfo{
		name: name,
		stat: stat,
	}
}

// FileInfoUidGid extends os.FileInfo and adds callbacks for Uid and Gid retrieval,
// as an alternative to *syscall.Stat_t objects on unix systems.
type FileInfoUidGid interface {
	os.FileInfo
	Uid() uint32
	Gid() uint32
}

// FileInfoUidGid extends os.FileInfo and adds a callbacks for extended data retrieval.
type FileInfoExtendedData interface {
	os.FileInfo
	Extended() []StatExtended
}

func fileStatFromInfo(fi os.FileInfo) (uint32, *FileStat) {
	mtime := fi.ModTime().Unix()
	atime := mtime
	var flags uint32 = sshFileXferAttrSize |
		sshFileXferAttrPermissions |
		sshFileXferAttrACmodTime

	fileStat := &FileStat{
		Size:  uint64(fi.Size()),
		Mode:  fromFileMode(fi.Mode()),
		Mtime: uint32(mtime),
		Atime: uint32(atime),
	}

	// os specific file stat decoding
	fileStatFromInfoOs(fi, &flags, fileStat)

	// The call above will include the sshFileXferAttrUIDGID in case
	// the os.FileInfo can be casted to *syscall.Stat_t on unix.
	// If fi implements FileInfoUidGid, retrieve Uid, Gid from it instead.
	if fiExt, ok := fi.(FileInfoUidGid); ok {
		flags |= sshFileXferAttrUIDGID
		fileStat.UID = fiExt.Uid()
		fileStat.GID = fiExt.Gid()
	}

	// if fi implements FileInfoExtendedData, retrieve extended data from it
	if fiExt, ok := fi.(FileInfoExtendedData); ok {
		fileStat.Extended = fiExt.Extended()
		if len(fileStat.Extended) > 0 {
			flags |= sshFileXferAttrExtended
		}
	}

	return flags, fileStat
}
