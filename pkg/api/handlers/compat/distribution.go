//go:build !remote

package compat

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/containers/podman/v6/libpod"
	"github.com/containers/podman/v6/pkg/api/handlers/utils"
	api "github.com/containers/podman/v6/pkg/api/types"
	"github.com/containers/podman/v6/pkg/auth"
	"github.com/docker/distribution/registry/api/errcode"
	"github.com/hashicorp/go-multierror"
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

	var merr *multierror.Error

	for _, candidate := range resolved.PullCandidates {
		ref, err := alltransports.ParseImageName("docker://" + candidate.Value.String())
		if err != nil {
			merr = multierror.Append(merr, err)
			continue
		}

		src, err := ref.NewImageSource(r.Context(), sys)
		if err != nil {
			merr = multierror.Append(merr, err)
			continue
		}

		manifestBytes, manifestType, err := image.UnparsedInstance(src, nil).Manifest(r.Context())
		if err != nil {
			_ = src.Close()
			merr = multierror.Append(merr, err)
			continue
		}

		inspect, err := distributionInspectFromManifest(r.Context(), src, candidate.Value, manifestBytes, manifestType)
		_ = src.Close()
		if err != nil {
			merr = multierror.Append(merr, err)
			continue
		}

		utils.WriteResponse(w, http.StatusOK, inspect)
		return
	}

	combinedErr := merr.ErrorOrNil()
	if combinedErr == nil {
		utils.InternalServerError(w, errors.New("failed to inspect distribution"))
		return
	}

	var registryErrCode errcode.ErrorCoder
	if errors.As(combinedErr, &registryErrCode) {
		switch registryErrCode.ErrorCode().Descriptor().HTTPStatusCode {
		case http.StatusUnauthorized, http.StatusNotFound:
			utils.Error(w, http.StatusUnauthorized, combinedErr)
		default:
			utils.InternalServerError(w, combinedErr)
		}
		return
	}
	utils.InternalServerError(w, combinedErr)
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

	normalizedType := manifest.NormalizedMIMEType(manifestType)
	if manifest.MIMETypeIsMultiImage(normalizedType) {
		list, err := manifest.ListFromBlob(manifestBytes, manifestType)
		if err != nil {
			return dockerRegistry.DistributionInspect{}, err
		}
		for _, d := range list.Instances() {
			instance, err := list.Instance(d)
			if err != nil {
				return dockerRegistry.DistributionInspect{}, err
			}
			if instance.ReadOnly.Platform != nil {
				inspect.Platforms = append(inspect.Platforms, *instance.ReadOnly.Platform)
			}
		}
	} else {
		m, err := manifest.FromBlob(manifestBytes, manifestType)
		if err != nil {
			return dockerRegistry.DistributionInspect{}, err
		}
		info, err := m.Inspect(func(bi types.BlobInfo) ([]byte, error) {
			reader, _, err := src.GetBlob(ctx, bi, none.NoCache)
			if err != nil {
				return nil, err
			}
			defer reader.Close()
			return io.ReadAll(reader)
		})
		if err != nil {
			return dockerRegistry.DistributionInspect{}, err
		}
		if info.Os != "" || info.Architecture != "" {
			inspect.Platforms = append(inspect.Platforms, ocispec.Platform{
				Architecture: info.Architecture,
				OS:           info.Os,
				Variant:      info.Variant,
			})
		}
	}

	return inspect, nil
}
