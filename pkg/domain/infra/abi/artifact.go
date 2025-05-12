//go:build !remote

package abi

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/containers/common/libimage"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/containers/podman/v5/pkg/libartifact/types"
	"github.com/opencontainers/go-digest"
)

func (ir *ImageEngine) ArtifactInspect(ctx context.Context, name string, _ entities.ArtifactInspectOptions) (*entities.ArtifactInspectReport, error) {
	artStore, err := ir.Libpod.ArtifactStore()
	if err != nil {
		return nil, err
	}
	art, err := artStore.Inspect(ctx, name)
	if err != nil {
		return nil, err
	}
	artDigest, err := art.GetDigest()
	if err != nil {
		return nil, err
	}
	artInspectReport := entities.ArtifactInspectReport{
		Artifact: art,
		Digest:   artDigest.String(),
	}
	return &artInspectReport, nil
}

func (ir *ImageEngine) ArtifactList(ctx context.Context, _ entities.ArtifactListOptions) ([]*entities.ArtifactListReport, error) {
	reports := make([]*entities.ArtifactListReport, 0)
	artStore, err := ir.Libpod.ArtifactStore()
	if err != nil {
		return nil, err
	}
	lrs, err := artStore.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, lr := range lrs {
		artListReport := entities.ArtifactListReport{
			Artifact: lr,
		}
		reports = append(reports, &artListReport)
	}
	return reports, nil
}

func (ir *ImageEngine) ArtifactPull(ctx context.Context, name string, opts entities.ArtifactPullOptions) (*entities.ArtifactPullReport, error) {
	pullOptions := &libimage.CopyOptions{}
	pullOptions.AuthFilePath = opts.AuthFilePath
	pullOptions.CertDirPath = opts.CertDirPath
	pullOptions.Username = opts.Username
	pullOptions.Password = opts.Password
	pullOptions.SignaturePolicyPath = opts.SignaturePolicyPath
	pullOptions.InsecureSkipTLSVerify = opts.InsecureSkipTLSVerify
	pullOptions.Writer = opts.Writer
	pullOptions.OciDecryptConfig = opts.OciDecryptConfig
	pullOptions.MaxRetries = opts.MaxRetries
	if opts.RetryDelay != "" {
		duration, err := time.ParseDuration(opts.RetryDelay)
		if err != nil {
			return nil, fmt.Errorf("unable to parse value provided %q: %w", opts.RetryDelay, err)
		}
		pullOptions.RetryDelay = &duration
	}

	if !opts.Quiet && pullOptions.Writer == nil {
		pullOptions.Writer = os.Stderr
	}
	artStore, err := ir.Libpod.ArtifactStore()
	if err != nil {
		return nil, err
	}
	artifactDigest, err := artStore.Pull(ctx, name, *pullOptions)
	if err != nil {
		return nil, err
	}

	return &entities.ArtifactPullReport{
		ArtifactDigest: &artifactDigest,
	}, nil
}

func (ir *ImageEngine) ArtifactRm(ctx context.Context, name string, opts entities.ArtifactRemoveOptions) (*entities.ArtifactRemoveReport, error) {
	var (
		namesOrDigests []string
	)
	artStore, err := ir.Libpod.ArtifactStore()
	if err != nil {
		return nil, err
	}

	if opts.All {
		allArtifacts, err := artStore.List(ctx)
		if err != nil {
			return nil, err
		}
		for _, art := range allArtifacts {
			// Using the digest here instead of name to protect against
			// an artifact that lacks a name
			manifestDigest, err := art.GetDigest()
			if err != nil {
				return nil, err
			}
			namesOrDigests = append(namesOrDigests, manifestDigest.Encoded())
		}
	}

	if name != "" {
		namesOrDigests = append(namesOrDigests, name)
	}

	artifactDigests := make([]*digest.Digest, 0, len(namesOrDigests))
	for _, namesOrDigest := range namesOrDigests {
		artifactDigest, err := artStore.Remove(ctx, namesOrDigest)
		if err != nil {
			return nil, err
		}
		artifactDigests = append(artifactDigests, artifactDigest)
	}
	artifactRemoveReport := entities.ArtifactRemoveReport{
		ArtifactDigests: artifactDigests,
	}
	return &artifactRemoveReport, err
}

