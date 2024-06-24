package e2e_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v5/pkg/machine/define"
	"github.com/containers/podman/v5/pkg/machine/provider"
	"github.com/containers/podman/v5/pkg/machine/vmconfigs"
	"github.com/containers/podman/v5/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

const (
	defaultDiskSize uint = 11
)

var (
	tmpDir         = os.TempDir()
	fqImageName    string
	suiteImageName string
)

func init() {
	if value, ok := os.LookupEnv("TMPDIR"); ok {
		var err error
		tmpDir, err = setTmpDir(value)
		if err != nil {
			fmt.Printf("failed to set TMPDIR: %q\n", err)
		}
	}
}

// TestLibpod ginkgo master function
func TestMachine(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Podman Machine tests")
}

var testProvider vmconfigs.VMProvider

var _ = BeforeSuite(func() {
	var (
		err       error
		pullError error
	)
	testProvider, err = provider.Get()
	if err != nil {
		Fail("unable to create testProvider")
	}
	if testProvider.VMType() == define.WSLVirt {
		pullError = pullWSLDisk()
	} else {
		pullError = pullOCITestDisk(tmpDir, testProvider.VMType())
	}
	if pullError != nil {
		Fail(fmt.Sprintf("failed to pull wsl disk: %q", pullError))
	}
})

var _ = SynchronizedAfterSuite(func() {}, func() {})

func setup() (string, *machineTestBuilder) {
	// Set TMPDIR if this needs a new directory
	if value, ok := os.LookupEnv("TMPDIR"); ok {
		var err error
		tmpDir, err = setTmpDir(value)
		if err != nil {
			Fail(fmt.Sprintf("failed to set TMPDIR: %q", err))
		}
	}
	homeDir, err := os.MkdirTemp(tmpDir, "podman_test")
	if err != nil {
		Fail(fmt.Sprintf("failed to create home directory: %q", err))
	}
	if err := os.MkdirAll(filepath.Join(homeDir, ".ssh"), 0700); err != nil {
		Fail(fmt.Sprintf("failed to create ssh dir: %q", err))
	}
	sshConfig, err := os.Create(filepath.Join(homeDir, ".ssh", "config"))
	if err != nil {
		Fail(fmt.Sprintf("failed to create ssh config: %q", err))
	}
	if _, err := sshConfig.WriteString("IdentitiesOnly=yes"); err != nil {
		Fail(fmt.Sprintf("failed to write ssh config: %q", err))
	}
	if err := sshConfig.Close(); err != nil {
		Fail(fmt.Sprintf("unable to close ssh config file descriptor: %q", err))
	}
	if err := os.Setenv("HOME", homeDir); err != nil {
		Fail("failed to set home dir")
	}
	if runtime.GOOS == "windows" {
		if err := os.Setenv("USERPROFILE", homeDir); err != nil {
			Fail("unable to set home dir on windows")
		}
	}
	if err := os.Setenv("XDG_RUNTIME_DIR", homeDir); err != nil {
		Fail("failed to set xdg_runtime dir")
	}
	if err := os.Unsetenv("SSH_AUTH_SOCK"); err != nil {
		Fail("unable to unset SSH_AUTH_SOCK")
	}
	mb, err := newMB()
	if err != nil {
		Fail(fmt.Sprintf("failed to create machine test: %q", err))
	}
	src, err := os.Open(fqImageName)
	if err != nil {
		Fail(fmt.Sprintf("failed to open file %s: %q", fqImageName, err))
	}
	defer func() {
		if err := src.Close(); err != nil {
			Fail(fmt.Sprintf("failed to close src reader %q: %q", src.Name(), err))
		}
	}()
	mb.imagePath = fqImageName
	return homeDir, mb
}

func teardown(origHomeDir string, testDir string, mb *machineTestBuilder) {
	r := new(rmMachine)
	for _, name := range mb.names {
		if _, err := mb.setName(name).setCmd(r.withForce()).run(); err != nil {
			GinkgoWriter.Printf("error occurred rm'ing machine: %q\n", err)
		}
	}

	if err := utils.GuardedRemoveAll(testDir); err != nil {
		Fail(fmt.Sprintf("failed to remove test dir: %q", err))
	}
	// this needs to be last in teardown
	if err := os.Setenv("HOME", origHomeDir); err != nil {
		Fail("failed to set home dir")
	}
	if runtime.GOOS == "windows" {
		if err := os.Setenv("USERPROFILE", origHomeDir); err != nil {
			Fail("failed to set windows home dir back to original")
		}
	}
}

func setTmpDir(value string) (string, error) {
	switch {
	case runtime.GOOS != "darwin":
		tmpDir = value
	case len(value) >= 22:
		return "", errors.New(value + " path length should be less than 22 characters")
	case value == "":
		return "", errors.New("TMPDIR cannot be empty. Set to directory mounted on podman machine (e.g. /private/tmp)")
	default:
		cfg, err := config.Default()
		if err != nil {
			return "", err
		}
		volumes := cfg.Machine.Volumes.Get()
		containsPath := false
		for _, volume := range volumes {
			parts := strings.Split(volume, ":")
			hostPath := parts[0]
			if strings.Contains(value, hostPath) {
				containsPath = true
				break
			}
		}
		if !containsPath {
			return "", fmt.Errorf("%s cannot be used. Change to directory mounted on podman machine (e.g. /private/tmp)", value)
		}
		tmpDir = value
	}
	return tmpDir, nil
}
