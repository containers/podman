package tunnel

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v5/pkg/bindings/artifacts"
	"github.com/containers/podman/v5/pkg/domain/entities"
)

func (ir *ImageEngine) ArtifactExtract(_ context.Context, name string, target string, opts entities.ArtifactExtractOptions) error {
	options := artifacts.ExtractOptions{
		Digest: &opts.Digest,
		Title:  &opts.Title,
	}

	return artifacts.Extract(ir.ClientCtx, name, target, &options)
}

func (ir *ImageEngine) ArtifactExtractTarStream(_ context.Context, w io.Writer, name string, opts entities.ArtifactExtractOptions) error {
	return fmt.Errorf("not implemented")
}

func (ir *ImageEngine) ArtifactInspect(_ context.Context, name string, opts entities.ArtifactInspectOptions) (*entities.ArtifactInspectReport, error) {
	return artifacts.Inspect(ir.ClientCtx, name, &artifacts.InspectOptions{})
}

func (ir *ImageEngine) ArtifactList(_ context.Context, opts entities.ArtifactListOptions) ([]*entities.ArtifactListReport, error) {
	return artifacts.List(ir.ClientCtx, &artifacts.ListOptions{})
}

func (ir *ImageEngine) ArtifactPull(_ context.Context, name string, opts entities.ArtifactPullOptions) (*entities.ArtifactPullReport, error) {
	options := artifacts.PullOptions{
		Username:   &opts.Username,
		Password:   &opts.Password,
		Quiet:      &opts.Quiet,
		RetryDelay: &opts.RetryDelay,
		Retry:      opts.MaxRetries,
	}

	switch opts.InsecureSkipTLSVerify {
	case types.OptionalBoolTrue:
		options.WithSkipTLSVerify(true)
	case types.OptionalBoolFalse:
		options.WithSkipTLSVerify(false)
	}

	return artifacts.Pull(ir.ClientCtx, name, &options)
}

func (ir *ImageEngine) ArtifactRm(_ context.Context, name string, opts entities.ArtifactRemoveOptions) (*entities.ArtifactRemoveReport, error) {
	if opts.All {
		// Note: This will be added when artifacts remove all endpoint is implemented
		return nil, fmt.Errorf("not implemented")
	}
	return artifacts.Remove(ir.ClientCtx, name, &artifacts.RemoveOptions{})
}

func (ir *ImageEngine) ArtifactPush(_ context.Context, name string, opts entities.ArtifactPushOptions) (*entities.ArtifactPushReport, error) {
	options := artifacts.PushOptions{
		Username:   &opts.Username,
		Password:   &opts.Password,
		Quiet:      &opts.Quiet,
		RetryDelay: &opts.RetryDelay,
		Retry:      opts.Retry,
	}

	switch opts.SkipTLSVerify {
	case types.OptionalBoolTrue:
		options.WithSkipTLSVerify(true)
	case types.OptionalBoolFalse:
		options.WithSkipTLSVerify(false)
	}

	return artifacts.Push(ir.ClientCtx, name, &options)
}

func (ir *ImageEngine) ArtifactAdd(_ context.Context, name string, artifactBlob []entities.ArtifactBlob, opts entities.ArtifactAddOptions) (*entities.ArtifactAddReport, error) {
	var (
		artifactAddReport *entities.ArtifactAddReport
		err               error
	)

	options := artifacts.AddOptions{
		Append:           &opts.Append,
		ArtifactMIMEType: &opts.ArtifactMIMEType,
		FileMIMEType:     &opts.FileMIMEType,
	}

	if len(opts.Annotations) > 0 {
		for k, v := range opts.Annotations {
			options.Annotations = append(options.Annotations, k+"="+v)
		}
	}

	for i, blob := range artifactBlob {
		if i > 0 {
			// When adding more than 1 blob, set append true after the first
			options.WithAppend(true)
		}
		f, err := os.Open(blob.BlobFilePath)
		if err != nil {
			return nil, err
		}
		defer f.Close()

		artifactAddReport, err = artifacts.Add(ir.ClientCtx, name, blob.FileName, f, &options)
		if err != nil {
			return nil, err
		}
	}

	return artifactAddReport, err
}
