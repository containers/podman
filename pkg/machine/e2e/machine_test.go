package e2e

import (
	"fmt"
	"io"
	"io/ioutil"
	url2 "net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/containers/podman/v4/pkg/machine"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

const (
	defaultStream string = "podman-testing"
)

var (
	tmpDir         = "/var/tmp"
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
	fcd, err := machine.GetFCOSDownload(defaultStream)
	if err != nil {
		Fail("unable to get virtual machine image")
	}
	suiteImageName = strings.TrimSuffix(path.Base(fcd.Location), ".xz")
	fqImageName = filepath.Join(tmpDir, suiteImageName)
	if _, err := os.Stat(fqImageName); err != nil {
		if os.IsNotExist(err) {
			getMe, err := url2.Parse(fcd.Location)
			if err != nil {
				Fail(fmt.Sprintf("unable to create url for download: %q", err))
			}
			now := time.Now()
			if err := machine.DownloadVMImage(getMe, fqImageName+".xz"); err != nil {
				Fail(fmt.Sprintf("unable to download machine image: %q", err))
			}
			fmt.Println("Download took: ", time.Since(now).String())
			if err := machine.Decompress(fqImageName+".xz", fqImageName); err != nil {
				Fail(fmt.Sprintf("unable to decompress image file: %q", err))
			}
		} else {
			Fail(fmt.Sprintf("unable to check for cache image: %q", err))
		}
	}
})

var _ = SynchronizedAfterSuite(func() {},
	func() {
		fmt.Println("After")
	})

func setup() (string, *machineTestBuilder) {
	// Set TMPDIR if this needs a new directory
	homeDir, err := ioutil.TempDir("", "podman_test")
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
	s := new(stopMachine)
	for _, name := range mb.names {
		if _, err := mb.setName(name).setCmd(s).run(); err != nil {
			fmt.Printf("error occurred rm'ing machine: %q\n", err)
		}
	}
	if err := os.RemoveAll(testDir); err != nil {
		Fail(fmt.Sprintf("failed to remove test dir: %q", err))
	}
	// this needs to be last in teardown
	if err := os.Setenv("HOME", origHomeDir); err != nil {
		Fail("failed to set home dir")
	}
}
