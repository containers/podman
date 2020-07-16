// +build !remote

package integration

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	. "github.com/containers/libpod/v2/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var unknownKindYaml = `
apiVerson: v1
kind: UnknownKind
metadata:
  labels:
    app: app1
  name: unknown
spec:
  hostname: unknown
`

var podYamlTemplate = `
apiVersion: v1
kind: Pod
metadata:
  creationTimestamp: "2019-07-17T14:44:08Z"
  labels:
    app: {{ .Name }}
  name: {{ .Name }}
{{ with .Annotations }}
  annotations:
  {{ range $key, $value := . }}
    {{ $key }}: {{ $value }}
  {{ end }}
{{ end }}

spec:
  hostname: {{ .Hostname }}
  containers:
{{ with .Ctrs }}
  {{ range . }}
  - command:
    {{ range .Cmd }}
    - {{.}}
    {{ end }}
    args:
    {{ range .Arg }}
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
    imagePullPolicy: {{ .PullPolicy }}
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

var deploymentYamlTemplate = `
apiVersion: v1
kind: Deployment
metadata:
  creationTimestamp: "2019-07-17T14:44:08Z"
  labels:
    app: {{ .Name }}
  name: {{ .Name }}
{{ with .Annotations }}
  annotations:
  {{ range $key, $value := . }}
    {{ $key }}: {{ $value }}
  {{ end }}
{{ end }}

spec:
  replicas: {{ .Replicas }}
  selector:
    matchLabels:
      app: {{ .Name }}
  template:
  {{ with .PodTemplate }}
    metadata:
      labels:
        app: {{ .Name }}
      {{ with .Annotations }}
        annotations:
        {{ range $key, $value := . }}
          {{ $key }}: {{ $value }}
        {{ end }}
      {{ end }}
    spec:
      hostname: {{ .Hostname }}
      containers:
    {{ with .Ctrs }}
      {{ range . }}
      - command:
        {{ range .Cmd }}
        - {{.}}
        {{ end }}
        args:
        {{ range .Arg }}
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
        imagePullPolicy: {{ .PullPolicy }}
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
  {{ end }}
`

var (
	defaultCtrName        = "testCtr"
	defaultCtrCmd         = []string{"top"}
	defaultCtrArg         = []string{"-d", "1.5"}
	defaultCtrImage       = ALPINE
	defaultPodName        = "testPod"
	defaultDeploymentName = "testDeployment"
	seccompPwdEPERM       = []byte(`{"defaultAction":"SCMP_ACT_ALLOW","syscalls":[{"name":"getcwd","action":"SCMP_ACT_ERRNO"}]}`)
)

func writeYaml(content string, fileName string) error {
	f, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(content)
	if err != nil {
		return err
	}

	return nil
}

func generatePodKubeYaml(pod *Pod, fileName string) error {
	templateBytes := &bytes.Buffer{}

	t, err := template.New("pod").Parse(podYamlTemplate)
	if err != nil {
		return err
	}

	if err := t.Execute(templateBytes, pod); err != nil {
		return err
	}

	return writeYaml(templateBytes.String(), fileName)
}

func generateDeploymentKubeYaml(deployment *Deployment, fileName string) error {
	templateBytes := &bytes.Buffer{}

	t, err := template.New("deployment").Parse(deploymentYamlTemplate)
	if err != nil {
		return err
	}

	if err := t.Execute(templateBytes, deployment); err != nil {
		return err
	}

	return writeYaml(templateBytes.String(), fileName)
}

// Pod describes the options a kube yaml can be configured at pod level
type Pod struct {
	Name        string
	Hostname    string
	Ctrs        []*Ctr
	Annotations map[string]string
}

