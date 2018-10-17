package buildah

import (
	"context"
	"io"
	"strings"

	"github.com/containers/buildah/util"
	cp "github.com/containers/image/copy"
	"github.com/containers/image/docker/reference"
	tarfile "github.com/containers/image/docker/tarfile"
	ociarchive "github.com/containers/image/oci/archive"
	"github.com/containers/image/signature"
	is "github.com/containers/image/storage"
	"github.com/containers/image/transports"
	"github.com/containers/image/transports/alltransports"
	"github.com/containers/image/types"
	"github.com/containers/storage"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// PullOptions can be used to alter how an image is copied in from somewhere.
type PullOptions struct {
	// SignaturePolicyPath specifies an override location for the signature
	// policy which should be used for verifying the new image as it is
	// being written.  Except in specific circumstances, no value should be
	// specified, indicating that the shared, system-wide default policy
	// should be used.
	SignaturePolicyPath string
	// ReportWriter is an io.Writer which will be used to log the writing
	// of the new image.
	ReportWriter io.Writer
	// Store is the local storage store which holds the source image.
	Store storage.Store
	// github.com/containers/image/types SystemContext to hold credentials
	// and other authentication/authorization information.
	SystemContext *types.SystemContext
	// Transport is a value which is prepended to the image's name, if the
	// image name alone can not be resolved to a reference to a source
	// image.  No separator is implicitly added.
	Transport string
}

func localImageNameForReference(ctx context.Context, store storage.Store, srcRef types.ImageReference, spec string) (string, error) {
	if srcRef == nil {
		return "", errors.Errorf("reference to image is empty")
	}
	split := strings.SplitN(spec, ":", 2)
	file := split[len(split)-1]
	var name string
	switch srcRef.Transport().Name() {
	case util.DockerArchive:
		tarSource, err := tarfile.NewSourceFromFile(file)
		if err != nil {
			return "", errors.Wrapf(err, "error opening tarfile %q as a source image", file)
		}
		manifest, err := tarSource.LoadTarManifest()
		if err != nil {
			return "", errors.Errorf("error retrieving manifest.json from tarfile %q: %v", file, err)
		}
		// to pull the first image stored in the tar file
		if len(manifest) == 0 {
			// use the hex of the digest if no manifest is found
			name, err = getImageDigest(ctx, srcRef, nil)
			if err != nil {
				return "", err
			}
		} else {
			if len(manifest[0].RepoTags) > 0 {
				name = manifest[0].RepoTags[0]
			} else {
				// If the input image has no repotags, we need to feed it a dest anyways
				name, err = getImageDigest(ctx, srcRef, nil)
				if err != nil {
					return "", err
				}
			}
		}
	case util.OCIArchive:
		// retrieve the manifest from index.json to access the image name
		manifest, err := ociarchive.LoadManifestDescriptor(srcRef)
		if err != nil {
			return "", errors.Wrapf(err, "error loading manifest for %q", transports.ImageName(srcRef))
		}
		// if index.json has no reference name, compute the image digest instead
		if manifest.Annotations == nil || manifest.Annotations["org.opencontainers.image.ref.name"] == "" {
			name, err = getImageDigest(ctx, srcRef, nil)
			if err != nil {
				return "", err
			}
		} else {
			name = manifest.Annotations["org.opencontainers.image.ref.name"]
		}
	case util.DirTransport:
		// supports pull from a directory
		name = split[1]
		// remove leading "/"
		if name[:1] == "/" {
			name = name[1:]
		}
	default:
		ref := srcRef.DockerReference()
		if ref == nil {
			name = srcRef.StringWithinTransport()
			_, err := is.Transport.ParseStoreReference(store, name)
			if err == nil {
				return name, nil
			}
			logrus.Debugf("error parsing local storage reference %q: %v", name, err)
			if strings.LastIndex(name, "/") != -1 {
				name = name[strings.LastIndex(name, "/")+1:]
				_, err = is.Transport.ParseStoreReference(store, name)
				if err == nil {
					return name, errors.Wrapf(err, "error parsing local storage reference %q", name)
				}
			}
			return "", errors.Errorf("reference to image %q is not a named reference", transports.ImageName(srcRef))
		}

		if named, ok := ref.(reference.Named); ok {
			name = named.Name()
			if namedTagged, ok := ref.(reference.NamedTagged); ok {
				name = name + ":" + namedTagged.Tag()
			}
			if canonical, ok := ref.(reference.Canonical); ok {
				name = name + "@" + canonical.Digest().String()
			}
		}
	}

	if _, err := is.Transport.ParseStoreReference(store, name); err != nil {
		return "", errors.Wrapf(err, "error parsing computed local image name %q", name)
	}
	return name, nil
}

// Pull copies the contents of the image from somewhere else to local storage.
func Pull(ctx context.Context, imageName string, options PullOptions) (types.ImageReference, error) {
	systemContext := getSystemContext(options.SystemContext, options.SignaturePolicyPath)
	return pullImage(ctx, options.Store, imageName, options, systemContext)
}

func pullImage(ctx context.Context, store storage.Store, imageName string, options PullOptions, sc *types.SystemContext) (types.ImageReference, error) {
	spec := imageName
	srcRef, err := alltransports.ParseImageName(spec)
	if err != nil {
		if options.Transport == "" {
			options.Transport = DefaultTransport
		}
		logrus.Debugf("error parsing image name %q, trying with transport %q: %v", spec, options.Transport, err)
		transport := options.Transport
		if transport != DefaultTransport {
			transport = transport + ":"
		}
		spec = transport + spec
		srcRef2, err2 := alltransports.ParseImageName(spec)
		if err2 != nil {
			return nil, errors.Wrapf(err2, "error parsing image name %q", spec)
		}
		srcRef = srcRef2
	}
	logrus.Debugf("parsed image name %q", spec)

	blocked, err := isReferenceBlocked(srcRef, sc)
	if err != nil {
		return nil, errors.Wrapf(err, "error checking if pulling from registry for %q is blocked", transports.ImageName(srcRef))
	}
	if blocked {
		return nil, errors.Errorf("pull access to registry for %q is blocked by configuration", transports.ImageName(srcRef))
	}

	destName, err := localImageNameForReference(ctx, store, srcRef, spec)
	if err != nil {
		return nil, errors.Wrapf(err, "error computing local image name for %q", transports.ImageName(srcRef))
	}
	if destName == "" {
		return nil, errors.Errorf("error computing local image name for %q", transports.ImageName(srcRef))
	}

	destRef, err := is.Transport.ParseStoreReference(store, destName)
	if err != nil {
		return nil, errors.Wrapf(err, "error parsing image name %q", destName)
	}

	policy, err := signature.DefaultPolicy(sc)
	if err != nil {
		return nil, errors.Wrapf(err, "error obtaining default signature policy")
	}

	policyContext, err := signature.NewPolicyContext(policy)
	if err != nil {
		return nil, errors.Wrapf(err, "error creating new signature policy context")
	}

	defer func() {
		if err2 := policyContext.Destroy(); err2 != nil {
			logrus.Debugf("error destroying signature policy context: %v", err2)
		}
	}()

	logrus.Debugf("copying %q to %q", spec, destName)
	if _, err := cp.Image(ctx, policyContext, destRef, srcRef, getCopyOptions(options.ReportWriter, srcRef, sc, destRef, nil, "")); err != nil {
		return nil, err
	}
	return destRef, nil
}

// getImageDigest creates an image object and uses the hex value of the digest as the image ID
// for parsing the store reference
func getImageDigest(ctx context.Context, src types.ImageReference, sc *types.SystemContext) (string, error) {
	newImg, err := src.NewImage(ctx, sc)
	if err != nil {
		return "", errors.Wrapf(err, "error opening image %q for reading", transports.ImageName(src))
	}
	defer newImg.Close()

	digest := newImg.ConfigInfo().Digest
	if err = digest.Validate(); err != nil {
		return "", errors.Wrapf(err, "error getting config info from image %q", transports.ImageName(src))
	}
	return "@" + digest.Hex(), nil
}
