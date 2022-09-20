package kube

import (
	"fmt"
	"io"
	"os"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/generate"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/cmd/podman/utils"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	generateOptions     = entities.GenerateKubeOptions{}
	generateFile        = ""
	generateDescription = `Command generates Kubernetes Pod, Service or PersistenVolumeClaim YAML (v1 specification) from Podman containers, pods or volumes.

  Whether the input is for a container or pod, Podman will always generate the specification as a pod.`

	kubeGenerateCmd = &cobra.Command{
		Use:               "generate [options] {CONTAINER...|POD...|VOLUME...}",
		Short:             "Generate Kubernetes YAML from containers, pods or volumes.",
		Long:              generateDescription,
		RunE:              generateKube,
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: common.AutocompleteForGenerate,
		Example: `podman kube generate ctrID
  podman kube generate podID
  podman kube generate --service podID
  podman kube generate volumeName
  podman kube generate ctrID podID volumeName --service`,
	}

	generateKubeCmd = &cobra.Command{
		Use:               "kube [options] {CONTAINER...|POD...|VOLUME...}",
		Short:             kubeGenerateCmd.Short,
		Long:              kubeGenerateCmd.Long,
		RunE:              kubeGenerateCmd.RunE,
		Args:              kubeGenerateCmd.Args,
		ValidArgsFunction: kubeGenerateCmd.ValidArgsFunction,
		Example:           kubeGenerateCmd.Example,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: generateKubeCmd,
		Parent:  generate.GenerateCmd,
	})
	generateFlags(generateKubeCmd)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: kubeGenerateCmd,
		Parent:  kubeCmd,
	})
	generateFlags(kubeGenerateCmd)
}

func generateFlags(cmd *cobra.Command) {
	flags := cmd.Flags()
	flags.BoolVarP(&generateOptions.Service, "service", "s", false, "Generate YAML for a Kubernetes service object")

	filenameFlagName := "filename"
	flags.StringVarP(&generateFile, filenameFlagName, "f", "", "Write output to the specified path")
	_ = cmd.RegisterFlagCompletionFunc(filenameFlagName, completion.AutocompleteDefault)

	flags.SetNormalizeFunc(utils.AliasFlags)
}

func generateKube(cmd *cobra.Command, args []string) error {
	report, err := registry.ContainerEngine().GenerateKube(registry.GetContext(), args, generateOptions)
	if err != nil {
		return err
	}
	content, err := io.ReadAll(report.Reader)
	if err != nil {
		return err
	}
	if r, ok := report.Reader.(io.ReadCloser); ok {
		defer r.Close()
	}

	if cmd.Flags().Changed("filename") {
		if _, err := os.Stat(generateFile); err == nil {
			return fmt.Errorf("cannot write to %q; file exists", generateFile)
		}
		if err := os.WriteFile(generateFile, content, 0644); err != nil {
			return fmt.Errorf("cannot write to %q: %w", generateFile, err)
		}
		return nil
	}

	fmt.Println(string(content))
	return nil
}
