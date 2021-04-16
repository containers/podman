package buildah

import (
	"context"
	"fmt"
	"math/rand"
	"runtime"
	"strings"

	"github.com/containers/buildah/define"
	"github.com/containers/buildah/util"
	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/image"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/pkg/shortnames"
	is "github.com/containers/image/v5/storage"
	"github.com/containers/image/v5/transports"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"
	"github.com/containers/storage"
	digest "github.com/opencontainers/go-digest"
	"github.com/openshift/imagebuilder"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	// BaseImageFakeName is the "name" of a source image which we interpret
	// as "no image".
	BaseImageFakeName = imagebuilder.NoBaseImageSpecifier
)

func pullAndFindImage(ctx context.Context, store storage.Store, srcRef types.ImageReference, options BuilderOptions, sc *types.SystemContext) (*storage.Image, types.ImageReference, error) {
	pullOptions := PullOptions{
		ReportWriter:     options.ReportWriter,
		Store:            store,
		SystemContext:    options.SystemContext,
		BlobDirectory:    options.BlobDirectory,
		MaxRetries:       options.MaxPullRetries,
		RetryDelay:       options.PullRetryDelay,
		OciDecryptConfig: options.OciDecryptConfig,
	}
	ref, err := pullImage(ctx, store, srcRef, pullOptions, sc)
	if err != nil {
		logrus.Debugf("error pulling image %q: %v", transports.ImageName(srcRef), err)
		return nil, nil, err
	}
	img, err := is.Transport.GetStoreImage(store, ref)
	if err != nil {
		logrus.Debugf("error reading pulled image %q: %v", transports.ImageName(srcRef), err)
		return nil, nil, errors.Wrapf(err, "error locating image %q in local storage", transports.ImageName(ref))
	}
	return img, ref, nil
}

func getImageName(name string, img *storage.Image) string {
	imageName := name
	if len(img.Names) > 0 {
		imageName = img.Names[0]
		// When the image used by the container is a tagged image
		// the container name might be set to the original image instead of
		// the image given in the "from" command line.
		// This loop is supposed to fix this.
		for _, n := range img.Names {
			if strings.Contains(n, name) {
				imageName = n
				break
			}
		}
	}
	return imageName
}

func imageNamePrefix(imageName string) string {
	prefix := imageName
	s := strings.Split(prefix, ":")
	if len(s) > 0 {
		prefix = s[0]
	}
	s = strings.Split(prefix, "/")
	if len(s) > 0 {
		prefix = s[len(s)-1]
	}
	s = strings.Split(prefix, "@")
	if len(s) > 0 {
		prefix = s[0]
	}
	return prefix
}

func newContainerIDMappingOptions(idmapOptions *define.IDMappingOptions) storage.IDMappingOptions {
	var options storage.IDMappingOptions
	if idmapOptions != nil {
		options.HostUIDMapping = idmapOptions.HostUIDMapping
		options.HostGIDMapping = idmapOptions.HostGIDMapping
		uidmap, gidmap := convertRuntimeIDMaps(idmapOptions.UIDMap, idmapOptions.GIDMap)
		if len(uidmap) > 0 && len(gidmap) > 0 {
			options.UIDMap = uidmap
			options.GIDMap = gidmap
		} else {
			options.HostUIDMapping = true
			options.HostGIDMapping = true
		}
	}
	return options
}

func resolveLocalImage(systemContext *types.SystemContext, store storage.Store, options BuilderOptions) (types.ImageReference, string, string, *storage.Image, error) {
	candidates, _, _, err := util.ResolveName(options.FromImage, options.Registry, systemContext, store)
	if err != nil {
		return nil, "", "", nil, errors.Wrapf(err, "error resolving local image %q", options.FromImage)
	}
	for _, imageName := range candidates {
		img, err := store.Image(imageName)
		if err != nil {
			if errors.Cause(err) == storage.ErrImageUnknown {
				continue
			}
			return nil, "", "", nil, err
		}
		ref, err := is.Transport.ParseStoreReference(store, img.ID)
		if err != nil {
			return nil, "", "", nil, errors.Wrapf(err, "error parsing reference to image %q", img.ID)
		}
		if !imageMatch(context.Background(), ref, systemContext) {
			logrus.Debugf("Found local image %s but it does not match the provided context", imageName)
			continue
		}
		return ref, ref.Transport().Name(), imageName, img, nil
	}

	return nil, "", "", nil, nil
}

