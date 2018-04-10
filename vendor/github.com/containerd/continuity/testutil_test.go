package continuity

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
)

func tree(w io.Writer, dir string) error {
	fmt.Fprintf(w, "%s\n", dir)
	return _tree(w, dir, "")
}

func _tree(w io.Writer, dir string, indent string) error {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return err
	}
	for i, f := range files {
		fPath := filepath.Join(dir, f.Name())
		fIndent := indent
		if i < len(files)-1 {
			fIndent += "|-- "
		} else {
			fIndent += "`-- "
		}
		target := ""
		if f.Mode()&os.ModeSymlink == os.ModeSymlink {
			realPath, err := os.Readlink(fPath)
			if err != nil {
				target += fmt.Sprintf(" -> unknown (error: %v)", err)
			} else {
				target += " -> " + realPath
			}
		}
		fmt.Fprintf(w, "%s%s%s\n",
			fIndent, f.Name(), target)
		if f.IsDir() {
			dIndent := indent
			if i < len(files)-1 {
				dIndent += "|   "
			} else {
				dIndent += "    "
			}
			if err := _tree(w, fPath, dIndent); err != nil {
				return err
			}
		}
	}
	return nil
}
