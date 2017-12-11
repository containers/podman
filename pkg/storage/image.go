package storage

import (
	"errors"
	"fmt"
	"net"
	"path"
	"regexp"
	"strings"

	"github.com/containers/image/copy"
	"github.com/containers/image/docker/reference"
	"github.com/containers/image/image"
	"github.com/containers/image/signature"
	istorage "github.com/containers/image/storage"
	"github.com/containers/image/transports/alltransports"
	"github.com/containers/image/types"
	"github.com/containers/storage"
	distreference "github.com/docker/distribution/reference"
)

// ImageResult wraps a subset of information about an image: its ID, its names,
// and the size, if known, or nil if it isn't.
type ImageResult struct {
	ID       string
	Names    []string
	Digests  []string
	Size     *uint64
	ImageRef string
}

type indexInfo struct {
	name   string
	secure bool
}

type imageService struct {
	store                 storage.Store
	defaultTransport      string
	insecureRegistryCIDRs []*net.IPNet
	indexConfigs          map[string]*indexInfo
	registries            []string
}

// ImageServer wraps up various CRI-related activities into a reusable
// implementation.
type ImageServer interface {
	// ListImages returns list of all images which match the filter.
	ListImages(systemContext *types.SystemContext, filter string) ([]ImageResult, error)
	// ImageStatus returns status of an image which matches the filter.
	ImageStatus(systemContext *types.SystemContext, filter string) (*ImageResult, error)
	// PullImage imports an image from the specified location.
	PullImage(systemContext *types.SystemContext, imageName string, options *copy.Options) (types.ImageReference, error)
	// UntagImage removes a name from the specified image, and if it was
	// the only name the image had, removes the image.
	UntagImage(systemContext *types.SystemContext, imageName string) error
	// RemoveImage deletes the specified image.
	RemoveImage(systemContext *types.SystemContext, imageName string) error
	// GetStore returns the reference to the storage library Store which
	// the image server uses to hold images, and is the destination used
	// when it's asked to pull an image.
	GetStore() storage.Store
	// CanPull preliminary checks whether we're allowed to pull an image
	CanPull(imageName string, options *copy.Options) (bool, error)
	// ResolveNames takes an image reference and if it's unqualified (w/o hostname),
	// it uses crio's default registries to qualify it.
	ResolveNames(imageName string) ([]string, error)
}

func (svc *imageService) getRef(name string) (types.ImageReference, error) {
	ref, err := alltransports.ParseImageName(name)
	if err != nil {
		ref2, err2 := istorage.Transport.ParseStoreReference(svc.store, "@"+name)
		if err2 != nil {
			ref3, err3 := istorage.Transport.ParseStoreReference(svc.store, name)
			if err3 != nil {
				return nil, err
			}
			ref2 = ref3
		}
		ref = ref2
	}
	return ref, nil
}

func (svc *imageService) ListImages(systemContext *types.SystemContext, filter string) ([]ImageResult, error) {
	results := []ImageResult{}
	if filter != "" {
		ref, err := svc.getRef(filter)
		if err != nil {
			return nil, err
		}
		if image, err := istorage.Transport.GetStoreImage(svc.store, ref); err == nil {
			img, err := ref.NewImage(systemContext)
			if err != nil {
				return nil, err
			}
			size := imageSize(img)
			img.Close()
			results = append(results, ImageResult{
				ID:    image.ID,
				Names: image.Names,
				Size:  size,
			})
		}
	} else {
		images, err := svc.store.Images()
		if err != nil {
			return nil, err
		}
		for _, image := range images {
			ref, err := istorage.Transport.ParseStoreReference(svc.store, "@"+image.ID)
			if err != nil {
				return nil, err
			}
			img, err := ref.NewImage(systemContext)
			if err != nil {
				return nil, err
			}
			size := imageSize(img)
			img.Close()
			results = append(results, ImageResult{
				ID:    image.ID,
				Names: image.Names,
				Size:  size,
			})
		}
	}
	return results, nil
}

