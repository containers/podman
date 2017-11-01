package image

import (
	"encoding/json"
	"runtime"

	"github.com/containers/image/manifest"
	"github.com/containers/image/types"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
)

type platformSpec struct {
	Architecture string   `json:"architecture"`
	OS           string   `json:"os"`
	OSVersion    string   `json:"os.version,omitempty"`
	OSFeatures   []string `json:"os.features,omitempty"`
	Variant      string   `json:"variant,omitempty"`
	Features     []string `json:"features,omitempty"` // removed in OCI
}

// A manifestDescriptor references a platform-specific manifest.
type manifestDescriptor struct {
	manifest.Schema2Descriptor
	Platform platformSpec `json:"platform"`
}

type manifestList struct {
	SchemaVersion int                  `json:"schemaVersion"`
	MediaType     string               `json:"mediaType"`
	Manifests     []manifestDescriptor `json:"manifests"`
}

func manifestSchema2FromManifestList(src types.ImageSource, manblob []byte) (genericManifest, error) {
	list := manifestList{}
	if err := json.Unmarshal(manblob, &list); err != nil {
		return nil, err
	}
	var targetManifestDigest digest.Digest
	for _, d := range list.Manifests {
		if d.Platform.Architecture == runtime.GOARCH && d.Platform.OS == runtime.GOOS {
			targetManifestDigest = d.Digest
			break
		}
	}
	if targetManifestDigest == "" {
		return nil, errors.New("no supported platform found in manifest list")
	}
	manblob, mt, err := src.GetTargetManifest(targetManifestDigest)
	if err != nil {
		return nil, err
	}

	matches, err := manifest.MatchesDigest(manblob, targetManifestDigest)
	if err != nil {
		return nil, errors.Wrap(err, "Error computing manifest digest")
	}
	if !matches {
		return nil, errors.Errorf("Manifest image does not match selected manifest digest %s", targetManifestDigest)
	}

	return manifestInstanceFromBlob(src, manblob, mt)
}
