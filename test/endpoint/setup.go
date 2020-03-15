package endpoint

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	iopodman "github.com/containers/libpod/cmd/podman/varlink"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/containers/storage/pkg/stringid"
	"github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
)

func Setup(tempDir string) *EndpointTestIntegration {
	var (
		endpoint string
	)
	cwd, _ := os.Getwd()
	INTEGRATION_ROOT = filepath.Join(cwd, "../../")

	podmanBinary := filepath.Join(cwd, "../../bin/podman")
	if os.Getenv("PODMAN_BINARY") != "" {
		podmanBinary = os.Getenv("PODMAN_BINARY")
	}
	conmonBinary := filepath.Join("/usr/libexec/podman/conmon")
	altConmonBinary := "/usr/bin/conmon"
	if _, err := os.Stat(conmonBinary); os.IsNotExist(err) {
		conmonBinary = altConmonBinary
	}
	if os.Getenv("CONMON_BINARY") != "" {
		conmonBinary = os.Getenv("CONMON_BINARY")
	}
	storageOptions := STORAGE_OPTIONS
	if os.Getenv("STORAGE_OPTIONS") != "" {
		storageOptions = os.Getenv("STORAGE_OPTIONS")
	}
	cgroupManager := CGROUP_MANAGER
	if rootless.IsRootless() {
		cgroupManager = "cgroupfs"
	}
	if os.Getenv("CGROUP_MANAGER") != "" {
		cgroupManager = os.Getenv("CGROUP_MANAGER")
	}

	ociRuntime := os.Getenv("OCI_RUNTIME")
	if ociRuntime == "" {
		var err error
		ociRuntime, err = exec.LookPath("runc")
		// If we cannot find the runc binary, setting to something static as we have no way
		// to return an error.  The tests will fail and point out that the runc binary could
		// not be found nicely.
		if err != nil {
			ociRuntime = "/usr/bin/runc"
		}
	}
	os.Setenv("DISABLE_HC_SYSTEMD", "true")
	CNIConfigDir := "/etc/cni/net.d"

	storageFs := STORAGE_FS
	if rootless.IsRootless() {
		storageFs = ROOTLESS_STORAGE_FS
	}

	uuid := stringid.GenerateNonCryptoID()
	if !rootless.IsRootless() {
		endpoint = fmt.Sprintf("unix:/run/podman/io.podman-%s", uuid)
	} else {
		runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
		socket := fmt.Sprintf("io.podman-%s", uuid)
		fqpath := filepath.Join(runtimeDir, socket)
		endpoint = fmt.Sprintf("unix:%s", fqpath)
	}

	eti := EndpointTestIntegration{
		ArtifactPath:        ARTIFACT_DIR,
		CNIConfigDir:        CNIConfigDir,
		CgroupManager:       cgroupManager,
		ConmonBinary:        conmonBinary,
		CrioRoot:            filepath.Join(tempDir, "crio"),
		ImageCacheDir:       ImageCacheDir,
		ImageCacheFS:        storageFs,
		OCIRuntime:          ociRuntime,
		PodmanBinary:        podmanBinary,
		RunRoot:             filepath.Join(tempDir, "crio-run"),
		SignaturePolicyPath: filepath.Join(INTEGRATION_ROOT, "test/policy.json"),
		StorageOptions:      storageOptions,
		TmpDir:              tempDir,
		//Timings:             nil,
		VarlinkBinary:   VarlinkBinary,
		VarlinkCommand:  nil,
		VarlinkEndpoint: endpoint,
		VarlinkSession:  nil,
	}
	return &eti
}

func (p *EndpointTestIntegration) Cleanup() {
	// Remove all containers
	// TODO Make methods to do all this?

	p.stopAllContainers()

	//TODO need to make stop all pods

	p.StopVarlink()
	// Nuke tempdir
	if err := os.RemoveAll(p.TmpDir); err != nil {
		fmt.Printf("%q\n", err)
	}

	// Clean up the registries configuration file ENV variable set in Create
	resetRegistriesConfigEnv()
}

