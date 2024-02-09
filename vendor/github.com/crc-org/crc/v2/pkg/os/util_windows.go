package os

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/user"
	"strings"

	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

// ReadFileUTF16LE reads a UTF-16LE file and returns in a []byte
// ini/inf files in windows are of this format, reading a UTF-16
// file directly without this would result in malformed texts
func ReadFileUTF16LE(filename string) ([]byte, error) {
	// Read the file into a []byte
	raw, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	// Make an tranformer that converts MS-Win default to UTF8
	win16le := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM)
	// Make a transformer that is like win16le, but abides by BOM
	utf16bom := unicode.BOMOverride(win16le.NewDecoder())

	// Make a Reader that uses utf16bom
	unicodeReader := transform.NewReader(bytes.NewReader(raw), utf16bom)
	decoded, err := io.ReadAll(unicodeReader)
	return decoded, err
}

func GetCurrentUsername() (string, error) {
	u, err := user.Current()
	if err != nil {
		return "", err
	}
	userAndDomain := strings.Split(u.Username, "\\")
	if len(userAndDomain) > 1 {
		return userAndDomain[1], nil
	}
	return "", errors.New("unable to find the username of current user")
}
