package copy

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"
)

// XDockerContainerPathStatHeader is the *key* in http headers pointing to the
// base64 encoded JSON payload of stating a path in a container.
const XDockerContainerPathStatHeader = "X-Docker-Container-Path-Stat"

// FileInfo describes a file or directory and is returned by
// (*CopyItem).Stat().
type FileInfo struct {
	Name       string      `json:"name"`
	Size       int64       `json:"size"`
	Mode       os.FileMode `json:"mode"`
	ModTime    time.Time   `json:"mtime"`
	IsDir      bool        `json:"isDir"`
	IsStream   bool        `json:"isStream"`
	LinkTarget string      `json:"linkTarget"`
}

// EncodeFileInfo serializes the specified FileInfo as a base64 encoded JSON
// payload.  Intended for Docker compat.
func EncodeFileInfo(info *FileInfo) (string, error) {
	buf, err := json.Marshal(&info)
	if err != nil {
		return "", errors.Wrap(err, "failed to serialize file stats")
	}
	return base64.URLEncoding.EncodeToString(buf), nil
}

// ExtractFileInfoFromHeader extracts a base64 encoded JSON payload of a
// FileInfo in the http header.  If no such header entry is found, nil is
// returned.  Intended for Docker compat.
func ExtractFileInfoFromHeader(header *http.Header) (*FileInfo, error) {
	rawData := header.Get(XDockerContainerPathStatHeader)
	if len(rawData) == 0 {
		return nil, nil
	}

	info := FileInfo{}
	base64Decoder := base64.NewDecoder(base64.URLEncoding, strings.NewReader(rawData))
	if err := json.NewDecoder(base64Decoder).Decode(&info); err != nil {
		return nil, err
	}

	return &info, nil
}
