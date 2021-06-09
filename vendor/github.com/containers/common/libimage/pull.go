package libimage

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/containers/common/pkg/config"
	registryTransport "github.com/containers/image/v5/docker"
	dockerArchiveTransport "github.com/containers/image/v5/docker/archive"
	"github.com/containers/image/v5/docker/reference"
	ociArchiveTransport "github.com/containers/image/v5/oci/archive"
	ociTransport "github.com/containers/image/v5/oci/layout"
	"github.com/containers/image/v5/pkg/shortnames"
	storageTransport "github.com/containers/image/v5/storage"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"
	"github.com/containers/storage"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// PullOptions allows for custommizing image pulls.
type PullOptions struct {
	CopyOptions

	// If true, all tags of the image will be pulled from the container
	// registry.  Only supported for the docker transport.
	AllTags bool
}

// Pull pulls the specified name.  Name may refer to any of the supported
// transports from github.com/containers/image.  If no transport is encoded,
// name will be treated as a reference to a registry (i.e., docker transport).
//
// Note that pullPolicy is only used when pulling from a container registry but
// it *must* be different than the default value `config.PullPolicyUnsupported`.  This
// way, callers are forced to decide on the pull behaviour.  The reasoning
// behind is that some (commands of some) tools have different default pull
// policies (e.g., buildah-bud versus podman-build).  Making the pull-policy
// choice explicit is an attempt to prevent silent regressions.
//
// The error is storage.ErrImageUnknown iff the pull policy is set to "never"
// and no local image has been found.  This allows for an easier integration
// into some users of this package (e.g., Buildah).
func (r *Runtime) Pull(ctx context.Context, name string, pullPolicy config.PullPolicy, options *PullOptions) ([]*Image, error) {
	logrus.Debugf("Pulling image %s (policy: %s)", name, pullPolicy)

	if options == nil {
		options = &PullOptions{}
	}

	var possiblyUnqualifiedName string // used for short-name resolution
	ref, err := alltransports.ParseImageName(name)
	if err != nil {
		// If the image clearly refers to a local one, we can look it up directly.
		// In fact, we need to since they are not parseable.
		if strings.HasPrefix(name, "sha256:") || (len(name) == 64 && !strings.ContainsAny(name, "/.:@")) {
			if pullPolicy == config.PullPolicyAlways {
				return nil, errors.Errorf("pull policy is always but image has been referred to by ID (%s)", name)
			}
			local, _, err := r.LookupImage(name, nil)
			if err != nil {
				return nil, err
			}
			return []*Image{local}, err
		}

		// If the input does not include a transport assume it refers
		// to a registry.
		dockerRef, dockerErr := alltransports.ParseImageName("docker://" + name)
		if dockerErr != nil {
			return nil, err
		}
		ref = dockerRef
		possiblyUnqualifiedName = name
	} else if ref.Transport().Name() == registryTransport.Transport.Name() {
		// Normalize the input if we're referring to the docker
		// transport directly. That makes sure that a `docker://fedora`
		// will resolve directly to `docker.io/library/fedora:latest`
		// and not be subject to short-name resolution.
		named := ref.DockerReference()
		if named == nil {
			return nil, errors.New("internal error: unexpected nil reference")
		}
		possiblyUnqualifiedName = named.String()
	}

	if options.AllTags && ref.Transport().Name() != registryTransport.Transport.Name() {
		return nil, errors.Errorf("pulling all tags is not supported for %s transport", ref.Transport().Name())
	}

	if r.eventChannel != nil {
		r.writeEvent(&Event{ID: "", Name: name, Time: time.Now(), Type: EventTypeImagePull})
	}

	var (
		pulledImages []string
		pullError    error
	)

	// Dispatch the copy operation.
	switch ref.Transport().Name() {

	// DOCKER REGISTRY
	case registryTransport.Transport.Name():
		pulledImages, pullError = r.copyFromRegistry(ctx, ref, possiblyUnqualifiedName, pullPolicy, options)

	// DOCKER ARCHIVE
	case dockerArchiveTransport.Transport.Name():
		pulledImages, pullError = r.copyFromDockerArchive(ctx, ref, &options.CopyOptions)

	// ALL OTHER TRANSPORTS
	default:
		pulledImages, pullError = r.copyFromDefault(ctx, ref, &options.CopyOptions)
	}

	if pullError != nil {
		return nil, pullError
	}

	localImages := []*Image{}
	lookupOptions := &LookupImageOptions{IgnorePlatform: true}
	for _, name := range pulledImages {
		local, _, err := r.LookupImage(name, lookupOptions)
		if err != nil {
			return nil, errors.Wrapf(err, "error locating pulled image %q name in containers storage", name)
		}
		localImages = append(localImages, local)
	}

	return localImages, pullError
}

