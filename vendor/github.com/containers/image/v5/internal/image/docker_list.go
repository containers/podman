package image

import (
	"context"
	"fmt"

	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/types"
)

func manifestSchema2FromManifestList(ctx context.Context, sys *types.SystemContext, src types.ImageSource, manblob []byte) (genericManifest, error) {
	list, err := manifest.Schema2ListFromManifest(manblob)
	if err != nil {
		return nil, fmt.Errorf("parsing schema2 manifest list: %w", err)
	}
	targetManifestDigest, err := list.ChooseInstance(sys)
	if err != nil {
		return nil, fmt.Errorf("choosing image instance: %w", err)
	}
	manblob, mt, err := src.GetManifest(ctx, &targetManifestDigest)
	if err != nil {
		return nil, fmt.Errorf("fetching target platform image selected from manifest list: %w", err)
	}

	matches, err := manifest.MatchesDigest(manblob, targetManifestDigest)
	if err != nil {
		return nil, fmt.Errorf("computing manifest digest: %w", err)
	}
	if !matches {
		return nil, fmt.Errorf("Image manifest does not match selected manifest digest %s", targetManifestDigest)
	}

	return manifestInstanceFromBlob(ctx, sys, src, manblob, mt)
}
