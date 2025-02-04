//go:build linux || freebsd

package integration

import (
	"encoding/json"
	"fmt"

	"github.com/containers/podman/v5/pkg/libartifact"
	. "github.com/containers/podman/v5/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman artifact", func() {
	BeforeEach(func() {
		SkipIfRemote("artifacts are not supported on the remote client yet due to being in development still")
	})

	It("podman artifact ls", func() {
		artifact1File, err := createArtifactFile(4192)
		Expect(err).ToNot(HaveOccurred())
		artifact1Name := "localhost/test/artifact1"
		add1 := podmanTest.PodmanExitCleanly([]string{"artifact", "add", artifact1Name, artifact1File}...)

		artifact2File, err := createArtifactFile(10240)
		Expect(err).ToNot(HaveOccurred())
		artifact2Name := "localhost/test/artifact2"
		podmanTest.PodmanExitCleanly([]string{"artifact", "add", artifact2Name, artifact2File}...)

		// Should be three items in the list
		listSession := podmanTest.PodmanExitCleanly([]string{"artifact", "ls"}...)
		Expect(listSession.OutputToStringArray()).To(HaveLen(3))

		// --format should work
		listFormatSession := podmanTest.PodmanExitCleanly([]string{"artifact", "ls", "--format", "{{.Repository}}"}...)
		output := listFormatSession.OutputToStringArray()

		// There should be only 2 "lines" because the header should not be output
		Expect(output).To(HaveLen(2))

		// Make sure the names are what we expect
		Expect(output).To(ContainElement(artifact1Name))
		Expect(output).To(ContainElement(artifact2Name))

		// Check default digest length (should be 12)
		defaultFormatSession := podmanTest.PodmanExitCleanly([]string{"artifact", "ls", "--format", "{{.Digest}}"}...)
		defaultOutput := defaultFormatSession.OutputToStringArray()[0]
		Expect(defaultOutput).To(HaveLen(12))

		// Check with --no-trunc and verify the len of the digest is the same as the len what was returned when the artifact
		// was added
		noTruncSession := podmanTest.PodmanExitCleanly([]string{"artifact", "ls", "--no-trunc", "--format", "{{.Digest}}"}...)
		truncOutput := noTruncSession.OutputToStringArray()[0]
		Expect(truncOutput).To(HaveLen(len(add1.OutputToString())))

		// check with --noheading and verify the header is not present through a line count AND substring match
		noHeaderSession := podmanTest.PodmanExitCleanly([]string{"artifact", "ls", "--noheading"}...)
		noHeaderOutput := noHeaderSession.OutputToStringArray()
		Expect(noHeaderOutput).To(HaveLen(2))
		Expect(noHeaderOutput).ToNot(ContainElement("REPOSITORY"))

	})

	It("podman artifact simple add", func() {
		artifact1File, err := createArtifactFile(1024)
		Expect(err).ToNot(HaveOccurred())

		artifact1Name := "localhost/test/artifact1"
		podmanTest.PodmanExitCleanly([]string{"artifact", "add", artifact1Name, artifact1File}...)

		inspectSingleSession := podmanTest.PodmanExitCleanly([]string{"artifact", "inspect", artifact1Name}...)

		a := libartifact.Artifact{}
		inspectOut := inspectSingleSession.OutputToString()
		err = json.Unmarshal([]byte(inspectOut), &a)
		Expect(err).ToNot(HaveOccurred())
		Expect(a.Name).To(Equal(artifact1Name))

		// Adding an artifact with an existing name should fail
		addAgain := podmanTest.Podman([]string{"artifact", "add", artifact1Name, artifact1File})
		addAgain.WaitWithDefaultTimeout()
		Expect(addAgain).ShouldNot(ExitCleanly())
		Expect(addAgain.ErrorToString()).To(Equal(fmt.Sprintf("Error: artifact %s already exists", artifact1Name)))
	})

	It("podman artifact add with options", func() {
		artifact1Name := "localhost/test/artifact1"
		artifact1File, err := createArtifactFile(1024)
		Expect(err).ToNot(HaveOccurred())

		artifactType := "octet/foobar"
		annotation1 := "color=blue"
		annotation2 := "flavor=lemon"

		podmanTest.PodmanExitCleanly([]string{"artifact", "add", "--type", artifactType, "--annotation", annotation1, "--annotation", annotation2, artifact1Name, artifact1File}...)
		inspectSingleSession := podmanTest.PodmanExitCleanly([]string{"artifact", "inspect", artifact1Name}...)
		a := libartifact.Artifact{}
		err = json.Unmarshal([]byte(inspectSingleSession.OutputToString()), &a)
		Expect(err).ToNot(HaveOccurred())
		Expect(a.Name).To(Equal(artifact1Name))
		Expect(a.Manifest.ArtifactType).To(Equal(artifactType))
		Expect(a.Manifest.Layers[0].Annotations["color"]).To(Equal("blue"))
		Expect(a.Manifest.Layers[0].Annotations["flavor"]).To(Equal("lemon"))

		failSession := podmanTest.Podman([]string{"artifact", "add", "--annotation", "org.opencontainers.image.title=foobar", "foobar", artifact1File})
		failSession.WaitWithDefaultTimeout()
		Expect(failSession).Should(Exit(125))
		Expect(failSession.ErrorToString()).Should(Equal("Error: cannot override filename with org.opencontainers.image.title annotation"))
	})

	It("podman artifact add multiple", func() {
		artifact1File1, err := createArtifactFile(1024)
		Expect(err).ToNot(HaveOccurred())
		artifact1File2, err := createArtifactFile(8192)
		Expect(err).ToNot(HaveOccurred())

		artifact1Name := "localhost/test/artifact1"

		podmanTest.PodmanExitCleanly([]string{"artifact", "add", artifact1Name, artifact1File1, artifact1File2}...)

		inspectSingleSession := podmanTest.PodmanExitCleanly([]string{"artifact", "inspect", artifact1Name}...)

		a := libartifact.Artifact{}
		inspectOut := inspectSingleSession.OutputToString()
		err = json.Unmarshal([]byte(inspectOut), &a)
		Expect(err).ToNot(HaveOccurred())
		Expect(a.Name).To(Equal(artifact1Name))

		Expect(a.Manifest.Layers).To(HaveLen(2))
	})

	It("podman artifact push and pull", func() {
		artifact1File, err := createArtifactFile(1024)
		Expect(err).ToNot(HaveOccurred())

		lock, port, err := setupRegistry(nil)
		if err == nil {
			defer lock.Unlock()
		}
		Expect(err).ToNot(HaveOccurred())

		artifact1Name := fmt.Sprintf("localhost:%s/test/artifact1", port)
		podmanTest.PodmanExitCleanly([]string{"artifact", "add", artifact1Name, artifact1File}...)

		podmanTest.PodmanExitCleanly([]string{"artifact", "push", "-q", "--tls-verify=false", artifact1Name}...)

		podmanTest.PodmanExitCleanly([]string{"artifact", "rm", artifact1Name}...)

		podmanTest.PodmanExitCleanly([]string{"artifact", "pull", "--tls-verify=false", artifact1Name}...)

		inspectSingleSession := podmanTest.PodmanExitCleanly([]string{"artifact", "inspect", artifact1Name}...)

		a := libartifact.Artifact{}
		inspectOut := inspectSingleSession.OutputToString()
		err = json.Unmarshal([]byte(inspectOut), &a)
		Expect(err).ToNot(HaveOccurred())
		Expect(a.Name).To(Equal(artifact1Name))
	})

	It("podman artifact remove", func() {
		// Trying to remove an image that does not exist should fail
		rmFail := podmanTest.Podman([]string{"artifact", "rm", "foobar"})
		rmFail.WaitWithDefaultTimeout()
		Expect(rmFail).Should(Exit(125))
		Expect(rmFail.ErrorToString()).Should(Equal(fmt.Sprintf("Error: no artifact found with name or digest of %s", "foobar")))

		// Add an artifact to remove later
		artifact1File, err := createArtifactFile(4192)
		Expect(err).ToNot(HaveOccurred())
		artifact1Name := "localhost/test/artifact1"
		addArtifact1 := podmanTest.PodmanExitCleanly([]string{"artifact", "add", artifact1Name, artifact1File}...)

		// Removing that artifact should work
		rmWorks := podmanTest.PodmanExitCleanly([]string{"artifact", "rm", artifact1Name}...)
		// The digests printed by removal should be the same as the digest that was added
		Expect(addArtifact1.OutputToString()).To(Equal(rmWorks.OutputToString()))

		// Inspecting that the removed artifact should fail
		inspectArtifact := podmanTest.Podman([]string{"artifact", "inspect", artifact1Name})
		inspectArtifact.WaitWithDefaultTimeout()
		Expect(inspectArtifact).Should(Exit(125))
		Expect(inspectArtifact.ErrorToString()).To(Equal(fmt.Sprintf("Error: no artifact found with name or digest of %s", artifact1Name)))
	})

	It("podman artifact inspect with full or partial digest", func() {
		artifact1File, err := createArtifactFile(4192)
		Expect(err).ToNot(HaveOccurred())
		artifact1Name := "localhost/test/artifact1"
		addArtifact1 := podmanTest.PodmanExitCleanly([]string{"artifact", "add", artifact1Name, artifact1File}...)

		artifactDigest := addArtifact1.OutputToString()

		podmanTest.PodmanExitCleanly([]string{"artifact", "inspect", artifactDigest}...)
		podmanTest.PodmanExitCleanly([]string{"artifact", "inspect", artifactDigest[:12]}...)

	})
})
