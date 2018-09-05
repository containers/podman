package integration

import (
	"fmt"
	"os"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman install/uninstall", func() {
	var (
		tempdir    string
		err        error
		podmanTest PodmanTest
	)

	BeforeEach(func() {
		tempdir, err = CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}
		podmanTest = PodmanCreate(tempdir)
		podmanTest.RestoreAllArtifacts()
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		timedResult := fmt.Sprintf("Test: %s completed in %f seconds", f.TestText, f.Duration.Seconds())
		GinkgoWriter.Write([]byte(timedResult))
	})

	It("podman run install on an image with no label", func() {
		session := podmanTest.Podman([]string{"container", "install", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman run install on an image with a label --display only", func() {
		correct := "Running install command: podman run -it --name foobar -e NAME=foobar -e IMAGE=quay.io/baude/alpine_labels:latest quay.io/baude/alpine_labels:latest echo install"
		podmanTest.RestoreArtifact(labels)
		session := podmanTest.Podman([]string{"container", "install", "--display", "--name", "foobar", labels})
		session.WaitWithDefaultTimeout()
		output := session.OutputToStringArray()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(output[len(output)-2]).To(Equal(correct))
	})

	It("podman run install on an image with a label", func() {
		podmanTest.RestoreArtifact(labels)
		session := podmanTest.Podman([]string{"container", "install", "--name", "foobar", labels})
		session.WaitWithDefaultTimeout()
		output := session.OutputToStringArray()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(strings.TrimSpace(output[len(output)-2])).To(Equal("install"))
	})

	It("podman run uninstall on an image with no label", func() {
		session := podmanTest.Podman([]string{"container", "uninstall", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman run uninstall on an image with a label --display only", func() {
		correct := "Running uninstall command: podman run -it --name foobar -e NAME=foobar -e IMAGE=quay.io/baude/alpine_labels:latest quay.io/baude/alpine_labels:latest echo uninstall"
		podmanTest.RestoreArtifact(labels)
		session := podmanTest.Podman([]string{"container", "uninstall", "--display", "--name", "foobar", labels})
		session.WaitWithDefaultTimeout()
		output := session.OutputToStringArray()
		fmt.Println(output[len(output)-2])
		Expect(session.ExitCode()).To(Equal(0))
		Expect(output[len(output)-2]).To(Equal(correct))
	})

	It("podman run uninstall on an image with a label", func() {
		podmanTest.RestoreArtifact(labels)
		session := podmanTest.Podman([]string{"container", "uninstall", "--name", "barfoo", labels})
		session.WaitWithDefaultTimeout()
		output := session.OutputToStringArray()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(strings.TrimSpace(output[len(output)-3])).To(Equal("uninstall"))
	})
})