func imageMatch(ctx context.Context, ref types.ImageReference, systemContext *types.SystemContext) bool {
	img, err := ref.NewImage(ctx, systemContext)
	if err != nil {
		logrus.Warnf("Failed to create newImage in imageMatch: %v", err)
		return false
	}
	defer img.Close()
	data, err := img.Inspect(ctx)
	if err != nil {
		logrus.Warnf("Failed to inspect img %s: %v", ref, err)
		return false
	}
	os := systemContext.OSChoice
	if os == "" {
		os = runtime.GOOS
	}
	arch := systemContext.ArchitectureChoice
	if arch == "" {
		arch = runtime.GOARCH
	}
	if os == data.Os && arch == data.Architecture {
		if systemContext.VariantChoice == "" || systemContext.VariantChoice == data.Variant {
			return true
		}
	}
	return false
}

func resolveImage(ctx context.Context, systemContext *types.SystemContext, store storage.Store, options BuilderOptions) (types.ImageReference, string, *storage.Image, error) {
	if systemContext == nil {
		systemContext = &types.SystemContext{}
	}

	fromImage := options.FromImage
	// If the image name includes a transport we can use it as it.  Special
	// treatment for docker references which are subject to pull policies
	// that we're handling below.
	srcRef, err := alltransports.ParseImageName(options.FromImage)
	if err == nil {
		if srcRef.Transport().Name() == docker.Transport.Name() {
			fromImage = srcRef.DockerReference().String()
		} else {
			pulledImg, pulledReference, err := pullAndFindImage(ctx, store, srcRef, options, systemContext)
			return pulledReference, srcRef.Transport().Name(), pulledImg, err
		}
	}

	localImageRef, _, localImageName, localImage, err := resolveLocalImage(systemContext, store, options)
	if err != nil {
		return nil, "", nil, err
	}

	// If we could resolve the image locally, check if it was clearly
	// referring to a local image, either by ID or digest.  In that case,
	// we don't need to perform a remote lookup.
	if localImage != nil && (strings.HasPrefix(localImage.ID, options.FromImage) || strings.HasPrefix(options.FromImage, "sha256:")) {
		return localImageRef, localImageRef.Transport().Name(), localImage, nil
	}

	if options.PullPolicy == define.PullNever || options.PullPolicy == define.PullIfMissing {
		if localImage != nil {
			return localImageRef, localImageRef.Transport().Name(), localImage, nil
		}
		if options.PullPolicy == define.PullNever {
			return nil, "", nil, errors.Errorf("pull policy is %q but %q could not be found locally", "never", options.FromImage)
		}
	}

	// If we found a local image, we must use it's name.
	// See #2904.
	if localImageRef != nil {
		fromImage = localImageName
	}

	resolved, err := shortnames.Resolve(systemContext, fromImage)
	if err != nil {
		return nil, "", nil, err
	}

	// Print the image-resolution description unless we're looking for a
	// new image and already found a local image.  In many cases, the
	// description will be more confusing than helpful (e.g., `buildah from
	// localImage`).
	if desc := resolved.Description(); len(desc) > 0 {
		logrus.Debug(desc)
		if !(options.PullPolicy == define.PullIfNewer && localImage != nil) {
			if options.ReportWriter != nil {
				if _, err := options.ReportWriter.Write([]byte(desc + "\n")); err != nil {
					return nil, "", nil, err
				}
			}
		}
	}

	var pullErrors []error
	for _, pullCandidate := range resolved.PullCandidates {
		ref, err := docker.NewReference(pullCandidate.Value)
		if err != nil {
			return nil, "", nil, err
		}

		// We're tasked to pull a "newer" image.  If there's no local
		// image, we have no base for comparison, so we'll pull the
		// first available image.
		//
		// If there's a local image, the `pullCandidate` is considered
		// to be newer if its time stamp differs from the local one.
		// Otherwise, we don't pull and skip it.
		if options.PullPolicy == define.PullIfNewer && localImage != nil {
			remoteImage, err := ref.NewImage(ctx, systemContext)
			if err != nil {
				logrus.Debugf("unable to remote-inspect image %q: %v", pullCandidate.Value.String(), err)
				pullErrors = append(pullErrors, err)
				continue
			}
			defer remoteImage.Close()

			remoteData, err := remoteImage.Inspect(ctx)
			if err != nil {
				logrus.Debugf("unable to remote-inspect image %q: %v", pullCandidate.Value.String(), err)
				pullErrors = append(pullErrors, err)
				continue
			}

			// FIXME: we should compare image digests not time stamps.
			// Comparing time stamps is flawed.  Be aware that fixing
			// it may entail non-trivial changes to the tests.  Please
			// refer to https://github.com/containers/buildah/issues/2779
			// for more.
			if localImage.Created.Equal(*remoteData.Created) {
				continue
			}
		}

		pulledImg, pulledReference, err := pullAndFindImage(ctx, store, ref, options, systemContext)
		if err != nil {
			logrus.Debugf("unable to pull and read image %q: %v", pullCandidate.Value.String(), err)
			pullErrors = append(pullErrors, err)
			continue
		}

		// Make sure to record the short-name alias if necessary.
		if err = pullCandidate.Record(); err != nil {
			return nil, "", nil, err
		}

		return pulledReference, "", pulledImg, nil
	}

	// If we were looking for a newer image but could not find one, return
	// the local image if present.
	if options.PullPolicy == define.PullIfNewer && localImage != nil {
		return localImageRef, localImageRef.Transport().Name(), localImage, nil
	}

	return nil, "", nil, resolved.FormatPullErrors(pullErrors)
}

