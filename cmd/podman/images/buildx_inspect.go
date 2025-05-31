package images

import (
	"fmt"
	"strings"

	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/spf13/cobra"
)

type buildNode struct {
	Name            string
	Endpoint        string
	Status          string
	BuildkitVersion string
	Platforms       []string
}

type buildxInspectOutput struct {
	builderName string
	driverName  string
	Nodes       []buildNode
}

var buildxInspectCmd = &cobra.Command{
	Use:   "inspect",
	Short: "Inspects the current buildx builder instance",
	Long:  "Displays information about the current builder instance (compatibility with Docker buildx inspect)",
	RunE:  runBuildxInspect,
	Example: `podman buildx inspect
	podman buildx inspect --bootstrap`,
}

func init() {
	buildxInspectCmd.Flags().Bool("bootstrap", false, "Currently a No Op for podman")
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: buildxInspectCmd,
		Parent:  buildxCmd,
	})
}

func runBuildxInspect(cmd *cobra.Command, args []string) error {
	// Constants are based on default values for Docker buildx inspect.
	defaultNode := buildNode{
		Name:            "default",
		Endpoint:        "default",
		Status:          "running",
		BuildkitVersion: "N/A",
		Platforms:       []string{"unknown/unknown"},
	}

	info, err := registry.ContainerEngine().Info(registry.Context())

	if err != nil {
		return fmt.Errorf("retrieving podman information: %w", err)
	}

	if registry.IsRemote() {
		runtimeOS := info.Host.OS
		runtimeArch := info.Host.Arch
		defaultNode.Platforms = []string{fmt.Sprintf("%s/%s", runtimeOS, runtimeArch)}
	} else {
		platforms := info.Host.EmulatedArchitectures
		platforms = append(platforms, fmt.Sprintf("%s/%s", info.Host.OS, info.Host.Arch))
		defaultNode.Platforms = platforms
	}

	out := buildxInspectOutput{
		builderName: "default",
		driverName:  "podman",
		Nodes:       []buildNode{defaultNode},
	}

	fmt.Printf("Name:   %s\n", out.builderName)
	fmt.Printf("Driver: %s\n", out.driverName)
	fmt.Println()

	fmt.Println("Nodes:")
	fmt.Printf("Name:             %s\n", out.Nodes[0].Name)
	fmt.Printf("Endpoint:         %s\n", out.Nodes[0].Endpoint)
	fmt.Printf("Status:           %s\n", out.Nodes[0].Status)
	fmt.Printf("Buildkit version: %s\n", out.Nodes[0].BuildkitVersion)

	fmt.Printf("Platforms:        %s\n", strings.Join(out.Nodes[0].Platforms, ", "))
	fmt.Println("Labels: ")
	return nil
}
