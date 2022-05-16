package machine

import (
	"os"
	"strings"
)

func isUnixSocket(file os.DirEntry) bool {
	// Assume a socket on Windows, since sock mode is not supported yet https://github.com/golang/go/issues/33357
	return !file.Type().IsDir() && strings.HasSuffix(file.Name(), ".sock")
}