// copyFromDefault is the default copier for a number of transports.  Other
// transports require some specific dancing, sometimes Yoga.
func (r *Runtime) copyFromDefault(ctx context.Context, ref types.ImageReference, options *CopyOptions) ([]string, error) {
	c, err := r.newCopier(options)
	if err != nil {
		return nil, err
	}
	defer c.close()

	// Figure out a name for the storage destination.
	var storageName, imageName string
	switch ref.Transport().Name() {

	case ociTransport.Transport.Name():
		split := strings.SplitN(ref.StringWithinTransport(), ":", 2)
		storageName = toLocalImageName(split[0])
		imageName = storageName

	case ociArchiveTransport.Transport.Name():
		manifest, err := ociArchiveTransport.LoadManifestDescriptor(ref)
		if err != nil {
			return nil, err
		}
		// if index.json has no reference name, compute the image digest instead
		if manifest.Annotations == nil || manifest.Annotations["org.opencontainers.image.ref.name"] == "" {
			storageName, err = getImageDigest(ctx, ref, nil)
			if err != nil {
				return nil, err
			}
			imageName = "sha256:" + storageName[1:]
		} else {
			storageName = manifest.Annotations["org.opencontainers.image.ref.name"]
			named, err := NormalizeName(storageName)
			if err != nil {
				return nil, err
			}
			imageName = named.String()
			storageName = imageName
		}

	case storageTransport.Transport.Name():
		storageName = ref.StringWithinTransport()
		named := ref.DockerReference()
		if named == nil {
			return nil, errors.Errorf("could not get an image name for storage reference %q", ref)
		}
		imageName = named.String()

	default:
		storageName = toLocalImageName(ref.StringWithinTransport())
		imageName = storageName
	}

	// Create a storage reference.
	destRef, err := storageTransport.Transport.ParseStoreReference(r.store, storageName)
	if err != nil {
		return nil, errors.Wrapf(err, "parsing %q", storageName)
	}

	_, err = c.copy(ctx, ref, destRef)
	return []string{imageName}, err
}

// storageReferencesFromArchiveReader returns a slice of image references inside the
// archive reader.  A docker archive may include more than one image and this
// method allows for extracting them into containers storage references which
// can later be used from copying.
func (r *Runtime) storageReferencesReferencesFromArchiveReader(ctx context.Context, readerRef types.ImageReference, reader *dockerArchiveTransport.Reader) ([]types.ImageReference, []string, error) {
	destNames, err := reader.ManifestTagsForReference(readerRef)
	if err != nil {
		return nil, nil, err
	}

	var imageNames []string
	if len(destNames) == 0 {
		destName, err := getImageDigest(ctx, readerRef, &r.systemContext)
		if err != nil {
			return nil, nil, err
		}
		destNames = append(destNames, destName)
		// Make sure the image can be loaded after the pull by
		// replacing the @ with sha256:.
		imageNames = append(imageNames, "sha256:"+destName[1:])
	} else {
		for i := range destNames {
			ref, err := NormalizeName(destNames[i])
			if err != nil {
				return nil, nil, err
			}
			destNames[i] = ref.String()
		}
		imageNames = destNames
	}

	references := []types.ImageReference{}
	for _, destName := range destNames {
		destRef, err := storageTransport.Transport.ParseStoreReference(r.store, destName)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "error parsing dest reference name %#v", destName)
		}
		references = append(references, destRef)
	}

	return references, imageNames, nil
}

