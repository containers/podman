package buildah

import (
	"io"
	"os"
	"path/filepath"

	"github.com/containers/buildah/pkg/unshare"
	cp "github.com/containers/image/copy"
	"github.com/containers/image/types"
	"github.com/containers/storage"
)

const (
	// OCI used to define the "oci" image format
	OCI = "oci"
	// DOCKER used to define the "docker" image format
	DOCKER = "docker"
)

func getCopyOptions(store storage.Store, reportWriter io.Writer, sourceReference types.ImageReference, sourceSystemContext *types.SystemContext, destinationReference types.ImageReference, destinationSystemContext *types.SystemContext, manifestType string) *cp.Options {
	sourceCtx := getSystemContext(store, nil, "")
	if sourceSystemContext != nil {
		*sourceCtx = *sourceSystemContext
	}

	destinationCtx := getSystemContext(store, nil, "")
	if destinationSystemContext != nil {
		*destinationCtx = *destinationSystemContext
	}
	return &cp.Options{
		ReportWriter:          reportWriter,
		SourceCtx:             sourceCtx,
		DestinationCtx:        destinationCtx,
		ForceManifestMIMEType: manifestType,
	}
}

func getSystemContext(store storage.Store, defaults *types.SystemContext, signaturePolicyPath string) *types.SystemContext {
	sc := &types.SystemContext{}
	if defaults != nil {
		*sc = *defaults
	}
	if signaturePolicyPath != "" {
		sc.SignaturePolicyPath = signaturePolicyPath
	}
	if store != nil {
		if sc.BlobInfoCacheDir == "" {
			sc.BlobInfoCacheDir = filepath.Join(store.GraphRoot(), "cache")
		}
		if sc.SystemRegistriesConfPath == "" && unshare.IsRootless() {
			userRegistriesFile := filepath.Join(store.GraphRoot(), "registries.conf")
			if _, err := os.Stat(userRegistriesFile); err == nil {
				sc.SystemRegistriesConfPath = userRegistriesFile
			}
		}
	}
	return sc
}
