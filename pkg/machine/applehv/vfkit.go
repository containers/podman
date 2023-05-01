//go:build arm64 && darwin

package applehv

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v4/pkg/machine"
	"github.com/sirupsen/logrus"
)

// getDefaultDevices is a constructor for vfkit devices
func getDefaultDevices(imagePath, logPath string) []string {
	defaultDevices := []string{
		"virtio-rng",
		"virtio-net,nat,mac=72:20:43:d4:38:62",
		fmt.Sprintf("virtio-blk,path=%s", imagePath),
		fmt.Sprintf("virtio-serial,logFilePath=%s", logPath),
	}
	return defaultDevices
}

func newVfkitHelper(name, endpoint, imagePath string) (*VfkitHelper, error) {
	dataDir, err := machine.GetDataDir(machine.AppleHvVirt)
	if err != nil {
		return nil, err
	}
	logPath := filepath.Join(dataDir, fmt.Sprintf("%s.log", name))
	logLevel := logrus.GetLevel()

	cfg, err := config.Default()
	if err != nil {
		return nil, err
	}
	vfkitPath, err := cfg.FindHelperBinary("vfkit", false)
	if err != nil {
		return nil, err
	}
	return &VfkitHelper{
		Bootloader:        fmt.Sprintf("efi,variable-store=%s/%s-efi-store,create", dataDir, name),
		Devices:           getDefaultDevices(imagePath, logPath),
		LogLevel:          logLevel,
		PathToVfkitBinary: vfkitPath,
		Endpoint:          endpoint,
	}, nil
}

// toCmdLine creates the command line for calling vfkit.  the vfkit binary is
// NOT part of the cmdline
func (vf *VfkitHelper) toCmdLine(cpus, memory string) []string {
	endpoint := vf.Endpoint
	if strings.HasPrefix(endpoint, "http") {
		endpoint = strings.Replace(endpoint, "http", "tcp", 1)
	}
	cmd := []string{
		vf.PathToVfkitBinary,
		"--cpus", cpus,
		"--memory", memory,
		"--bootloader", vf.Bootloader,
		"--restful-uri", endpoint,
		// this is on for development but can probably be disabled later, or
		// we can leave it as optional.
		//"--log-level", "debug",
	}
	for _, d := range vf.Devices {
		cmd = append(cmd, "--device", d)
	}
	return cmd
}

func (vf *VfkitHelper) startVfkit(m *MacMachine) error {
	dnr, err := os.OpenFile(os.DevNull, os.O_RDONLY, 0755)
	if err != nil {
		return err
	}
	dnw, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0755)
	if err != nil {
		return err
	}

	defer func() {
		if err := dnr.Close(); err != nil {
			logrus.Error(err)
		}
	}()
	defer func() {
		if err := dnw.Close(); err != nil {
			logrus.Error(err)
		}
	}()

	attr := new(os.ProcAttr)
	attr.Files = []*os.File{dnr, dnw}
	cmdLine := vf.toCmdLine(
		strconv.Itoa(int(m.ResourceConfig.CPUs)),
		strconv.Itoa(int(m.ResourceConfig.Memory)),
	)
	logrus.Debugf("vfkit cmd: %v", cmdLine)
	//stderrBuf := &bytes.Buffer{}
	cmd := &exec.Cmd{
		Args:   cmdLine,
		Path:   vf.PathToVfkitBinary,
		Stdin:  dnr,
		Stdout: dnw,
		// this makes no sense to me ... this only works if I pipe stdout to stderr
		Stderr: os.Stdout,
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	defer cmd.Process.Release() //nolint:errcheck
	return nil
}
