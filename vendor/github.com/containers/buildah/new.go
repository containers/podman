package buildah

import (
	"context"
	"fmt"
	"strings"

	"github.com/containers/buildah/util"
	"github.com/containers/image/pkg/sysregistries"
	is "github.com/containers/image/storage"
	"github.com/containers/image/transports"
	"github.com/containers/image/transports/alltransports"
	"github.com/containers/image/types"
	"github.com/containers/storage"
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/openshift/imagebuilder"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	// BaseImageFakeName is the "name" of a source image which we interpret
	// as "no image".
	BaseImageFakeName = imagebuilder.NoBaseImageSpecifier

	// DefaultTransport is a prefix that we apply to an image name if we
	// can't find one in the local Store, in order to generate a source
	// reference for the image that we can then copy to the local Store.
	DefaultTransport = "docker://"

	// minimumTruncatedIDLength is the minimum length of an identifier that
	// we'll accept as possibly being a truncated image ID.
	minimumTruncatedIDLength = 3
)

func pullAndFindImage(ctx context.Context, store storage.Store, imageName string, options BuilderOptions, sc *types.SystemContext) (*storage.Image, types.ImageReference, error) {
	pullOptions := PullOptions{
		ReportWriter:  options.ReportWriter,
		Store:         store,
		SystemContext: options.SystemContext,
		Transport:     options.Transport,
	}
	ref, err := pullImage(ctx, store, imageName, pullOptions, sc)
	if err != nil {
		logrus.Debugf("error pulling image %q: %v", imageName, err)
		return nil, nil, err
	}
	img, err := is.Transport.GetStoreImage(store, ref)
	if err != nil {
		logrus.Debugf("error reading pulled image %q: %v", imageName, err)
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
	s := strings.Split(imageName, "/")
	if len(s) > 0 {
		prefix = s[len(s)-1]
	}
	s = strings.Split(prefix, ":")
	if len(s) > 0 {
		prefix = s[0]
	}
	s = strings.Split(prefix, "@")
	if len(s) > 0 {
		prefix = s[0]
	}
	return prefix
}

func newContainerIDMappingOptions(idmapOptions *IDMappingOptions) storage.IDMappingOptions {
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

func resolveImage(ctx context.Context, systemContext *types.SystemContext, store storage.Store, options BuilderOptions) (types.ImageReference, *storage.Image, error) {
	type failure struct {
		resolvedImageName string
		err               error
	}

	candidates, searchRegistriesWereUsedButEmpty, err := util.ResolveName(options.FromImage, options.Registry, systemContext, store)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "error parsing reference to image %q", options.FromImage)
	}
	failures := []failure{}
	for _, image := range candidates {
		var err error
		if len(image) >= minimumTruncatedIDLength {
			if img, err := store.Image(image); err == nil && img != nil && strings.HasPrefix(img.ID, image) {
				ref, err := is.Transport.ParseStoreReference(store, img.ID)
				if err != nil {
					return nil, nil, errors.Wrapf(err, "error parsing reference to image %q", img.ID)
				}
				return ref, img, nil
			}
		}

		if options.PullPolicy == PullAlways {
			pulledImg, pulledReference, err := pullAndFindImage(ctx, store, image, options, systemContext)
			if err != nil {
				logrus.Debugf("unable to pull and read image %q: %v", image, err)
				failures = append(failures, failure{resolvedImageName: image, err: err})
				continue
			}
			return pulledReference, pulledImg, nil
		}

		srcRef, err := alltransports.ParseImageName(image)
		if err != nil {
			if options.Transport == "" {
				logrus.Debugf("error parsing image name %q: %v", image, err)
				failures = append(failures, failure{
					resolvedImageName: image,
					err:               errors.Wrapf(err, "error parsing image name"),
				})
				continue
			}
			logrus.Debugf("error parsing image name %q as given, trying with transport %q: %v", image, options.Transport, err)
			transport := options.Transport
			if transport != DefaultTransport {
				transport = transport + ":"
			}
			srcRef2, err := alltransports.ParseImageName(transport + image)
			if err != nil {
				logrus.Debugf("error parsing image name %q: %v", transport+image, err)
				failures = append(failures, failure{
					resolvedImageName: image,
					err:               errors.Wrapf(err, "error parsing attempted image name %q", transport+image),
				})
				continue
			}
			srcRef = srcRef2
		}

		destImage, err := localImageNameForReference(ctx, store, srcRef, options.FromImage)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "error computing local image name for %q", transports.ImageName(srcRef))
		}
		if destImage == "" {
			return nil, nil, errors.Errorf("error computing local image name for %q", transports.ImageName(srcRef))
		}

		ref, err := is.Transport.ParseStoreReference(store, destImage)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "error parsing reference to image %q", destImage)
		}
		img, err := is.Transport.GetStoreImage(store, ref)
		if err == nil {
			return ref, img, nil
		}

		if errors.Cause(err) == storage.ErrImageUnknown && options.PullPolicy != PullIfMissing {
			logrus.Debugf("no such image %q: %v", transports.ImageName(ref), err)
			failures = append(failures, failure{
				resolvedImageName: image,
				err:               fmt.Errorf("no such image %q", transports.ImageName(ref)),
			})
			continue
		}

		pulledImg, pulledReference, err := pullAndFindImage(ctx, store, image, options, systemContext)
		if err != nil {
			logrus.Debugf("unable to pull and read image %q: %v", image, err)
			failures = append(failures, failure{resolvedImageName: image, err: err})
			continue
		}
		return pulledReference, pulledImg, nil
	}

	if len(failures) != len(candidates) {
		return nil, nil, fmt.Errorf("internal error: %d candidates (%#v) vs. %d failures (%#v)", len(candidates), candidates, len(failures), failures)
	}

	registriesConfPath := sysregistries.RegistriesConfPath(systemContext)
	switch len(failures) {
	case 0:
		if searchRegistriesWereUsedButEmpty {
			return nil, nil, errors.Errorf("image name %q is a short name and no search registries are defined in %s.", options.FromImage, registriesConfPath)
		}
		return nil, nil, fmt.Errorf("internal error: no pull candidates were available for %q for an unknown reason", options.FromImage)

	case 1:
		err := failures[0].err
		if failures[0].resolvedImageName != options.FromImage {
			err = errors.Wrapf(err, "while pulling %q as %q", options.FromImage, failures[0].resolvedImageName)
		}
		if searchRegistriesWereUsedButEmpty {
			err = errors.Wrapf(err, "(image name %q is a short name and no search registries are defined in %s)", options.FromImage, registriesConfPath)
		}
		return nil, nil, err

	default:
		// NOTE: a multi-line error string:
		e := fmt.Sprintf("The following failures happened while trying to pull image specified by %q based on search registries in %s:", options.FromImage, registriesConfPath)
		for _, f := range failures {
			e = e + fmt.Sprintf("\n* %q: %s", f.resolvedImageName, f.err.Error())
		}
		return nil, nil, errors.New(e)
	}
}

