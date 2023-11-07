package farm

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	lmanifests "github.com/containers/common/libimage/manifests"
	"github.com/containers/common/pkg/supplemented"
	cp "github.com/containers/image/v5/copy"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/hashicorp/go-multierror"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
)

type listBuilder interface {
	build(ctx context.Context, images map[entities.BuildReport]entities.ImageEngine) (string, error)
}

type listBuilderOptions struct {
	cleanup bool
	iidFile string
}

type listLocal struct {
	listName    string
	localEngine entities.ImageEngine
	options     listBuilderOptions
}

// newLocalManifestListBuilder returns a manifest list builder which saves a
// manifest list and images to local storage.
func newLocalManifestListBuilder(listName string, localEngine entities.ImageEngine, options listBuilderOptions) listBuilder {
	return &listLocal{
		listName:    listName,
		options:     options,
		localEngine: localEngine,
	}
}

// Build retrieves images from the build reports and assembles them into a
// manifest list in local container storage.
func (l *listLocal) build(ctx context.Context, images map[entities.BuildReport]entities.ImageEngine) (string, error) {
	namedManifest := l.listName
	exists, err := l.localEngine.ManifestExists(ctx, l.listName)
	if err != nil {
		return "", err
	}
	// Create list if it doesn't exist
	if !exists.Value {
		namedManifest, err = l.localEngine.ManifestCreate(ctx, l.listName, []string{}, entities.ManifestCreateOptions{})
		if err != nil {
			return "", fmt.Errorf("creating namedManifest list %q: %w", l.listName, err)
		}
	}

	// Pull the images into local storage
	var (
		pullGroup multierror.Group
		refsMutex sync.Mutex
	)
	refs := []string{}
	for image, engine := range images {
		image, engine := image, engine
		pullOptions := entities.PullToLocalOptions{
			ImageID:     image.ID,
			SaveFormat:  image.SaveFormat,
			Destination: l.localEngine,
		}
		pullGroup.Go(func() error {
			logrus.Infof("copying image %s", image.ID)
			defer logrus.Infof("copied image %s", image.ID)
			ref, err := engine.PullToLocal(ctx, pullOptions)
			if err != nil {
				return fmt.Errorf("pulling image %q to local storage: %w", image, err)
			}
			refsMutex.Lock()
			defer refsMutex.Unlock()
			refs = append(refs, ref)
			return nil
		})
	}
	pullErrors := pullGroup.Wait()
	err = pullErrors.ErrorOrNil()
	if err != nil {
		return "", fmt.Errorf("building: %w", err)
	}

	if l.options.cleanup {
		var rmGroup multierror.Group
		for image, engine := range images {
			if engine.FarmNodeName(ctx) == entities.LocalFarmImageBuilderName {
				continue
			}
			image, engine := image, engine
			rmGroup.Go(func() error {
				_, err := engine.Remove(ctx, []string{image.ID}, entities.ImageRemoveOptions{})
				if len(err) > 0 {
					return err[0]
				}
				return nil
			})
		}
		rmErrors := rmGroup.Wait()
		if rmErrors != nil {
			if err = rmErrors.ErrorOrNil(); err != nil {
				return "", fmt.Errorf("removing intermediate images: %w", err)
			}
		}
	}

	// Clear the list in the event it already existed
	if exists.Value {
		_, err = l.localEngine.ManifestListClear(ctx, namedManifest)
		if err != nil {
			return "", fmt.Errorf("error clearing list %q", namedManifest)
		}
	}

	// Add the images to the list
	listID, err := l.localEngine.ManifestAdd(ctx, namedManifest, refs, entities.ManifestAddOptions{})
	if err != nil {
		return "", fmt.Errorf("adding images %q to list: %w", refs, err)
	}

	// Write the namedManifest list's ID file if we're expected to
	if l.options.iidFile != "" {
		if err := os.WriteFile(l.options.iidFile, []byte("sha256:"+listID), 0644); err != nil {
			return "", err
		}
	}

	return l.listName, nil
}

type listFiles struct {
	directory string
	options   listBuilderOptions
}

// newFileManifestListBuilder returns a manifest list builder which saves a manifest
// list and images to a specified directory in the non-standard dir: format.
func newFileManifestListBuilder(directory string, options listBuilderOptions) (listBuilder, error) {
	if options.iidFile != "" {
		return nil, fmt.Errorf("saving to dir: format doesn't use image IDs, --iidfile not supported")
	}
	return &listFiles{directory: directory, options: options}, nil
}