// copyFromDockerArchive copies one image from the specified reference.
func (r *Runtime) copyFromDockerArchive(ctx context.Context, ref types.ImageReference, options *CopyOptions) ([]string, error) {
	// There may be more than one image inside the docker archive, so we
	// need a quick glimpse inside.
	reader, readerRef, err := dockerArchiveTransport.NewReaderForReference(&r.systemContext, ref)
	if err != nil {
		return nil, err
	}

	return r.copyFromDockerArchiveReaderReference(ctx, reader, readerRef, options)
}

// copyFromDockerArchiveReaderReference copies the specified readerRef from reader.
func (r *Runtime) copyFromDockerArchiveReaderReference(ctx context.Context, reader *dockerArchiveTransport.Reader, readerRef types.ImageReference, options *CopyOptions) ([]string, error) {
	c, err := r.newCopier(options)
	if err != nil {
		return nil, err
	}
	defer c.close()

	// Get a slice of storage references we can copy.
	references, destNames, err := r.storageReferencesReferencesFromArchiveReader(ctx, readerRef, reader)
	if err != nil {
		return nil, err
	}

	// Now copy all of the images.  Use readerRef for performance.
	for _, destRef := range references {
		if _, err := c.copy(ctx, readerRef, destRef); err != nil {
			return nil, err
		}
	}

	return destNames, nil
}

// copyFromRegistry pulls the specified, possibly unqualified, name from a
// registry.  On successful pull it returns the used fully-qualified name that
// can later be used to look up the image in the local containers storage.
//
// If options.All is set, all tags from the specified registry will be pulled.
func (r *Runtime) copyFromRegistry(ctx context.Context, ref types.ImageReference, inputName string, pullPolicy config.PullPolicy, options *PullOptions) ([]string, error) {
	// Sanity check.
	if err := pullPolicy.Validate(); err != nil {
		return nil, err
	}

	if !options.AllTags {
		return r.copySingleImageFromRegistry(ctx, inputName, pullPolicy, options)
	}

	named := reference.TrimNamed(ref.DockerReference())
	tags, err := registryTransport.GetRepositoryTags(ctx, &r.systemContext, ref)
	if err != nil {
		return nil, err
	}

	pulledTags := []string{}
	for _, tag := range tags {
		select { // Let's be gentle with Podman remote.
		case <-ctx.Done():
			return nil, errors.Errorf("pulling cancelled")
		default:
			// We can continue.
		}
		tagged, err := reference.WithTag(named, tag)
		if err != nil {
			return nil, errors.Wrapf(err, "error creating tagged reference (name %s, tag %s)", named.String(), tag)
		}
		pulled, err := r.copySingleImageFromRegistry(ctx, tagged.String(), pullPolicy, options)
		if err != nil {
			return nil, err
		}
		pulledTags = append(pulledTags, pulled...)
	}

	return pulledTags, nil
}

