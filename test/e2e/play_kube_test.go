package integration

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	. "github.com/containers/podman/v2/test/utils"
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
  name: {{ .Name }}
  labels:
    app: {{ .Name }}
{{ with .Labels }}
  {{ range $key, $value := . }}
    {{ $key }}: {{ $value }}
  {{ end }}
{{ end }}
{{ with .Annotations }}
  annotations:
  {{ range $key, $value := . }}
    {{ $key }}: {{ $value }}
  {{ end }}
{{ end }}

spec:
  restartPolicy: {{ .RestartPolicy }}
  hostname: {{ .Hostname }}
  hostAliases:
{{ range .HostAliases }}
  - hostnames:
  {{ range .HostName }}
    - {{ . }}
  {{ end }}
    ip: {{ .IP }}
{{ end }}
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
    ports:
    - containerPort: {{ .Port }}
      hostIP: {{ .HostIP }}
      hostPort: {{ .Port }}
      protocol: TCP
    workingDir: /
    volumeMounts:
    {{ if .VolumeMount }}
    - name: {{.VolumeName}}
      mountPath: {{ .VolumeMountPath }}
      readonly: {{.VolumeReadOnly}}
      {{ end }}
    {{ end }}
  {{ end }}
{{ end }}
{{ with .Volumes }}
  volumes:
  {{ range . }}
  - name: {{ .Name }}
    hostPath:
      path: {{ .Path }}
      type: {{ .Type }}
  {{ end }}
{{ end }}
status: {}
`

var deploymentYamlTemplate = `
apiVersion: v1
kind: Deployment
metadata:
  creationTimestamp: "2019-07-17T14:44:08Z"
  name: {{ .Name }}
  labels:
    app: {{ .Name }}
{{ with .Labels }}
  {{ range $key, $value := . }}
    {{ $key }}: {{ $value }}
  {{ end }}
{{ end }}
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
        {{- with .Labels }}{{ range $key, $value := . }}
        {{ $key }}: {{ $value }}
        {{- end }}{{ end }}
      {{- with .Annotations }}
      annotations:
      {{- range $key, $value := . }}
        {{ $key }}: {{ $value }}
      {{- end }}
      {{- end }}
    spec:
      restartPolicy: {{ .RestartPolicy }}
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
	defaultVolName        = "testVol"
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
	Name          string
	RestartPolicy string
	Hostname      string
	HostAliases   []HostAlias
	Ctrs          []*Ctr
	Volumes       []*Volume
	Labels        map[string]string
	Annotations   map[string]string
}

type HostAlias struct {
	IP       string
	HostName []string
}

// getPod takes a list of podOptions and returns a pod with sane defaults
// and the configured options
// if no containers are added, it will add the default container
func getPod(options ...podOption) *Pod {
	p := Pod{
		Name:          defaultPodName,
		RestartPolicy: "Never",
		Hostname:      "",
		HostAliases:   nil,
		Ctrs:          make([]*Ctr, 0),
		Volumes:       make([]*Volume, 0),
		Labels:        make(map[string]string),
		Annotations:   make(map[string]string),
	}
	for _, option := range options {
		option(&p)
	}
	if len(p.Ctrs) == 0 {
		p.Ctrs = []*Ctr{getCtr()}
	}
	return &p
}

type podOption func(*Pod)

func withPodName(name string) podOption {
	return func(pod *Pod) {
		pod.Name = name
	}
}

func withHostname(h string) podOption {
	return func(pod *Pod) {
		pod.Hostname = h
	}
}

func withHostAliases(ip string, host []string) podOption {
	return func(pod *Pod) {
		pod.HostAliases = append(pod.HostAliases, HostAlias{
			IP:       ip,
			HostName: host,
		})
	}
}

func withCtr(c *Ctr) podOption {
	return func(pod *Pod) {
		pod.Ctrs = append(pod.Ctrs, c)
	}
}

func withRestartPolicy(policy string) podOption {
	return func(pod *Pod) {
		pod.RestartPolicy = policy
	}
}

func withLabel(k, v string) podOption {
	return func(pod *Pod) {
		pod.Labels[k] = v
	}
}

func withAnnotation(k, v string) podOption {
	return func(pod *Pod) {
		pod.Annotations[k] = v
	}
}

func withVolume(v *Volume) podOption {
	return func(pod *Pod) {
		pod.Volumes = append(pod.Volumes, v)
	}
}

// Deployment describes the options a kube yaml can be configured at deployment level
type Deployment struct {
	Name        string
	Replicas    int32
	Labels      map[string]string
	Annotations map[string]string
	PodTemplate *Pod
}

