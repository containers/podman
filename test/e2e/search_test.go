//go:build linux || freebsd

package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"text/template"

	"github.com/containers/podman/v5/pkg/domain/entities"
	. "github.com/containers/podman/v5/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type endpoint struct {
	Host string
	Port string
}

func (e *endpoint) Address() string {
	return fmt.Sprintf("%s:%s", e.Host, e.Port)
}

var _ = Describe("Podman search", func() {

	const regFileContents = `
[registries.search]
registries = ['{{.Host}}:{{.Port}}']

[registries.insecure]
registries = ['{{.Host}}:{{.Port}}']`
	registryFileTmpl := template.Must(template.New("registryFile").Parse(regFileContents))

	const badRegFileContents = `
[registries.search]
registries = ['{{.Host}}:{{.Port}}']
# empty
[registries.insecure]
registries = []`
	registryFileBadTmpl := template.Must(template.New("registryFileBad").Parse(badRegFileContents))

	It("podman search", func() {
		search := podmanTest.Podman([]string{"search", "alpine"})
		search.WaitWithDefaultTimeout()
		Expect(search).Should(ExitCleanly())
		Expect(len(search.OutputToStringArray())).To(BeNumerically(">", 1))
		Expect(search.OutputToString()).To(ContainSubstring("alpine"))
	})

	It("podman search single registry flag", func() {
		search := podmanTest.Podman([]string{"search", "quay.io/skopeo/stable:latest"})
		search.WaitWithDefaultTimeout()
		Expect(search).Should(ExitCleanly())
		Expect(search.OutputToString()).To(ContainSubstring("quay.io/skopeo/stable"))
	})

	It("podman search image with description", func() {
		search := podmanTest.Podman([]string{"search", "quay.io/podman/stable"})
		search.WaitWithDefaultTimeout()
		Expect(search).Should(ExitCleanly())
		output := string(search.Out.Contents())
		Expect(output).To(MatchRegexp(`(?m)NAME\s+DESCRIPTION$`))
		Expect(output).To(MatchRegexp(`(?m)quay.io/podman/stable\s+.*PODMAN logo`))
	})

	It("podman search image with --compatible", func() {
		search := podmanTest.Podman([]string{"search", "--compatible", "quay.io/podman/stable"})
		search.WaitWithDefaultTimeout()
		Expect(search).Should(ExitCleanly())
		output := string(search.Out.Contents())
		Expect(output).To(MatchRegexp(`(?m)NAME\s+DESCRIPTION\s+STARS\s+OFFICIAL\s+AUTOMATED$`))
	})

	It("podman search format flag", func() {
		search := podmanTest.Podman([]string{"search", "--format", "table {{.Index}} {{.Name}}", "testdigest_v2s2"})
		search.WaitWithDefaultTimeout()
		Expect(search).Should(ExitCleanly())
		Expect(len(search.OutputToStringArray())).To(BeNumerically(">", 1))
		Expect(search.OutputToString()).To(ContainSubstring("quay.io/libpod/testdigest_v2s2"))
	})

	It("podman search format json", func() {
		search := podmanTest.Podman([]string{"search", "--format", "json", "testdigest_v2s1"})
		search.WaitWithDefaultTimeout()
		Expect(search).Should(ExitCleanly())
		Expect(search.OutputToString()).To(BeValidJSON())
		Expect(search.OutputToString()).To(ContainSubstring("quay.io/libpod/testdigest_v2s1"))
		Expect(search.OutputToString()).To(ContainSubstring("Test image used by buildah regression tests"))

		// Test for https://github.com/containers/podman/issues/11894
		contents := make([]entities.ImageSearchReport, 0)
		err := json.Unmarshal(search.Out.Contents(), &contents)
		Expect(err).ToNot(HaveOccurred())
		Expect(contents).ToNot(BeEmpty(), "No results from image search")
		for _, element := range contents {
			Expect(element.Description).ToNot(HaveSuffix("..."))
		}
	})

	It("podman search format json list tags", func() {
		search := podmanTest.Podman([]string{"search", "--list-tags", "--format", "json", ALPINE})
		search.WaitWithDefaultTimeout()
		Expect(search).Should(ExitCleanly())
		Expect(search.OutputToString()).To(BeValidJSON())
		Expect(search.OutputToString()).To(ContainSubstring("quay.io/libpod/alpine"))
		Expect(search.OutputToString()).To(ContainSubstring("3.10.2"))
		Expect(search.OutputToString()).To(ContainSubstring("3.2"))
	})

	// Test for https://github.com/containers/podman/issues/11894
	It("podman search no-trunc=false flag", func() {
		search := podmanTest.Podman([]string{"search", "--no-trunc=false", "alpine", "--format={{.Description}}"})
		search.WaitWithDefaultTimeout()
		Expect(search).Should(ExitCleanly())

		for _, line := range search.OutputToStringArray() {
			if len(line) > 44 {
				Expect(line).To(HaveSuffix("..."), line+" should have been truncated")
			}
		}
	})

	It("podman search limit flag", func() {
		search := podmanTest.Podman([]string{"search", "quay.io/alpine"})
		search.WaitWithDefaultTimeout()
		Expect(search).Should(ExitCleanly())
		Expect(len(search.OutputToStringArray())).To(BeNumerically(">", 10))

		search = podmanTest.Podman([]string{"search", "--limit", "3", "quay.io/alpine"})
		search.WaitWithDefaultTimeout()
		Expect(search).Should(ExitCleanly())
		Expect(search.OutputToStringArray()).To(HaveLen(4))

		search = podmanTest.Podman([]string{"search", "--limit", "30", "quay.io/alpine"})
		search.WaitWithDefaultTimeout()
		Expect(search).Should(ExitCleanly())
		Expect(search.OutputToStringArray()).To(HaveLen(31))
	})

	It("podman search with filter stars", func() {
		search := podmanTest.Podman([]string{"search", "--filter", "stars=10", "--format", "{{.Stars}}", "alpine"})
		search.WaitWithDefaultTimeout()
		Expect(search).Should(ExitCleanly())
		output := search.OutputToStringArray()
		for i := 0; i < len(output); i++ {
			Expect(strconv.Atoi(output[i])).To(BeNumerically(">=", 10))
		}
	})

	It("podman search with filter is-official", func() {
		search := podmanTest.Podman([]string{"search", "--filter", "is-official", "--format", "{{.Official}}", "alpine"})
		search.WaitWithDefaultTimeout()
		Expect(search).Should(ExitCleanly())
		output := search.OutputToStringArray()
		for i := 0; i < len(output); i++ {
			Expect(output[i]).To(Equal("[OK]"))
		}
	})

	It("podman search with filter is-automated", func() {
		search := podmanTest.Podman([]string{"search", "--filter", "is-automated=false", "--format", "{{.Automated}}", "alpine"})
		search.WaitWithDefaultTimeout()
		Expect(search).Should(ExitCleanly())
		output := search.OutputToStringArray()
		for i := 0; i < len(output); i++ {
			Expect(output[i]).To(Equal(""))
		}
	})

	It("podman search format list tags with custom", func() {
		search := podmanTest.Podman([]string{"search", "--list-tags", "--format", "{{.Name}}", "--limit", "1", ALPINE})
		search.WaitWithDefaultTimeout()
		Expect(search).Should(ExitCleanly())
		Expect(search.OutputToString()).To(Equal("quay.io/libpod/alpine"))
	})

	It("podman search attempts HTTP if tls-verify flag is set false", func() {
		if podmanTest.Host.Arch == "ppc64le" {
			Skip("No registry image for ppc64le")
		}
		port := GetPort()
		fakereg := podmanTest.Podman([]string{"run", "-d", "--name", "registry",
			"-p", fmt.Sprintf("%d:5000", port),
			REGISTRY_IMAGE, "/entrypoint.sh", "/etc/docker/registry/config.yml"})
		fakereg.WaitWithDefaultTimeout()
		Expect(fakereg).Should(ExitCleanly())

		if !WaitContainerReady(podmanTest, "registry", "listening on", 20, 1) {
			Fail("Cannot start docker registry on port %s", port)
		}
		ep := endpoint{Port: strconv.Itoa(port), Host: "localhost"}
		search := podmanTest.Podman([]string{"search",
			fmt.Sprintf("%s/fake/image:andtag", ep.Address()), "--tls-verify=false"})
		search.WaitWithDefaultTimeout()

		// if this test succeeded, there will be no output (there is no entry named fake/image:andtag in an empty registry)
		// and the exit code will be 0
		Expect(search).Should(ExitCleanly())
		Expect(search.OutputToString()).Should(BeEmpty())
	})

	It("podman search in local registry", func() {
		if podmanTest.Host.Arch == "ppc64le" {
			Skip("No registry image for ppc64le")
		}
		port := GetPort()
		registry := podmanTest.Podman([]string{"run", "-d", "--name", "registry3",
			"-p", fmt.Sprintf("%d:5000", port), REGISTRY_IMAGE,
			"/entrypoint.sh", "/etc/docker/registry/config.yml"})
		registry.WaitWithDefaultTimeout()
		Expect(registry).Should(ExitCleanly())

		if !WaitContainerReady(podmanTest, "registry3", "listening on", 20, 1) {
			Fail("Cannot start docker registry on port %s", port)
		}
		ep := endpoint{Port: strconv.Itoa(port), Host: "localhost"}
		err = podmanTest.RestoreArtifact(ALPINE)
		Expect(err).ToNot(HaveOccurred())
		image := fmt.Sprintf("%s/my-alpine", ep.Address())
		push := podmanTest.Podman([]string{"push", "-q", "--tls-verify=false", "--remove-signatures", ALPINE, image})
		push.WaitWithDefaultTimeout()
		Expect(push).Should(ExitCleanly())
		search := podmanTest.Podman([]string{"search", image, "--tls-verify=false"})
		search.WaitWithDefaultTimeout()

		Expect(search).Should(ExitCleanly())
		Expect(search.OutputToString()).ShouldNot(BeEmpty())

		// podman search v2 registry with empty query
		searchEmpty := podmanTest.Podman([]string{"search", fmt.Sprintf("%s/", ep.Address()), "--tls-verify=false"})
		searchEmpty.WaitWithDefaultTimeout()
		Expect(searchEmpty).Should(ExitCleanly())
		Expect(searchEmpty.OutputToStringArray()).ToNot(BeEmpty())
		Expect(search.OutputToString()).To(ContainSubstring("my-alpine"))
	})

	It("podman search attempts HTTP if registry is in registries.insecure and force secure is false", func() {
		if podmanTest.Host.Arch == "ppc64le" {
			Skip("No registry image for ppc64le")
		}

		port := GetPort()
		ep := endpoint{Port: strconv.Itoa(port), Host: "localhost"}
		registry := podmanTest.Podman([]string{"run", "-d", "-p", fmt.Sprintf("%d:5000", port),
			"--name", "registry4", REGISTRY_IMAGE, "/entrypoint.sh", "/etc/docker/registry/config.yml"})
		registry.WaitWithDefaultTimeout()
		Expect(registry).Should(ExitCleanly())

		if !WaitContainerReady(podmanTest, "registry4", "listening on", 20, 1) {
			Fail("unable to start registry on port %s", port)
		}

		err = podmanTest.RestoreArtifact(ALPINE)
		Expect(err).ToNot(HaveOccurred())
		image := fmt.Sprintf("%s/my-alpine", ep.Address())
		push := podmanTest.Podman([]string{"push", "-q", "--tls-verify=false", "--remove-signatures", ALPINE, image})
		push.WaitWithDefaultTimeout()
		Expect(push).Should(ExitCleanly())

		// registries.conf set up
		var buffer bytes.Buffer
		err = registryFileTmpl.Execute(&buffer, ep)
		Expect(err).ToNot(HaveOccurred())
		podmanTest.setRegistriesConfigEnv(buffer.Bytes())
		err = os.WriteFile(fmt.Sprintf("%s/registry4.conf", tempdir), buffer.Bytes(), 0644)
		Expect(err).ToNot(HaveOccurred())
		if IsRemote() {
			podmanTest.RestartRemoteService()
			defer podmanTest.RestartRemoteService()
		}

		search := podmanTest.Podman([]string{"search", image})
		search.WaitWithDefaultTimeout()

		Expect(search).Should(ExitCleanly())
		Expect(search.OutputToString()).To(ContainSubstring("my-alpine"))

		// cleanup
		resetRegistriesConfigEnv()
	})

	It("podman search doesn't attempt HTTP if force secure is true", func() {
		if podmanTest.Host.Arch == "ppc64le" {
			Skip("No registry image for ppc64le")
		}
		port := GetPort()
		ep := endpoint{Port: strconv.Itoa(port), Host: "localhost"}
		registry := podmanTest.Podman([]string{"run", "-d", "-p", fmt.Sprintf("%d:5000", port),
			"--name", "registry5", REGISTRY_IMAGE})
		registry.WaitWithDefaultTimeout()
		Expect(registry).Should(ExitCleanly())

		if !WaitContainerReady(podmanTest, "registry5", "listening on", 20, 1) {
			Fail("Cannot start docker registry on port %s", port)
		}

		err = podmanTest.RestoreArtifact(ALPINE)
		Expect(err).ToNot(HaveOccurred())
		image := fmt.Sprintf("%s/my-alpine", ep.Address())
		push := podmanTest.Podman([]string{"push", "-q", "--tls-verify=false", "--remove-signatures", ALPINE, image})
		push.WaitWithDefaultTimeout()
		Expect(push).Should(ExitCleanly())

		var buffer bytes.Buffer
		err = registryFileTmpl.Execute(&buffer, ep)
		Expect(err).ToNot(HaveOccurred())
		podmanTest.setRegistriesConfigEnv(buffer.Bytes())
		err = os.WriteFile(fmt.Sprintf("%s/registry5.conf", tempdir), buffer.Bytes(), 0644)
		Expect(err).ToNot(HaveOccurred())

		search := podmanTest.Podman([]string{"search", image, "--tls-verify=true"})
		search.WaitWithDefaultTimeout()

		Expect(search).Should(ExitWithError(125, fmt.Sprintf(`couldn't search registry "localhost:%d": pinging container registry localhost:%d: Get "https://localhost:%d/v2/": http: server gave HTTP response to HTTPS client`, port, port, port)))
		Expect(search.OutputToString()).Should(BeEmpty())

		// cleanup
		resetRegistriesConfigEnv()
	})

	It("podman search doesn't attempt HTTP if registry is not listed as insecure", func() {
		if podmanTest.Host.Arch == "ppc64le" {
			Skip("No registry image for ppc64le")
		}
		port := GetPort()
		ep := endpoint{Port: strconv.Itoa(port), Host: "localhost"}
		registry := podmanTest.Podman([]string{"run", "-d", "-p", fmt.Sprintf("%d:5000", port),
			"--name", "registry6", REGISTRY_IMAGE})
		registry.WaitWithDefaultTimeout()
		Expect(registry).Should(ExitCleanly())

		if !WaitContainerReady(podmanTest, "registry6", "listening on", 20, 1) {
			Fail("Cannot start docker registry on port %s", port)
		}

		err = podmanTest.RestoreArtifact(ALPINE)
		Expect(err).ToNot(HaveOccurred())
		image := fmt.Sprintf("%s/my-alpine", ep.Address())
		push := podmanTest.Podman([]string{"push", "-q", "--tls-verify=false", "--remove-signatures", ALPINE, image})
		push.WaitWithDefaultTimeout()
		Expect(push).Should(ExitCleanly())

		var buffer bytes.Buffer
		err = registryFileBadTmpl.Execute(&buffer, ep)
		Expect(err).ToNot(HaveOccurred())
		podmanTest.setRegistriesConfigEnv(buffer.Bytes())
		err = os.WriteFile(fmt.Sprintf("%s/registry6.conf", tempdir), buffer.Bytes(), 0644)
		Expect(err).ToNot(HaveOccurred())

		if IsRemote() {
			podmanTest.RestartRemoteService()
			defer podmanTest.RestartRemoteService()
		}

		search := podmanTest.Podman([]string{"search", image})
		search.WaitWithDefaultTimeout()

		Expect(search).Should(ExitWithError(125, fmt.Sprintf(`couldn't search registry "localhost:%d": pinging container registry localhost:%d: Get "https://localhost:%d/v2/": http: server gave HTTP response to HTTPS client`, port, port, port)))
		Expect(search.OutputToString()).Should(BeEmpty())

		// cleanup
		resetRegistriesConfigEnv()
	})

	// search should fail with nonexistent authfile
	It("podman search fail with nonexistent --authfile", func() {
		search := podmanTest.Podman([]string{"search", "--authfile", "/tmp/nonexistent", ALPINE})
		search.WaitWithDefaultTimeout()
		Expect(search).To(ExitWithError(125, "credential file is not accessible: faccessat /tmp/nonexistent: no such file or directory"))
	})

	// Registry is unreliable (#18484), this is another super-common flake
	It("podman search with wildcards", FlakeAttempts(3), func() {
		search := podmanTest.Podman([]string{"search", "registry.access.redhat.com/*openshift*"})
		search.WaitWithDefaultTimeout()
		Expect(search).Should(ExitCleanly())
		Expect(len(search.OutputToStringArray())).To(BeNumerically(">", 1))
	})

	It("podman search repository tags", func() {
		search := podmanTest.Podman([]string{"search", "--list-tags", "--limit", "30", "quay.io/podman/stable"})
		search.WaitWithDefaultTimeout()
		Expect(search).Should(ExitCleanly())
		Expect(search.OutputToStringArray()).To(HaveLen(31))

		search = podmanTest.Podman([]string{"search", "--list-tags", "quay.io/podman/stable"})
		search.WaitWithDefaultTimeout()
		Expect(search).Should(ExitCleanly())
		Expect(len(search.OutputToStringArray())).To(BeNumerically(">", 2))

		search = podmanTest.Podman([]string{"search", "--filter=is-official", "--list-tags", "quay.io/podman/stable"})
		search.WaitWithDefaultTimeout()
		Expect(search).To(ExitWithError(125, "filters are not applicable to list tags result"))

		// With trailing slash
		search = podmanTest.Podman([]string{"search", "--list-tags", "quay.io/podman/"})
		search.WaitWithDefaultTimeout()
		Expect(search).To(ExitWithError(125, `reference "podman/" must be a docker reference`))
		Expect(search.OutputToStringArray()).To(BeEmpty())

		// No trailing slash
		search = podmanTest.Podman([]string{"search", "--list-tags", "quay.io/podman"})
		search.WaitWithDefaultTimeout()
		Expect(search).To(ExitWithError(125, "getting repository tags: fetching tags list: StatusCode: 404"))
		Expect(search.OutputToStringArray()).To(BeEmpty())
	})

	It("podman search with limit over 100", func() {
		search := podmanTest.Podman([]string{"search", "--limit", "100", "quay.io/podman"})
		search.WaitWithDefaultTimeout()
		Expect(search).Should(ExitCleanly())
		Expect(len(search.OutputToStringArray())).To(BeNumerically("<=", 101))
	})
})
