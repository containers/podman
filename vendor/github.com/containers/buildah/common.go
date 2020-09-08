package buildah

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/containers/common/pkg/retry"
	cp "github.com/containers/image/v5/copy"
	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/types"
	encconfig "github.com/containers/ocicrypt/config"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/unshare"
)

const (
	// OCI used to define the "oci" image format
	OCI = "oci"
	// DOCKER used to define the "docker" image format
	DOCKER = "docker"
)

func getCopyOptions(store storage.Store, reportWriter io.Writer, sourceSystemContext *types.SystemContext, destinationSystemContext *types.SystemContext, manifestType string, removeSignatures bool, addSigner string, ociEncryptLayers *[]int, ociEncryptConfig *encconfig.EncryptConfig, ociDecryptConfig *encconfig.DecryptConfig) *cp.Options {
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
		RemoveSignatures:      removeSignatures,
		SignBy:                addSigner,
		OciEncryptConfig:      ociEncryptConfig,
		OciDecryptConfig:      ociDecryptConfig,
		OciEncryptLayers:      ociEncryptLayers,
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

func retryCopyImage(ctx context.Context, policyContext *signature.PolicyContext, dest, src, registry types.ImageReference, copyOptions *cp.Options, maxRetries int, retryDelay time.Duration) ([]byte, error) {
	var (
		manifestBytes []byte
		err           error
		lastErr       error
	)
	err = retry.RetryIfNecessary(ctx, func() error {
		manifestBytes, err = cp.Image(ctx, policyContext, dest, src, copyOptions)
		if registry != nil && registry.Transport().Name() != docker.Transport.Name() {
			lastErr = err
			return nil
		}
		return err
	}, &retry.RetryOptions{MaxRetry: maxRetries, Delay: retryDelay})
	if lastErr != nil {
		err = lastErr
	}
	return manifestBytes, err
}
