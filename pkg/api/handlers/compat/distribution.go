package compat

import (
	"context"
	"encoding/json"
	"net/http"
	"os"

	"github.com/containers/podman/v5/libpod"
	api "github.com/containers/podman/v5/pkg/api/types"

	"github.com/containers/podman/v5/pkg/api/handlers/utils"
	"github.com/distribution/reference"
	"github.com/docker/distribution"
	"github.com/docker/distribution/manifest/manifestlist"
	"github.com/docker/distribution/manifest/schema1"
	"github.com/docker/distribution/manifest/schema2"
	registrytypes "github.com/docker/docker/api/types/registry"
	distributionpkg "github.com/docker/docker/distribution"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/registry"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

func InspectDistribution(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	name := utils.GetName(r)

	ref, err := reference.ParseAnyReference(name)
	if err != nil {
		utils.Error(w, http.StatusBadRequest, errors.Wrapf(err, "error parsing image reference %q", name))
		return
	}

	namedRef, ok := ref.(reference.Named)
	if !ok {
		if _, ok := ref.(reference.Digested); ok {
			// full image ID
			utils.Error(w, http.StatusNotFound, errors.New("no manifest found for full image ID"))
			return
		}
		utils.Error(w, http.StatusBadRequest, errors.Errorf("unknown image reference format: %s", name))
		return
	}

	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	registryService, err := getRegistryService(runtime)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, errors.Wrap(err, "error creating registry service"))
		return
	}

	authConfig, _ := registrytypes.DecodeAuthConfig(r.Header.Get(registrytypes.AuthHeader))
	repos, err := distributionpkg.GetRepositories(r.Context(), namedRef, &distributionpkg.ImagePullConfig{
		Config: distributionpkg.Config{
			AuthConfig:      authConfig,
			RegistryService: registryService,
		},
	})
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, errors.Wrap(err, "error getting repositories"))
		return
	}

	var lastErr error
	for _, repo := range repos {
		distributionInspect, err := fetchManifest(r.Context(), repo, namedRef)
		if err != nil {
			lastErr = err
			continue
		}
		utils.WriteResponse(w, http.StatusOK, distributionInspect)
		return
	}
	utils.Error(w, http.StatusInternalServerError, lastErr)
}

func getRegistryService(runtime *libpod.Runtime) (*registry.Service, error) {
	serviceConfig := getServiceConfig(runtime)

	var insecureRegs []string
	var mirrors []string
	for _, reg := range serviceConfig.IndexConfigs {
		if !reg.Secure {
			insecureRegs = append(insecureRegs, reg.Name)
		}
		mirrors = append(mirrors, reg.Mirrors...)
	}

	serviceOpts := registry.ServiceOptions{
		InsecureRegistries: insecureRegs,
		Mirrors:            mirrors,
	}

	registryService, err := registry.NewService(serviceOpts)
	return registryService, errors.Wrap(err, "error creating registry service")
}

// Code below is taken from https://github.com/moby/moby/blob/00ab386b5a2ebcf85b6a03b800f593c3a140c6a8/api/server/router/distribution/distribution_routes.go
// Copyright 2022 github.com/moby/moby authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
func fetchManifest(ctx context.Context, distrepo distribution.Repository, namedRef reference.Named) (registrytypes.DistributionInspect, error) {
	var distributionInspect registrytypes.DistributionInspect
	if canonicalRef, ok := namedRef.(reference.Canonical); !ok {
		namedRef = reference.TagNameOnly(namedRef)

		taggedRef, ok := namedRef.(reference.NamedTagged)
		if !ok {
			return registrytypes.DistributionInspect{}, errdefs.InvalidParameter(errors.Errorf("image reference not tagged: %s", namedRef))
		}

		descriptor, err := distrepo.Tags(ctx).Get(ctx, taggedRef.Tag())
		if err != nil {
			return registrytypes.DistributionInspect{}, err
		}
		distributionInspect.Descriptor = ocispec.Descriptor{
			MediaType: descriptor.MediaType,
			Digest:    descriptor.Digest,
			Size:      descriptor.Size,
		}
	} else {
		// TODO(nishanttotla): Once manifests can be looked up as a blob, the
		// descriptor should be set using blobsrvc.Stat(ctx, canonicalRef.Digest())
		// instead of having to manually fill in the fields
		distributionInspect.Descriptor.Digest = canonicalRef.Digest()
	}

	// we have a digest, so we can retrieve the manifest
	mnfstsrvc, err := distrepo.Manifests(ctx)
	if err != nil {
		return registrytypes.DistributionInspect{}, err
	}
	mnfst, err := mnfstsrvc.Get(ctx, distributionInspect.Descriptor.Digest)
	if err != nil {
		switch err {
		case reference.ErrReferenceInvalidFormat,
			reference.ErrTagInvalidFormat,
			reference.ErrDigestInvalidFormat,
			reference.ErrNameContainsUppercase,
			reference.ErrNameEmpty,
			reference.ErrNameTooLong,
			reference.ErrNameNotCanonical:
			return registrytypes.DistributionInspect{}, errdefs.InvalidParameter(err)
		}
		return registrytypes.DistributionInspect{}, err
	}

	mediaType, payload, err := mnfst.Payload()
	if err != nil {
		return registrytypes.DistributionInspect{}, err
	}
	// update MediaType because registry might return something incorrect
	distributionInspect.Descriptor.MediaType = mediaType
	if distributionInspect.Descriptor.Size == 0 {
		distributionInspect.Descriptor.Size = int64(len(payload))
	}

	// retrieve platform information depending on the type of manifest
	switch mnfstObj := mnfst.(type) {
	case *manifestlist.DeserializedManifestList:
		for _, m := range mnfstObj.Manifests {
			distributionInspect.Platforms = append(distributionInspect.Platforms, ocispec.Platform{
				Architecture: m.Platform.Architecture,
				OS:           m.Platform.OS,
				OSVersion:    m.Platform.OSVersion,
				OSFeatures:   m.Platform.OSFeatures,
				Variant:      m.Platform.Variant,
			})
		}
	case *schema2.DeserializedManifest:
		blobStore := distrepo.Blobs(ctx)
		configJSON, err := blobStore.Get(ctx, mnfstObj.Config.Digest)
		var platform ocispec.Platform
		if err == nil {
			err := json.Unmarshal(configJSON, &platform)
			if err == nil && (platform.OS != "" || platform.Architecture != "") {
				distributionInspect.Platforms = append(distributionInspect.Platforms, platform)
			}
		}
	case *schema1.SignedManifest:
		if os.Getenv("DOCKER_ENABLE_DEPRECATED_PULL_SCHEMA_1_IMAGE") == "" {
			return registrytypes.DistributionInspect{}, distributionpkg.DeprecatedSchema1ImageError(namedRef)
		}
		platform := ocispec.Platform{
			Architecture: mnfstObj.Architecture,
			OS:           "linux",
		}
		distributionInspect.Platforms = append(distributionInspect.Platforms, platform)
	}
	return distributionInspect, nil
}
