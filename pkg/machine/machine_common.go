//go:build amd64 || arm64
// +build amd64 arm64

package machine

import (
	"os"
)

// getDevNullFiles returns pointers to Read-only and Write-only DevNull files
func GetDevNullFiles() (*os.File, *os.File, error) {
	dnr, err := os.OpenFile(os.DevNull, os.O_RDONLY, 0755)
	if err != nil {
		return nil, nil, err
	}

	dnw, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0755)
	if err != nil {
		if e := dnr.Close(); e != nil {
			err = e
		}
		return nil, nil, err
	}

	return dnr, dnw, nil
}
