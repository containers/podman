package artifact

import (
	"fmt"
	"os"
	"time"

	"github.com/containers/podman/v6/cmd/podman/common"
	"github.com/containers/podman/v6/cmd/podman/registry"
	"github.com/containers/podman/v6/cmd/podman/validate"
	"github.com/containers/podman/v6/pkg/domain/entities"
	"github.com/docker/go-units"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/common/pkg/report"
	"go.podman.io/image/v5/docker/reference"
)

var (
	listCmd = &cobra.Command{
		Use:               "ls [options]",
		Aliases:           []string{"list"},
		Short:             "List OCI artifacts",
		Long:              "List OCI artifacts in local store",
		RunE:              list,
		Args:              validate.NoArgs,
		ValidArgsFunction: completion.AutocompleteNone,
		Example:           `podman artifact ls`,
	}
	listFlag = listFlagType{}
)

type listFlagType struct {
	format    string
	noHeading bool
	noTrunc   bool
}

type artifactListOutput struct {
	Digest      string
	Repository  string
	Size        string
	Tag         string
	Created     string
	VirtualSize string
}

var (
	defaultArtifactListOutputFormat = "{{range .}}{{.Repository}}\t{{.Tag}}\t{{.Digest}}\t{{.Created}}\t{{.Size}}\n{{end -}}"
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: listCmd,
		Parent:  artifactCmd,
	})
	flags := listCmd.Flags()
	formatFlagName := "format"
	flags.StringVar(&listFlag.format, formatFlagName, defaultArtifactListOutputFormat, "Format volume output using JSON or a Go template")
	_ = listCmd.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(&artifactListOutput{}))
	flags.BoolVarP(&listFlag.noHeading, "noheading", "n", false, "Do not print column headings")
	flags.BoolVar(&listFlag.noTrunc, "no-trunc", false, "Do not truncate output")
}

func list(cmd *cobra.Command, _ []string) error {
	reports, err := registry.ImageEngine().ArtifactList(registry.Context(), entities.ArtifactListOptions{})
	if err != nil {
		return err
	}

	return outputTemplate(cmd, reports)
}

func outputTemplate(cmd *cobra.Command, lrs []*entities.ArtifactListReport) error {
	var err error
	artifacts := make([]artifactListOutput, 0)
	for _, lr := range lrs {
		var (
			tag string
		)
		artifactName, err := lr.Artifact.GetName()
		if err != nil {
			return err
		}
		repo, err := reference.Parse(artifactName)
		if err != nil {
			return err
		}
		named, ok := repo.(reference.Named)
		if !ok {
			return fmt.Errorf("%q is an invalid artifact name", artifactName)
		}
		if tagged, ok := named.(reference.Tagged); ok {
			tag = tagged.Tag()
		}

		// Note: Right now we only support things that are single manifests
		// We should certainly expand this support for things like arch, etc
		// as we move on
		artifactDigest, err := lr.Artifact.GetDigest()
		if err != nil {
			return err
		}

		artifactHash := artifactDigest.Encoded()[0:12]
		// If the user does not want truncated hashes
		if listFlag.noTrunc {
			artifactHash = artifactDigest.Encoded()
		}

		var created string
		createdAnnotation, ok := lr.Manifest.Annotations[imgspecv1.AnnotationCreated]
		if ok {
			createdTime, err := time.Parse(time.RFC3339Nano, createdAnnotation)
			if err != nil {
				return err
			}
			created = units.HumanDuration(time.Since(createdTime)) + " ago"
		}

		artifacts = append(artifacts, artifactListOutput{
			Digest:      artifactHash,
			Repository:  named.Name(),
			Size:        units.HumanSize(float64(lr.Artifact.TotalSizeBytes())),
			Tag:         tag,
			Created:     created,
			VirtualSize: fmt.Sprintf("%d", lr.Artifact.TotalSizeBytes()),
		})
	}

	headers := report.Headers(artifactListOutput{}, map[string]string{
		"REPOSITORY": "REPOSITORY",
		"Tag":        "TAG",
		"Created":    "CREATED",
		"Size":       "SIZE",
		"Digest":     "DIGEST",
	})

	rpt := report.New(os.Stdout, cmd.Name())
	defer rpt.Flush()

	switch {
	case cmd.Flag("format").Changed:
		rpt, err = rpt.Parse(report.OriginUser, listFlag.format)
	default:
		rpt, err = rpt.Parse(report.OriginPodman, listFlag.format)
	}
	if err != nil {
		return err
	}

	if rpt.RenderHeaders && !listFlag.noHeading {
		if err := rpt.Execute(headers); err != nil {
			return fmt.Errorf("failed to write report column headers: %w", err)
		}
	}
	return rpt.Execute(artifacts)
}
