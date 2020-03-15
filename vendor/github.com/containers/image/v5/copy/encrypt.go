package copy

import (
	"strings"

	"github.com/containers/image/v5/types"
)

// isOciEncrypted returns a bool indicating if a mediatype is encrypted
// This function will be moved to be part of OCI spec when adopted.
func isOciEncrypted(mediatype string) bool {
	return strings.HasSuffix(mediatype, "+encrypted")
}

// isEncrypted checks if an image is encrypted
func isEncrypted(i types.Image) bool {
	layers := i.LayerInfos()
	for _, l := range layers {
		if isOciEncrypted(l.MediaType) {
			return true
		}
	}
	return false
}
