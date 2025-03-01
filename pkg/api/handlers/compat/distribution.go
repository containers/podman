//go:build !remote

package compat

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/podman/v5/pkg/api/handlers/utils"
	registrytypes "github.com/docker/docker/api/types/registry"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func InspectDistribution(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	name := utils.GetName(r)

	namedRef, err := reference.ParseNormalizedNamed(name)
	if err != nil {
		utils.Error(w, http.StatusUnauthorized, fmt.Errorf("not a valid image reference: %q", name))
		return
	}

	namedRef = reference.TagNameOnly(namedRef)

	imgRef, err := docker.NewReference(namedRef)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("error creating image reference: %w", err))
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
		utils.Error(w, http.StatusUnauthorized, fmt.Errorf("error getting image source: %s", msg))
		return
	}
	defer imgSrc.Close()

	manBlob, manType, err := imgSrc.GetManifest(r.Context(), nil)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("error getting manifest: %w", err))
		return
	}

	digest, err := manifest.Digest(manBlob)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("error getting manifest digest: %w", err))
		return
	}

	var (
		annotations  map[string]string
		artifactType string
	)

	// todo: handle other manifest types
	ociIndex, err := manifest.OCI1IndexFromManifest(manBlob)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("error getting OCI index from manifest: %w", err))
		return
	}

	annotations = ociIndex.Annotations
	artifactType = ociIndex.ArtifactType
	platforms := make([]ocispec.Platform, len(ociIndex.Manifests))
	for i, m := range ociIndex.Manifests {
		platforms[i] = *m.Platform
	}

	distributionInspect := registrytypes.DistributionInspect{
		Descriptor: ocispec.Descriptor{
			Digest:       digest,
			Size:         int64(len(manBlob)),
			MediaType:    manType,
			Annotations:  annotations,
			ArtifactType: artifactType,
		},
		Platforms: platforms,
	}

	utils.WriteResponse(w, http.StatusOK, distributionInspect)
}
