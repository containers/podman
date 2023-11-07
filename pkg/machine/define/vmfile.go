package define

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

const MaxSocketPathLength int = 103

type VMFile struct {
	// Path is the fully qualified path to a file
	Path string
	// Symlink is a shortened version of Path by using
	// a symlink
	Symlink *string `json:"symlink,omitempty"`
}

// GetPath returns the working path for a machinefile.  it returns
// the symlink unless one does not exist
func (m *VMFile) GetPath() string {
	if m.Symlink == nil {
		return m.Path
	}
	return *m.Symlink
}

// Delete removes the machinefile symlink (if it exists) and
// the actual path
func (m *VMFile) Delete() error {
	if m.Symlink != nil {
		if err := os.Remove(*m.Symlink); err != nil && !errors.Is(err, os.ErrNotExist) {
			logrus.Errorf("unable to remove symlink %q", *m.Symlink)
		}
	}
	if err := os.Remove(m.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// Read the contents of a given file and return in []bytes
func (m *VMFile) Read() ([]byte, error) {
	return os.ReadFile(m.GetPath())
}

// NewMachineFile is a constructor for VMFile
func NewMachineFile(path string, symlink *string) (*VMFile, error) {
	if len(path) < 1 {
		return nil, errors.New("invalid machine file path")
	}
	if symlink != nil && len(*symlink) < 1 {
		return nil, errors.New("invalid symlink path")
	}
	mf := VMFile{Path: path}
	if symlink != nil && len(path) > MaxSocketPathLength {
		if err := mf.makeSymlink(symlink); err != nil && !errors.Is(err, os.ErrExist) {
			return nil, err
		}
	}
	return &mf, nil
}

// makeSymlink for macOS creates a symlink in $HOME/.podman/
// for a machinefile like a socket
func (m *VMFile) makeSymlink(symlink *string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	sl := filepath.Join(homeDir, ".podman", *symlink)
	// make the symlink dir and throw away if it already exists
	if err := os.MkdirAll(filepath.Dir(sl), 0700); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	m.Symlink = &sl
	return os.Symlink(m.Path, sl)
}
