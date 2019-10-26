// +build !remoteclient

package integration

import (
	"os"
	"path/filepath"
	"text/template"

	. "github.com/containers/libpod/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var yamlTemplate = `
apiVersion: v1
kind: Pod
metadata:
  creationTimestamp: "2019-07-17T14:44:08Z"
  labels:
    app: {{ .Name }}
  name: {{ .Name }}
spec:
  hostname: {{ .Hostname }}
  containers:
{{ with .Ctrs }}
  {{ range . }}
  - command:
    {{ range .Cmd }}
    - {{.}}
    {{ end }}
    env:
    - name: PATH
      value: /usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
    - name: TERM
      value: xterm
    - name: HOSTNAME
    - name: container
      value: podman
    image: {{ .Image }}
    name: {{ .Name }}
    resources: {}
    {{ if .SecurityContext }}
    securityContext:
      allowPrivilegeEscalation: true
      {{ if .Caps }}
      capabilities:
        {{ with .CapAdd }}
        add:
          {{ range . }}
          - {{.}}
          {{ end }}
        {{ end }}
        {{ with .CapDrop }}
        drop:
          {{ range . }}
          - {{.}}
          {{ end }}
        {{ end }}
      {{ end }}
      privileged: false
      readOnlyRootFilesystem: false
    workingDir: /
    {{ end }}
  {{ end }}
{{ end }}
status: {}
`

var (
	defaultCtrName  = "testCtr"
	defaultCtrCmd   = []string{"top"}
	defaultCtrImage = ALPINE
	defaultPodName  = "testPod"
)

func generateKubeYaml(pod *Pod, fileName string) error {
	f, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer f.Close()

	t, err := template.New("pod").Parse(yamlTemplate)
	if err != nil {
		return err
	}

	if err := t.Execute(f, pod); err != nil {
		return err
	}

	return nil
}

// Pod describes the options a kube yaml can be configured at pod level
type Pod struct {
	Name     string
	Hostname string
	Ctrs     []*Ctr
}

// getPod takes a list of podOptions and returns a pod with sane defaults
// and the configured options
// if no containers are added, it will add the default container
func getPod(options ...podOption) *Pod {
	p := Pod{defaultPodName, "", make([]*Ctr, 0)}
	for _, option := range options {
		option(&p)
	}
	if len(p.Ctrs) == 0 {
		p.Ctrs = []*Ctr{getCtr()}
	}
	return &p
}

type podOption func(*Pod)

func withHostname(h string) podOption {
	return func(pod *Pod) {
		pod.Hostname = h
	}
}

func withCtr(c *Ctr) podOption {
	return func(pod *Pod) {
		pod.Ctrs = append(pod.Ctrs, c)
	}
}

// Ctr describes the options a kube yaml can be configured at container level
type Ctr struct {
	Name            string
	Image           string
	Cmd             []string
	SecurityContext bool
	Caps            bool
	CapAdd          []string
	CapDrop         []string
}

// getCtr takes a list of ctrOptions and returns a Ctr with sane defaults
// and the configured options
func getCtr(options ...ctrOption) *Ctr {
	c := Ctr{defaultCtrName, defaultCtrImage, defaultCtrCmd, true, false, nil, nil}
	for _, option := range options {
		option(&c)
	}
	return &c
}

type ctrOption func(*Ctr)

func withCmd(cmd []string) ctrOption {
	return func(c *Ctr) {
		c.Cmd = cmd
	}
}

func withImage(img string) ctrOption {
	return func(c *Ctr) {
		c.Image = img
	}
}

func withSecurityContext(sc bool) ctrOption {
	return func(c *Ctr) {
		c.SecurityContext = sc
	}
}

func withCapAdd(caps []string) ctrOption {
	return func(c *Ctr) {
		c.CapAdd = caps
		c.Caps = true
	}
}

func withCapDrop(caps []string) ctrOption {
	return func(c *Ctr) {
		c.CapDrop = caps
		c.Caps = true
	}
}

var _ = Describe("Podman generate kube", func() {
	var (
		tempdir    string
		err        error
		podmanTest *PodmanTestIntegration
		kubeYaml   string
	)

	BeforeEach(func() {
		tempdir, err = CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}
		podmanTest = PodmanTestCreate(tempdir)
		podmanTest.Setup()
		podmanTest.SeedImages()

		kubeYaml = filepath.Join(podmanTest.TempDir, "kube.yaml")
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)
	})

	It("podman play kube fail with nonexist authfile", func() {
		err := generateKubeYaml(getPod(), kubeYaml)
		Expect(err).To(BeNil())

		kube := podmanTest.Podman([]string{"play", "kube", "--authfile", "/tmp/nonexist", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Not(Equal(0)))

	})

	It("podman play kube test correct command", func() {
		err := generateKubeYaml(getPod(), kubeYaml)
		Expect(err).To(BeNil())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		inspect := podmanTest.Podman([]string{"inspect", defaultCtrName})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.ExitCode()).To(Equal(0))
		Expect(inspect.OutputToString()).To(ContainSubstring(defaultCtrCmd[0]))
	})

	It("podman play kube test correct output", func() {
		p := getPod(withCtr(getCtr(withCmd([]string{"echo", "hello"}))))

		err := generateKubeYaml(p, kubeYaml)
		Expect(err).To(BeNil())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		logs := podmanTest.Podman([]string{"logs", defaultCtrName})
		logs.WaitWithDefaultTimeout()
		Expect(logs.ExitCode()).To(Equal(0))
		Expect(logs.OutputToString()).To(ContainSubstring("hello"))

		inspect := podmanTest.Podman([]string{"inspect", defaultCtrName, "--format", "'{{ .Config.Cmd }}'"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.ExitCode()).To(Equal(0))
		Expect(inspect.OutputToString()).To(ContainSubstring("hello"))
	})

	It("podman play kube test hostname", func() {
		err := generateKubeYaml(getPod(), kubeYaml)
		Expect(err).To(BeNil())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		inspect := podmanTest.Podman([]string{"inspect", defaultCtrName, "--format", "{{ .Config.Hostname }}"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.ExitCode()).To(Equal(0))
		Expect(inspect.OutputToString()).To(Equal(defaultPodName))
	})

	It("podman play kube test with customized hostname", func() {
		hostname := "myhostname"
		err := generateKubeYaml(getPod(withHostname(hostname)), kubeYaml)
		Expect(err).To(BeNil())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		inspect := podmanTest.Podman([]string{"inspect", defaultCtrName, "--format", "{{ .Config.Hostname }}"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.ExitCode()).To(Equal(0))
		Expect(inspect.OutputToString()).To(Equal(hostname))
	})

	It("podman play kube cap add", func() {
		capAdd := "CAP_SYS_ADMIN"
		ctr := getCtr(withCapAdd([]string{capAdd}), withCmd([]string{"cat", "/proc/self/status"}))

		err := generateKubeYaml(getPod(withCtr(ctr)), kubeYaml)
		Expect(err).To(BeNil())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		inspect := podmanTest.Podman([]string{"inspect", defaultCtrName})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.ExitCode()).To(Equal(0))
		Expect(inspect.OutputToString()).To(ContainSubstring(capAdd))
	})

	It("podman play kube cap drop", func() {
		capDrop := "CAP_CHOWN"
		ctr := getCtr(withCapDrop([]string{capDrop}))

		err := generateKubeYaml(getPod(withCtr(ctr)), kubeYaml)
		Expect(err).To(BeNil())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		inspect := podmanTest.Podman([]string{"inspect", defaultCtrName})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.ExitCode()).To(Equal(0))
		Expect(inspect.OutputToString()).To(ContainSubstring(capDrop))
	})

	It("podman play kube no security context", func() {
		// expect play kube to not fail if no security context is specified
		err := generateKubeYaml(getPod(withCtr(getCtr(withSecurityContext(false)))), kubeYaml)
		Expect(err).To(BeNil())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		inspect := podmanTest.Podman([]string{"inspect", defaultCtrName})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.ExitCode()).To(Equal(0))
	})
})
