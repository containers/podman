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
	"github.com/containers/podman/v4/pkg/machine/compression"
	"github.com/containers/podman/v4/pkg/machine/define"
	"github.com/containers/podman/v4/pkg/machine/provider"
	"github.com/containers/podman/v4/utils"
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

var testProvider machine.VirtProvider

var _ = BeforeSuite(func() {
	var err error
	testProvider, err = provider.Get()
	if err != nil {
		Fail("unable to create testProvider")
	}

	downloadLocation := os.Getenv("MACHINE_IMAGE")

	if len(downloadLocation) < 1 {
		downloadLocation = getDownloadLocation(testProvider)
		// we cannot simply use OS here because hyperv uses fcos; so WSL is just
		// special here
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
			diskImage, err := define.NewMachineFile(fqImageName+compressionExtension, nil)
			if err != nil {
				Fail(fmt.Sprintf("unable to create vmfile %q: %v", fqImageName+compressionExtension, err))
			}
			if err := compression.Decompress(diskImage, fqImageName); err != nil {
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
	cConfDir := filepath.Join(homeDir, "containers")
	if err := os.MkdirAll(cConfDir, 0755); err != nil {
		Fail(fmt.Sprintf("failed to create %q: %s", cConfDir, err.Error()))
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

	fmt.Println("Set HOME to:", homeDir)
	if err := os.Setenv("USERPROFILE", homeDir); err != nil {
		Fail("failed to set USERPROFILE dir")
	}

	fmt.Println("Set USERPROFILE to:", homeDir)

	if err := os.Setenv("XDG_RUNTIME_DIR", homeDir); err != nil {
		Fail("failed to set xdg_runtime_dir")
	}

	if err := os.Setenv("XDG_CONFIG_HOME", homeDir); err != nil {
		Fail("failed to set xdg_CONFIG_HOME")
	}

	fmt.Println("Set XDG_CONFIG_HOME to ", homeDir)

	cConf := filepath.Join(cConfDir, "containers.conf")
	cc, err := os.Create(cConf)
	if err != nil {
		Fail("failed to create test container.conf")
	}

	if err := cc.Close(); err != nil {
		Fail(fmt.Sprintf("unable to close file %q: %s", cConf, err.Error()))
	}

	if err := os.Setenv("CONTAINERS_CONF", cConf); err != nil {
		Fail("failed to set CONTAINERS_CONF environment var")
	}

	fmt.Println("set CONTAINERS_CONF to ", cConf)

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
	if err := n.Close(); err != nil {
		Fail(fmt.Sprintf("failed to close image copy handler: %q", err))
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

// dumpDebug is called after each test and can be used to display useful debug information
// about the environment
func dumpDebug(mb *machineTestBuilder, testDir string, sr SpecReport) {
	if !sr.Failed() {
		return
	}
	fmt.Println("///////// DEBUG FOR FAILURE")
	fmt.Println("test dir was: ", testDir)
	debugMachine := basicMachine{}
	// List connections
	_, _ = mb.setCmd(debugMachine.withPodmanCommand([]string{"system", "connection", "ls"})).run()
	// List machines
	_, _ = mb.setCmd(debugMachine.withPodmanCommand([]string{"machine", "ls"})).run()
}
