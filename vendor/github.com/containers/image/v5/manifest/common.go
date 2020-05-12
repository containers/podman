package manifest

import (
	"fmt"

	"github.com/containers/image/v5/pkg/compression"
	"github.com/containers/image/v5/types"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// dupStringSlice returns a deep copy of a slice of strings, or nil if the
// source slice is empty.
func dupStringSlice(list []string) []string {
	if len(list) == 0 {
		return nil
	}
	dup := make([]string, len(list))
	copy(dup, list)
	return dup
}

// dupStringStringMap returns a deep copy of a map[string]string, or nil if the
// passed-in map is nil or has no keys.
func dupStringStringMap(m map[string]string) map[string]string {
	if len(m) == 0 {
		return nil
	}
	result := make(map[string]string)
	for k, v := range m {
		result[k] = v
	}
	return result
}

// layerInfosToStrings converts a list of layer infos, presumably obtained from a Manifest.LayerInfos()
// method call, into a format suitable for inclusion in a types.ImageInspectInfo structure.
func layerInfosToStrings(infos []LayerInfo) []string {
	layers := make([]string, len(infos))
	for i, info := range infos {
		layers[i] = info.Digest.String()
	}
	return layers
}

// compressionMIMETypeSet describes a set of MIME type “variants” that represent differently-compressed
// versions of “the same kind of content”.
// The map key is the return value of compression.Algorithm.Name(), or mtsUncompressed;
// the map value is a MIME type, or mtsUnsupportedMIMEType to mean "recognized but unsupported".
type compressionMIMETypeSet map[string]string

const mtsUncompressed = ""        // A key in compressionMIMETypeSet for the uncompressed variant
const mtsUnsupportedMIMEType = "" // A value in compressionMIMETypeSet that means “recognized but unsupported”

// compressionVariantMIMEType returns a variant of mimeType for the specified algorithm (which may be nil
// to mean "no compression"), based on variantTable.
func compressionVariantMIMEType(variantTable []compressionMIMETypeSet, mimeType string, algorithm *compression.Algorithm) (string, error) {
	if mimeType == mtsUnsupportedMIMEType { // Prevent matching against the {algo:mtsUnsupportedMIMEType} entries
		return "", fmt.Errorf("cannot update unknown MIME type")
	}
	for _, variants := range variantTable {
		for _, mt := range variants {
			if mt == mimeType { // Found the variant
				name := mtsUncompressed
				if algorithm != nil {
					name = algorithm.Name()
				}
				if res, ok := variants[name]; ok {
					if res != mtsUnsupportedMIMEType {
						return res, nil
					}
					if name != mtsUncompressed {
						return "", fmt.Errorf("%s compression is not supported", name)
					}
					return "", errors.New("uncompressed variant is not supported")
				}
				if name != mtsUncompressed {
					return "", fmt.Errorf("unknown compression algorithm %s", name)
				}
				// We can't very well say “the idea of no compression is unknown”
				return "", errors.New("uncompressed variant is not supported")
			}
		}
	}
	if algorithm != nil {
		return "", fmt.Errorf("unsupported MIME type for compression: %s", mimeType)
	}
	return "", fmt.Errorf("unsupported MIME type for decompression: %s", mimeType)
}

// updatedMIMEType returns the result of applying edits in updated (MediaType, CompressionOperation) to
// mimeType, based on variantTable. It may use updated.Digest for error messages.
func updatedMIMEType(variantTable []compressionMIMETypeSet, mimeType string, updated types.BlobInfo) (string, error) {
	// Note that manifests in containers-storage might be reporting the
	// wrong media type since the original manifests are stored while layers
	// are decompressed in storage.  Hence, we need to consider the case
	// that an already {de}compressed layer should be {de}compressed;
	// compressionVariantMIMEType does that by not caring whether the original is
	// {de}compressed.
	switch updated.CompressionOperation {
	case types.PreserveOriginal:
		// Keep the original media type.
		return mimeType, nil

	case types.Decompress:
		return compressionVariantMIMEType(variantTable, mimeType, nil)

	case types.Compress:
		if updated.CompressionAlgorithm == nil {
			logrus.Debugf("Error preparing updated manifest: blob %q was compressed but does not specify by which algorithm: falling back to use the original blob", updated.Digest)
			return mimeType, nil
		}
		return compressionVariantMIMEType(variantTable, mimeType, updated.CompressionAlgorithm)

	default:
		return "", fmt.Errorf("unknown compression operation (%d)", updated.CompressionOperation)
	}
}
