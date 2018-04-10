package util

import (
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"
	"time"

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

// ResolveName checks if name is a valid image name, and if that name doesn't include a domain
// portion, returns a list of the names which it might correspond to in the registries.
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
		logrus.Debugf("unable to complete image name %q: %v", name, err)
		return []string{name}
	}
	if sc.DockerInsecureSkipTLSVerify {
		if unverifiedRegistries, err := sysregistries.GetInsecureRegistries(sc); err == nil {
			registries = append(registries, unverifiedRegistries...)
		}
	}

	// Create all of the combinations.  Some registries need an additional component added, so
	// use our lookaside map to keep track of them.  If there are no configured registries, at
	// least return the name as it was passed to us.
	candidates := []string{}
	for _, registry := range append([]string{firstRegistry}, registries...) {
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
	if len(candidates) == 0 {
		candidates = append(candidates, name)
	}
	return candidates
}

// ExpandNames takes unqualified names, parses them as image names, and returns
// the fully expanded result, including a tag.  Names which don't include a registry
// name will be marked for the most-preferred registry (i.e., the first one in our
// configuration).
func ExpandNames(names []string) ([]string, error) {
	expanded := make([]string, 0, len(names))
	for _, n := range names {
		name, err := reference.ParseNormalizedNamed(n)
		if err != nil {
			return nil, errors.Wrapf(err, "error parsing name %q", n)
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
func FindImage(store storage.Store, image string) (*storage.Image, error) {
	var img *storage.Image
	ref, err := is.Transport.ParseStoreReference(store, image)
	if err == nil {
		img, err = is.Transport.GetStoreImage(store, ref)
	}
	if err != nil {
		img2, err2 := store.Image(image)
		if err2 != nil {
			if ref == nil {
				return nil, errors.Wrapf(err, "error parsing reference to image %q", image)
			}
			return nil, errors.Wrapf(err, "unable to locate image %q", image)
		}
		img = img2
	}
	return img, nil
}

// AddImageNames adds the specified names to the specified image.
func AddImageNames(store storage.Store, image *storage.Image, addNames []string) error {
	names, err := ExpandNames(addNames)
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

// GetLocalTime discover the UTC offset and then add that to the
// passed in time to arrive at the local time.
func GetLocalTime(localTime time.Time) time.Time {
	t := time.Now()
	_, offset := t.Local().Zone()
	localTime = localTime.Add(time.Second * time.Duration(offset))
	return localTime
}

// WriteError writes `lastError` into `w` if not nil and return the next error `err`
func WriteError(w io.Writer, err error, lastError error) error {
	if lastError != nil {
		fmt.Fprintln(w, lastError)
	}
	return err
}
