package util

import (
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"

	"github.com/containers/image/directory"
	dockerarchive "github.com/containers/image/docker/archive"
	"github.com/containers/image/docker/reference"
	ociarchive "github.com/containers/image/oci/archive"
	"github.com/containers/image/pkg/sysregistries"
	is "github.com/containers/image/storage"
	"github.com/containers/image/tarball"
	"github.com/containers/image/types"
	"github.com/containers/storage"
	"github.com/docker/distribution/registry/api/errcode"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	minimumTruncatedIDLength = 3
)

var (
	// RegistryDefaultPathPrefix contains a per-registry listing of default prefixes
	// to prepend to image names that only contain a single path component.
	RegistryDefaultPathPrefix = map[string]string{
		"index.docker.io": "library",
		"docker.io":       "library",
	}
	// Transports contains the possible transports used for images
	Transports = map[string]string{
		dockerarchive.Transport.Name(): "",
		ociarchive.Transport.Name():    "",
		directory.Transport.Name():     "",
		tarball.Transport.Name():       "",
	}
	// DockerArchive is the transport we prepend to an image name
	// when saving to docker-archive
	DockerArchive = dockerarchive.Transport.Name()
	// OCIArchive is the transport we prepend to an image name
	// when saving to oci-archive
	OCIArchive = ociarchive.Transport.Name()
	// DirTransport is the transport for pushing and pulling
	// images to and from a directory
	DirTransport = directory.Transport.Name()
	// TarballTransport is the transport for importing a tar archive
	// and creating a filesystem image
	TarballTransport = tarball.Transport.Name()
)

// ResolveName checks if name is a valid image name, and if that name doesn't
// include a domain portion, returns a list of the names which it might
// correspond to in the set of configured registries.
func ResolveName(name string, firstRegistry string, sc *types.SystemContext, store storage.Store) []string {
	if name == "" {
		return nil
	}

	// Maybe it's a truncated image ID.  Don't prepend a registry name, then.
	if len(name) >= minimumTruncatedIDLength {
		if img, err := store.Image(name); err == nil && img != nil && strings.HasPrefix(img.ID, name) {
			// It's a truncated version of the ID of an image that's present in local storage;
			// we need to expand the ID.
			return []string{img.ID}
		}
	}

	// If the image is from a different transport
	split := strings.SplitN(name, ":", 2)
	if len(split) == 2 {
		if _, ok := Transports[split[0]]; ok {
			return []string{split[1]}
		}
	}

	// If the image name already included a domain component, we're done.
	named, err := reference.ParseNormalizedNamed(name)
	if err != nil {
		return []string{name}
	}
	if named.String() == name {
		// Parsing produced the same result, so there was a domain name in there to begin with.
		return []string{name}
	}
	if reference.Domain(named) != "" && RegistryDefaultPathPrefix[reference.Domain(named)] != "" {
		// If this domain can cause us to insert something in the middle, check if that happened.
		repoPath := reference.Path(named)
		domain := reference.Domain(named)
		defaultPrefix := RegistryDefaultPathPrefix[reference.Domain(named)] + "/"
		if strings.HasPrefix(repoPath, defaultPrefix) && path.Join(domain, repoPath[len(defaultPrefix):]) == name {
			// Yup, parsing just inserted a bit in the middle, so there was a domain name there to begin with.
			return []string{name}
		}
	}

	// Figure out the list of registries.
	registries, err := sysregistries.GetRegistries(sc)
	if err != nil {
		logrus.Debugf("unable to read configured registries to complete %q: %v", name, err)
		registries = []string{}
	}
	if sc.DockerInsecureSkipTLSVerify {
		if unverifiedRegistries, err := sysregistries.GetInsecureRegistries(sc); err == nil {
			registries = append(registries, unverifiedRegistries...)
		}
	}

	// Create all of the combinations.  Some registries need an additional component added, so
	// use our lookaside map to keep track of them.  If there are no configured registries, we'll
	// return a name using "localhost" as the registry name.
	candidates := []string{}
	initRegistries := []string{"localhost"}
	if firstRegistry != "" && firstRegistry != "localhost" {
		initRegistries = append([]string{firstRegistry}, initRegistries...)
	}
	for _, registry := range append(initRegistries, registries...) {
		if registry == "" {
			continue
		}
		middle := ""
		if prefix, ok := RegistryDefaultPathPrefix[registry]; ok && strings.IndexRune(name, '/') == -1 {
			middle = prefix
		}
		candidate := path.Join(registry, middle, name)
		candidates = append(candidates, candidate)
	}
	return candidates
}

