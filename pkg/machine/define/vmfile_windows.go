package define

import (
	"errors"
	"fmt"
	"golang.org/x/sys/windows"
	"os"
	"os/exec"

	"github.com/sirupsen/logrus"
)

// Delete removes the machinefile symlink (if it exists) and
// the actual path
func (m *VMFile) Delete() error {
	if m.Symlink != nil {
		if err := os.Remove(*m.Symlink); err != nil && !errors.Is(err, os.ErrNotExist) {
			logrus.Errorf("unable to remove symlink %q", *m.Symlink)
		}
	}
	if err := os.Remove(m.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
		if errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
			fmt.Println("BAUDE BAUDE BAUDE BAUDE")
			cmd := exec.Command("handle", []string{m.GetPath()}...)
			b, err := cmd.CombinedOutput()
			if err != nil {
				fmt.Println(err)
			}
			fmt.Println(string(b))
			fmt.Println("BAUDE BAUDE BAUDE BAUDE")
		}
		return err
	}
	return nil
}
