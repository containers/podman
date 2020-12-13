package images

import (
	"context"
	"fmt"
	"github.com/containers/podman/v2/cmd/podman/common"
	"github.com/containers/podman/v2/cmd/podman/registry"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/containers/podman/v2/pkg/specgen"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"os"
	"strings"
)

var (
	scanDescription = `Scan a container image for known vulnerabilities.  The image name or digest can be used.`
	scanCommand     = &cobra.Command{
		Use:     "scan [run-options] IMAGE [scanner-options]",
		Short:   "Scan a container image for known vulnerabilities",
		Long:    scanDescription,
		RunE:    scan,
		Example: `podman scan centos:latest`,
	}

	imageScanCommand     = &cobra.Command{
		Use:     "scan [run-options] IMAGE [scanner-options]",
		Short:   "Scan a container image for known vulnerabilities",
		Long:    scanDescription,
		RunE:    scan,
		Example: `podman image scan centos:latest`,
	}
)

var (
	scanOptions = entities.ImageScanOptions{
		ContainerRunOptions: entities.ContainerRunOptions{
			OutputStream: os.Stdout,
			ErrorStream:  os.Stderr,
			Rm: true,
		},
	}

	cliVals common.ContainerCLIOpts
)

func scanFlags(cmd *cobra.Command) {
	flags := cmd.Flags()

	common.DefineCreateFlags(cmd, &cliVals)
	common.DefineNetFlags(cmd)

	// TODO: derive default from containers.conf configuration
	scannerFlagName := "scanner"
	flags.StringVar(&scanOptions.ScannerImage, scannerFlagName, "docker.io/library/grype:latest", "The vulnerability scanner container image to use")
}
func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: scanCommand,
	})
	scanFlags(scanCommand)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: imageScanCommand,
		Parent:  imageCmd,
	})
	scanFlags(imageScanCommand)

}

func scan(cmd *cobra.Command, args []string) error {
	var err error

	if len(args) < 1 {
		return fmt.Errorf("requires exactly one image, given %+v", args)
	}
	var imageToScan = args[0]
	var mountTarget = "/podman-scan-image-mount"
	var scanMount = fmt.Sprintf("type=image,src=%s,target=%s", imageToScan, mountTarget)

	cliVals.Mount = append(cliVals.Mount, scanMount)

	// allow parsing to continue with the first arg being the scanner image (not the image to scan)
	args[0] = scanOptions.ScannerImage

	cliVals.Net, err = common.NetFlagsToNetOptions(cmd)
	if err != nil {
		return err
	}

	if af := cliVals.Authfile; len(af) > 0 {
		if _, err := os.Stat(af); err != nil {
			return err
		}
	}

	s := specgen.NewSpecGenerator(scanOptions.ScannerImage, false)
	if err := common.FillOutSpecGen(s, &cliVals, args); err != nil {
		return err
	}
	s.RawImageName = scanOptions.ScannerImage
	scanOptions.Spec = s

	// TODO: support templating in config
	// now force the scanner to use the mount as the first argument
	s.Command = append([]string{"dir:"+mountTarget}, s.Command...)

	if _, err := createPodIfNecessary(s, cliVals.Net); err != nil {
		return err
	}

	report, err := registry.ContainerEngine().ContainerRun(registry.GetContext(), scanOptions.ContainerRunOptions)
	// report.ExitCode is set by ContainerRun even it it returns an error
	if report != nil {
		registry.SetExitCode(report.ExitCode)
	}
	if err != nil {
		return err
	}

	if scanOptions.Detach {
		fmt.Println(report.Id)
		return nil
	}
	return nil

}

// createPodIfNecessary automatically creates a pod when requested.  if the pod name
// has the form new:ID, the pod ID is created and the name in the spec generator is replaced
// with ID.
func createPodIfNecessary(s *specgen.SpecGenerator, netOpts *entities.NetOptions) (*entities.PodCreateReport, error) {
	if !strings.HasPrefix(s.Pod, "new:") {
		return nil, nil
	}
	podName := strings.Replace(s.Pod, "new:", "", 1)
	if len(podName) < 1 {
		return nil, errors.Errorf("new pod name must be at least one character")
	}
	createOptions := entities.PodCreateOptions{
		Name:          podName,
		Infra:         true,
		Net:           netOpts,
		CreateCommand: os.Args,
		Hostname:      s.ContainerBasicConfig.Hostname,
	}
	// Unset config values we passed to the pod to prevent them being used twice for the container and pod.
	s.ContainerBasicConfig.Hostname = ""
	s.ContainerNetworkConfig = specgen.ContainerNetworkConfig{}

	s.Pod = podName
	return registry.ContainerEngine().PodCreate(context.Background(), createOptions)
}