func containerNameExist(name string, containers []storage.Container) bool {
	for _, container := range containers {
		for _, cname := range container.Names {
			if cname == name {
				return true
			}
		}
	}
	return false
}

func findUnusedContainer(name string, containers []storage.Container) string {
	suffix := 1
	tmpName := name
	for containerNameExist(tmpName, containers) {
		tmpName = fmt.Sprintf("%s-%d", name, suffix)
		suffix++
	}
	return tmpName
}

func newBuilder(ctx context.Context, store storage.Store, options BuilderOptions) (*Builder, error) {
	var (
		ref types.ImageReference
		img *storage.Image
		err error
	)
	if options.FromImage == BaseImageFakeName {
		options.FromImage = ""
	}

	systemContext := getSystemContext(store, options.SystemContext, options.SignaturePolicyPath)

	if options.FromImage != "" && options.FromImage != "scratch" {
		ref, _, img, err = resolveImage(ctx, systemContext, store, options)
		if err != nil {
			return nil, err
		}
	}
	imageSpec := options.FromImage
	imageID := ""
	imageDigest := ""
	topLayer := ""
	if img != nil {
		imageSpec = getImageName(imageNamePrefix(imageSpec), img)
		imageID = img.ID
		topLayer = img.TopLayer
	}
	var src types.Image
	if ref != nil {
		srcSrc, err := ref.NewImageSource(ctx, systemContext)
		if err != nil {
			return nil, errors.Wrapf(err, "error instantiating image for %q", transports.ImageName(ref))
		}
		defer srcSrc.Close()
		manifestBytes, manifestType, err := srcSrc.GetManifest(ctx, nil)
		if err != nil {
			return nil, errors.Wrapf(err, "error loading image manifest for %q", transports.ImageName(ref))
		}
		if manifestDigest, err := manifest.Digest(manifestBytes); err == nil {
			imageDigest = manifestDigest.String()
		}
		var instanceDigest *digest.Digest
		if manifest.MIMETypeIsMultiImage(manifestType) {
			list, err := manifest.ListFromBlob(manifestBytes, manifestType)
			if err != nil {
				return nil, errors.Wrapf(err, "error parsing image manifest for %q as list", transports.ImageName(ref))
			}
			instance, err := list.ChooseInstance(systemContext)
			if err != nil {
				return nil, errors.Wrapf(err, "error finding an appropriate image in manifest list %q", transports.ImageName(ref))
			}
			instanceDigest = &instance
		}
		src, err = image.FromUnparsedImage(ctx, systemContext, image.UnparsedInstance(srcSrc, instanceDigest))
		if err != nil {
			return nil, errors.Wrapf(err, "error instantiating image for %q instance %q", transports.ImageName(ref), instanceDigest)
		}
	}

	name := "working-container"
	if options.Container != "" {
		name = options.Container
	} else {
		if imageSpec != "" {
			name = imageNamePrefix(imageSpec) + "-" + name
		}
	}
	var container *storage.Container
	tmpName := name
	if options.Container == "" {
		containers, err := store.Containers()
		if err != nil {
			return nil, errors.Wrapf(err, "unable to check for container names")
		}
		tmpName = findUnusedContainer(tmpName, containers)
	}

	conflict := 100
	for {
		coptions := storage.ContainerOptions{
			LabelOpts:        options.CommonBuildOpts.LabelOpts,
			IDMappingOptions: newContainerIDMappingOptions(options.IDMappingOptions),
			Volatile:         true,
		}
		container, err = store.CreateContainer("", []string{tmpName}, imageID, "", "", &coptions)
		if err == nil {
			name = tmpName
			break
		}
		if errors.Cause(err) != storage.ErrDuplicateName || options.Container != "" {
			return nil, errors.Wrapf(err, "error creating container")
		}
		tmpName = fmt.Sprintf("%s-%d", name, rand.Int()%conflict)
		conflict = conflict * 10
	}
	defer func() {
		if err != nil {
			if err2 := store.DeleteContainer(container.ID); err2 != nil {
				logrus.Errorf("error deleting container %q: %v", container.ID, err2)
			}
		}
	}()

	uidmap, gidmap := convertStorageIDMaps(container.UIDMap, container.GIDMap)

	defaultNamespaceOptions, err := DefaultNamespaceOptions()
	if err != nil {
		return nil, err
	}

	namespaceOptions := defaultNamespaceOptions
	namespaceOptions.AddOrReplace(options.NamespaceOptions...)

	builder := &Builder{
		store:                 store,
		Type:                  containerType,
		FromImage:             imageSpec,
		FromImageID:           imageID,
		FromImageDigest:       imageDigest,
		Container:             name,
		ContainerID:           container.ID,
		ImageAnnotations:      map[string]string{},
		ImageCreatedBy:        "",
		ProcessLabel:          container.ProcessLabel(),
		MountLabel:            container.MountLabel(),
		DefaultMountsFilePath: options.DefaultMountsFilePath,
		Isolation:             options.Isolation,
		NamespaceOptions:      namespaceOptions,
		ConfigureNetwork:      options.ConfigureNetwork,
		CNIPluginPath:         options.CNIPluginPath,
		CNIConfigDir:          options.CNIConfigDir,
		IDMappingOptions: define.IDMappingOptions{
			HostUIDMapping: len(uidmap) == 0,
			HostGIDMapping: len(uidmap) == 0,
			UIDMap:         uidmap,
			GIDMap:         gidmap,
		},
		Capabilities:    copyStringSlice(options.Capabilities),
		CommonBuildOpts: options.CommonBuildOpts,
		TopLayer:        topLayer,
		Args:            options.Args,
		Format:          options.Format,
		TempVolumes:     map[string]bool{},
		Devices:         options.Devices,
	}

	if options.Mount {
		_, err = builder.Mount(container.MountLabel())
		if err != nil {
			return nil, errors.Wrapf(err, "error mounting build container %q", builder.ContainerID)
		}
	}

	if err := builder.initConfig(ctx, src); err != nil {
		return nil, errors.Wrapf(err, "error preparing image configuration")
	}
	err = builder.Save()
	if err != nil {
		return nil, errors.Wrapf(err, "error saving builder state for container %q", builder.ContainerID)
	}

	return builder, nil
}