func getDeployment(options ...deploymentOption) *Deployment {
	d := Deployment{
		Name:        defaultDeploymentName,
		Replicas:    1,
		Labels:      make(map[string]string),
		Annotations: make(map[string]string),
		PodTemplate: getPod(),
	}
	for _, option := range options {
		option(&d)
	}

	return &d
}

type deploymentOption func(*Deployment)

func withDeploymentLabel(k, v string) deploymentOption {
	return func(deployment *Deployment) {
		deployment.Labels[k] = v
	}
}

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
	HostIP          string
	Port            string
	VolumeMount     bool
	VolumeMountPath string
	VolumeName      string
	VolumeReadOnly  bool
}

// getCtr takes a list of ctrOptions and returns a Ctr with sane defaults
// and the configured options
func getCtr(options ...ctrOption) *Ctr {
	c := Ctr{defaultCtrName, defaultCtrImage, defaultCtrCmd, defaultCtrArg, true, false, nil, nil, "", "", "", false, "", "", false}
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

func withHostIP(ip string, port string) ctrOption {
	return func(c *Ctr) {
		c.HostIP = ip
		c.Port = port
	}
}

func withVolumeMount(mountPath string, readonly bool) ctrOption {
	return func(c *Ctr) {
		c.VolumeMountPath = mountPath
		c.VolumeName = defaultVolName
		c.VolumeReadOnly = readonly
		c.VolumeMount = true
	}
}

func getCtrNameInPod(pod *Pod) string {
	return fmt.Sprintf("%s-%s", pod.Name, defaultCtrName)
}

type Volume struct {
	Name string
	Path string
	Type string
}

// getVolume takes a type and a location for a volume
// giving it a default name of volName
func getVolume(vType, vPath string) *Volume {
	return &Volume{
		Name: defaultVolName,
		Path: vPath,
		Type: vType,
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

	It("podman play kube test restartPolicy", func() {
		// podName,  set,  expect
		testSli := [][]string{
			{"testPod1", "", "always"}, // Default eqaul to always
			{"testPod2", "Always", "always"},
			{"testPod3", "OnFailure", "on-failure"},
			{"testPod4", "Never", "no"},
		}
		for _, v := range testSli {
			pod := getPod(withPodName(v[0]), withRestartPolicy(v[1]))
			err := generatePodKubeYaml(pod, kubeYaml)
			Expect(err).To(BeNil())

			kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
			kube.WaitWithDefaultTimeout()
			Expect(kube.ExitCode()).To(Equal(0))

			inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod), "--format", "{{.HostConfig.RestartPolicy.Name}}"})
			inspect.WaitWithDefaultTimeout()
			Expect(inspect.ExitCode()).To(Equal(0))
			Expect(inspect.OutputToString()).To(Equal(v[2]))
		}
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

	It("podman play kube test HostAliases", func() {
		pod := getPod(withHostAliases("192.168.1.2", []string{
			"test1.podman.io",
			"test2.podman.io",
		}),
			withHostAliases("192.168.1.3", []string{
				"test3.podman.io",
				"test4.podman.io",
			}),
		)
		err := generatePodKubeYaml(pod, kubeYaml)
		Expect(err).To(BeNil())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod), "--format", "{{ .HostConfig.ExtraHosts }}"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.ExitCode()).To(Equal(0))
		Expect(inspect.OutputToString()).
			To(Equal("[test1.podman.io:192.168.1.2 test2.podman.io:192.168.1.2 test3.podman.io:192.168.1.3 test4.podman.io:192.168.1.3]"))
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
		SkipIfRemote("FIXME This is broken")
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
		SkipIfRemote("FIXME: This should work with --remote")
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

	It("podman play kube test with network portbindings", func() {
		ip := "127.0.0.100"
		port := "5000"
		ctr := getCtr(withHostIP(ip, port), withImage(BB))

		pod := getPod(withCtr(ctr))
		err := generatePodKubeYaml(pod, kubeYaml)
		Expect(err).To(BeNil())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		inspect := podmanTest.Podman([]string{"port", getCtrNameInPod(pod)})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.ExitCode()).To(Equal(0))
		Expect(inspect.OutputToString()).To(Equal("5000/tcp -> 127.0.0.100:5000"))
	})

	It("podman play kube test with non-existent empty HostPath type volume", func() {
		hostPathLocation := filepath.Join(tempdir, "file")

		pod := getPod(withVolume(getVolume(`""`, hostPathLocation)))
		err := generatePodKubeYaml(pod, kubeYaml)
		Expect(err).To(BeNil())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).NotTo(Equal(0))
	})

	It("podman play kube test with empty HostPath type volume", func() {
		hostPathLocation := filepath.Join(tempdir, "file")
		f, err := os.Create(hostPathLocation)
		Expect(err).To(BeNil())
		f.Close()

		pod := getPod(withVolume(getVolume(`""`, hostPathLocation)))
		err = generatePodKubeYaml(pod, kubeYaml)
		Expect(err).To(BeNil())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))
	})

	It("podman play kube test with non-existent File HostPath type volume", func() {
		hostPathLocation := filepath.Join(tempdir, "file")

		pod := getPod(withVolume(getVolume("File", hostPathLocation)))
		err := generatePodKubeYaml(pod, kubeYaml)
		Expect(err).To(BeNil())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).NotTo(Equal(0))
	})

	It("podman play kube test with File HostPath type volume", func() {
		hostPathLocation := filepath.Join(tempdir, "file")
		f, err := os.Create(hostPathLocation)
		Expect(err).To(BeNil())
		f.Close()

		pod := getPod(withVolume(getVolume("File", hostPathLocation)))
		err = generatePodKubeYaml(pod, kubeYaml)
		Expect(err).To(BeNil())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))
	})

	It("podman play kube test with FileOrCreate HostPath type volume", func() {
		hostPathLocation := filepath.Join(tempdir, "file")

		pod := getPod(withVolume(getVolume("FileOrCreate", hostPathLocation)))
		err := generatePodKubeYaml(pod, kubeYaml)
		Expect(err).To(BeNil())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		// the file should have been created
		_, err = os.Stat(hostPathLocation)
		Expect(err).To(BeNil())
	})

	It("podman play kube test with DirectoryOrCreate HostPath type volume", func() {
		hostPathLocation := filepath.Join(tempdir, "file")

		pod := getPod(withVolume(getVolume("DirectoryOrCreate", hostPathLocation)))
		err := generatePodKubeYaml(pod, kubeYaml)
		Expect(err).To(BeNil())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		// the file should have been created
		st, err := os.Stat(hostPathLocation)
		Expect(err).To(BeNil())
		Expect(st.Mode().IsDir()).To(Equal(true))
	})

	It("podman play kube test with Socket HostPath type volume should fail if not socket", func() {
		hostPathLocation := filepath.Join(tempdir, "file")
		f, err := os.Create(hostPathLocation)
		Expect(err).To(BeNil())
		f.Close()

		pod := getPod(withVolume(getVolume("Socket", hostPathLocation)))
		err = generatePodKubeYaml(pod, kubeYaml)
		Expect(err).To(BeNil())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).NotTo(Equal(0))
	})

	It("podman play kube test with read only volume", func() {
		hostPathLocation := filepath.Join(tempdir, "file")
		f, err := os.Create(hostPathLocation)
		Expect(err).To(BeNil())
		f.Close()

		ctr := getCtr(withVolumeMount(hostPathLocation, true), withImage(BB))
		pod := getPod(withVolume(getVolume("File", hostPathLocation)), withCtr(ctr))
		err = generatePodKubeYaml(pod, kubeYaml)
		Expect(err).To(BeNil())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod), "--format", "'{{.HostConfig.Binds}}'"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.ExitCode()).To(Equal(0))

		correct := fmt.Sprintf("%s:%s:%s", hostPathLocation, hostPathLocation, "ro")
		Expect(inspect.OutputToString()).To(ContainSubstring(correct))
	})

	It("podman play kube applies labels to pods", func() {
		var numReplicas int32 = 5
		expectedLabelKey := "key1"
		expectedLabelValue := "value1"
		deployment := getDeployment(
			withReplicas(numReplicas),
			withPod(getPod(withLabel(expectedLabelKey, expectedLabelValue))),
		)
		err := generateDeploymentKubeYaml(deployment, kubeYaml)
		Expect(err).To(BeNil())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube.ExitCode()).To(Equal(0))

		correctLabels := expectedLabelKey + ":" + expectedLabelValue
		for _, pod := range getPodNamesInDeployment(deployment) {
			inspect := podmanTest.Podman([]string{"pod", "inspect", pod.Name, "--format", "'{{ .Labels }}'"})
			inspect.WaitWithDefaultTimeout()
			Expect(inspect.ExitCode()).To(Equal(0))
			Expect(inspect.OutputToString()).To(ContainSubstring(correctLabels))
		}
	})
})
