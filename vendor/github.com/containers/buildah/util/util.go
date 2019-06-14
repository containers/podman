package util

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"strings"
	"syscall"

	"github.com/containers/image/docker/reference"
	"github.com/containers/image/pkg/sysregistriesv2"
	"github.com/containers/image/signature"
	is "github.com/containers/image/storage"
	"github.com/containers/image/transports"
	"github.com/containers/image/types"
	"github.com/containers/storage"
	"github.com/docker/distribution/registry/api/errcode"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	minimumTruncatedIDLength = 3
	// DefaultTransport is a prefix that we apply to an image name if we
	// can't find one in the local Store, in order to generate a source
	// reference for the image that we can then copy to the local Store.
	DefaultTransport = "docker://"
)

var (
	// RegistryDefaultPathPrefix contains a per-registry listing of default prefixes
	// to prepend to image names that only contain a single path component.
	RegistryDefaultPathPrefix = map[string]string{
		"index.docker.io": "library",
		"docker.io":       "library",
	}
)

// ResolveName checks if name is a valid image name, and if that name doesn't
// include a domain portion, returns a list of the names which it might
// correspond to in the set of configured registries, the transport used to
// pull the image, and a boolean which is true iff
// 1) the list of search registries was used, and 2) it was empty.
//
// The returned image names never include a transport: prefix, and if transport != "",
// (transport, image) should be a valid input to alltransports.ParseImageName.
// transport == "" indicates that image that already exists in a local storage,
// and the name is valid for store.Image() / storage.Transport.ParseStoreReference().
//
// NOTE: The "list of search registries is empty" check does not count blocked registries,
// and neither the implied "localhost" nor a possible firstRegistry are counted
func ResolveName(name string, firstRegistry string, sc *types.SystemContext, store storage.Store) ([]string, string, bool, error) {
	if name == "" {
		return nil, "", false, nil
	}

	// Maybe it's a truncated image ID.  Don't prepend a registry name, then.
	if len(name) >= minimumTruncatedIDLength {
		if img, err := store.Image(name); err == nil && img != nil && strings.HasPrefix(img.ID, name) {
			// It's a truncated version of the ID of an image that's present in local storage;
			// we need only expand the ID.
			return []string{img.ID}, "", false, nil
		}
	}

	// If the image includes a transport's name as a prefix, use it as-is.
	if strings.HasPrefix(name, DefaultTransport) {
		return []string{strings.TrimPrefix(name, DefaultTransport)}, DefaultTransport, false, nil
	}
	split := strings.SplitN(name, ":", 2)
	if len(split) == 2 {
		if trans := transports.Get(split[0]); trans != nil {
			return []string{split[1]}, trans.Name(), false, nil
		}
	}
	// If the image name already included a domain component, we're done.
	named, err := reference.ParseNormalizedNamed(name)
	if err != nil {
		return nil, "", false, errors.Wrapf(err, "error parsing image name %q", name)
	}
	if named.String() == name {
		// Parsing produced the same result, so there was a domain name in there to begin with.
		return []string{name}, DefaultTransport, false, nil
	}
	if reference.Domain(named) != "" && RegistryDefaultPathPrefix[reference.Domain(named)] != "" {
		// If this domain can cause us to insert something in the middle, check if that happened.
		repoPath := reference.Path(named)
		domain := reference.Domain(named)
		tag := ""
		if tagged, ok := named.(reference.Tagged); ok {
			tag = ":" + tagged.Tag()
		}
		digest := ""
		if digested, ok := named.(reference.Digested); ok {
			digest = "@" + digested.Digest().String()
		}
		defaultPrefix := RegistryDefaultPathPrefix[reference.Domain(named)] + "/"
		if strings.HasPrefix(repoPath, defaultPrefix) && path.Join(domain, repoPath[len(defaultPrefix):])+tag+digest == name {
			// Yup, parsing just inserted a bit in the middle, so there was a domain name there to begin with.
			return []string{name}, DefaultTransport, false, nil
		}
	}

	// Figure out the list of registries.
	var registries []string
	searchRegistries, err := sysregistriesv2.UnqualifiedSearchRegistries(sc)
	if err != nil {
		logrus.Debugf("unable to read configured registries to complete %q: %v", name, err)
		searchRegistries = nil
	}
	for _, registry := range searchRegistries {
		reg, err := sysregistriesv2.FindRegistry(sc, registry)
		if err != nil {
			logrus.Debugf("unable to read registry configuraitno for %#v: %v", registry, err)
			continue
		}
		if reg == nil || !reg.Blocked {
			registries = append(registries, registry)
		}
	}
	searchRegistriesAreEmpty := len(registries) == 0

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
	return candidates, DefaultTransport, searchRegistriesAreEmpty, nil
}

// ExpandNames takes unqualified names, parses them as image names, and returns
// the fully expanded result, including a tag.  Names which don't include a registry
// name will be marked for the most-preferred registry (i.e., the first one in our
// configuration).
func ExpandNames(names []string, firstRegistry string, systemContext *types.SystemContext, store storage.Store) ([]string, error) {
	expanded := make([]string, 0, len(names))
	for _, n := range names {
		var name reference.Named
		nameList, _, _, err := ResolveName(n, firstRegistry, systemContext, store)
		if err != nil {
			return nil, errors.Wrapf(err, "error parsing name %q", n)
		}
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
		expanded = append(expanded, name.String())
	}
	return expanded, nil
}

