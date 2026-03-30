//go:build !remote

package compat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/containers/podman/v6/libpod"
	"github.com/containers/podman/v6/pkg/api/handlers/utils"
	api "github.com/containers/podman/v6/pkg/api/types"
	"github.com/containers/podman/v6/pkg/auth"
	"github.com/docker/distribution/registry/api/errcode"
	dockerRegistry "github.com/moby/moby/api/types/registry"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"go.podman.io/image/v5/docker/reference"
	"go.podman.io/image/v5/image"
	"go.podman.io/image/v5/manifest"
	"go.podman.io/image/v5/pkg/blobinfocache/none"
	"go.podman.io/image/v5/pkg/shortnames"
	"go.podman.io/image/v5/transports/alltransports"
	"go.podman.io/image/v5/types"
)

func DistributionInspect(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	imageName := utils.GetName(r)
	if _, err := reference.ParseNormalizedNamed(imageName); err != nil {
		utils.Error(w, http.StatusBadRequest, err)
		return
	}

	normalizedImageName, err := utils.NormalizeToDockerHub(r, imageName)
	if err != nil {
		utils.InternalServerError(w, fmt.Errorf("normalizing image: %w", err))
		return
	}
	if _, err := reference.ParseNormalizedNamed(normalizedImageName); err != nil {
		utils.Error(w, http.StatusBadRequest, err)
		return
	}

	authConf, authfile, err := auth.GetCredentials(r)
	if err != nil {
		utils.Error(w, http.StatusBadRequest, err)
		return
	}
	defer auth.RemoveAuthfile(authfile)

	sys := runtime.SystemContext()
	sys.AuthFilePath = authfile
	sys.DockerAuthConfig = authConf
	if err := utils.PossiblyEnforceDockerHub(r, sys); err != nil {
		utils.InternalServerError(w, err)
		return
	}

	resolved, err := shortnames.Resolve(sys, normalizedImageName)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}

	var latestErr error
	appendErr := func(e error) {
		if latestErr == nil {
			latestErr = e
			return
		}
		latestErr = fmt.Errorf("tried %v: %w", e, latestErr)
	}

	for _, candidate := range resolved.PullCandidates {
		ref, err := alltransports.ParseImageName("docker://" + candidate.Value.String())
		if err != nil {
			appendErr(err)
			continue
		}

		src, err := ref.NewImageSource(r.Context(), sys)
		if err != nil {
			appendErr(err)
			continue
		}

		manifestBytes, manifestType, err := image.UnparsedInstance(src, nil).Manifest(r.Context())
		if err != nil {
			_ = src.Close()
			appendErr(err)
			continue
		}

		inspect, err := distributionInspectFromManifest(r.Context(), src, candidate.Value, manifestBytes, manifestType)
		_ = src.Close()
		if err != nil {
			appendErr(err)
			continue
		}

		utils.WriteResponse(w, http.StatusOK, inspect)
		return
	}

	if latestErr == nil {
		utils.InternalServerError(w, errors.New("failed to inspect distribution"))
		return
	}

	var registryErrCode errcode.ErrorCoder
	if errors.As(latestErr, &registryErrCode) {
		switch registryErrCode.ErrorCode().Descriptor().HTTPStatusCode {
		case http.StatusUnauthorized, http.StatusNotFound:
			utils.Error(w, http.StatusUnauthorized, latestErr)
		default:
			utils.InternalServerError(w, latestErr)
		}
		return
	}
	utils.InternalServerError(w, latestErr)
}

func distributionInspectFromManifest(ctx context.Context, src types.ImageSource, named reference.Named, manifestBytes []byte, manifestType string) (dockerRegistry.DistributionInspect, error) {
	inspect := dockerRegistry.DistributionInspect{
		Descriptor: ocispec.Descriptor{
			MediaType: manifestType,
			Size:      int64(len(manifestBytes)),
		},
	}

	if canonical, ok := named.(reference.Canonical); ok {
		inspect.Descriptor.Digest = canonical.Digest()
	} else {
		digest, err := manifest.Digest(manifestBytes)
		if err != nil {
			return dockerRegistry.DistributionInspect{}, err
		}
		inspect.Descriptor.Digest = digest
	}

	switch manifest.NormalizedMIMEType(manifestType) {
	case manifest.DockerV2ListMediaType:
		list, err := manifest.Schema2ListFromManifest(manifestBytes)
		if err != nil {
			return dockerRegistry.DistributionInspect{}, err
		}
		for _, m := range list.Manifests {
			inspect.Platforms = append(inspect.Platforms, ocispec.Platform{
				Architecture: m.Platform.Architecture,
				OS:           m.Platform.OS,
				OSVersion:    m.Platform.OSVersion,
				OSFeatures:   m.Platform.OSFeatures,
				Variant:      m.Platform.Variant,
			})
		}
	case ocispec.MediaTypeImageIndex:
		index, err := manifest.OCI1IndexFromManifest(manifestBytes)
		if err != nil {
			return dockerRegistry.DistributionInspect{}, err
		}
		for _, m := range index.Manifests {
			if m.Platform != nil {
				inspect.Platforms = append(inspect.Platforms, *m.Platform)
			}
		}
	case manifest.DockerV2Schema2MediaType:
		schema2Manifest, err := manifest.Schema2FromManifest(manifestBytes)
		if err != nil {
			return dockerRegistry.DistributionInspect{}, err
		}
		maybeAddPlatformFromConfig(ctx, src, manifest.BlobInfoFromSchema2Descriptor(schema2Manifest.ConfigDescriptor), &inspect.Platforms)
	case ocispec.MediaTypeImageManifest:
		ociManifest, err := manifest.OCI1FromManifest(manifestBytes)
		if err != nil {
			return dockerRegistry.DistributionInspect{}, err
		}
		maybeAddPlatformFromConfig(ctx, src, manifest.BlobInfoFromOCI1Descriptor(ociManifest.Config), &inspect.Platforms)
	}

	return inspect, nil
}

func maybeAddPlatformFromConfig(ctx context.Context, src types.ImageSource, blobInfo types.BlobInfo, platforms *[]ocispec.Platform) {
	reader, _, err := src.GetBlob(ctx, blobInfo, none.NoCache)
	if err != nil {
		return
	}
	defer reader.Close()

	configJSON, err := io.ReadAll(reader)
	if err != nil {
		return
	}

	var platform ocispec.Platform
	if err := json.Unmarshal(configJSON, &platform); err != nil {
		return
	}
	if platform.OS != "" || platform.Architecture != "" {
		*platforms = append(*platforms, platform)
	}
}