// copySingleImageFromRegistry pulls the specified, possibly unqualified, name
// from a registry.  On successful pull it returns the used fully-qualified
// name that can later be used to look up the image in the local containers
// storage.
func (r *Runtime) copySingleImageFromRegistry(ctx context.Context, imageName string, pullPolicy config.PullPolicy, options *PullOptions) ([]string, error) {
	// Sanity check.
	if err := pullPolicy.Validate(); err != nil {
		return nil, err
	}

	var (
		localImage        *Image
		resolvedImageName string
		err               error
	)

	// Always check if there's a local image.  If, we should use it's
	// resolved name for pulling.  Assume we're doing a `pull foo`.
	// If there's already a local image "localhost/foo", then we should
	// attempt pulling that instead of doing the full short-name dance.
	localImage, resolvedImageName, err = r.LookupImage(imageName, nil)
	if err != nil && errors.Cause(err) != storage.ErrImageUnknown {
		logrus.Errorf("Looking up %s in local storage: %v", imageName, err)
	}

	if pullPolicy == config.PullPolicyNever {
		if localImage != nil {
			logrus.Debugf("Pull policy %q but no local image has been found for %s", pullPolicy, imageName)
			return []string{resolvedImageName}, nil
		}
		logrus.Debugf("Pull policy %q and %s resolved to local image %s", pullPolicy, imageName, resolvedImageName)
		return nil, errors.Wrap(storage.ErrImageUnknown, imageName)
	}

	if pullPolicy == config.PullPolicyMissing && localImage != nil {
		return []string{resolvedImageName}, nil
	}

	// If we looked up the image by ID, we cannot really pull from anywhere.
	if localImage != nil && strings.HasPrefix(localImage.ID(), imageName) {
		switch pullPolicy {
		case config.PullPolicyAlways:
			return nil, errors.Errorf("pull policy is always but image has been referred to by ID (%s)", imageName)
		default:
			return []string{resolvedImageName}, nil
		}
	}

	// If we found a local image, we should use it's locally resolved name
	// (see containers/buildah #2904).
	if localImage != nil {
		if imageName != resolvedImageName {
			logrus.Debugf("Image %s resolved to local image %s which will be used for pulling", imageName, resolvedImageName)
		}
		imageName = resolvedImageName
	}

	sys := r.systemContextCopy()
	resolved, err := shortnames.Resolve(sys, imageName)
	if err != nil {
		return nil, err
	}

	// NOTE: Below we print the description from the short-name resolution.
	// In theory we could print it here.  In practice, however, this is
	// causing a hard time for Buildah uses who are doing a `buildah from
	// image` and expect just the container name to be printed if the image
	// is present locally.
	// The pragmatic solution is to only print the description when we found
	// a _newer_ image that we're about to pull.
	wroteDesc := false
	writeDesc := func() error {
		if wroteDesc {
			return nil
		}
		wroteDesc = true
		if desc := resolved.Description(); len(desc) > 0 {
			logrus.Debug(desc)
			if options.Writer != nil {
				if _, err := options.Writer.Write([]byte(desc + "\n")); err != nil {
					return err
				}
			}
		}
		return nil
	}

	c, err := r.newCopier(&options.CopyOptions)
	if err != nil {
		return nil, err
	}
	defer c.close()

	var pullErrors []error
	for _, candidate := range resolved.PullCandidates {
		candidateString := candidate.Value.String()
		logrus.Debugf("Attempting to pull candidate %s for %s", candidateString, imageName)
		srcRef, err := registryTransport.NewReference(candidate.Value)
		if err != nil {
			return nil, err
		}

		if pullPolicy == config.PullPolicyNewer && localImage != nil {
			isNewer, err := localImage.HasDifferentDigest(ctx, srcRef)
			if err != nil {
				pullErrors = append(pullErrors, err)
				continue
			}

			if !isNewer {
				logrus.Debugf("Skipping pull candidate %s as the image is not newer (pull policy %s)", candidateString, pullPolicy)
				continue
			}
		}

		destRef, err := storageTransport.Transport.ParseStoreReference(r.store, candidate.Value.String())
		if err != nil {
			return nil, err
		}

		if err := writeDesc(); err != nil {
			return nil, err
		}
		if options.Writer != nil {
			if _, err := io.WriteString(options.Writer, fmt.Sprintf("Trying to pull %s...\n", candidateString)); err != nil {
				return nil, err
			}
		}
		if _, err := c.copy(ctx, srcRef, destRef); err != nil {
			logrus.Debugf("Error pulling candidate %s: %v", candidateString, err)
			pullErrors = append(pullErrors, err)
			continue
		}
		if err := candidate.Record(); err != nil {
			// Only log the recording errors.  Podman has seen
			// reports where users set most of the system to
			// read-only which can cause issues.
			logrus.Errorf("Error recording short-name alias %q: %v", candidateString, err)
		}

		logrus.Debugf("Pulled candidate %s successfully", candidateString)
		return []string{candidate.Value.String()}, nil
	}

	if localImage != nil && pullPolicy == config.PullPolicyNewer {
		return []string{resolvedImageName}, nil
	}

	if len(pullErrors) == 0 {
		return nil, errors.Errorf("internal error: no image pulled (pull policy %s)", pullPolicy)
	}

	return nil, resolved.FormatPullErrors(pullErrors)
}
