//go:build !remote

package compat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/image"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/pkg/blobinfocache/none"
	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v5/pkg/api/handlers/utils"
	registrytypes "github.com/docker/docker/api/types/registry"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func InspectDistribution(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	name := utils.GetName(r)

	namedRef, imgRef, err := parseImageReference(name)
	if err != nil {
		utils.Error(w, http.StatusUnauthorized, err)
		return
	}

	imgSrc, err := imgRef.NewImageSource(r.Context(), nil)
	if err != nil {
		var msg string
		var unauthErr docker.ErrUnauthorizedForCredentials
		if errors.As(err, &unauthErr) {
			msg = "401 Unauthorized"
		} else {
			msg = err.Error()
		}
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("error getting manifest: %s", msg))
		return
	}
	defer imgSrc.Close()

	manBlob, manType, err := image.UnparsedInstance(imgSrc, nil).Manifest(r.Context())
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("error getting manifest: %w", err))
		return
	}

	digest, err := manifest.Digest(manBlob)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("error getting manifest digest: %w", err))
		return
	}

	if err := validateDigest(namedRef, digest); err != nil {
		utils.Error(w, http.StatusInternalServerError, err)
		return
	}

	distributionInspect := registrytypes.DistributionInspect{
		Descriptor: ocispec.Descriptor{
			Digest:    digest,
			Size:      int64(len(manBlob)),
			MediaType: manType,
		},
	}

	platforms, err := getPlatformsFromManifest(r.Context(), imgSrc, manBlob, manType)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, err)
		return
	}
	distributionInspect.Platforms = platforms

	populateDescriptorByManifestType(&distributionInspect.Descriptor, manBlob, manType)

	utils.WriteResponse(w, http.StatusOK, distributionInspect)
}

func parseImageReference(name string) (reference.Named, types.ImageReference, error) {
	namedRef, err := reference.ParseNormalizedNamed(name)
	if err != nil {
		return nil, nil, fmt.Errorf("not a valid image reference: %q", name)
	}

	namedRef = reference.TagNameOnly(namedRef)

	imgRef, err := docker.NewReference(namedRef)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating image reference: %w", err)
	}

	return namedRef, imgRef, nil
}

func validateDigest(namedRef reference.Named, calculatedDigest digest.Digest) error {
	if digested, ok := namedRef.(reference.Digested); ok {
		expectedDigest := digested.Digest()
		if calculatedDigest != expectedDigest {
			return fmt.Errorf("manifest digest %s does not match reference digest %s", calculatedDigest, expectedDigest)
		}
	}
	return nil
}

func getPlatformsFromManifest(ctx context.Context, imgSrc types.ImageSource, manBlob []byte, manType string) ([]ocispec.Platform, error) {
	if manType == "" {
		manType = manifest.GuessMIMEType(manBlob)
	}

	if manifest.MIMETypeIsMultiImage(manType) {
		manifestList, err := manifest.ListFromBlob(manBlob, manType)
		if err != nil {
			return nil, fmt.Errorf("error parsing manifest list: %w", err)
		}

		instanceDigests := manifestList.Instances()
		platforms := make([]ocispec.Platform, 0, len(instanceDigests))
		for _, digest := range instanceDigests {
			instance, err := manifestList.Instance(digest)
			if err != nil {
				return nil, fmt.Errorf("error getting manifest list instance: %w", err)
			}
			platforms = append(platforms, *instance.ReadOnly.Platform)
		}
		return platforms, nil

	} else {
		// handle non-multi-image manifests
		mnfst, err := manifest.FromBlob(manBlob, manType)
		if err != nil {
			return nil, fmt.Errorf("error parsing manifest: %w", err)
		}
		stream, _, err := imgSrc.GetBlob(ctx, types.BlobInfo{Digest: mnfst.ConfigInfo().Digest}, none.NoCache) // todo: use cache?
		if err != nil {
			return nil, fmt.Errorf("error getting manifest blob with digest %s: %w", mnfst.ConfigInfo().Digest, err)
		}
		defer stream.Close()

		configBlob, err := io.ReadAll(stream) // todo: read at most
		if err != nil {
			return nil, fmt.Errorf("error reading config blob: %w", err)
		}

		var config ocispec.Image
		if err := json.Unmarshal(configBlob, &config); err != nil {
			return nil, fmt.Errorf("error parsing config blob: %w", err)
		}

		return []ocispec.Platform{{
			Architecture: config.Architecture,
			OS:           config.OS,
			OSVersion:    config.OSVersion,
			OSFeatures:   config.OSFeatures,
			Variant:      config.Variant,
		}}, nil
	}
}

func populateDescriptorByManifestType(descriptor *ocispec.Descriptor, manBlob []byte, manType string) {
	switch manType {
	case ocispec.MediaTypeImageIndex:
		if ociIndex, err := manifest.OCI1IndexFromManifest(manBlob); err == nil {
			descriptor.Annotations = ociIndex.Annotations
			descriptor.ArtifactType = ociIndex.ArtifactType
		}
	}
}