func (svc *imageService) ImageStatus(systemContext *types.SystemContext, nameOrID string) (*ImageResult, error) {
	ref, err := alltransports.ParseImageName(nameOrID)
	if err != nil {
		ref2, err2 := istorage.Transport.ParseStoreReference(svc.store, "@"+nameOrID)
		if err2 != nil {
			ref3, err3 := istorage.Transport.ParseStoreReference(svc.store, nameOrID)
			if err3 != nil {
				return nil, err
			}
			ref2 = ref3
		}
		ref = ref2
	}
	image, err := istorage.Transport.GetStoreImage(svc.store, ref)
	if err != nil {
		return nil, err
	}

	img, err := ref.NewImage(systemContext)
	if err != nil {
		return nil, err
	}
	size := imageSize(img)
	img.Close()

	result := ImageResult{
		ID:    image.ID,
		Names: image.Names,
		Size:  size,
	}
	if len(image.Names) > 0 {
		result.ImageRef = image.Names[0]
		if ref2, err2 := istorage.Transport.ParseStoreReference(svc.store, image.Names[0]); err2 == nil {
			if dref := ref2.DockerReference(); dref != nil {
				result.ImageRef = reference.FamiliarString(dref)
			}
		}
	}

	return &result, nil
}

func imageSize(img types.Image) *uint64 {
	if sum, err := img.Size(); err == nil {
		usum := uint64(sum)
		return &usum
	}
	return nil
}

func (svc *imageService) CanPull(imageName string, options *copy.Options) (bool, error) {
	srcRef, err := svc.prepareImage(imageName, options)
	if err != nil {
		return false, err
	}
	rawSource, err := srcRef.NewImageSource(options.SourceCtx)
	if err != nil {
		return false, err
	}
	src, err := image.FromSource(options.SourceCtx, rawSource)
	if err != nil {
		rawSource.Close()
		return false, err
	}
	src.Close()
	return true, nil
}

// prepareImage creates an image reference from an image string and set options
// for the source context
func (svc *imageService) prepareImage(imageName string, options *copy.Options) (types.ImageReference, error) {
	if imageName == "" {
		return nil, storage.ErrNotAnImage
	}

	srcRef, err := alltransports.ParseImageName(imageName)
	if err != nil {
		if svc.defaultTransport == "" {
			return nil, err
		}
		srcRef2, err2 := alltransports.ParseImageName(svc.defaultTransport + imageName)
		if err2 != nil {
			return nil, err
		}
		srcRef = srcRef2
	}

	if options.SourceCtx == nil {
		options.SourceCtx = &types.SystemContext{}
	}

	hostname := reference.Domain(srcRef.DockerReference())
	if secure := svc.isSecureIndex(hostname); !secure {
		options.SourceCtx.DockerInsecureSkipTLSVerify = !secure
	}
	return srcRef, nil
}

func (svc *imageService) PullImage(systemContext *types.SystemContext, imageName string, options *copy.Options) (types.ImageReference, error) {
	policy, err := signature.DefaultPolicy(systemContext)
	if err != nil {
		return nil, err
	}
	policyContext, err := signature.NewPolicyContext(policy)
	if err != nil {
		return nil, err
	}
	if options == nil {
		options = &copy.Options{}
	}

	srcRef, err := svc.prepareImage(imageName, options)
	if err != nil {
		return nil, err
	}

	dest := imageName
	if srcRef.DockerReference() != nil {
		dest = srcRef.DockerReference().Name()
		if tagged, ok := srcRef.DockerReference().(reference.NamedTagged); ok {
			dest = dest + ":" + tagged.Tag()
		}
		if canonical, ok := srcRef.DockerReference().(reference.Canonical); ok {
			dest = dest + "@" + canonical.Digest().String()
		}
	}
	destRef, err := istorage.Transport.ParseStoreReference(svc.store, dest)
	if err != nil {
		return nil, err
	}
	err = copy.Image(policyContext, destRef, srcRef, options)
	if err != nil {
		return nil, err
	}
	return destRef, nil
}