// FindImage locates the locally-stored image which corresponds to a given name.
func FindImage(store storage.Store, firstRegistry string, systemContext *types.SystemContext, image string) (types.ImageReference, *storage.Image, error) {
	var ref types.ImageReference
	var img *storage.Image
	var err error
	names, _, _, err := ResolveName(image, firstRegistry, systemContext, store)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "error parsing name %q", image)
	}
	for _, name := range names {
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
		return nil, nil, errors.Wrapf(err, "error locating image with name %q (%v)", image, names)
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

// Runtime is the default command to use to run the container.
func Runtime() string {
	runtime := os.Getenv("BUILDAH_RUNTIME")
	if runtime != "" {
		return runtime
	}
	return DefaultRuntime
}

// StringInSlice returns a boolean indicating if the exact value s is present
// in the slice slice.
func StringInSlice(s string, slice []string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

// GetContainerIDs uses ID mappings to compute the container-level IDs that will
// correspond to a UID/GID pair on the host.
func GetContainerIDs(uidmap, gidmap []specs.LinuxIDMapping, uid, gid uint32) (uint32, uint32, error) {
	uidMapped := true
	for _, m := range uidmap {
		uidMapped = false
		if uid >= m.HostID && uid < m.HostID+m.Size {
			uid = (uid - m.HostID) + m.ContainerID
			uidMapped = true
			break
		}
	}
	if !uidMapped {
		return 0, 0, errors.Errorf("container uses ID mappings (%#v), but doesn't map UID %d", uidmap, uid)
	}
	gidMapped := true
	for _, m := range gidmap {
		gidMapped = false
		if gid >= m.HostID && gid < m.HostID+m.Size {
			gid = (gid - m.HostID) + m.ContainerID
			gidMapped = true
			break
		}
	}
	if !gidMapped {
		return 0, 0, errors.Errorf("container uses ID mappings (%#v), but doesn't map GID %d", gidmap, gid)
	}
	return uid, gid, nil
}

// GetHostIDs uses ID mappings to compute the host-level IDs that will
// correspond to a UID/GID pair in the container.
func GetHostIDs(uidmap, gidmap []specs.LinuxIDMapping, uid, gid uint32) (uint32, uint32, error) {
	uidMapped := true
	for _, m := range uidmap {
		uidMapped = false
		if uid >= m.ContainerID && uid < m.ContainerID+m.Size {
			uid = (uid - m.ContainerID) + m.HostID
			uidMapped = true
			break
		}
	}
	if !uidMapped {
		return 0, 0, errors.Errorf("container uses ID mappings (%#v), but doesn't map UID %d", uidmap, uid)
	}
	gidMapped := true
	for _, m := range gidmap {
		gidMapped = false
		if gid >= m.ContainerID && gid < m.ContainerID+m.Size {
			gid = (gid - m.ContainerID) + m.HostID
			gidMapped = true
			break
		}
	}
	if !gidMapped {
		return 0, 0, errors.Errorf("container uses ID mappings (%#v), but doesn't map GID %d", gidmap, gid)
	}
	return uid, gid, nil
}

// GetHostRootIDs uses ID mappings in spec to compute the host-level IDs that will
// correspond to UID/GID 0/0 in the container.
func GetHostRootIDs(spec *specs.Spec) (uint32, uint32, error) {
	if spec.Linux == nil {
		return 0, 0, nil
	}
	return GetHostIDs(spec.Linux.UIDMappings, spec.Linux.GIDMappings, 0, 0)
}

// GetPolicyContext sets up, initializes and returns a new context for the specified policy
func GetPolicyContext(ctx *types.SystemContext) (*signature.PolicyContext, error) {
	policy, err := signature.DefaultPolicy(ctx)
	if err != nil {
		return nil, err
	}

	policyContext, err := signature.NewPolicyContext(policy)
	if err != nil {
		return nil, err
	}
	return policyContext, nil
}

// logIfNotErrno logs the error message unless err is either nil or one of the
// listed syscall.Errno values.  It returns true if it logged an error.
func logIfNotErrno(err error, what string, ignores ...syscall.Errno) (logged bool) {
	if err == nil {
		return false
	}
	if errno, isErrno := err.(syscall.Errno); isErrno {
		for _, ignore := range ignores {
			if errno == ignore {
				return false
			}
		}
	}
	logrus.Error(what)
	return true
}

// LogIfNotRetryable logs "what" if err is set and is not an EINTR or EAGAIN
// syscall.Errno.  Returns "true" if we can continue.
func LogIfNotRetryable(err error, what string) (retry bool) {
	return !logIfNotErrno(err, what, syscall.EINTR, syscall.EAGAIN)
}

// LogIfUnexpectedWhileDraining logs "what" if err is set and is not an EINTR
// or EAGAIN or EIO syscall.Errno.
func LogIfUnexpectedWhileDraining(err error, what string) {
	logIfNotErrno(err, what, syscall.EINTR, syscall.EAGAIN, syscall.EIO)
}
