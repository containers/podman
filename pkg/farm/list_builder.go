package farm

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/hashicorp/go-multierror"
	"github.com/sirupsen/logrus"
	"go.podman.io/image/v5/docker"
	"go.podman.io/image/v5/docker/reference"
	"go.podman.io/image/v5/types"
)

type listBuilderOptions struct {
	cleanup       bool
	iidFile       string
	authfile      string
	skipTLSVerify *bool
}

// Bug #25039: listLocal now holds the reference of the manifest to be built
// as a referenceNamed, rather than as a plain string. It is now made available in two
// string formats via two member functions (listName() and listReference())
//
//	reference  := name [ ":" tag ] [ "@" digest ]
//	name       := [domain '/'] path-component ['/' path-component]*
//
// Prior to this fix, the program was incorrectly using the fuller `reference` format
// in places that actually required the shorter `name` format, and failing as a consequence.
// The program only worked when the reference was supplied untagged (i.e some.domain/name).
// In such situations, `name` and `reference` formats would be identical.
//
// In the body of the code, listName has been refactored to listReference() where that is
// appropriate form to use in that context.
type listLocal struct {
	listRef     reference.Named
	localEngine entities.ImageEngine
	options     listBuilderOptions
}

func (l *listLocal) listName() string {
	// i.e. domain/path-component
	return l.listRef.Name()
}

func (l *listLocal) listReference() string {
	// i.e. domain/path-component:tag
	return l.listRef.String()
}

// Bug #25039: manifestListBuilder amended to address problems arising from the
// provision of an incorrectly specified reference. If the provided reference
// will not parse correctly, the function now throws an error.
func newManifestListBuilder(listName string, localEngine entities.ImageEngine, options listBuilderOptions) (*listLocal, error) {
	ref, err := reference.ParseNamed(listName)

	if err != nil {
		return nil, fmt.Errorf("could not parse reference %q: %w", listName, err)
	}
	return &listLocal{
		listRef:     ref,
		options:     options,
		localEngine: localEngine,
	}, err
}

// Build retrieves images from the build reports and assembles them into a
// manifest list in local container storage.
func (l *listLocal) build(ctx context.Context, images map[entities.BuildReport]entities.ImageEngine) (string, error) {
	// Set skipTLSVerify based on whether it was changed by the caller
	skipTLSVerify := types.OptionalBoolUndefined
	if l.options.skipTLSVerify != nil {
		skipTLSVerify = types.NewOptionalBool(*l.options.skipTLSVerify)
	}

	exists, err := l.localEngine.ManifestExists(ctx, l.listReference())
	if err != nil {
		return "", err
	}
	// Create list if it doesn't exist
	//
	// Bug #25039: we can safely use the longer form, (i.e. potentially tagged), listReference() here. (previously listName)
	if !exists.Value {
		_, err = l.localEngine.ManifestCreate(ctx, l.listReference(), []string{}, entities.ManifestCreateOptions{SkipTLSVerify: skipTLSVerify})
		if err != nil {
			return "", fmt.Errorf("creating manifest list %q: %w", l.listReference(), err)
		}
	}

	// Push the images to the registry given by the user
	var (
		pushGroup multierror.Group
		refsMutex sync.Mutex
	)
	refs := []string{}
	for image, engine := range images {
		pushGroup.Go(func() error {
			logrus.Infof("pushing image %s", image.ID)
			defer logrus.Infof("pushed image %s", image.ID)

			// Bug #25039: prior to fix, would crash here owing to the UnknownDigestSuffix being inappropriately applied to
			// the longer `reference` format . We can only use the shorter `name` format here.
			report, err := engine.Push(ctx, image.ID, l.listName()+docker.UnknownDigestSuffix, entities.ImagePushOptions{Authfile: l.options.authfile, Quiet: false, SkipTLSVerify: skipTLSVerify})
			if err != nil {
				return fmt.Errorf("pushing image %q to registry: %w", image, err)
			}
			refsMutex.Lock()
			defer refsMutex.Unlock()

			refs = append(refs, "docker://"+l.listName()+"@"+report.ManifestDigest)
			return nil
		})
	}
	pushErrors := pushGroup.Wait()
	err = pushErrors.ErrorOrNil()
	if err != nil {
		return "", fmt.Errorf("building: %w", err)
	}

	if l.options.cleanup {
		var rmGroup multierror.Group
		for image, engine := range images {
			if engine.FarmNodeName(ctx) == entities.LocalFarmImageBuilderName {
				continue
			}
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
		_, err = l.localEngine.ManifestListClear(ctx, l.listReference())
		if err != nil {
			return "", fmt.Errorf("error clearing list %q", l.listReference())
		}
	}

	// Add the images to the list
	listID, err := l.localEngine.ManifestAdd(ctx, l.listReference(), refs, entities.ManifestAddOptions{Authfile: l.options.authfile, SkipTLSVerify: skipTLSVerify})
	if err != nil {
		return "", fmt.Errorf("adding images %q to list: %w", refs, err)
	}
	_, err = l.localEngine.ManifestPush(ctx, l.listReference(), l.listReference(), entities.ImagePushOptions{Authfile: l.options.authfile, SkipTLSVerify: skipTLSVerify})
	if err != nil {
		return "", err
	}

	// Write the manifest list's ID file if we're expected to
	if l.options.iidFile != "" {
		if err := os.WriteFile(l.options.iidFile, []byte("sha256:"+listID), 0644); err != nil {
			return "", err
		}
	}

	return l.listReference(), nil
}