func (ir *ImageEngine) ArtifactPush(ctx context.Context, name string, opts entities.ArtifactPushOptions) (*entities.ArtifactPushReport, error) {
	var retryDelay *time.Duration

	artStore, err := ir.Libpod.ArtifactStore()
	if err != nil {
		return nil, err
	}

	if opts.RetryDelay != "" {
		rd, err := time.ParseDuration(opts.RetryDelay)
		if err != nil {
			return nil, err
		}
		retryDelay = &rd
	}

	copyOpts := libimage.CopyOptions{
		SystemContext:                    nil,
		SourceLookupReferenceFunc:        nil,
		DestinationLookupReferenceFunc:   nil,
		CompressionFormat:                nil,
		CompressionLevel:                 nil,
		ForceCompressionFormat:           false,
		AuthFilePath:                     opts.Authfile,
		BlobInfoCacheDirPath:             "",
		CertDirPath:                      opts.CertDir,
		DirForceCompress:                 false,
		ImageListSelection:               0,
		InsecureSkipTLSVerify:            opts.SkipTLSVerify,
		MaxRetries:                       opts.Retry,
		RetryDelay:                       retryDelay,
		ManifestMIMEType:                 "",
		OciAcceptUncompressedLayers:      false,
		OciEncryptConfig:                 nil,
		OciEncryptLayers:                 opts.OciEncryptLayers,
		OciDecryptConfig:                 nil,
		Progress:                         nil,
		PolicyAllowStorage:               false,
		SignaturePolicyPath:              opts.SignaturePolicy,
		Signers:                          opts.Signers,
		SignBy:                           opts.SignBy,
		SignPassphrase:                   opts.SignPassphrase,
		SignBySigstorePrivateKeyFile:     opts.SignBySigstorePrivateKeyFile,
		SignSigstorePrivateKeyPassphrase: opts.SignSigstorePrivateKeyPassphrase,
		RemoveSignatures:                 opts.RemoveSignatures,
		Architecture:                     "",
		OS:                               "",
		Variant:                          "",
		Username:                         "",
		Password:                         "",
		Credentials:                      opts.CredentialsCLI,
		IdentityToken:                    "",
		Writer:                           opts.Writer,
	}
	artifactDigest, err := artStore.Push(ctx, name, name, copyOpts)
	if err != nil {
		return nil, err
	}

	return &entities.ArtifactPushReport{
		ArtifactDigest: &artifactDigest,
	}, nil
}

func (ir *ImageEngine) ArtifactAdd(ctx context.Context, name string, artifactBlobs []entities.ArtifactBlob, opts *entities.ArtifactAddOptions) (*entities.ArtifactAddReport, error) {
	artStore, err := ir.Libpod.ArtifactStore()
	if err != nil {
		return nil, err
	}

	if opts.Annotations == nil {
		opts.Annotations = make(map[string]string)
	}

	addOptions := types.AddOptions{
		Annotations:  opts.Annotations,
		ArtifactType: opts.ArtifactType,
		Append:       opts.Append,
		FileType:     opts.FileType,
	}

	artifactDigest, err := artStore.Add(ctx, name, artifactBlobs, &addOptions)
	if err != nil {
		return nil, err
	}
	return &entities.ArtifactAddReport{
		ArtifactDigest: artifactDigest,
	}, nil
}

func (ir *ImageEngine) ArtifactExtract(ctx context.Context, name string, target string, opts *entities.ArtifactExtractOptions) error {
	artStore, err := ir.Libpod.ArtifactStore()
	if err != nil {
		return err
	}
	extractOpt := &types.ExtractOptions{
		FilterBlobOptions: types.FilterBlobOptions{
			Digest: opts.Digest,
			Title:  opts.Title,
		},
	}

	return artStore.Extract(ctx, name, target, extractOpt)
}

func (ir *ImageEngine) ArtifactExtractTarStream(ctx context.Context, w io.Writer, name string, opts *entities.ArtifactExtractOptions) error {
	if opts == nil {
		opts = &entities.ArtifactExtractOptions{}
	}
	artStore, err := ir.Libpod.ArtifactStore()
	if err != nil {
		return err
	}
	extractOpt := &types.ExtractOptions{
		FilterBlobOptions: types.FilterBlobOptions{
			Digest: opts.Digest,
			Title:  opts.Title,
		},
	}

	return artStore.ExtractTarStream(ctx, w, name, extractOpt)
}
