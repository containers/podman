package artifact

import (
	"fmt"
	"path/filepath"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v5/cmd/podman/common"
	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/containers/podman/v5/pkg/domain/utils"
	"github.com/spf13/cobra"
)

var (
	addCmd = &cobra.Command{
		Use:               "add [options] ARTIFACT PATH [...PATH]",
		Short:             "Add an OCI artifact to the local store",
		Long:              "Add an OCI artifact to the local store from the local filesystem",
		RunE:              add,
		Args:              cobra.MinimumNArgs(2),
		ValidArgsFunction: common.AutocompleteArtifactAdd,
		Example: `podman artifact add quay.io/myimage/myartifact:latest /tmp/foobar.txt
podman artifact add --file-type text/yaml quay.io/myimage/myartifact:latest /tmp/foobar.yaml
podman artifact add --append quay.io/myimage/myartifact:latest /tmp/foobar.tar.gz`,
		Annotations: map[string]string{registry.EngineMode: registry.ABIMode},
	}
)

type artifactAddOptions struct {
	ArtifactType string
	Annotations  []string
	Append       bool
	FileType     string
}

var (
	addOpts artifactAddOptions
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: addCmd,
		Parent:  artifactCmd,
	})
	flags := addCmd.Flags()

	annotationFlagName := "annotation"
	flags.StringArrayVar(&addOpts.Annotations, annotationFlagName, nil, "set an `annotation` for the specified files of artifact")
	_ = addCmd.RegisterFlagCompletionFunc(annotationFlagName, completion.AutocompleteNone)

	addTypeFlagName := "type"
	flags.StringVar(&addOpts.ArtifactType, addTypeFlagName, "", "Use type to describe an artifact")
	_ = addCmd.RegisterFlagCompletionFunc(addTypeFlagName, completion.AutocompleteNone)

	appendFlagName := "append"
	flags.BoolVarP(&addOpts.Append, appendFlagName, "a", false, "Append files to an existing artifact")

	fileTypeFlagName := "file-type"
	flags.StringVarP(&addOpts.FileType, fileTypeFlagName, "", "", "Set file type to use for the artifact (layer)")
	_ = addCmd.RegisterFlagCompletionFunc(fileTypeFlagName, completion.AutocompleteNone)
}

func add(cmd *cobra.Command, args []string) error {
	artifactName := args[0]
	blobs := args[1:]
	opts := new(entities.ArtifactAddOptions)

	annots, err := utils.ParseAnnotations(addOpts.Annotations)
	if err != nil {
		return err
	}
	opts.Annotations = annots
	opts.ArtifactType = addOpts.ArtifactType
	opts.Append = addOpts.Append
	opts.FileType = addOpts.FileType

	artifactBlobs := make([]entities.ArtifactBlob, 0, len(blobs))

	for _, blobPath := range blobs {
		artifactBlob := entities.ArtifactBlob{
			BlobFilePath: blobPath,
			FileName:     filepath.Base(blobPath),
		}

		artifactBlobs = append(artifactBlobs, artifactBlob)
	}

	report, err := registry.ImageEngine().ArtifactAdd(registry.Context(), artifactName, artifactBlobs, opts)
	if err != nil {
		return err
	}
	fmt.Println(report.ArtifactDigest.Encoded())
	return nil
}