func (p *EndpointTestIntegration) listContainers() []iopodman.Container {
	containers := p.Varlink("ListContainers", "", false)
	var varlinkContainers map[string][]iopodman.Container
	if err := json.Unmarshal(containers.OutputToBytes(), &varlinkContainers); err != nil {
		logrus.Error("failed to unmarshal containers")
	}
	return varlinkContainers["containers"]
}

func (p *EndpointTestIntegration) stopAllContainers() {
	containers := p.listContainers()
	for _, container := range containers {
		p.stopContainer(container.Id)
	}
}

func (p *EndpointTestIntegration) stopContainer(cid string) {
	p.Varlink("StopContainer", fmt.Sprintf("{\"name\":\"%s\", \"timeout\":0}", cid), false)
}

func resetRegistriesConfigEnv() {
	os.Setenv("REGISTRIES_CONFIG_PATH", "")
}

func (p *EndpointTestIntegration) createArtifact(image string) {
	if os.Getenv("NO_TEST_CACHE") != "" {
		return
	}
	dest := strings.Split(image, "/")
	destName := fmt.Sprintf("/tmp/%s.tar", strings.Replace(strings.Join(strings.Split(dest[len(dest)-1], "/"), ""), ":", "-", -1))
	fmt.Printf("Caching %s at %s...", image, destName)
	if _, err := os.Stat(destName); os.IsNotExist(err) {
		pull := p.Varlink("PullImage", fmt.Sprintf("{\"name\":\"%s\"}", image), false)
		Expect(pull.ExitCode()).To(Equal(0))

		imageSave := iopodman.ImageSaveOptions{
			//Name:image,
			//Output: destName,
			//Format: "oci-archive",
		}
		imageSave.Name = image
		imageSave.Output = destName
		imageSave.Format = "oci-archive"
		foo := make(map[string]iopodman.ImageSaveOptions)
		foo["options"] = imageSave
		f, _ := json.Marshal(foo)
		save := p.Varlink("ImageSave", string(f), false)
		result := save.OutputToMoreResponse()
		Expect(save.ExitCode()).To(Equal(0))
		Expect(os.Rename(result.Id, destName)).To(BeNil())
		fmt.Printf("\n")
	} else {
		fmt.Printf(" already exists.\n")
	}
}

func populateCache(p *EndpointTestIntegration) {
	p.CrioRoot = p.ImageCacheDir
	p.StartVarlink()
	for _, image := range CACHE_IMAGES {
		p.RestoreArtifactToCache(image)
	}
	p.StopVarlink()
}

func (p *EndpointTestIntegration) RestoreArtifactToCache(image string) error {
	fmt.Printf("Restoring %s...\n", image)
	dest := strings.Split(image, "/")
	destName := fmt.Sprintf("/tmp/%s.tar", strings.Replace(strings.Join(strings.Split(dest[len(dest)-1], "/"), ""), ":", "-", -1))
	//fmt.Println(destName, p.ImageCacheDir)
	load := p.Varlink("LoadImage", fmt.Sprintf("{\"name\": \"%s\", \"inputFile\": \"%s\"}", image, destName), false)
	Expect(load.ExitCode()).To(BeZero())
	return nil
}

func (p *EndpointTestIntegration) startTopContainer(name string) string {
	t := true
	args := iopodman.Create{
		Args:   []string{"docker.io/library/alpine:latest", "top"},
		Tty:    &t,
		Detach: &t,
	}
	if len(name) > 0 {
		args.Name = &name
	}
	b, err := json.Marshal(args)
	if err != nil {
		ginkgo.Fail("failed to marshal data for top container")
	}
	input := fmt.Sprintf("{\"create\":%s}", string(b))
	top := p.Varlink("CreateContainer", input, false)
	if top.ExitCode() != 0 {
		ginkgo.Fail("failed to start top container")
	}
	start := p.Varlink("StartContainer", fmt.Sprintf("{\"name\":\"%s\"}", name), false)
	if start.ExitCode() != 0 {
		ginkgo.Fail("failed to start top container")
	}
	return start.OutputToString()
}