// getPod takes a list of podOptions and returns a pod with sane defaults
// and the configured options
// if no containers are added, it will add the default container
func getPod(options ...podOption) *Pod {
	p := Pod{defaultPodName, "", make([]*Ctr, 0), make(map[string]string)}
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

func withAnnotation(k, v string) podOption {
	return func(pod *Pod) {
		pod.Annotations[k] = v
	}
}

// Deployment describes the options a kube yaml can be configured at deployment level
type Deployment struct {
	Name        string
	Replicas    int32
	Annotations map[string]string
	PodTemplate *Pod
}

func getDeployment(options ...deploymentOption) *Deployment {
	d := Deployment{defaultDeploymentName, 1, make(map[string]string), getPod()}
	for _, option := range options {
		option(&d)
	}

	return &d
}

type deploymentOption func(*Deployment)

func withDeploymentAnnotation(k, v string) deploymentOption {
	return func(deployment *Deployment) {
		deployment.Annotations[k] = v
	}
}

func withPod(pod *Pod) deploymentOption {
	return func(d *Deployment) {
		d.PodTemplate = pod
	}
}

func withReplicas(replicas int32) deploymentOption {
	return func(d *Deployment) {
		d.Replicas = replicas
	}
}

// getPodNamesInDeployment returns list of Pod objects
// with just their name set, so that it can be passed around
// and into getCtrNameInPod for ease of testing
func getPodNamesInDeployment(d *Deployment) []Pod {
	var pods []Pod
	var i int32

	for i = 0; i < d.Replicas; i++ {
		p := Pod{}
		p.Name = fmt.Sprintf("%s-pod-%d", d.Name, i)
		pods = append(pods, p)
	}

	return pods
}

// Ctr describes the options a kube yaml can be configured at container level
type Ctr struct {
	Name            string
	Image           string
	Cmd             []string
	Arg             []string
	SecurityContext bool
	Caps            bool
	CapAdd          []string
	CapDrop         []string
	PullPolicy      string
}

// getCtr takes a list of ctrOptions and returns a Ctr with sane defaults
// and the configured options
func getCtr(options ...ctrOption) *Ctr {
	c := Ctr{defaultCtrName, defaultCtrImage, defaultCtrCmd, defaultCtrArg, true, false, nil, nil, ""}
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

func withArg(arg []string) ctrOption {
	return func(c *Ctr) {
		c.Arg = arg
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

func withPullPolicy(policy string) ctrOption {
	return func(c *Ctr) {
		c.PullPolicy = policy
	}
}

func getCtrNameInPod(pod *Pod) string {
	return fmt.Sprintf("%s-%s", pod.Name, defaultCtrName)
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

	It("podman play kube fail with yaml of unsupported kind", func() {
		err := writeYaml(unknownKindYaml, kubeYaml)
		Expect(err).To(BeNil())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Not(Equal(0)))

	})

	It("podman play kube fail with nonexist authfile", func() {
		err := generatePodKubeYaml(getPod(), kubeYaml)
		Expect(err).To(BeNil())

		kube := podmanTest.Podman([]string{"play", "kube", "--authfile", "/tmp/nonexist", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Not(Equal(0)))

	})

	It("podman play kube test correct command", func() {
		pod := getPod()
		err := generatePodKubeYaml(pod, kubeYaml)
		Expect(err).To(BeNil())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod), "--format", "'{{ .Config.Cmd }}'"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.ExitCode()).To(Equal(0))
		// Use the defined command to override the image's command
		correctCmd := "[" + strings.Join(defaultCtrCmd, " ") + " " + strings.Join(defaultCtrArg, " ")
		Expect(inspect.OutputToString()).To(ContainSubstring(correctCmd))
	})

	It("podman play kube test correct command with only set command in yaml file", func() {
		pod := getPod(withCtr(getCtr(withCmd([]string{"echo", "hello"}), withArg(nil))))
		err := generatePodKubeYaml(pod, kubeYaml)
		Expect(err).To(BeNil())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod), "--format", "'{{ .Config.Cmd }}'"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.ExitCode()).To(Equal(0))
		// Use the defined command to override the image's command, and don't set the args
		// so the full command in result should not contains the image's command
		Expect(inspect.OutputToString()).To(ContainSubstring(`[echo hello]`))
	})

	It("podman play kube test correct command with only set args in yaml file", func() {
		pod := getPod(withCtr(getCtr(withImage(redis), withCmd(nil), withArg([]string{"echo", "hello"}))))
		err := generatePodKubeYaml(pod, kubeYaml)
		Expect(err).To(BeNil())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod), "--format", "'{{ .Config.Cmd }}'"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.ExitCode()).To(Equal(0))
		// this image's ENTRYPOINT is called `docker-entrypoint.sh`
		// so result should be `docker-entrypoint.sh + withArg(...)`
		Expect(inspect.OutputToString()).To(ContainSubstring(`[docker-entrypoint.sh echo hello]`))
	})

	It("podman play kube test correct output", func() {
		p := getPod(withCtr(getCtr(withCmd([]string{"echo", "hello"}), withArg([]string{"world"}))))

		err := generatePodKubeYaml(p, kubeYaml)
		Expect(err).To(BeNil())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		logs := podmanTest.Podman([]string{"logs", getCtrNameInPod(p)})
		logs.WaitWithDefaultTimeout()
		Expect(logs.ExitCode()).To(Equal(0))
		Expect(logs.OutputToString()).To(ContainSubstring("hello world"))

		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(p), "--format", "'{{ .Config.Cmd }}'"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.ExitCode()).To(Equal(0))
		Expect(inspect.OutputToString()).To(ContainSubstring(`[echo hello world]`))
	})

	It("podman play kube test hostname", func() {
		pod := getPod()
		err := generatePodKubeYaml(pod, kubeYaml)
		Expect(err).To(BeNil())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod), "--format", "{{ .Config.Hostname }}"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.ExitCode()).To(Equal(0))
		Expect(inspect.OutputToString()).To(Equal(defaultPodName))
	})

	It("podman play kube test with customized hostname", func() {
		hostname := "myhostname"
		pod := getPod(withHostname(hostname))
		err := generatePodKubeYaml(getPod(withHostname(hostname)), kubeYaml)
		Expect(err).To(BeNil())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod), "--format", "{{ .Config.Hostname }}"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.ExitCode()).To(Equal(0))
		Expect(inspect.OutputToString()).To(Equal(hostname))
	})

	It("podman play kube cap add", func() {
		capAdd := "CAP_SYS_ADMIN"
		ctr := getCtr(withCapAdd([]string{capAdd}), withCmd([]string{"cat", "/proc/self/status"}), withArg(nil))

		pod := getPod(withCtr(ctr))
		err := generatePodKubeYaml(pod, kubeYaml)
		Expect(err).To(BeNil())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod)})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.ExitCode()).To(Equal(0))
		Expect(inspect.OutputToString()).To(ContainSubstring(capAdd))
	})

	It("podman play kube cap drop", func() {
		capDrop := "CAP_CHOWN"
		ctr := getCtr(withCapDrop([]string{capDrop}))

		pod := getPod(withCtr(ctr))
		err := generatePodKubeYaml(pod, kubeYaml)
		Expect(err).To(BeNil())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod)})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.ExitCode()).To(Equal(0))
		Expect(inspect.OutputToString()).To(ContainSubstring(capDrop))
	})

	It("podman play kube no security context", func() {
		// expect play kube to not fail if no security context is specified
		pod := getPod(withCtr(getCtr(withSecurityContext(false))))
		err := generatePodKubeYaml(pod, kubeYaml)
		Expect(err).To(BeNil())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod)})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.ExitCode()).To(Equal(0))
	})

	It("podman play kube seccomp container level", func() {
		// expect play kube is expected to set a seccomp label if it's applied as an annotation
		jsonFile, err := podmanTest.CreateSeccompJson(seccompPwdEPERM)
		if err != nil {
			fmt.Println(err)
			Skip("Failed to prepare seccomp.json for test.")
		}

		ctrAnnotation := "container.seccomp.security.alpha.kubernetes.io/" + defaultCtrName
		ctr := getCtr(withCmd([]string{"pwd"}), withArg(nil))

		pod := getPod(withCtr(ctr), withAnnotation(ctrAnnotation, "localhost/"+filepath.Base(jsonFile)))
		err = generatePodKubeYaml(pod, kubeYaml)
		Expect(err).To(BeNil())

		// CreateSeccompJson will put the profile into podmanTest.TempDir. Use --seccomp-profile-root to tell play kube where to look
		kube := podmanTest.Podman([]string{"play", "kube", "--seccomp-profile-root", podmanTest.TempDir, kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		logs := podmanTest.Podman([]string{"logs", getCtrNameInPod(pod)})
		logs.WaitWithDefaultTimeout()
		Expect(logs.ExitCode()).To(Equal(0))
		Expect(logs.OutputToString()).To(ContainSubstring("Operation not permitted"))
	})

	It("podman play kube seccomp pod level", func() {
		// expect play kube is expected to set a seccomp label if it's applied as an annotation
		jsonFile, err := podmanTest.CreateSeccompJson(seccompPwdEPERM)
		if err != nil {
			fmt.Println(err)
			Skip("Failed to prepare seccomp.json for test.")
		}
		defer os.Remove(jsonFile)

		ctr := getCtr(withCmd([]string{"pwd"}), withArg(nil))

		pod := getPod(withCtr(ctr), withAnnotation("seccomp.security.alpha.kubernetes.io/pod", "localhost/"+filepath.Base(jsonFile)))
		err = generatePodKubeYaml(pod, kubeYaml)
		Expect(err).To(BeNil())

		// CreateSeccompJson will put the profile into podmanTest.TempDir. Use --seccomp-profile-root to tell play kube where to look
		kube := podmanTest.Podman([]string{"play", "kube", "--seccomp-profile-root", podmanTest.TempDir, kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		logs := podmanTest.Podman([]string{"logs", getCtrNameInPod(pod)})
		logs.WaitWithDefaultTimeout()
		Expect(logs.ExitCode()).To(Equal(0))
		Expect(logs.OutputToString()).To(ContainSubstring("Operation not permitted"))
	})

	It("podman play kube with pull policy of never should be 125", func() {
		ctr := getCtr(withPullPolicy("never"), withImage(BB_GLIBC))
		err := generatePodKubeYaml(getPod(withCtr(ctr)), kubeYaml)
		Expect(err).To(BeNil())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(125))
	})

	It("podman play kube with pull policy of missing", func() {
		ctr := getCtr(withPullPolicy("missing"), withImage(BB))
		err := generatePodKubeYaml(getPod(withCtr(ctr)), kubeYaml)
		Expect(err).To(BeNil())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))
	})

	It("podman play kube with pull always", func() {
		oldBB := "docker.io/library/busybox:1.30.1"
		pull := podmanTest.Podman([]string{"pull", oldBB})
		pull.WaitWithDefaultTimeout()

		tag := podmanTest.Podman([]string{"tag", oldBB, BB})
		tag.WaitWithDefaultTimeout()
		Expect(tag.ExitCode()).To(BeZero())

		rmi := podmanTest.Podman([]string{"rmi", oldBB})
		rmi.WaitWithDefaultTimeout()
		Expect(rmi.ExitCode()).To(BeZero())

		inspect := podmanTest.Podman([]string{"inspect", BB})
		inspect.WaitWithDefaultTimeout()
		oldBBinspect := inspect.InspectImageJSON()

		ctr := getCtr(withPullPolicy("always"), withImage(BB))
		err := generatePodKubeYaml(getPod(withCtr(ctr)), kubeYaml)
		Expect(err).To(BeNil())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		inspect = podmanTest.Podman([]string{"inspect", BB})
		inspect.WaitWithDefaultTimeout()
		newBBinspect := inspect.InspectImageJSON()
		Expect(oldBBinspect[0].Digest).To(Not(Equal(newBBinspect[0].Digest)))
	})

	It("podman play kube with latest image should always pull", func() {
		oldBB := "docker.io/library/busybox:1.30.1"
		pull := podmanTest.Podman([]string{"pull", oldBB})
		pull.WaitWithDefaultTimeout()

		tag := podmanTest.Podman([]string{"tag", oldBB, BB})
		tag.WaitWithDefaultTimeout()
		Expect(tag.ExitCode()).To(BeZero())

		rmi := podmanTest.Podman([]string{"rmi", oldBB})
		rmi.WaitWithDefaultTimeout()
		Expect(rmi.ExitCode()).To(BeZero())

		inspect := podmanTest.Podman([]string{"inspect", BB})
		inspect.WaitWithDefaultTimeout()
		oldBBinspect := inspect.InspectImageJSON()

		ctr := getCtr(withImage(BB))
		err := generatePodKubeYaml(getPod(withCtr(ctr)), kubeYaml)
		Expect(err).To(BeNil())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		inspect = podmanTest.Podman([]string{"inspect", BB})
		inspect.WaitWithDefaultTimeout()
		newBBinspect := inspect.InspectImageJSON()
		Expect(oldBBinspect[0].Digest).To(Not(Equal(newBBinspect[0].Digest)))
	})

	It("podman play kube with image data", func() {
		testyaml := `
apiVersion: v1
kind: Pod
metadata:
  name: demo_pod
spec:
  containers:
  - image: demo
    name: demo_kube
`
		pull := podmanTest.Podman([]string{"create", "--workdir", "/etc", "--name", "newBB", "--label", "key1=value1", "alpine"})

		pull.WaitWithDefaultTimeout()
		Expect(pull.ExitCode()).To(BeZero())

		c := podmanTest.Podman([]string{"commit", "-c", "STOPSIGNAL=51", "newBB", "demo"})
		c.WaitWithDefaultTimeout()
		Expect(c.ExitCode()).To(Equal(0))

		conffile := filepath.Join(podmanTest.TempDir, "kube.yaml")
		tempdir, err = CreateTempDirInTempDir()
		Expect(err).To(BeNil())

		err := ioutil.WriteFile(conffile, []byte(testyaml), 0755)
		Expect(err).To(BeNil())

		kube := podmanTest.Podman([]string{"play", "kube", conffile})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		inspect := podmanTest.Podman([]string{"inspect", "demo_pod-demo_kube"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.ExitCode()).To(Equal(0))

		ctr := inspect.InspectContainerToJSON()
		Expect(ctr[0].Config.WorkingDir).To(ContainSubstring("/etc"))
		Expect(ctr[0].Config.Labels["key1"]).To(ContainSubstring("value1"))
		Expect(ctr[0].Config.Labels["key1"]).To(ContainSubstring("value1"))
		Expect(ctr[0].Config.StopSignal).To(Equal(uint(51)))
	})

	// Deployment related tests
	It("podman play kube deployment 1 replica test correct command", func() {
		deployment := getDeployment()
		err := generateDeploymentKubeYaml(deployment, kubeYaml)
		Expect(err).To(BeNil())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		podNames := getPodNamesInDeployment(deployment)
		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(&podNames[0]), "--format", "'{{ .Config.Cmd }}'"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.ExitCode()).To(Equal(0))
		// yaml's command shuold override the image's Entrypoint
		correctCmd := "[" + strings.Join(defaultCtrCmd, " ") + " " + strings.Join(defaultCtrArg, " ")
		Expect(inspect.OutputToString()).To(ContainSubstring(correctCmd))
	})

	It("podman play kube deployment more than 1 replica test correct command", func() {
		var i, numReplicas int32
		numReplicas = 5
		deployment := getDeployment(withReplicas(numReplicas))
		err := generateDeploymentKubeYaml(deployment, kubeYaml)
		Expect(err).To(BeNil())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		podNames := getPodNamesInDeployment(deployment)
		correctCmd := "[" + strings.Join(defaultCtrCmd, " ") + " " + strings.Join(defaultCtrArg, " ")
		for i = 0; i < numReplicas; i++ {
			inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(&podNames[i]), "--format", "'{{ .Config.Cmd }}'"})
			inspect.WaitWithDefaultTimeout()
			Expect(inspect.ExitCode()).To(Equal(0))
			Expect(inspect.OutputToString()).To(ContainSubstring(correctCmd))
		}
	})
})
