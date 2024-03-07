package e2e_test

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/containers/podman/v5/pkg/machine/compression"
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
	mb.imagePath = filepath.Join(homeDir, suiteImageName)
	dest, err := os.Create(mb.imagePath)
	if err != nil {
		Fail(fmt.Sprintf("failed to create file %s: %q", mb.imagePath, err))
	}
	defer func() {
		if err := dest.Close(); err != nil {
			Fail(fmt.Sprintf("failed to close destination file %q: %q\n", dest.Name(), err))
		}
	}()
	fmt.Printf("--> copying %q to %q\n", src.Name(), dest.Name())
	if runtime.GOOS != "darwin" {
		if _, err := io.Copy(dest, src); err != nil {
			Fail(fmt.Sprintf("failed to copy %ss to %s: %q", fqImageName, mb.imagePath, err))
		}
	} else {
		if err := copySparse(dest, src); err != nil {
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
	if runtime.GOOS == "windows" {
		if err := os.Setenv("USERPROFILE", origHomeDir); err != nil {
			Fail("failed to set windows home dir back to original")
		}
	}
}

// copySparse is a helper method for tests only; caller is responsible for closures
func copySparse(dst io.WriteSeeker, src io.Reader) (retErr error) {
	spWriter := compression.NewSparseWriter(dst)
	defer func() {
		if err := spWriter.Close(); err != nil && retErr == nil {
			retErr = err
		}
	}()
	_, err := io.Copy(spWriter, src)
	return err
}
