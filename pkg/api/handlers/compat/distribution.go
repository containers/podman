//go:build !remote

package compat

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/image"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v5/libpod"
	"github.com/containers/podman/v5/pkg/api/handlers/utils"
	api "github.com/containers/podman/v5/pkg/api/types"
	registrytypes "github.com/docker/docker/api/types/registry"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func InspectDistribution(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	_, imgRef, err := parseImageReference(utils.GetName(r))
	if err != nil {
		utils.Error(w, http.StatusUnauthorized, err)
		return
	}

	imgSrc, err := imgRef.NewImageSource(r.Context(), nil)
	if err != nil {
		var unauthErr docker.ErrUnauthorizedForCredentials
		if errors.As(err, &unauthErr) {
			utils.Error(w, http.StatusUnauthorized, errors.New("401 Unauthorized"))
		} else {
			utils.Error(w, http.StatusUnauthorized, fmt.Errorf("image not found: %w", err))
		}
		return
	}
	defer imgSrc.Close()

	unparsedImage := image.UnparsedInstance(imgSrc, nil)
	manBlob, manType, err := unparsedImage.Manifest(r.Context())
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("error getting manifest: %w", err))
		return
	}
	img, err := image.FromUnparsedImage(r.Context(), runtime.SystemContext(), unparsedImage)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("error getting manifest: %w", err))
		return
	}

	digest, err := manifest.Digest(manBlob)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("error getting manifest digest: %w", err))
		return
	}

	distributionInspect := registrytypes.DistributionInspect{
		Descriptor: ocispec.Descriptor{
			Digest:    digest,
			Size:      int64(len(manBlob)),
			MediaType: manType,
		},
	}

	platforms, err := getPlatformsFromManifest(r.Context(), img, manBlob, manType)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, err)
		return
	}
	distributionInspect.Platforms = platforms

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

func getPlatformsFromManifest(ctx context.Context, img types.Image, manBlob []byte, manType string) ([]ocispec.Platform, error) {
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
			if instance.ReadOnly.Platform == nil {
				continue
			}
			platforms = append(platforms, *instance.ReadOnly.Platform)
		}
		return platforms, nil
	}

	switch manType {
	case ocispec.MediaTypeImageManifest, manifest.DockerV2Schema2MediaType, manifest.DockerV2Schema1MediaType, manifest.DockerV2Schema1SignedMediaType:
		config, err := img.OCIConfig(ctx)
		if err != nil {
			return nil, fmt.Errorf("error getting OCI config: %w", err)
		}
		return []ocispec.Platform{config.Platform}, nil
	}
	return []ocispec.Platform{}, nil
}
