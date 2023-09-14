package e2e_test

import (
	"fmt"
	"io"
	url2 "net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/containers/podman/v4/pkg/machine"
	"github.com/containers/podman/v4/pkg/machine/provider"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

const (
	defaultStream machine.FCOSStream = machine.Testing
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

var _ = BeforeSuite(func() {

	testProvider, err := provider.Get()
	if err != nil {
		Fail("unable to create testProvider")
	}

	downloadLocation := os.Getenv("MACHINE_IMAGE")

	if len(downloadLocation) < 1 {
		downloadLocation = getDownloadLocation(testProvider)
		// we cannot simply use OS here because hyperv uses fcos; so WSL is just
		// special here
		if testProvider.VMType() != machine.WSLVirt {
			downloadLocation = getDownloadLocation(testProvider)
		}
	}

	compressionExtension := fmt.Sprintf(".%s", testProvider.Compression().String())
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
			if err := machine.Decompress(fqImageName+compressionExtension, fqImageName); err != nil {
				Fail(fmt.Sprintf("unable to decompress image file: %q", err))
			}
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
	f, err := os.Open(fqImageName)
	if err != nil {
		Fail(fmt.Sprintf("failed to open file %s: %q", fqImageName, err))
	}
	mb.imagePath = filepath.Join(homeDir, suiteImageName)
	n, err := os.Create(mb.imagePath)
	if err != nil {
		Fail(fmt.Sprintf("failed to create file %s: %q", mb.imagePath, err))
	}
	if _, err := io.Copy(n, f); err != nil {
		Fail(fmt.Sprintf("failed to copy %ss to %s: %q", fqImageName, mb.imagePath, err))
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
	if err := machine.GuardedRemoveAll(testDir); err != nil {
		Fail(fmt.Sprintf("failed to remove test dir: %q", err))
	}
	// this needs to be last in teardown
	if err := os.Setenv("HOME", origHomeDir); err != nil {
		Fail("failed to set home dir")
	}
}
