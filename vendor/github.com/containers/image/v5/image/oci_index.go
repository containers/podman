package image

import (
	"context"

	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/types"
	"github.com/pkg/errors"
)

func manifestOCI1FromImageIndex(ctx context.Context, sys *types.SystemContext, src types.ImageSource, manblob []byte) (genericManifest, error) {
	index, err := manifest.OCI1IndexFromManifest(manblob)
	if err != nil {
		return nil, errors.Wrapf(err, "parsing OCI1 index")
	}
	targetManifestDigest, err := index.ChooseInstance(sys)
	if err != nil {
		return nil, errors.Wrapf(err, "choosing image instance")
	}
	manblob, mt, err := src.GetManifest(ctx, &targetManifestDigest)
	if err != nil {
		return nil, errors.Wrapf(err, "loading manifest for target platform")
	}

	matches, err := manifest.MatchesDigest(manblob, targetManifestDigest)
	if err != nil {
		return nil, errors.Wrap(err, "computing manifest digest")
	}
	if !matches {
		return nil, errors.Errorf("Image manifest does not match selected manifest digest %s", targetManifestDigest)
	}

	return manifestInstanceFromBlob(ctx, sys, src, manblob, mt)
}