func (svc *imageService) UntagImage(systemContext *types.SystemContext, nameOrID string) error {
	ref, err := alltransports.ParseImageName(nameOrID)
	if err != nil {
		ref2, err2 := istorage.Transport.ParseStoreReference(svc.store, "@"+nameOrID)
		if err2 != nil {
			ref3, err3 := istorage.Transport.ParseStoreReference(svc.store, nameOrID)
			if err3 != nil {
				return err
			}
			ref2 = ref3
		}
		ref = ref2
	}

	img, err := istorage.Transport.GetStoreImage(svc.store, ref)
	if err != nil {
		return err
	}

	if nameOrID != img.ID {
		namedRef, err := svc.prepareImage(nameOrID, &copy.Options{})
		if err != nil {
			return err
		}

		name := nameOrID
		if namedRef.DockerReference() != nil {
			name = namedRef.DockerReference().Name()
			if tagged, ok := namedRef.DockerReference().(reference.NamedTagged); ok {
				name = name + ":" + tagged.Tag()
			}
			if canonical, ok := namedRef.DockerReference().(reference.Canonical); ok {
				name = name + "@" + canonical.Digest().String()
			}
		}

		prunedNames := make([]string, 0, len(img.Names))
		for _, imgName := range img.Names {
			if imgName != name && imgName != nameOrID {
				prunedNames = append(prunedNames, imgName)
			}
		}

		if len(prunedNames) > 0 {
			return svc.store.SetNames(img.ID, prunedNames)
		}
	}

	return ref.DeleteImage(systemContext)
}

func (svc *imageService) RemoveImage(systemContext *types.SystemContext, nameOrID string) error {
	ref, err := alltransports.ParseImageName(nameOrID)
	if err != nil {
		ref2, err2 := istorage.Transport.ParseStoreReference(svc.store, "@"+nameOrID)
		if err2 != nil {
			ref3, err3 := istorage.Transport.ParseStoreReference(svc.store, nameOrID)
			if err3 != nil {
				return err
			}
			ref2 = ref3
		}
		ref = ref2
	}
	return ref.DeleteImage(systemContext)
}

func (svc *imageService) GetStore() storage.Store {
	return svc.store
}

func (svc *imageService) isSecureIndex(indexName string) bool {
	if index, ok := svc.indexConfigs[indexName]; ok {
		return index.secure
	}

	host, _, err := net.SplitHostPort(indexName)
	if err != nil {
		// assume indexName is of the form `host` without the port and go on.
		host = indexName
	}

	addrs, err := net.LookupIP(host)
	if err != nil {
		ip := net.ParseIP(host)
		if ip != nil {
			addrs = []net.IP{ip}
		}

		// if ip == nil, then `host` is neither an IP nor it could be looked up,
		// either because the index is unreachable, or because the index is behind an HTTP proxy.
		// So, len(addrs) == 0 and we're not aborting.
	}

	// Try CIDR notation only if addrs has any elements, i.e. if `host`'s IP could be determined.
	for _, addr := range addrs {
		for _, ipnet := range svc.insecureRegistryCIDRs {
			// check if the addr falls in the subnet
			if ipnet.Contains(addr) {
				return false
			}
		}
	}

	return true
}

func isValidHostname(hostname string) bool {
	return hostname != "" && !strings.Contains(hostname, "/") &&
		(strings.Contains(hostname, ".") ||
			strings.Contains(hostname, ":") || hostname == "localhost")
}

func isReferenceFullyQualified(reposName reference.Named) bool {
	indexName, _, _ := splitReposName(reposName)
	return indexName != ""
}

const (
	// defaultHostname is the default built-in hostname
	defaultHostname = "docker.io"
	// legacyDefaultHostname is automatically converted to DefaultHostname
	legacyDefaultHostname = "index.docker.io"
	// defaultRepoPrefix is the prefix used for default repositories in default host
	defaultRepoPrefix = "library/"
)

// splitReposName breaks a reposName into an index name and remote name
func splitReposName(reposName reference.Named) (indexName string, remoteName reference.Named, err error) {
	var remoteNameStr string
	indexName, remoteNameStr = distreference.SplitHostname(reposName)
	if !isValidHostname(indexName) {
		// This is a Docker Index repos (ex: samalba/hipache or ubuntu)
		// 'docker.io'
		indexName = ""
		remoteName = reposName
	} else {
		remoteName, err = withName(remoteNameStr)
	}
	return
}

func validateName(name string) error {
	if err := validateID(strings.TrimPrefix(name, defaultHostname+"/")); err == nil {
		return fmt.Errorf("Invalid repository name (%s), cannot specify 64-byte hexadecimal strings", name)
	}
	return nil
}

var validHex = regexp.MustCompile(`^([a-f0-9]{64})$`)