// ExpandNames takes unqualified names, parses them as image names, and returns
// the fully expanded result, including a tag.  Names which don't include a registry
// name will be marked for the most-preferred registry (i.e., the first one in our
// configuration).
func ExpandNames(names []string, firstRegistry string, systemContext *types.SystemContext, store storage.Store) ([]string, error) {
	expanded := make([]string, 0, len(names))
	for _, n := range names {
		var name reference.Named
		nameList := ResolveName(n, firstRegistry, systemContext, store)
		if len(nameList) == 0 {
			named, err := reference.ParseNormalizedNamed(n)
			if err != nil {
				return nil, errors.Wrapf(err, "error parsing name %q", n)
			}
			name = named
		} else {
			named, err := reference.ParseNormalizedNamed(nameList[0])
			if err != nil {
				return nil, errors.Wrapf(err, "error parsing name %q", nameList[0])
			}
			name = named
		}
		name = reference.TagNameOnly(name)
		tag := ""
		digest := ""
		if tagged, ok := name.(reference.NamedTagged); ok {
			tag = ":" + tagged.Tag()
		}
		if digested, ok := name.(reference.Digested); ok {
			digest = "@" + digested.Digest().String()
		}
		expanded = append(expanded, name.Name()+tag+digest)
	}
	return expanded, nil
}

// FindImage locates the locally-stored image which corresponds to a given name.
func FindImage(store storage.Store, firstRegistry string, systemContext *types.SystemContext, image string) (types.ImageReference, *storage.Image, error) {
	var ref types.ImageReference
	var img *storage.Image
	var err error
	for _, name := range ResolveName(image, firstRegistry, systemContext, store) {
		ref, err = is.Transport.ParseStoreReference(store, name)
		if err != nil {
			logrus.Debugf("error parsing reference to image %q: %v", name, err)
			continue
		}
		img, err = is.Transport.GetStoreImage(store, ref)
		if err != nil {
			img2, err2 := store.Image(name)
			if err2 != nil {
				logrus.Debugf("error locating image %q: %v", name, err2)
				continue
			}
			img = img2
		}
		break
	}
	if ref == nil || img == nil {
		return nil, nil, errors.Wrapf(err, "error locating image with name %q", image)
	}
	return ref, img, nil
}

// AddImageNames adds the specified names to the specified image.
func AddImageNames(store storage.Store, firstRegistry string, systemContext *types.SystemContext, image *storage.Image, addNames []string) error {
	names, err := ExpandNames(addNames, firstRegistry, systemContext, store)
	if err != nil {
		return err
	}
	err = store.SetNames(image.ID, append(image.Names, names...))
	if err != nil {
		return errors.Wrapf(err, "error adding names (%v) to image %q", names, image.ID)
	}
	return nil
}

// GetFailureCause checks the type of the error "err" and returns a new
// error message that reflects the reason of the failure.
// In case err type is not a familiar one the error "defaultError" is returned.
func GetFailureCause(err, defaultError error) error {
	switch nErr := errors.Cause(err).(type) {
	case errcode.Errors:
		return err
	case errcode.Error, *url.Error:
		return nErr
	default:
		return defaultError
	}
}

// WriteError writes `lastError` into `w` if not nil and return the next error `err`
func WriteError(w io.Writer, err error, lastError error) error {
	if lastError != nil {
		fmt.Fprintln(w, lastError)
	}
	return err
}