func newBuilder(ctx context.Context, store storage.Store, options BuilderOptions) (*Builder, error) {
	var ref types.ImageReference
	var img *storage.Image
	var err error

	if options.FromImage == BaseImageFakeName {
		options.FromImage = ""
	}
	if options.Transport == "" {
		options.Transport = DefaultTransport
	}

	systemContext := getSystemContext(options.SystemContext, options.SignaturePolicyPath)

	if options.FromImage != "" && options.FromImage != "scratch" {
		ref, img, err = resolveImage(ctx, systemContext, store, options)
		if err != nil {
			return nil, err
		}
	}
	image := options.FromImage
	imageID := ""
	topLayer := ""
	if img != nil {
		image = getImageName(imageNamePrefix(image), img)
		imageID = img.ID
		topLayer = img.TopLayer
	}
	var src types.ImageCloser
	if ref != nil {
		src, err = ref.NewImage(ctx, systemContext)
		if err != nil {
			return nil, errors.Wrapf(err, "error instantiating image for %q", transports.ImageName(ref))
		}
		defer src.Close()
	}

	name := "working-container"
	if options.Container != "" {
		name = options.Container
	} else {
		if image != "" {
			name = imageNamePrefix(image) + "-" + name
		}
	}

	coptions := storage.ContainerOptions{}
	coptions.IDMappingOptions = newContainerIDMappingOptions(options.IDMappingOptions)

	container, err := store.CreateContainer("", []string{name}, imageID, "", "", &coptions)
	suffix := 1
	for err != nil && errors.Cause(err) == storage.ErrDuplicateName && options.Container == "" {
		suffix++
		tmpName := fmt.Sprintf("%s-%d", name, suffix)
		if container, err = store.CreateContainer("", []string{tmpName}, imageID, "", "", &coptions); err == nil {
			name = tmpName
		}
	}
	if err != nil {
		return nil, errors.Wrapf(err, "error creating container")
	}

	defer func() {
		if err != nil {
			if err2 := store.DeleteContainer(container.ID); err2 != nil {
				logrus.Errorf("error deleting container %q: %v", container.ID, err2)
			}
		}
	}()

	if err = ReserveSELinuxLabels(store, container.ID); err != nil {
		return nil, err
	}
	processLabel, mountLabel, err := label.InitLabels(options.CommonBuildOpts.LabelOpts)
	if err != nil {
		return nil, err
	}
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
		FromImage:             image,
		FromImageID:           imageID,
		Container:             name,
		ContainerID:           container.ID,
		ImageAnnotations:      map[string]string{},
		ImageCreatedBy:        "",
		ProcessLabel:          processLabel,
		MountLabel:            mountLabel,
		DefaultMountsFilePath: options.DefaultMountsFilePath,
		Isolation:             options.Isolation,
		NamespaceOptions:      namespaceOptions,
		ConfigureNetwork:      options.ConfigureNetwork,
		CNIPluginPath:         options.CNIPluginPath,
		CNIConfigDir:          options.CNIConfigDir,
		IDMappingOptions: IDMappingOptions{
			HostUIDMapping: len(uidmap) == 0,
			HostGIDMapping: len(uidmap) == 0,
			UIDMap:         uidmap,
			GIDMap:         gidmap,
		},
		AddCapabilities:  copyStringSlice(options.AddCapabilities),
		DropCapabilities: copyStringSlice(options.DropCapabilities),
		CommonBuildOpts:  options.CommonBuildOpts,
		TopLayer:         topLayer,
		Args:             options.Args,
		Format:           options.Format,
	}

	if options.Mount {
		_, err = builder.Mount(mountLabel)
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
