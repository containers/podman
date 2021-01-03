package images

import (
	"fmt"
	"os"

	"github.com/containers/podman/v2/cmd/podman/common"
	"github.com/containers/podman/v2/cmd/podman/registry"
	"github.com/containers/podman/v2/libpod/define"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/containers/podman/v2/pkg/specgen"
	"github.com/spf13/cobra"
)

var (
	imageScanCommand     = &cobra.Command{
		Use:     "scan [run-options] IMAGE [scanner-options]",
		Short:   "Scan a container image for known vulnerabilities",
		Long:    `Scan a container image for known vulnerabilities. The image name or digest can be used.`,
		Args: cobra.MinimumNArgs(1),
		RunE:    scan,
		Example: `podman image scan centos:latest`,
	}

	scanOptions entities.ImageScanOptions
)


func scanFlags(cmd *cobra.Command) {
	flags := cmd.Flags()

	// TODO: derive default from containers.conf configuration
	scannerFlagName := "scanner"
	flags.StringVar(&scanOptions.ScannerImage, scannerFlagName, "docker.io/anchore/grype:latest", "The vulnerability scanner container image to use")
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode},
		Command: imageScanCommand,
		Parent:  imageCmd,
	})
	scanFlags(imageScanCommand)
}

func scan(cmd *cobra.Command, args []string) error {
	imageToScan := args[0]
	mountTarget := "/podman-scan-image-mount"
	// allow parsing to continue with the first arg being the scanner image (not the image to scan)
	args[0] = scanOptions.ScannerImage

	ctrConfig := registry.PodmanConfig()

	ctrOpts := common.ContainerCLIOpts{
		Mount: []string{fmt.Sprintf("type=image,src=%s,target=%s,readwrite=false", imageToScan, mountTarget)},
		ImageVolume: common.DefaultImageVolume,
		HealthInterval: common.DefaultHealthCheckInterval,
		HealthRetries: common.DefaultHealthCheckRetries,
		HealthStartPeriod: common.DefaultHealthCheckStartPeriod,
		HealthTimeout: common.DefaultHealthCheckTimeout,
		HTTPProxy: true,
		MemorySwappiness: -1,
		// TODO: scanners may need these options to be made available to the user, yes?
		Net:          &entities.NetOptions{},
		CGroupsMode:  ctrConfig.Cgroups(),
		Devices:      ctrConfig.Devices(),
		Env:          ctrConfig.Env(),
		Ulimit:       ctrConfig.Ulimits(),
		InitPath:     ctrConfig.InitPath(),
		Pull:         ctrConfig.Engine.PullPolicy,
		SdNotifyMode: define.SdNotifyModeContainer,
		ShmSize:      ctrConfig.ShmSize(),
		StopTimeout:  ctrConfig.Engine.StopTimeout,
		Systemd:  "true",
		Timezone: ctrConfig.TZ(),
		Umask:    ctrConfig.Umask(),
		UserNS:   os.Getenv("PODMAN_USERNS"),
		Volume:   ctrConfig.Volumes(),
		SeccompPolicy: "default",
	}

	ctrOpts.Env = append(ctrOpts.Env, fmt.Sprintf("PODMAN_SCAN_MOUNT=%s", mountTarget))

	s := specgen.NewSpecGenerator(scanOptions.ScannerImage, false)
	if err := common.FillOutSpecGen(s, &ctrOpts, args); err != nil {
		return err
	}

	s.RawImageName = scanOptions.ScannerImage
	s.Terminal = true

	runOpts := entities.ContainerRunOptions{
		OutputStream: os.Stdout,
		ErrorStream:  os.Stderr,
		InputStream:  os.Stdin,
		Rm:           true,
		Spec:         s,
		SigProxy:     true,
		DetachKeys:   ctrConfig.DetachKeys(),
	}

	report, err := registry.ContainerEngine().ContainerRun(registry.GetContext(), runOpts)
	// report.ExitCode is set by ContainerRun even it it returns an error
	if report != nil {
		registry.SetExitCode(report.ExitCode)
	}
	if err != nil {
		return err
	}

	if runOpts.Detach {
		fmt.Println(report.Id)
		return nil
	}
	return nil

}
