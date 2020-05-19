package endpoint

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/containers/libpod/pkg/rootless"
	iopodman "github.com/containers/libpod/pkg/varlink"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/gomega/gexec"
)

var (
	ARTIFACT_DIR       = "/tmp/.artifacts"
	CGROUP_MANAGER     = "systemd"
	defaultWaitTimeout = 90
	//RESTORE_IMAGES     = []string{ALPINE, BB}
	INTEGRATION_ROOT string
	ImageCacheDir    = "/tmp/podman/imagecachedir"
	VarlinkBinary    = "/usr/bin/varlink"
	ALPINE           = "docker.io/library/alpine:latest"
	infra            = "k8s.gcr.io/pause:3.2"
	BB               = "docker.io/library/busybox:latest"
	redis            = "docker.io/library/redis:alpine"
	fedoraMinimal    = "quay.io/libpod/fedora-minimal:latest"
)

type EndpointTestIntegration struct {
	ArtifactPath  string
	CNIConfigDir  string
	CgroupManager string
	ConmonBinary  string
	CrioRoot      string
	//Host                HostOS
	ImageCacheDir       string
	ImageCacheFS        string
	OCIRuntime          string
	PodmanBinary        string
	RemoteTest          bool
	RunRoot             string
	SignaturePolicyPath string
	StorageOptions      string
	TmpDir              string
	Timings             []string
	VarlinkBinary       string
	VarlinkCommand      *exec.Cmd
	VarlinkEndpoint     string
	VarlinkSession      *os.Process
}

func (p *EndpointTestIntegration) StartVarlink() {
	p.startVarlink(false)
}

func (p *EndpointTestIntegration) StartVarlinkWithCache() {
	p.startVarlink(true)
}

func (p *EndpointTestIntegration) startVarlink(useImageCache bool) {
	var (
		counter int
	)
	if os.Geteuid() == 0 {
		os.MkdirAll("/run/podman", 0755)
	}
	varlinkEndpoint := p.VarlinkEndpoint
	//p.SetVarlinkAddress(p.RemoteSocket)

	args := []string{"varlink", "--timeout", "0", varlinkEndpoint}
	podmanOptions := getVarlinkOptions(p, args)
	if useImageCache {
		cacheOptions := []string{"--storage-opt", fmt.Sprintf("%s.imagestore=%s", p.ImageCacheFS, p.ImageCacheDir)}
		podmanOptions = append(cacheOptions, podmanOptions...)
	}
	command := exec.Command(p.PodmanBinary, podmanOptions...)
	fmt.Printf("Running: %s %s\n", p.PodmanBinary, strings.Join(podmanOptions, " "))
	command.Start()
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	p.VarlinkCommand = command
	p.VarlinkSession = command.Process
	for {
		if result := p.endpointReady(); result == 0 {
			break
		}
		fmt.Println("Waiting for varlink connection to become active", counter)
		time.Sleep(250 * time.Millisecond)
		counter++
		if counter > 40 {
			Fail("varlink endpoint never became ready")
		}
	}
}

func (p *EndpointTestIntegration) endpointReady() int {
	session := p.Varlink("GetVersion", "", false)
	return session.ExitCode()
}

func (p *EndpointTestIntegration) StopVarlink() {
	var out bytes.Buffer
	var pids []int
	varlinkSession := p.VarlinkSession

	if !rootless.IsRootless() {
		if err := varlinkSession.Kill(); err != nil {
			fmt.Fprintf(os.Stderr, "error on varlink stop-kill %q", err)
		}
		if _, err := varlinkSession.Wait(); err != nil {
			fmt.Fprintf(os.Stderr, "error on varlink stop-wait %q", err)
		}

	} else {
		//p.ResetVarlinkAddress()
		parentPid := fmt.Sprintf("%d", p.VarlinkSession.Pid)
		pgrep := exec.Command("pgrep", "-P", parentPid)
		fmt.Printf("running: pgrep %s\n", parentPid)
		pgrep.Stdout = &out
		err := pgrep.Run()
		if err != nil {
			fmt.Fprint(os.Stderr, "unable to find varlink pid")
		}

		for _, s := range strings.Split(out.String(), "\n") {
			if len(s) == 0 {
				continue
			}
			p, err := strconv.Atoi(s)
			if err != nil {
				fmt.Fprintf(os.Stderr, "unable to convert %s to int", s)
			}
			if p != 0 {
				pids = append(pids, p)
			}
		}

		pids = append(pids, p.VarlinkSession.Pid)
		for _, pid := range pids {
			syscall.Kill(pid, syscall.SIGKILL)
		}
	}
	socket := strings.Split(p.VarlinkEndpoint, ":")[1]
	if err := os.Remove(socket); err != nil {
		fmt.Println(err)
	}
}

type EndpointSession struct {
	*gexec.Session
}

func getVarlinkOptions(p *EndpointTestIntegration, args []string) []string {
	podmanOptions := strings.Split(fmt.Sprintf("--root %s --runroot %s --runtime %s --conmon %s --cni-config-dir %s --cgroup-manager %s",
		p.CrioRoot, p.RunRoot, p.OCIRuntime, p.ConmonBinary, p.CNIConfigDir, p.CgroupManager), " ")
	if os.Getenv("HOOK_OPTION") != "" {
		podmanOptions = append(podmanOptions, os.Getenv("HOOK_OPTION"))
	}
	podmanOptions = append(podmanOptions, strings.Split(p.StorageOptions, " ")...)
	podmanOptions = append(podmanOptions, args...)
	return podmanOptions
}

func (p *EndpointTestIntegration) Varlink(endpoint, message string, more bool) *EndpointSession {
	//call unix:/run/user/1000/podman/io.podman/io.podman.GetContainerStats '{"name": "foobar" }'
	var (
		command *exec.Cmd
	)

	args := []string{"call"}
	if more {
		args = append(args, "-m")
	}
	args = append(args, []string{fmt.Sprintf("%s/io.podman.%s", p.VarlinkEndpoint, endpoint)}...)
	if len(message) > 0 {
		args = append(args, message)
	}
	command = exec.Command(p.VarlinkBinary, args...)
	session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
	if err != nil {
		Fail(fmt.Sprintf("unable to run varlink command: %s\n%v", strings.Join(args, " "), err))
	}
	session.Wait(defaultWaitTimeout)
	return &EndpointSession{session}
}

func (s *EndpointSession) StdErrToString() string {
	fields := strings.Fields(fmt.Sprintf("%s", s.Err.Contents()))
	return strings.Join(fields, " ")
}

func (s *EndpointSession) OutputToString() string {
	fields := strings.Fields(fmt.Sprintf("%s", s.Out.Contents()))
	return strings.Join(fields, " ")
}

func (s *EndpointSession) OutputToBytes() []byte {
	out := s.OutputToString()
	return []byte(out)
}

func (s *EndpointSession) OutputToStringMap() map[string]string {
	var out map[string]string
	json.Unmarshal(s.OutputToBytes(), &out)
	return out
}

func (s *EndpointSession) OutputToMapToInt() map[string]int {
	var out map[string]int
	json.Unmarshal(s.OutputToBytes(), &out)
	return out
}

func (s *EndpointSession) OutputToMoreResponse() iopodman.MoreResponse {
	out := make(map[string]iopodman.MoreResponse)
	json.Unmarshal(s.OutputToBytes(), &out)
	return out["reply"]
}
