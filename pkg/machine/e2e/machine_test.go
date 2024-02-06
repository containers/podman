package e2e_test

import (
	"fmt"
	"io"
	url2 "net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/containers/podman/v4/pkg/machine"
	"github.com/containers/podman/v4/pkg/machine/compression"
	"github.com/containers/podman/v4/pkg/machine/define"
	"github.com/containers/podman/v4/pkg/machine/provider"
	"github.com/containers/podman/v4/pkg/machine/vmconfigs"
	"github.com/containers/podman/v4/utils"
	crcOs "github.com/crc-org/crc/v2/pkg/os"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

const (
	defaultStream        = machine.Testing
	defaultDiskSize uint = 11
)

var (
	tmpDir         = os.TempDir()
	fqImageName    string
	suiteImageName string
)

func init() {
	if value, ok := os.LookupEnv("TMPDIR"); ok {
		tmpDir = value
	}
}

// TestLibpod ginkgo master function
func TestMachine(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Podman Machine tests")
}

var testProvider vmconfigs.VMProvider

var _ = BeforeSuite(func() {
	var err error
	testProvider, err = provider.Get()
	if err != nil {
		Fail("unable to create testProvider")
	}

	downloadLocation := os.Getenv("MACHINE_IMAGE")
	if downloadLocation == "" {
		downloadLocation, err = GetDownload(testProvider.VMType())
		if err != nil {
			Fail("unable to derive download disk from fedora coreos")
		}
	}

	if downloadLocation == "" {
		Fail("machine tests require a file reference to a disk image right now")
	}

	var compressionExtension string
	switch testProvider.VMType() {
	case define.AppleHvVirt:
		compressionExtension = ".gz"
	case define.HyperVVirt:
		compressionExtension = ".zip"
	default:
		compressionExtension = ".xz"
	}

	suiteImageName = strings.TrimSuffix(path.Base(downloadLocation), compressionExtension)
	fqImageName = filepath.Join(tmpDir, suiteImageName)
	if _, err := os.Stat(fqImageName); err != nil {
		if os.IsNotExist(err) {
			getMe, err := url2.Parse(downloadLocation)
			if err != nil {
				Fail(fmt.Sprintf("unable to create url for download: %q", err))
			}
			now := time.Now()
			if err := machine.DownloadVMImage(getMe, suiteImageName, fqImageName+compressionExtension); err != nil {
				Fail(fmt.Sprintf("unable to download machine image: %q", err))
			}
			GinkgoWriter.Println("Download took: ", time.Since(now).String())
			diskImage, err := define.NewMachineFile(fqImageName+compressionExtension, nil)
			if err != nil {
				Fail(fmt.Sprintf("unable to create vmfile %q: %v", fqImageName+compressionExtension, err))
			}
			compressionStart := time.Now()
			if err := compression.Decompress(diskImage, fqImageName); err != nil {
				Fail(fmt.Sprintf("unable to decompress image file: %q", err))
			}
			GinkgoWriter.Println("compression took: ", time.Since(compressionStart))
		} else {
			Fail(fmt.Sprintf("unable to check for cache image: %q", err))
		}
	}

})

var _ = SynchronizedAfterSuite(func() {}, func() {})

func setup() (string, *machineTestBuilder) {
	// Set TMPDIR if this needs a new directory
	homeDir, err := os.MkdirTemp("", "podman_test")
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
	mb.imagePath = filepath.Join(homeDir, suiteImageName)
	dest, err := os.Create(mb.imagePath)
	if err != nil {
		Fail(fmt.Sprintf("failed to create file %s: %q", mb.imagePath, err))
	}
	defer func() {
		if err := dest.Close(); err != nil {
			Fail(fmt.Sprintf("failed to close destination file %q: %q", dest.Name(), err))
		}
	}()
	fmt.Printf("--> copying %q to %q/n", src.Name(), dest.Name())
	if runtime.GOOS != "darwin" {
		if _, err := io.Copy(dest, src); err != nil {
			Fail(fmt.Sprintf("failed to copy %ss to %s: %q", fqImageName, mb.imagePath, err))
		}
	} else {
		if _, err := crcOs.CopySparse(dest, src); err != nil {
			Fail(fmt.Sprintf("failed to copy %q to %q: %q", src.Name(), dest.Name(), err))
		}
	}
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
}