// Build retrieves images from the build reports and assembles them into a
// manifest list in the configured directory.
func (m *listFiles) build(ctx context.Context, images map[entities.BuildReport]entities.ImageEngine) (string, error) {
	listFormat := v1.MediaTypeImageIndex
	imageFormat := v1.MediaTypeImageManifest

	tempDir, err := os.MkdirTemp("", "")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tempDir)

	name := fmt.Sprintf("dir:%s", tempDir)
	tempRef, err := alltransports.ParseImageName(name)
	if err != nil {
		return "", fmt.Errorf("parsing temporary image ref %q: %w", name, err)
	}
	if err := os.MkdirAll(m.directory, 0o755); err != nil {
		return "", err
	}
	output, err := alltransports.ParseImageName("dir:" + m.directory)
	if err != nil {
		return "", fmt.Errorf("parsing output directory ref %q: %w", "dir:"+m.directory, err)
	}

	// Pull the images into the temporary directory
	var (
		pullGroup  multierror.Group
		pullErrors *multierror.Error
		refsMutex  sync.Mutex
	)
	refs := make(map[entities.BuildReport]types.ImageReference)
	for image, engine := range images {
		image, engine := image, engine
		tempFile, err := os.CreateTemp(tempDir, "archive-*.tar")
		if err != nil {
			defer func() {
				pullErrors = pullGroup.Wait()
			}()
			perr := pullErrors.ErrorOrNil()
			if perr != nil {
				return "", perr
			}
			return "", err
		}
		defer tempFile.Close()

		pullGroup.Go(func() error {
			logrus.Infof("copying image %s", image.ID)
			defer logrus.Infof("copied image %s", image.ID)
			pullOptions := entities.PullToFileOptions{
				ImageID:    image.ID,
				SaveFormat: image.SaveFormat,
				SaveFile:   tempFile.Name(),
			}
			if image.SaveFormat == manifest.DockerV2Schema2MediaType {
				listFormat = manifest.DockerV2ListMediaType
				imageFormat = manifest.DockerV2Schema2MediaType
			}
			reference, err := engine.PullToFile(ctx, pullOptions)
			if err != nil {
				return fmt.Errorf("pulling image %q to temporary directory: %w", image, err)
			}
			ref, err := alltransports.ParseImageName(reference)
			if err != nil {
				return fmt.Errorf("pulling image %q to temporary directory: %w", image, err)
			}
			refsMutex.Lock()
			defer refsMutex.Unlock()
			refs[image] = ref
			return nil
		})
	}
	pullErrors = pullGroup.Wait()
	err = pullErrors.ErrorOrNil()
	if err != nil {
		return "", fmt.Errorf("building: %w", err)
	}

	if m.options.cleanup {
		var rmGroup multierror.Group
		for image, engine := range images {
			image, engine := image, engine
			rmGroup.Go(func() error {
				_, err := engine.Remove(ctx, []string{image.ID}, entities.ImageRemoveOptions{})
				if len(err) > 0 {
					return err[0]
				}
				return nil
			})
		}
		rmErrors := rmGroup.Wait()
		if rmErrors != nil {
			if err = rmErrors.ErrorOrNil(); err != nil {
				return "", fmt.Errorf("removing intermediate images: %w", err)
			}
		}
	}

	supplemental := []types.ImageReference{}
	var sys types.SystemContext
	// Create a manifest list
	list := lmanifests.Create()
	// Add the images to the list
	for image, ref := range refs {
		if _, err = list.Add(ctx, &sys, ref, true); err != nil {
			return "", fmt.Errorf("adding image %q to list: %w", image.ID, err)
		}
		supplemental = append(supplemental, ref)
	}
	// Save the list to the temporary directory to be the main manifest
	listBytes, err := list.Serialize(listFormat)
	if err != nil {
		return "", fmt.Errorf("serializing manifest list: %w", err)
	}
	if err = os.WriteFile(filepath.Join(tempDir, "manifest.json"), listBytes, fs.FileMode(0o600)); err != nil {
		return "", fmt.Errorf("writing temporary manifest list: %w", err)
	}

	// Now copy everything to the final dir: location
	defaultPolicy, err := signature.DefaultPolicy(&sys)
	if err != nil {
		return "", err
	}
	policyContext, err := signature.NewPolicyContext(defaultPolicy)
	if err != nil {
		return "", err
	}
	input := supplemented.Reference(tempRef, supplemental, cp.CopyAllImages, nil)
	copyOptions := cp.Options{
		ForceManifestMIMEType: imageFormat,
		ImageListSelection:    cp.CopyAllImages,
	}
	_, err = cp.Image(ctx, policyContext, output, input, &copyOptions)
	if err != nil {
		return "", fmt.Errorf("copying images to dir:%q: %w", m.directory, err)
	}

	return "dir:" + m.directory, nil
}
