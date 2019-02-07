// +build !remoteclient

package integration

import (
        "fmt"
        "io/ioutil"
        "os"
        "os/exec"
        "path/filepath"

        . "github.com/containers/libpod/test/utils"
        . "github.com/onsi/ginkgo"
        . "github.com/onsi/gomega"
)

var _ = Describe("Podman patch", func() {
    var (
        tempdir    string
        err        error
        podmanTest *PodmanTestIntegration
    )

    BeforeEach(func() {
        tempdir, err = CreateTempDirInTempDir()
        if err != nil {
                os.Exit(1)
        }
        podmanTest = PodmanTestCreate(tempdir)
        podmanTest.RestoreAllArtifacts()
    })

    AfterEach(func() {
        podmanTest.Cleanup()
        f := CurrentGinkgoTestDescription()
        timedResult := fmt.Sprintf("Test: %s completed in %f seconds", f.TestText, f.Duration.Seconds())
        GinkgoWriter.Write([]byte(timedResult))
    })

    It("podman patch file", func() {
        path, err := os.Getwd()
        if err != nil {
                os.Exit(1)
        }
        filePath := filepath.Join(path, "profile.patch")
        contentFile := []byte("27a28,29\n>\n> export BOUM='BOUM'")
        err = ioutil.WriteFile(filePath, contentFile, 0644)
        if err != nil {
                os.Exit(1)
        }

        session := podmanTest.Podman([]string{"create", ALPINE, "cat", "foo"})
        session.WaitWithDefaultTimeout()
        Expect(session.ExitCode()).To(Equal(0))
        name := session.OutputToString()

        session = podmanTest.Podman([]string{"patch", "/etc/profile", filepath.Join(path, "profile.patch"), name})
        session.WaitWithDefaultTimeout()
        Expect(session.ExitCode()).To(Equal(0))

        session = podmanTest.Podman([]string{"start", "-a", name})
        session.WaitWithDefaultTimeout()

        Expect(session.ExitCode()).To(Equal(0))
    })
})
