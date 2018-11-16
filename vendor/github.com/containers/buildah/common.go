package buildah

import (
	"io"
	"os"
	"path/filepath"

	cp "github.com/containers/image/copy"
	"github.com/containers/image/transports"
	"github.com/containers/image/types"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/sirupsen/logrus"
)

const (
	// OCI used to define the "oci" image format
	OCI = "oci"
	// DOCKER used to define the "docker" image format
	DOCKER = "docker"
)

// userRegistriesFile is the path to the per user registry configuration file.
var userRegistriesFile = filepath.Join(os.Getenv("HOME"), ".config/containers/registries.conf")

func getCopyOptions(reportWriter io.Writer, sourceReference types.ImageReference, sourceSystemContext *types.SystemContext, destinationReference types.ImageReference, destinationSystemContext *types.SystemContext, manifestType string) *cp.Options {
	sourceCtx := &types.SystemContext{}
	if sourceSystemContext != nil {
		*sourceCtx = *sourceSystemContext
	} else {
		if rootless.IsRootless() {
			if _, err := os.Stat(userRegistriesFile); err == nil {
				sourceCtx.SystemRegistriesConfPath = userRegistriesFile
			}

		}
	}
	sourceInsecure, err := isReferenceInsecure(sourceReference, sourceCtx)
	if err != nil {
		logrus.Debugf("error determining if registry for %q is insecure: %v", transports.ImageName(sourceReference), err)
	} else if sourceInsecure {
		sourceCtx.DockerInsecureSkipTLSVerify = true
		sourceCtx.OCIInsecureSkipTLSVerify = true
	}

	destinationCtx := &types.SystemContext{}
	if destinationSystemContext != nil {
		*destinationCtx = *destinationSystemContext
	} else {
		if rootless.IsRootless() {
			if _, err := os.Stat(userRegistriesFile); err == nil {
				destinationCtx.SystemRegistriesConfPath = userRegistriesFile
			}
		}
	}
	destinationInsecure, err := isReferenceInsecure(destinationReference, destinationCtx)
	if err != nil {
		logrus.Debugf("error determining if registry for %q is insecure: %v", transports.ImageName(destinationReference), err)
	} else if destinationInsecure {
		destinationCtx.DockerInsecureSkipTLSVerify = true
		destinationCtx.OCIInsecureSkipTLSVerify = true
	}

	return &cp.Options{
		ReportWriter:          reportWriter,
		SourceCtx:             sourceCtx,
		DestinationCtx:        destinationCtx,
		ForceManifestMIMEType: manifestType,
	}
}

func getSystemContext(defaults *types.SystemContext, signaturePolicyPath string) *types.SystemContext {
	sc := &types.SystemContext{}
	if defaults != nil {
		*sc = *defaults
	}
	if signaturePolicyPath != "" {
		sc.SignaturePolicyPath = signaturePolicyPath
	}
	if sc.SystemRegistriesConfPath == "" && rootless.IsRootless() {
		if _, err := os.Stat(userRegistriesFile); err == nil {
			sc.SystemRegistriesConfPath = userRegistriesFile
		}

	}
	return sc
}
