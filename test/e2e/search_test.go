package integration

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"text/template"

	. "github.com/containers/podman/v3/test/utils"
	. "github.com/onsi/ginkgo"
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
	var (
		tempdir    string
		err        error
		podmanTest *PodmanTestIntegration
	)

	var registryEndpoints = []endpoint{
		{"localhost", "5001"},
		{"localhost", "5002"},
		{"localhost", "5003"},
		{"localhost", "5004"},
		{"localhost", "5005"},
		{"localhost", "5006"},
		{"localhost", "5007"},
		{"localhost", "5008"},
		{"localhost", "5009"},
	}

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

	const regFileContents2 = `
[registries.search]
registries = ['{{.Host}}:{{.Port}}', '{{.Host}}:6000']

[registries.insecure]
registries = ['{{.Host}}:{{.Port}}']`
	registryFileTwoTmpl := template.Must(template.New("registryFileTwo").Parse(regFileContents2))

	BeforeEach(func() {
		tempdir, err = CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}

		podmanTest = PodmanTestCreate(tempdir)
		podmanTest.Setup()
		podmanTest.SeedImages()

	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)
	})

	It("podman search", func() {
		search := podmanTest.Podman([]string{"search", "alpine"})
		search.WaitWithDefaultTimeout()
		Expect(search.ExitCode()).To(Equal(0))
		Expect(len(search.OutputToStringArray())).To(BeNumerically(">", 1))
		Expect(search.LineInOutputContains("docker.io/library/alpine")).To(BeTrue())
	})

	It("podman search single registry flag", func() {
		search := podmanTest.Podman([]string{"search", "quay.io/skopeo/stable:latest"})
		search.WaitWithDefaultTimeout()
		Expect(search.ExitCode()).To(Equal(0))
		Expect(search.LineInOutputContains("quay.io/skopeo/stable")).To(BeTrue())
	})

	It("podman search image with description", func() {
		search := podmanTest.Podman([]string{"search", "quay.io/podman/stable"})
		search.WaitWithDefaultTimeout()
		Expect(search.ExitCode()).To(Equal(0))
		output := string(search.Out.Contents())
		match, _ := regexp.MatchString(`(?m)quay.io/podman/stable\s+.*PODMAN logo`, output)
		Expect(match).To(BeTrue())
	})

	It("podman search format flag", func() {
		search := podmanTest.Podman([]string{"search", "--format", "table {{.Index}} {{.Name}}", "alpine"})
		search.WaitWithDefaultTimeout()
		Expect(search.ExitCode()).To(Equal(0))
		Expect(len(search.OutputToStringArray())).To(BeNumerically(">", 1))
		Expect(search.LineInOutputContains("docker.io/library/alpine")).To(BeTrue())
	})

	It("podman search format json", func() {
		search := podmanTest.Podman([]string{"search", "--format", "json", "alpine"})
		search.WaitWithDefaultTimeout()
		Expect(search.ExitCode()).To(Equal(0))
		Expect(search.IsJSONOutputValid()).To(BeTrue())
		Expect(search.OutputToString()).To(ContainSubstring("docker.io/library/alpine"))
	})

	It("podman search format json list tags", func() {
		search := podmanTest.Podman([]string{"search", "--list-tags", "--format", "json", ALPINE})
		search.WaitWithDefaultTimeout()
		Expect(search.ExitCode()).To(Equal(0))
		Expect(search.IsJSONOutputValid()).To(BeTrue())
		Expect(search.OutputToString()).To(ContainSubstring("quay.io/libpod/alpine"))
		Expect(search.OutputToString()).To(ContainSubstring("3.10.2"))
		Expect(search.OutputToString()).To(ContainSubstring("3.2"))
	})

	It("podman search no-trunc flag", func() {
		Skip("Problematic test.  Skipping for long-term stability on release branch.")
		search := podmanTest.Podman([]string{"search", "--no-trunc", "alpine"})
		search.WaitWithDefaultTimeout()
		Expect(search.ExitCode()).To(Equal(0))
		Expect(len(search.OutputToStringArray())).To(BeNumerically(">", 1))
		Expect(search.LineInOutputContains("docker.io/library/alpine")).To(BeTrue())
		Expect(search.LineInOutputContains("...")).To(BeFalse())
	})

	It("podman search limit flag", func() {
		search := podmanTest.Podman([]string{"search", "docker.io/alpine"})
		search.WaitWithDefaultTimeout()
		Expect(search.ExitCode()).To(Equal(0))
		Expect(len(search.OutputToStringArray())).To(Equal(26))

		search = podmanTest.Podman([]string{"search", "--limit", "3", "docker.io/alpine"})
		search.WaitWithDefaultTimeout()
		Expect(search.ExitCode()).To(Equal(0))
		Expect(len(search.OutputToStringArray())).To(Equal(4))

		search = podmanTest.Podman([]string{"search", "--limit", "30", "docker.io/alpine"})
		search.WaitWithDefaultTimeout()
		Expect(search.ExitCode()).To(Equal(0))
		Expect(len(search.OutputToStringArray())).To(Equal(31))
	})

	It("podman search with filter stars", func() {
		search := podmanTest.Podman([]string{"search", "--filter", "stars=10", "--format", "{{.Stars}}", "alpine"})
		search.WaitWithDefaultTimeout()
		Expect(search.ExitCode()).To(Equal(0))
		output := search.OutputToStringArray()
		for i := 0; i < len(output); i++ {
			Expect(strconv.Atoi(output[i])).To(BeNumerically(">=", 10))
		}
	})

	It("podman search with filter is-official", func() {
		search := podmanTest.Podman([]string{"search", "--filter", "is-official", "--format", "{{.Official}}", "alpine"})
		search.WaitWithDefaultTimeout()
		Expect(search.ExitCode()).To(Equal(0))
		output := search.OutputToStringArray()
		for i := 0; i < len(output); i++ {
			Expect(output[i]).To(Equal("[OK]"))
		}
	})

	It("podman search with filter is-automated", func() {
		search := podmanTest.Podman([]string{"search", "--filter", "is-automated=false", "--format", "{{.Automated}}", "alpine"})
		search.WaitWithDefaultTimeout()
		Expect(search.ExitCode()).To(Equal(0))
		output := search.OutputToStringArray()
		for i := 0; i < len(output); i++ {
			Expect(output[i]).To(Equal(""))
		}
	})

	It("podman search attempts HTTP if tls-verify flag is set false", func() {
		if podmanTest.Host.Arch == "ppc64le" {
			Skip("No registry image for ppc64le")
		}
		lock := GetPortLock(registryEndpoints[0].Port)
		defer lock.Unlock()

		fakereg := podmanTest.Podman([]string{"run", "-d", "--name", "registry",
			"-p", fmt.Sprintf("%s:5000", registryEndpoints[0].Port),
			registry, "/entrypoint.sh", "/etc/docker/registry/config.yml"})
		fakereg.WaitWithDefaultTimeout()
		Expect(fakereg.ExitCode()).To(Equal(0))

		if !WaitContainerReady(podmanTest, "registry", "listening on", 20, 1) {
			Skip("Cannot start docker registry.")
		}

		search := podmanTest.Podman([]string{"search",
			fmt.Sprintf("%s/fake/image:andtag", registryEndpoints[0].Address()), "--tls-verify=false"})
		search.WaitWithDefaultTimeout()

		// if this test succeeded, there will be no output (there is no entry named fake/image:andtag in an empty registry)
		// and the exit code will be 0
		Expect(search.ExitCode()).To(Equal(0))
		Expect(search.OutputToString()).Should(BeEmpty())
		Expect(search.ErrorToString()).Should(BeEmpty())
	})

	It("podman search in local registry", func() {
		if podmanTest.Host.Arch == "ppc64le" {
			Skip("No registry image for ppc64le")
		}
		lock := GetPortLock(registryEndpoints[3].Port)
		defer lock.Unlock()
		registry := podmanTest.Podman([]string{"run", "-d", "--name", "registry3",
			"-p", fmt.Sprintf("%s:5000", registryEndpoints[3].Port), registry,
			"/entrypoint.sh", "/etc/docker/registry/config.yml"})
		registry.WaitWithDefaultTimeout()
		Expect(registry.ExitCode()).To(Equal(0))

		if !WaitContainerReady(podmanTest, "registry3", "listening on", 20, 1) {
			Skip("Cannot start docker registry.")
		}

		podmanTest.RestoreArtifact(ALPINE)
		image := fmt.Sprintf("%s/my-alpine", registryEndpoints[3].Address())
		push := podmanTest.Podman([]string{"push", "--tls-verify=false", "--remove-signatures", ALPINE, image})
		push.WaitWithDefaultTimeout()
		Expect(push.ExitCode()).To(Equal(0))
		search := podmanTest.Podman([]string{"search", image, "--tls-verify=false"})
		search.WaitWithDefaultTimeout()

		Expect(search.ExitCode()).To(Equal(0))
		Expect(search.OutputToString()).ShouldNot(BeEmpty())

		// podman search v2 registry with empty query
		searchEmpty := podmanTest.Podman([]string{"search", fmt.Sprintf("%s/", registryEndpoints[3].Address()), "--tls-verify=false"})
		searchEmpty.WaitWithDefaultTimeout()
		Expect(searchEmpty.ExitCode()).To(BeZero())
		Expect(len(searchEmpty.OutputToStringArray())).To(BeNumerically(">=", 1))
		match, _ := search.GrepString("my-alpine")
		Expect(match).Should(BeTrue())
	})

	It("podman search attempts HTTP if registry is in registries.insecure and force secure is false", func() {
		if podmanTest.Host.Arch == "ppc64le" {
			Skip("No registry image for ppc64le")
		}

		lock := GetPortLock(registryEndpoints[4].Port)
		defer lock.Unlock()
		registry := podmanTest.Podman([]string{"run", "-d", "-p", fmt.Sprintf("%s:5000", registryEndpoints[4].Port),
			"--name", "registry4", registry, "/entrypoint.sh", "/etc/docker/registry/config.yml"})
		registry.WaitWithDefaultTimeout()
		Expect(registry.ExitCode()).To(Equal(0))

		if !WaitContainerReady(podmanTest, "registry4", "listening on", 20, 1) {
			Skip("Cannot start docker registry.")
		}

		podmanTest.RestoreArtifact(ALPINE)
		image := fmt.Sprintf("%s/my-alpine", registryEndpoints[4].Address())
		push := podmanTest.Podman([]string{"push", "--tls-verify=false", "--remove-signatures", ALPINE, image})
		push.WaitWithDefaultTimeout()
		Expect(push.ExitCode()).To(Equal(0))

		// registries.conf set up
		var buffer bytes.Buffer
		registryFileTmpl.Execute(&buffer, registryEndpoints[4])
		podmanTest.setRegistriesConfigEnv(buffer.Bytes())
		ioutil.WriteFile(fmt.Sprintf("%s/registry4.conf", tempdir), buffer.Bytes(), 0644)
		if IsRemote() {
			podmanTest.RestartRemoteService()
			defer podmanTest.RestartRemoteService()
		}

		search := podmanTest.Podman([]string{"search", image})
		search.WaitWithDefaultTimeout()

		Expect(search.ExitCode()).To(Equal(0))
		match, _ := search.GrepString("my-alpine")
		Expect(match).Should(BeTrue())
		Expect(search.ErrorToString()).Should(BeEmpty())

		// cleanup
		resetRegistriesConfigEnv()
	})

	It("podman search doesn't attempt HTTP if force secure is true", func() {
		SkipIfRemote("FIXME This should work on podman-remote")
		if podmanTest.Host.Arch == "ppc64le" {
			Skip("No registry image for ppc64le")
		}
		lock := GetPortLock(registryEndpoints[5].Port)
		defer lock.Unlock()
		registry := podmanTest.Podman([]string{"run", "-d", "-p", fmt.Sprintf("%s:5000", registryEndpoints[5].Port),
			"--name", "registry5", registry})
		registry.WaitWithDefaultTimeout()
		Expect(registry.ExitCode()).To(Equal(0))

		if !WaitContainerReady(podmanTest, "registry5", "listening on", 20, 1) {
			Skip("Cannot start docker registry.")
		}

		podmanTest.RestoreArtifact(ALPINE)
		image := fmt.Sprintf("%s/my-alpine", registryEndpoints[5].Address())
		push := podmanTest.Podman([]string{"push", "--tls-verify=false", "--remove-signatures", ALPINE, image})
		push.WaitWithDefaultTimeout()
		Expect(push.ExitCode()).To(Equal(0))

		var buffer bytes.Buffer
		registryFileTmpl.Execute(&buffer, registryEndpoints[5])
		podmanTest.setRegistriesConfigEnv(buffer.Bytes())
		ioutil.WriteFile(fmt.Sprintf("%s/registry5.conf", tempdir), buffer.Bytes(), 0644)
		if IsRemote() {
			podmanTest.RestartRemoteService()
			defer podmanTest.RestartRemoteService()
		}

		search := podmanTest.Podman([]string{"search", image, "--tls-verify=true"})
		search.WaitWithDefaultTimeout()

		Expect(search.ExitCode()).To(Equal(0))
		Expect(search.OutputToString()).Should(BeEmpty())
		match, _ := search.ErrorGrepString("error")
		Expect(match).Should(BeTrue())

		// cleanup
		resetRegistriesConfigEnv()
	})

	It("podman search doesn't attempt HTTP if registry is not listed as insecure", func() {
		SkipIfRemote("FIXME This should work on podman-remote")
		if podmanTest.Host.Arch == "ppc64le" {
			Skip("No registry image for ppc64le")
		}
		lock := GetPortLock(registryEndpoints[6].Port)
		defer lock.Unlock()
		registry := podmanTest.Podman([]string{"run", "-d", "-p", fmt.Sprintf("%s:5000", registryEndpoints[6].Port),
			"--name", "registry6", registry})
		registry.WaitWithDefaultTimeout()
		Expect(registry.ExitCode()).To(Equal(0))

		if !WaitContainerReady(podmanTest, "registry6", "listening on", 20, 1) {
			Skip("Cannot start docker registry.")
		}

		podmanTest.RestoreArtifact(ALPINE)
		image := fmt.Sprintf("%s/my-alpine", registryEndpoints[6].Address())
		push := podmanTest.Podman([]string{"push", "--tls-verify=false", "--remove-signatures", ALPINE, image})
		push.WaitWithDefaultTimeout()
		Expect(push.ExitCode()).To(Equal(0))

		var buffer bytes.Buffer
		registryFileBadTmpl.Execute(&buffer, registryEndpoints[6])
		podmanTest.setRegistriesConfigEnv(buffer.Bytes())
		ioutil.WriteFile(fmt.Sprintf("%s/registry6.conf", tempdir), buffer.Bytes(), 0644)

		if IsRemote() {
			podmanTest.RestartRemoteService()
			defer podmanTest.RestartRemoteService()
		}

		search := podmanTest.Podman([]string{"search", image})
		search.WaitWithDefaultTimeout()

		Expect(search.ExitCode()).To(Equal(0))
		Expect(search.OutputToString()).Should(BeEmpty())
		match, _ := search.ErrorGrepString("error")
		Expect(match).Should(BeTrue())

		// cleanup
		resetRegistriesConfigEnv()
	})

	It("podman search doesn't attempt HTTP if one registry is not listed as insecure", func() {
		SkipIfRemote("FIXME This should work on podman-remote")
		if podmanTest.Host.Arch == "ppc64le" {
			Skip("No registry image for ppc64le")
		}
		lock7 := GetPortLock(registryEndpoints[7].Port)
		defer lock7.Unlock()
		lock8 := GetPortLock("6000")
		defer lock8.Unlock()

		registryLocal := podmanTest.Podman([]string{"run", "-d", "--net=host", "-p", fmt.Sprintf("%s:5000", registryEndpoints[7].Port),
			"--name", "registry7", registry})
		registryLocal.WaitWithDefaultTimeout()
		Expect(registryLocal.ExitCode()).To(Equal(0))

		if !WaitContainerReady(podmanTest, "registry7", "listening on", 20, 1) {
			Skip("Cannot start docker registry.")
		}

		registryLocal = podmanTest.Podman([]string{"run", "-d", "-p", "6000:5000", "--name", "registry8", registry})
		registryLocal.WaitWithDefaultTimeout()
		Expect(registryLocal.ExitCode()).To(Equal(0))

		if !WaitContainerReady(podmanTest, "registry8", "listening on", 20, 1) {
			Skip("Cannot start docker registry.")
		}

		podmanTest.RestoreArtifact(ALPINE)
		push := podmanTest.Podman([]string{"push", "--tls-verify=false", "--remove-signatures", ALPINE, "localhost:6000/my-alpine"})
		push.WaitWithDefaultTimeout()
		Expect(push.ExitCode()).To(Equal(0))

		// registries.conf set up
		var buffer bytes.Buffer
		registryFileTwoTmpl.Execute(&buffer, registryEndpoints[8])
		podmanTest.setRegistriesConfigEnv(buffer.Bytes())
		ioutil.WriteFile(fmt.Sprintf("%s/registry8.conf", tempdir), buffer.Bytes(), 0644)

		if IsRemote() {
			podmanTest.RestartRemoteService()
			defer podmanTest.RestartRemoteService()
		}

		search := podmanTest.Podman([]string{"search", "my-alpine"})
		search.WaitWithDefaultTimeout()

		Expect(search.ExitCode()).To(Equal(0))
		Expect(search.OutputToString()).Should(BeEmpty())
		match, _ := search.ErrorGrepString("error")
		Expect(match).Should(BeTrue())

		// cleanup
		resetRegistriesConfigEnv()
	})

	// search should fail with nonexistent authfile
	It("podman search fail with nonexistent --authfile", func() {
		search := podmanTest.Podman([]string{"search", "--authfile", "/tmp/nonexistent", ALPINE})
		search.WaitWithDefaultTimeout()
		Expect(search.ExitCode()).To(Not(Equal(0)))
	})

	It("podman search with wildcards", func() {
		search := podmanTest.Podman([]string{"search", "--limit", "30", "registry.redhat.io/*"})
		search.WaitWithDefaultTimeout()
		Expect(search.ExitCode()).To(Equal(0))
		Expect(len(search.OutputToStringArray())).To(Equal(31))

		search = podmanTest.Podman([]string{"search", "registry.redhat.io/*openshift*"})
		search.WaitWithDefaultTimeout()
		Expect(search.ExitCode()).To(Equal(0))
		Expect(len(search.OutputToStringArray()) > 1).To(BeTrue())
	})

	It("podman search repository tags", func() {
		search := podmanTest.Podman([]string{"search", "--list-tags", "--limit", "30", "docker.io/library/alpine"})
		search.WaitWithDefaultTimeout()
		Expect(search.ExitCode()).To(Equal(0))
		Expect(len(search.OutputToStringArray())).To(Equal(31))

		search = podmanTest.Podman([]string{"search", "--list-tags", "docker.io/library/alpine"})
		search.WaitWithDefaultTimeout()
		Expect(search.ExitCode()).To(Equal(0))
		Expect(len(search.OutputToStringArray()) > 2).To(BeTrue())

		search = podmanTest.Podman([]string{"search", "--filter=is-official", "--list-tags", "docker.io/library/alpine"})
		search.WaitWithDefaultTimeout()
		Expect(search.ExitCode()).To(Not(Equal(0)))

		search = podmanTest.Podman([]string{"search", "--list-tags", "docker.io/library/"})
		search.WaitWithDefaultTimeout()
		Expect(len(search.OutputToStringArray()) == 0).To(BeTrue())
	})

	It("podman search with limit over 100", func() {
		search := podmanTest.Podman([]string{"search", "--limit", "130", "registry.redhat.io/rhel"})
		search.WaitWithDefaultTimeout()
		Expect(search.ExitCode()).To(Equal(0))
		Expect(len(search.OutputToStringArray())).To(Equal(131))
	})
})
