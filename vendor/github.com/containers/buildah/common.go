package buildah

import (
	"context"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"syscall"
	"time"

	cp "github.com/containers/image/v5/copy"
	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/types"
	encconfig "github.com/containers/ocicrypt/config"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/unshare"
	"github.com/docker/distribution/registry/api/errcode"
	errcodev2 "github.com/docker/distribution/registry/api/v2"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
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

func isRetryable(err error) bool {
	err = errors.Cause(err)
	type unwrapper interface {
		Unwrap() error
	}
	if unwrapper, ok := err.(unwrapper); ok {
		err = unwrapper.Unwrap()
		return isRetryable(err)
	}
	if registryError, ok := err.(errcode.Error); ok {
		switch registryError.Code {
		case errcode.ErrorCodeUnauthorized, errcodev2.ErrorCodeNameUnknown, errcodev2.ErrorCodeManifestUnknown:
			return false
		}
		return true
	}
	if op, ok := err.(*net.OpError); ok {
		return isRetryable(op.Err)
	}
	if url, ok := err.(*url.Error); ok {
		return isRetryable(url.Err)
	}
	if errno, ok := err.(syscall.Errno); ok {
		if errno == syscall.ECONNREFUSED {
			return false
		}
	}
	if errs, ok := err.(errcode.Errors); ok {
		// if this error is a group of errors, process them all in turn
		for i := range errs {
			if !isRetryable(errs[i]) {
				return false
			}
		}
	}
	if errs, ok := err.(*multierror.Error); ok {
		// if this error is a group of errors, process them all in turn
		for i := range errs.Errors {
			if !isRetryable(errs.Errors[i]) {
				return false
			}
		}
	}
	return true
}

func retryCopyImage(ctx context.Context, policyContext *signature.PolicyContext, dest, src, registry types.ImageReference, action string, copyOptions *cp.Options, maxRetries int, retryDelay time.Duration) ([]byte, error) {
	manifestBytes, err := cp.Image(ctx, policyContext, dest, src, copyOptions)
	for retries := 0; err != nil && isRetryable(err) && registry != nil && registry.Transport().Name() == docker.Transport.Name() && retries < maxRetries; retries++ {
		if retryDelay == 0 {
			retryDelay = 5 * time.Second
		}
		logrus.Infof("Warning: %s failed, retrying in %s ... (%d/%d)", action, retryDelay, retries+1, maxRetries)
		time.Sleep(retryDelay)
		manifestBytes, err = cp.Image(ctx, policyContext, dest, src, copyOptions)
		if err == nil {
			break
		}
	}
	return manifestBytes, err
}