// validateID checks whether an ID string is a valid image ID.
func validateID(id string) error {
	if ok := validHex.MatchString(id); !ok {
		return fmt.Errorf("image ID %q is invalid", id)
	}
	return nil
}

// withName returns a named object representing the given string. If the input
// is invalid ErrReferenceInvalidFormat will be returned.
func withName(name string) (reference.Named, error) {
	name, err := normalize(name)
	if err != nil {
		return nil, err
	}
	if err := validateName(name); err != nil {
		return nil, err
	}
	r, err := distreference.WithName(name)
	return r, err
}

// splitHostname splits a repository name to hostname and remotename string.
// If no valid hostname is found, empty string will be returned as a resulting
// hostname. Repository name needs to be already validated before.
func splitHostname(name string) (hostname, remoteName string) {
	i := strings.IndexRune(name, '/')
	if i == -1 || (!strings.ContainsAny(name[:i], ".:") && name[:i] != "localhost") {
		hostname, remoteName = "", name
	} else {
		hostname, remoteName = name[:i], name[i+1:]
	}
	if hostname == legacyDefaultHostname {
		hostname = defaultHostname
	}
	if hostname == defaultHostname && !strings.ContainsRune(remoteName, '/') {
		remoteName = defaultRepoPrefix + remoteName
	}
	return
}

// normalize returns a repository name in its normalized form, meaning it
// will contain library/ prefix for official images.
func normalize(name string) (string, error) {
	host, remoteName := splitHostname(name)
	if strings.ToLower(remoteName) != remoteName {
		return "", errors.New("invalid reference format: repository name must be lowercase")
	}
	if host == defaultHostname {
		if strings.HasPrefix(remoteName, defaultRepoPrefix) {
			remoteName = strings.TrimPrefix(remoteName, defaultRepoPrefix)
		}
		return host + "/" + remoteName, nil
	}
	return name, nil
}

func (svc *imageService) ResolveNames(imageName string) ([]string, error) {
	r, err := reference.ParseNormalizedNamed(imageName)
	if err != nil {
		return nil, err
	}
	if isReferenceFullyQualified(r) {
		// this means the image is already fully qualified
		return []string{imageName}, nil
	}
	// we got an unqualified image here, we can't go ahead w/o registries configured
	// properly.
	if len(svc.registries) == 0 {
		return nil, errors.New("no registries configured while trying to pull an unqualified image")
	}
	// this means we got an image in the form of "busybox"
	// we need to use additional registries...
	// normalize the unqualified image to be domain/repo/image...
	_, rest := splitDomain(r.Name())
	images := []string{}
	for _, r := range svc.registries {
		images = append(images, path.Join(r, rest))
	}
	return images, nil
}

// GetImageService returns an ImageServer that uses the passed-in store, and
// which will prepend the passed-in defaultTransport value to an image name if
// a name that's passed to its PullImage() method can't be resolved to an image
// in the store and can't be resolved to a source on its own.
func GetImageService(store storage.Store, defaultTransport string, insecureRegistries []string, registries []string) (ImageServer, error) {
	if store == nil {
		var err error
		store, err = storage.GetStore(storage.DefaultStoreOptions)
		if err != nil {
			return nil, err
		}
	}

	seenRegistries := make(map[string]bool, len(registries))
	cleanRegistries := []string{}
	for _, r := range registries {
		if seenRegistries[r] {
			continue
		}
		cleanRegistries = append(cleanRegistries, r)
		seenRegistries[r] = true
	}

	is := &imageService{
		store:                 store,
		defaultTransport:      defaultTransport,
		indexConfigs:          make(map[string]*indexInfo),
		insecureRegistryCIDRs: make([]*net.IPNet, 0),
		registries:            cleanRegistries,
	}

	insecureRegistries = append(insecureRegistries, "127.0.0.0/8")
	// Split --insecure-registry into CIDR and registry-specific settings.
	for _, r := range insecureRegistries {
		// Check if CIDR was passed to --insecure-registry
		_, ipnet, err := net.ParseCIDR(r)
		if err == nil {
			// Valid CIDR.
			is.insecureRegistryCIDRs = append(is.insecureRegistryCIDRs, ipnet)
		} else {
			// Assume `host:port` if not CIDR.
			is.indexConfigs[r] = &indexInfo{
				name:   r,
				secure: false,
			}
		}
	}

	return is, nil
}
