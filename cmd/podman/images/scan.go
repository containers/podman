package images

import (
	"bytes"
	"context"
	"fmt"
	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v2/cmd/podman/parse"
	"io/ioutil"
	"os"
	"strings"
	"text/template"

	"github.com/containers/podman/v2/cmd/podman/common"
	"github.com/containers/podman/v2/cmd/podman/registry"
	"github.com/containers/podman/v2/libpod/define"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/containers/podman/v2/pkg/specgen"
	"github.com/spf13/cobra"
)

var (
	imageScanCommand = &cobra.Command{
		Use:     "scan [run-options] IMAGE [scanner-options]",
		Short:   "Scan a container image for known vulnerabilities",
		Long:    `Scan a container image for known vulnerabilities. The image name or digest can be used.`,
		Args:    cobra.MinimumNArgs(1),
		RunE:    scan,
		Example: `podman image scan centos:latest`,
	}

	scanOptions entities.ImageScanOptions
)

func scanFlags(cmd *cobra.Command) {
	flags := cmd.Flags()

	// TODO: derive default from containers.conf configuration
	flagName := "scanner"
	flags.StringVarP(
		&scanOptions.ScannerImage,
		flagName, "s", "docker.io/anchore/syft:latest",
		"The vulnerability scanner container image to use",
	)
	_ = cmd.RegisterFlagCompletionFunc(flagName, completion.AutocompleteNone)

	flagName = "mount-point"
	flags.StringVarP(
		&scanOptions.MountPoint,
		flagName, "m", "/podman-scan-image-mount",
		"Mount-point of the target image to be scanned",
	)
	_ = cmd.RegisterFlagCompletionFunc(flagName, completion.AutocompleteNone)

	flagName = "format"
	flags.StringVarP(
		&scanOptions.Format,
		flagName, "f", "squash",
		"The image format of the target image to be scanned (options: oci-dir, oci-archive, docker-dir, docker-archive, squash)",
	)
	_ = cmd.RegisterFlagCompletionFunc(flagName, completion.AutocompleteNone)

	flagName = "env"
	flags.StringSliceVarP(
		&scanOptions.Env,
		flagName, "e", nil,
		"Set environment variables in scanner tool container",
	)
	_ = cmd.RegisterFlagCompletionFunc(flagName, completion.AutocompleteNone)

}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode},
		Command: imageScanCommand,
		Parent:  imageCmd,
	})
	scanFlags(imageScanCommand)
}

func prepareImageMount(image string) (func(), string, error) {
	var location string
	var mountOptions string
	var cleanupFn func()

	switch {
	case strings.HasSuffix(scanOptions.Format, "-archive"):
		file, err := ioutil.TempFile("", "podman-scan-image-archive")
		if err != nil {
			return nil, "", fmt.Errorf("unable to create temporary image archive: %w", err)
		}
		cleanupFn = func() {
			os.Remove(file.Name())
		}
		location = file.Name()
		mountOptions = fmt.Sprintf("type=bind,source=%s,target=%s,ro=true", location, scanOptions.MountPoint)
	case strings.HasSuffix(scanOptions.Format, "-dir"):
		dir, err := ioutil.TempDir("dir", "podman-scan-image-dir")
		if err != nil {
			return nil, "", fmt.Errorf("unable to create temporary image archive: %w", err)
		}
		cleanupFn = func() {
			os.RemoveAll(dir)
		}
		location = dir
		mountOptions = fmt.Sprintf("type=bind,source=%s,target=%s,ro=true", location, scanOptions.MountPoint)
	case scanOptions.Format == "squash":
		mountOptions = fmt.Sprintf("type=image,src=%s,target=%s,readwrite=false", image, scanOptions.MountPoint)
		return nil, mountOptions, nil
	default:
		return nil, "", fmt.Errorf("unknown image save format: %q", scanOptions.Format)
	}

	if err := parse.ValidateFileName(location); err != nil {
		return cleanupFn, "", err
	}

	tempSaveOpts := entities.ImageSaveOptions{
		Format: scanOptions.Format,
		Output: location,
	}

	return cleanupFn, mountOptions, registry.ImageEngine().Save(context.Background(), image, nil, tempSaveOpts)

}

func scan(cmd *cobra.Command, args []string) error {
	imageToScan := args[0]

	// render all scanner args with the mount-point (if necessary)
	type ScanTemplate struct {
		MountPoint string
	}
	values := ScanTemplate{MountPoint: scanOptions.MountPoint}
	tpl := template.New("podman-scan")

	scannerArgs := append([]string{scanOptions.ScannerImage}, args[1:]...)
	for i, arg := range scannerArgs {
		parsedTpl, err := tpl.Parse(arg)
		if err != nil {
			continue
		}

		buf := bytes.NewBufferString("")
		if err = parsedTpl.Execute(buf, values); err != nil {
			continue
		}
		scannerArgs[i] = buf.String()
	}

	// save the image (if necessary)
	cleanupFn, mountOptions, err := prepareImageMount(imageToScan)
	if cleanupFn != nil {
		defer cleanupFn()
	}
	if err != nil {
		return err
	}

	ctrConfig := registry.PodmanConfig()

	ctrOpts := common.ContainerCLIOpts{
		Mount:             []string{mountOptions},
		ImageVolume:       common.DefaultImageVolume,
		HealthInterval:    common.DefaultHealthCheckInterval,
		HealthRetries:     common.DefaultHealthCheckRetries,
		HealthStartPeriod: common.DefaultHealthCheckStartPeriod,
		HealthTimeout:     common.DefaultHealthCheckTimeout,
		HTTPProxy:         true,
		MemorySwappiness:  -1,
		// TODO: scanners may need these options to be made available to the user, yes?
		Net:           &entities.NetOptions{},
		CGroupsMode:   ctrConfig.Cgroups(),
		Devices:       ctrConfig.Devices(),
		Env:           scanOptions.Env,
		Ulimit:        ctrConfig.Ulimits(),
		InitPath:      ctrConfig.InitPath(),
		Pull:          ctrConfig.Engine.PullPolicy,
		SdNotifyMode:  define.SdNotifyModeContainer,
		ShmSize:       ctrConfig.ShmSize(),
		StopTimeout:   ctrConfig.Engine.StopTimeout,
		Systemd:       "true",
		Timezone:      ctrConfig.TZ(),
		Umask:         ctrConfig.Umask(),
		UserNS:        os.Getenv("PODMAN_USERNS"),
		Volume:        ctrConfig.Volumes(),
		SeccompPolicy: "default",
	}

	s := specgen.NewSpecGenerator(scanOptions.ScannerImage, false)
	if err := common.FillOutSpecGen(s, &ctrOpts, scannerArgs); err != nil {
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
