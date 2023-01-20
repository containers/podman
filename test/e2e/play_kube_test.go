package integration

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/bindings"
	"github.com/containers/podman/v4/pkg/bindings/play"
	"github.com/containers/podman/v4/pkg/util"
	. "github.com/containers/podman/v4/test/utils"
	"github.com/containers/podman/v4/utils"
	"github.com/containers/storage/pkg/stringid"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	. "github.com/onsi/gomega/gexec"
	"github.com/opencontainers/selinux/go-selinux"
)

var secretYaml = `
apiVersion: v1
kind: Secret
metadata:
  name: newsecret
type: Opaque
data:
  username: dXNlcg==
  password: NTRmNDFkMTJlOGZh
`

var complexSecretYaml = `
apiVersion: v1
kind: Secret
metadata:
  name: newsecrettwo
type: Opaque
data:
  username: Y2RvZXJu
  password: dGVzdGluZ3Rlc3RpbmcK
  note: a3ViZSBzZWNyZXRzIGFyZSBjb29sIQo=
stringData:
  plain_note: This is a test
`

var secretPodYaml = `
apiVersion: v1
kind: Pod
metadata:
  name: mypod
spec:
  containers:
    - name: myctr
      image: quay.io/libpod/alpine_nginx:latest
      volumeMounts:
        - name: foo
          mountPath: /etc/foo
          readOnly: true
  volumes:
    - name: foo
      secret:
        secretName: newsecret
        optional: false
`

var secretPodYamlTwo = `
apiVersion: v1
kind: Pod
metadata:
  name: mypod2
spec:
  containers:
    - name: myctr
      image: quay.io/libpod/alpine_nginx:latest
      volumeMounts:
        - name: foo
          mountPath: /etc/foo
          readOnly: true
        - name: bar
          mountPath: /etc/bar
          readOnly: true
        - name: baz
          mountPath: /etc/baz
          readOnly: true
  volumes:
    - name: foo
      secret:
        secretName: newsecret
        optional: false
    - name: bar
      secret:
        secretName: newsecrettwo
        optional: false
    - name: baz
      secret:
        secretName: newsecrettwo
        optional: false
`

var optionalExistingSecretPodYaml = `
apiVersion: v1
kind: Pod
metadata:
  name: mypod
spec:
  containers:
    - name: myctr
      image: quay.io/libpod/alpine_nginx:latest
      volumeMounts:
        - name: foo
          mountPath: /etc/foo
          readOnly: true
  volumes:
    - name: foo
      secret:
        secretName: newsecret
        optional: true
`

var optionalNonExistingSecretPodYaml = `
apiVersion: v1
kind: Pod
metadata:
  name: mypod
spec:
  containers:
    - name: myctr
      image: quay.io/libpod/alpine_nginx:latest
      volumeMounts:
        - name: foo
          mountPath: /etc/foo
          readOnly: true
  volumes:
    - name: foo
      secret:
        secretName: oldsecret
        optional: true
`

var noOptionalExistingSecretPodYaml = `
apiVersion: v1
kind: Pod
metadata:
  name: mypod
spec:
  containers:
    - name: myctr
      image: quay.io/libpod/alpine_nginx:latest
      volumeMounts:
        - name: foo
          mountPath: /etc/foo
          readOnly: true
  volumes:
    - name: foo
      secret:
        secretName: newsecret
`

var noOptionalNonExistingSecretPodYaml = `
apiVersion: v1
kind: Pod
metadata:
  name: mypod
spec:
  containers:
    - name: myctr
      image: quay.io/libpod/alpine_nginx:latest
      volumeMounts:
        - name: foo
          mountPath: /etc/foo
          readOnly: true
  volumes:
    - name: foo
      secret:
        secretName: oldsecret`

var simplePodYaml = `
apiVersion: v1
kind: Pod
metadata:
  name: libpod-test
spec:
  containers:
  - image: quay.io/libpod/alpine_nginx:latest
    command:
      - sleep
      - "3600"`

var unknownKindYaml = `
apiVersion: v1
kind: UnknownKind
metadata:
  labels:
    app: app1
  name: unknown
spec:
  hostname: unknown
`

var workdirSymlinkPodYaml = `
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: test-symlink
  name: test-symlink
spec:
  containers:
  - image: test-symlink
    name: test-symlink
    resources: {}
  restartPolicy: Never
`

var podnameEqualsContainerNameYaml = `
apiVersion: v1
kind: Pod
metadata:
  name: podnameEqualsContainerNameYaml
spec:
  containers:
  - name: podnameEqualsContainerNameYaml
    image: quay.io/libpod/alpine:latest
`

var podWithoutAName = `
apiVersion: v1
kind: Pod
spec:
  containers:
  - name: podDoesntHaveAName
    image: quay.io/libpod/alpine:latest
    ports:
    - containerPort: 80
`

var subpathTestNamedVolume = `
apiVersion: v1
kind: Pod
metadata:
  name: testpod
spec:
    containers:
    - name: testctr
      image: quay.io/libpod/alpine_nginx:latest
      command:
        - sleep
        - inf
      volumeMounts:
      - mountPath: /var
        name: testing
        subPath: testing/onlythis
    volumes:
    - name: testing
      persistentVolumeClaim:
        claimName: testvol
`

var checkInfraImagePodYaml = `
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: check-infra-image
  name: check-infra-image
spec:
  containers:
    - name: alpine
      image: quay.io/libpod/alpine:latest
      command:
        - sleep
        - 24h
status: {}
`

var podWithoutConfigMapDefined = `
apiVersion: v1
kind: Pod
metadata:
  name: testpod1
spec:
  containers:
    - name: alpine
      image: quay.io/libpod/alpine:latest
      volumeMounts:
        - name: mycm
          mountPath: /mycm
  volumes:
    - name: mycm
      configMap:
        name: mycm
`

var sharedNamespacePodYaml = `
apiVersion: v1
kind: Pod
metadata:
  creationTimestamp: "2021-05-07T17:25:01Z"
  labels:
    app: testpod1
  name: testpod1
spec:
  containers:
  - command:
    - top
    - -d
    - "1.5"
    env:
    - name: HOSTNAME
      value: label-pod
    image: quay.io/libpod/alpine:latest
    name: alpine
    resources: {}
    securityContext:
      allowPrivilegeEscalation: true
      capabilities: {}
      privileged: false
      readOnlyRootFilesystem: false
      seLinuxOptions: {}
    workingDir: /
  dnsConfig: {}
  restartPolicy: Never
  shareProcessNamespace: true
status: {}
`
var livenessProbePodYaml = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: liveness-probe
  labels:
    app: alpine
spec:
  replicas: 1
  selector:
    matchLabels:
      app: alpine
  template:
    metadata:
      labels:
        app: alpine
    spec:
      containers:
      - command:
        - top
        - -d
        - "1.5"
        name: alpine
        image: quay.io/libpod/alpine:latest
        livenessProbe:
          exec:
            command:
            - echo
            - hello
          initialDelaySeconds: 5
          periodSeconds: 5
`
var livenessProbeUnhealthyPodYaml = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: liveness-unhealthy-probe
  labels:
    app: alpine
spec:
  replicas: 1
  selector:
    matchLabels:
      app: alpine
  template:
    metadata:
      labels:
        app: alpine
    spec:
      restartPolicy: Never
      containers:
      - command:
        - top
        - -d
        - "1.5"
        name: alpine
        image: quay.io/libpod/alpine:latest
        livenessProbe:
          exec:
            command:
            - cat
            - /randomfile
          initialDelaySeconds: 0
          periodSeconds: 1
`

var startupProbePodYaml = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: startup-healthy-probe
  labels:
    app: alpine
spec:
  replicas: 1
  selector:
    matchLabels:
      app: alpine
  template:
    metadata:
      labels:
        app: alpine
    spec:
      restartPolicy: Never
      containers:
      - command:
        - top
        - -d
        - "1.5"
        name: alpine
        image: quay.io/libpod/alpine:latest
        startupProbe:
          exec:
            command:
            - /bin/sh
            - -c
            - cat /testfile
          initialDelaySeconds: 0
          periodSeconds: 1
        livenessProbe:
          exec:
            command:
            - echo
            - liveness probe
          initialDelaySeconds: 0
          periodSeconds: 1
`

var selinuxLabelPodYaml = `
apiVersion: v1
kind: Pod
metadata:
  creationTimestamp: "2021-02-02T22:18:20Z"
  labels:
    app: label-pod
  name: label-pod
spec:
  containers:
  - command:
    - top
    - -d
    - "1.5"
    env:
    - name: HOSTNAME
      value: label-pod
    image: quay.io/libpod/alpine:latest
    name: test
    securityContext:
      allowPrivilegeEscalation: true
      privileged: false
      readOnlyRootFilesystem: false
      seLinuxOptions:
        user: unconfined_u
        role: system_r
        type: spc_t
        level: s0
    workingDir: /
status: {}
`

var configMapYamlTemplate = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Name }}
data:
{{ with .Data }}
  {{ range $key, $value := . }}
    {{ $key }}: {{ $value }}
  {{ end }}
{{ end }}
`

var persistentVolumeClaimYamlTemplate = `
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: {{ .Name }}
{{ with .Annotations }}
  annotations:
  {{ range $key, $value := . }}
    {{ $key }}: {{ $value }}
  {{ end }}
{{ end }}
spec:
  accessModes:
    - "ReadWriteOnce"
  resources:
    requests:
      storage: "1Gi"
  storageClassName: default
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
  hostNetwork: {{ .HostNetwork }}
{{ if .HostUsers }}
  hostUsers: {{ .HostUsers }}
{{ end }}
  hostAliases:
{{ range .HostAliases }}
  - hostnames:
  {{ range .HostName }}
    - {{ . }}
  {{ end }}
    ip: {{ .IP }}
{{ end }}
  initContainers:
{{ with .InitCtrs }}
  {{ range . }}
  - command:
    {{ range .Cmd }}
    - {{.}}
    {{ end }}
    image: {{ .Image }}
    name: {{ .Name }}
  {{ end }}
{{ end }}
{{ if .SecurityContext }}
  securityContext:
    {{ if .RunAsUser }}runAsUser: {{ .RunAsUser }}{{- end }}
    {{ if .RunAsGroup }}runAsGroup: {{ .RunAsGroup }}{{- end }}
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
    - name: HOSTNAME
    {{ range .Env }}
    - name: {{ .Name }}
    {{ if (eq .ValueFrom "configmap") }}
      valueFrom:
        configMapKeyRef:
          name: {{ .RefName }}
          key: {{ .RefKey }}
          optional: {{ .Optional }}
    {{ end }}
    {{ if (eq .ValueFrom "secret") }}
      valueFrom:
        secretKeyRef:
          name: {{ .RefName }}
          key: {{ .RefKey }}
          optional: {{ .Optional }}
    {{ end }}
    {{ if (eq .ValueFrom "") }}
      value: {{ .Value }}
    {{ end }}
    {{ end }}
    {{ with .EnvFrom}}
    envFrom:
    {{ range . }}
    {{ if (eq .From "configmap") }}
    - configMapRef:
        name: {{ .Name }}
        optional: {{ .Optional }}
    {{ end }}
    {{ if (eq .From "secret") }}
    - secretRef:
        name: {{ .Name }}
        optional: {{ .Optional }}
    {{ end }}
    {{ end }}
    {{ end }}
    image: {{ .Image }}
    name: {{ .Name }}
    imagePullPolicy: {{ .PullPolicy }}
    {{- if or .CPURequest .CPULimit .MemoryRequest .MemoryLimit }}
    resources:
      {{- if or .CPURequest .MemoryRequest }}
      requests:
        {{if .CPURequest }}cpu: {{ .CPURequest }}{{ end }}
        {{if .MemoryRequest }}memory: {{ .MemoryRequest }}{{ end }}
      {{- end }}
      {{- if or .CPULimit .MemoryLimit }}
      limits:
        {{if .CPULimit }}cpu: {{ .CPULimit }}{{ end }}
        {{if .MemoryLimit }}memory: {{ .MemoryLimit }}{{ end }}
      {{- end }}
    {{- end }}
    {{ if .SecurityContext }}
    securityContext:
      {{ if .RunAsUser }}runAsUser: {{ .RunAsUser }}{{- end }}
      {{ if .RunAsGroup }}runAsGroup: {{ .RunAsGroup }}{{- end }}
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
      subPath: {{ .VolumeSubPath }}
      readonly: {{.VolumeReadOnly}}
      {{ end }}
    {{ end }}
  {{ end }}
{{ end }}
{{ with .Volumes }}
  volumes:
  {{ range . }}
  - name: {{ .Name }}
    {{- if (eq .VolumeType "EmptyDir") }}
    emptyDir: {}
    {{- end }}
    {{- if (eq .VolumeType "HostPath") }}
    hostPath:
      path: {{ .HostPath.Path }}
      type: {{ .HostPath.Type }}
    {{- end }}
    {{- if (eq .VolumeType "PersistentVolumeClaim") }}
    persistentVolumeClaim:
      claimName: {{ .PersistentVolumeClaim.ClaimName }}
    {{- end }}
	{{- if (eq .VolumeType "ConfigMap") }}
    configMap:
      name: {{ .ConfigMap.Name }}
      optional: {{ .ConfigMap.Optional }}
      {{- with .ConfigMap.Items }}
      items:
      {{- range . }}
        - key: {{ .key }}
          path: {{ .path }}
    {{- end }}
    {{- end }}
    {{- end }}
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
      hostNetwork: {{ .HostNetwork }}
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
        - name: HOSTNAME
        {{ range .Env }}
        - name: {{ .Name }}
        {{ if (eq .ValueFrom "configmap") }}
          valueFrom:
            configMapKeyRef:
              name: {{ .RefName }}
              key: {{ .RefKey }}
              optional: {{ .Optional }}
        {{ end }}
        {{ if (eq .ValueFrom "secret") }}
          valueFrom:
            secretKeyRef:
              name: {{ .RefName }}
              key: {{ .RefKey }}
              optional: {{ .Optional }}
        {{ end }}
        {{ if (eq .ValueFrom "") }}
          value: {{ .Value }}
        {{ end }}
        {{ end }}
        {{ with .EnvFrom}}
        envFrom:
        {{ range . }}
        {{ if (eq .From "configmap") }}
        - configMapRef:
            name: {{ .Name }}
            optional: {{ .Optional }}
        {{ end }}
        {{ if (eq .From "secret") }}
        - secretRef:
            name: {{ .Name }}
            optional: {{ .Optional }}
        {{ end }}
        {{ end }}
        {{ end }}
        image: {{ .Image }}
        name: {{ .Name }}
        imagePullPolicy: {{ .PullPolicy }}
        {{- if or .CPURequest .CPULimit .MemoryRequest .MemoryLimit }}
        resources:
          {{- if or .CPURequest .MemoryRequest }}
          requests:
            {{if .CPURequest }}cpu: {{ .CPURequest }}{{ end }}
            {{if .MemoryRequest }}memory: {{ .MemoryRequest }}{{ end }}
          {{- end }}
          {{- if or .CPULimit .MemoryLimit }}
          limits:
            {{if .CPULimit }}cpu: {{ .CPULimit }}{{ end }}
            {{if .MemoryLimit }}memory: {{ .MemoryLimit }}{{ end }}
          {{- end }}
        {{- end }}
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
        {{- if (eq .VolumeType "HostPath") }}
        hostPath:
          path: {{ .HostPath.Path }}
          type: {{ .HostPath.Type }}
        {{- end }}
        {{- if (eq .VolumeType "PersistentVolumeClaim") }}
        persistentVolumeClaim:
          claimName: {{ .PersistentVolumeClaim.ClaimName }}
        {{- end }}
      {{ end }}
    {{ end }}
{{ end }}
`

var publishPortsPodWithoutPorts = `
apiVersion: v1
kind: Pod
metadata:
  name: nginx
spec:
  containers:
  - name: nginx
    image: quay.io/libpod/alpine_nginx:latest
`

var publishPortsPodWithContainerPort = `
apiVersion: v1
kind: Pod
metadata:
  name: nginx
spec:
  containers:
  - name: nginx
    image: quay.io/libpod/alpine_nginx:latest
    ports:
    - containerPort: 80
`

var publishPortsPodWithContainerHostPort = `
apiVersion: v1
kind: Pod
metadata:
  name: nginx
spec:
  containers:
  - name: nginx
    image: quay.io/libpod/alpine_nginx:latest
    ports:
    - containerPort: 80
      hostPort: 19001
`

var publishPortsEchoWithHostPortUDP = `
apiVersion: v1
kind: Pod
metadata:
  name: network-echo
spec:
  containers:
  - name: udp-echo
    image: quay.io/libpod/busybox:latest
    command:
    - "/bin/sh"
    - "-c"
    - "nc -ulk -p 19008 -e /bin/cat"
    ports:
    - containerPort: 19008
      hostPort: 19009
      protocol: udp
  - name: tcp-echo
    image: quay.io/libpod/busybox:latest
    command:
    - "/bin/sh"
    - "-c"
    - "nc -lk -p 19008 -e /bin/cat"
`

var publishPortsEchoWithHostPortTCP = `
apiVersion: v1
kind: Pod
metadata:
  name: network-echo
spec:
  containers:
  - name: udp-echo
    image: quay.io/libpod/busybox:latest
    command:
    - "/bin/sh"
    - "-c"
    - "nc -ulk -p 19008 -e /bin/cat"
  - name: tcp-echo
    image: quay.io/libpod/busybox:latest
    command:
    - "/bin/sh"
    - "-c"
    - "nc -lk -p 19008 -e /bin/cat"
    ports:
    - containerPort: 19008
      hostPort: 19011
      protocol: tcp
`

var podWithHostPIDDefined = `
apiVersion: v1
kind: Pod
metadata:
  name: test-hostpid
spec:
  hostPID: true
  containers:
  - name: alpine
    image: quay.io/libpod/alpine:latest
    command: ['sh', '-c', 'echo $$']
`

var (
	defaultCtrName        = "testCtr"
	defaultCtrCmd         = []string{"top"}
	defaultCtrArg         = []string{"-d", "1.5"}
	defaultCtrImage       = ALPINE
	defaultPodName        = "testPod"
	defaultVolName        = "testVol"
	defaultDeploymentName = "testDeployment"
	defaultConfigMapName  = "testConfigMap"
	defaultPVCName        = "testPVC"
	seccompPwdEPERM       = []byte(`{"defaultAction":"SCMP_ACT_ALLOW","syscalls":[{"name":"getcwd","action":"SCMP_ACT_ERRNO"}]}`)
	// CPU Period in ms
	defaultCPUPeriod = 100
	// Default secret in JSON. Note that the values ("foo" and "bar") are base64 encoded.
	defaultSecret = []byte(`{"FOO":"Zm9v","BAR":"YmFy"}`)
)

// getKubeYaml returns a kubernetes YAML document.
func getKubeYaml(kind string, object interface{}) (string, error) {
	var yamlTemplate string
	templateBytes := &bytes.Buffer{}

	switch kind {
	case "configmap":
		yamlTemplate = configMapYamlTemplate
	case "pod":
		yamlTemplate = podYamlTemplate
	case "deployment":
		yamlTemplate = deploymentYamlTemplate
	case "persistentVolumeClaim":
		yamlTemplate = persistentVolumeClaimYamlTemplate
	default:
		return "", fmt.Errorf("unsupported kubernetes kind")
	}

	t, err := template.New(kind).Parse(yamlTemplate)
	if err != nil {
		return "", err
	}

	if err := t.Execute(templateBytes, object); err != nil {
		return "", err
	}

	return templateBytes.String(), nil
}

// generateKubeYaml writes a kubernetes YAML document.
func generateKubeYaml(kind string, object interface{}, pathname string) error {
	k, err := getKubeYaml(kind, object)
	if err != nil {
		return err
	}

	return writeYaml(k, pathname)
}

// generateMultiDocKubeYaml writes multiple kube objects in one Yaml document.
func generateMultiDocKubeYaml(kubeObjects []string, pathname string) error {
	var multiKube string

	for _, k := range kubeObjects {
		multiKube += "---\n"
		multiKube += k
	}

	return writeYaml(multiKube, pathname)
}

func createSecret(podmanTest *PodmanTestIntegration, name string, value []byte) { //nolint:unparam
	secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
	err := os.WriteFile(secretFilePath, value, 0755)
	Expect(err).ToNot(HaveOccurred())

	secret := podmanTest.Podman([]string{"secret", "create", name, secretFilePath})
	secret.WaitWithDefaultTimeout()
	Expect(secret).Should(Exit(0))
}

// CM describes the options a kube yaml can be configured at configmap level
type CM struct {
	Name string
	Data map[string]string
}

func getConfigMap(options ...configMapOption) *CM {
	cm := CM{
		Name: defaultConfigMapName,
		Data: map[string]string{},
	}

	for _, option := range options {
		option(&cm)
	}

	return &cm
}

type configMapOption func(*CM)

func withConfigMapName(name string) configMapOption {
	return func(configmap *CM) {
		configmap.Name = name
	}
}

func withConfigMapData(k, v string) configMapOption {
	return func(configmap *CM) {
		configmap.Data[k] = v
	}
}

// PVC describes the options a kube yaml can be configured at persistent volume claim level
type PVC struct {
	Name        string
	Annotations map[string]string
}

func getPVC(options ...pvcOption) *PVC {
	pvc := PVC{
		Name:        defaultPVCName,
		Annotations: map[string]string{},
	}

	for _, option := range options {
		option(&pvc)
	}

	return &pvc
}

type pvcOption func(*PVC)

func withPVCName(name string) pvcOption {
	return func(pvc *PVC) {
		pvc.Name = name
	}
}

func withPVCAnnotations(k, v string) pvcOption {
	return func(pvc *PVC) {
		pvc.Annotations[k] = v
	}
}

// Pod describes the options a kube yaml can be configured at pod level
type Pod struct {
	Name            string
	RestartPolicy   string
	Hostname        string
	HostNetwork     bool
	HostUsers       *bool
	HostAliases     []HostAlias
	Ctrs            []*Ctr
	InitCtrs        []*Ctr
	Volumes         []*Volume
	Labels          map[string]string
	Annotations     map[string]string
	SecurityContext bool
	RunAsUser       string
	RunAsGroup      string
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
		HostNetwork:   false,
		HostAliases:   nil,
		Ctrs:          make([]*Ctr, 0),
		InitCtrs:      make([]*Ctr, 0),
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

func withPodSecurityContext(sc bool) podOption {
	return func(p *Pod) {
		p.SecurityContext = sc
	}
}

func withPodRunAsUser(runAsUser string) podOption {
	return func(p *Pod) {
		p.RunAsUser = runAsUser
	}
}

func withPodRunAsGroup(runAsGroup string) podOption {
	return func(p *Pod) {
		p.RunAsGroup = runAsGroup
	}
}

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

func withPodInitCtr(ic *Ctr) podOption {
	return func(pod *Pod) {
		pod.InitCtrs = append(pod.InitCtrs, ic)
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

func withHostNetwork() podOption {
	return func(pod *Pod) {
		pod.HostNetwork = true
	}
}

func withHostUsers(val bool) podOption {
	return func(pod *Pod) {
		pod.HostUsers = &val
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

// getPodNameInDeployment returns the Pod object
// with just its name set, so that it can be passed around
// and into getCtrNameInPod for ease of testing
func getPodNameInDeployment(d *Deployment) Pod {
	p := Pod{}
	p.Name = fmt.Sprintf("%s-pod", d.Name)

	return p
}

// Ctr describes the options a kube yaml can be configured at container level
type Ctr struct {
	Name            string
	Image           string
	Cmd             []string
	Arg             []string
	CPURequest      string
	CPULimit        string
	MemoryRequest   string
	MemoryLimit     string
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
	VolumeSubPath   string
	VolumeReadOnly  bool
	Env             []Env
	EnvFrom         []EnvFrom
	InitCtrType     string
	RunAsUser       string
	RunAsGroup      string
}

// getCtr takes a list of ctrOptions and returns a Ctr with sane defaults
// and the configured options
func getCtr(options ...ctrOption) *Ctr {
	c := Ctr{
		Name:            defaultCtrName,
		Image:           defaultCtrImage,
		Cmd:             defaultCtrCmd,
		Arg:             defaultCtrArg,
		SecurityContext: true,
		Caps:            false,
		CapAdd:          nil,
		CapDrop:         nil,
		PullPolicy:      "",
		HostIP:          "",
		Port:            "",
		VolumeMount:     false,
		VolumeMountPath: "",
		VolumeName:      "",
		VolumeReadOnly:  false,
		VolumeSubPath:   "",
		Env:             []Env{},
		EnvFrom:         []EnvFrom{},
		InitCtrType:     "",
	}
	for _, option := range options {
		option(&c)
	}
	return &c
}

type ctrOption func(*Ctr)

func withName(name string) ctrOption {
	return func(c *Ctr) {
		c.Name = name
	}
}

func withInitCtr() ctrOption {
	return func(c *Ctr) {
		c.InitCtrType = define.AlwaysInitContainer
	}
}

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

func withCPURequest(request string) ctrOption {
	return func(c *Ctr) {
		c.CPURequest = request
	}
}

func withCPULimit(limit string) ctrOption {
	return func(c *Ctr) {
		c.CPULimit = limit
	}
}

func withMemoryRequest(request string) ctrOption {
	return func(c *Ctr) {
		c.MemoryRequest = request
	}
}

func withMemoryLimit(limit string) ctrOption {
	return func(c *Ctr) {
		c.MemoryLimit = limit
	}
}

func withSecurityContext(sc bool) ctrOption {
	return func(c *Ctr) {
		c.SecurityContext = sc
	}
}

func withRunAsUser(runAsUser string) ctrOption {
	return func(c *Ctr) {
		c.RunAsUser = runAsUser
	}
}

func withRunAsGroup(runAsGroup string) ctrOption {
	return func(c *Ctr) {
		c.RunAsGroup = runAsGroup
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

func withVolumeMount(mountPath, subpath string, readonly bool) ctrOption {
	return func(c *Ctr) {
		c.VolumeMountPath = mountPath
		c.VolumeName = defaultVolName
		c.VolumeReadOnly = readonly
		c.VolumeMount = true
		if len(subpath) > 0 {
			c.VolumeSubPath = subpath
		}
	}
}

func withEnv(name, value, valueFrom, refName, refKey string, optional bool) ctrOption { //nolint:unparam
	return func(c *Ctr) {
		e := Env{
			Name:      name,
			Value:     value,
			ValueFrom: valueFrom,
			RefName:   refName,
			RefKey:    refKey,
			Optional:  optional,
		}

		c.Env = append(c.Env, e)
	}
}

func withEnvFrom(name, from string, optional bool) ctrOption {
	return func(c *Ctr) {
		e := EnvFrom{
			Name:     name,
			From:     from,
			Optional: optional,
		}

		c.EnvFrom = append(c.EnvFrom, e)
	}
}

func makeCtrNameInPod(pod *Pod, containerName string) string {
	return fmt.Sprintf("%s-%s", pod.Name, containerName)
}

func getCtrNameInPod(pod *Pod) string {
	return makeCtrNameInPod(pod, defaultCtrName)
}

type HostPath struct {
	Path string
	Type string
}

type PersistentVolumeClaim struct {
	ClaimName string
}

type ConfigMap struct {
	Name     string
	Items    []map[string]string
	Optional bool
}

type EmptyDir struct{}

type Volume struct {
	VolumeType string
	Name       string
	HostPath
	PersistentVolumeClaim
	ConfigMap
	EmptyDir
}

// getHostPathVolume takes a type and a location for a HostPath
// volume giving it a default name of volName
func getHostPathVolume(vType, vPath string) *Volume {
	return &Volume{
		VolumeType: "HostPath",
		Name:       defaultVolName,
		HostPath: HostPath{
			Path: vPath,
			Type: vType,
		},
	}
}

// getHostPathVolume takes a name for a Persistentvolumeclaim
// volume giving it a default name of volName
func getPersistentVolumeClaimVolume(vName string) *Volume {
	return &Volume{
		VolumeType: "PersistentVolumeClaim",
		Name:       defaultVolName,
		PersistentVolumeClaim: PersistentVolumeClaim{
			ClaimName: vName,
		},
	}
}

// getConfigMap returns a new ConfigMap Volume given the name and items
// of the ConfigMap.
func getConfigMapVolume(vName string, items []map[string]string, optional bool) *Volume { //nolint:unparam
	return &Volume{
		VolumeType: "ConfigMap",
		Name:       defaultVolName,
		ConfigMap: ConfigMap{
			Name:     vName,
			Items:    items,
			Optional: optional,
		},
	}
}

func getEmptyDirVolume() *Volume {
	return &Volume{
		VolumeType: "EmptyDir",
		Name:       defaultVolName,
		EmptyDir:   EmptyDir{},
	}
}

type Env struct {
	Name      string
	Value     string
	ValueFrom string
	RefName   string
	RefKey    string
	Optional  bool
}

type EnvFrom struct {
	Name     string
	From     string
	Optional bool
}

func milliCPUToQuota(milliCPU string) int {
	milli, _ := strconv.Atoi(strings.Trim(milliCPU, "m"))
	return milli * defaultCPUPeriod
}

func createSourceTarFile(fileName, fileContent, tarFilePath string) error {
	dir, err := os.MkdirTemp("", "podmanTest")
	if err != nil {
		return err
	}

	file, err := os.Create(filepath.Join(dir, fileName))
	if err != nil {
		return err
	}

	_, err = file.Write([]byte(fileContent))
	if err != nil {
		return err
	}

	err = file.Close()
	if err != nil {
		return err
	}

	tarFile, err := os.Create(tarFilePath)
	if err != nil {
		return err
	}
	defer tarFile.Close()

	return utils.TarToFilesystem(dir, tarFile)
}

func createAndTestSecret(podmanTest *PodmanTestIntegration, secretYamlString, secretName, fileName string) {
	err := writeYaml(secretYamlString, fileName)
	Expect(err).ToNot(HaveOccurred())

	kube := podmanTest.Podman([]string{"play", "kube", fileName})
	kube.WaitWithDefaultTimeout()
	Expect(kube).Should(Exit(0))

	secretList := podmanTest.Podman([]string{"secret", "list"})
	secretList.WaitWithDefaultTimeout()
	Expect(secretList).Should(Exit(0))
	Expect(secretList.OutputToString()).Should(ContainSubstring(secretName))
}

func deleteAndTestSecret(podmanTest *PodmanTestIntegration, secretName string) {
	secretRm := podmanTest.Podman([]string{"secret", "rm", secretName})
	secretRm.WaitWithDefaultTimeout()
	Expect(secretRm).Should(Exit(0))
}

func testPodWithSecret(podmanTest *PodmanTestIntegration, podYamlString, fileName string, succeed, exists bool) {
	err := writeYaml(podYamlString, fileName)
	Expect(err).ToNot(HaveOccurred())

	kube := podmanTest.Podman([]string{"play", "kube", fileName})
	kube.WaitWithDefaultTimeout()
	if !succeed {
		Expect(kube).Should(Exit(-1))
		return
	}
	Expect(kube).Should(Exit(0))

	exec := podmanTest.Podman([]string{"exec", "-it", "mypod-myctr", "cat", "/etc/foo/username"})
	exec.WaitWithDefaultTimeout()
	if exists {
		Expect(exec).Should(Exit(0))
		username, _ := base64.StdEncoding.DecodeString("dXNlcg==")
		Expect(exec.OutputToString()).Should(ContainSubstring(string(username)))
	} else {
		Expect(exec).Should(Exit(-1))
	}

	podRm := podmanTest.Podman([]string{"pod", "rm", "-f", "mypod"})
	podRm.WaitWithDefaultTimeout()
	Expect(podRm).Should(Exit(0))
}

func testHTTPServer(port string, shouldErr bool, expectedResponse string) {
	address := url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort("localhost", port),
	}

	interval := 250 * time.Millisecond
	var err error
	var resp *http.Response
	for i := 0; i < 6; i++ {
		resp, err = http.Get(address.String())
		if err != nil && shouldErr {
			Expect(err.Error()).To(ContainSubstring(expectedResponse))
			return
		}
		if err == nil {
			defer resp.Body.Close()
			break
		}
		time.Sleep(interval)
		interval *= 2
	}
	Expect(err).To(BeNil())

	body, err := io.ReadAll(resp.Body)
	Expect(err).To(BeNil())
	Expect(string(body)).Should(Equal(expectedResponse))
}

func verifyPodPorts(podmanTest *PodmanTestIntegration, podName string, ports ...string) {
	podInspect := podmanTest.Podman([]string{"pod", "inspect", podName, "--format", "{{.InfraContainerID}}"})
	podInspect.WaitWithDefaultTimeout()
	Expect(podInspect).To(Exit(0))
	infraID := podInspect.OutputToString()

	inspect := podmanTest.Podman([]string{"inspect", "--format", "{{.NetworkSettings.Ports}}", infraID})
	inspect.WaitWithDefaultTimeout()
	Expect(inspect).To(Exit(0))

	for _, port := range ports {
		Expect(inspect.OutputToString()).Should(ContainSubstring(port))
	}
}

var _ = Describe("Podman play kube", func() {
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
		kubeYaml = filepath.Join(podmanTest.TempDir, "kube.yaml")
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)
	})

	It("podman play kube fail with yaml of unsupported kind", func() {
		err := writeYaml(unknownKindYaml, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).To(ExitWithError())

	})

	It("podman play kube fail with custom selinux label", func() {
		if !selinux.GetEnabled() {
			Skip("SELinux not enabled")
		}
		err := writeYaml(selinuxLabelPodYaml, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", "label-pod-test", "--format", "'{{ .ProcessLabel }}'"})
		inspect.WaitWithDefaultTimeout()
		label := inspect.OutputToString()

		Expect(label).To(ContainSubstring("unconfined_u:system_r:spc_t:s0"))
	})

	It("podman play kube --no-host", func() {
		err := writeYaml(checkInfraImagePodYaml, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", "--no-hosts", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		podInspect := podmanTest.Podman([]string{"pod", "inspect", "check-infra-image"})
		podInspect.WaitWithDefaultTimeout()
		Expect(podInspect).Should(Exit(0))

		data := podInspect.InspectPodToJSON()
		for _, ctr := range data.Containers {
			if strings.HasSuffix(ctr.Name, "-infra") {
				continue
			}
			exec := podmanTest.Podman([]string{"exec", ctr.ID, "cat", "/etc/hosts"})
			exec.WaitWithDefaultTimeout()
			Expect(exec).Should(Exit(0))
			Expect(exec.OutputToString()).To(Not(ContainSubstring("check-infra-image")))
		}
	})

	It("podman play kube with non-existing configmap", func() {
		err := writeYaml(podWithoutConfigMapDefined, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(125))
		Expect(kube.ErrorToString()).To(ContainSubstring("failed to create volume \"mycm\": no such ConfigMap \"mycm\""))
	})

	It("podman play kube test HostAliases with --no-hosts", func() {
		pod := getPod(withHostAliases("192.168.1.2", []string{
			"test1.podman.io",
			"test2.podman.io",
		}),
			withHostAliases("192.168.1.3", []string{
				"test3.podman.io",
				"test4.podman.io",
			}),
		)
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", "--no-hosts", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(125))
		Expect(kube.ErrorToString()).To(ContainSubstring("HostAliases in yaml file will not work with --no-hosts"))
	})

	It("podman play kube should use customized infra_image", func() {
		conffile := filepath.Join(podmanTest.TempDir, "container.conf")

		infraImage := "k8s.gcr.io/pause:3.2"
		err := os.WriteFile(conffile, []byte(fmt.Sprintf("[engine]\ninfra_image=\"%s\"\n", infraImage)), 0644)
		Expect(err).ToNot(HaveOccurred())

		os.Setenv("CONTAINERS_CONF", conffile)
		defer os.Unsetenv("CONTAINERS_CONF")

		if IsRemote() {
			podmanTest.RestartRemoteService()
		}

		err = writeYaml(checkInfraImagePodYaml, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		podInspect := podmanTest.Podman([]string{"inspect", "check-infra-image", "--format", "{{ .InfraContainerID }}"})
		podInspect.WaitWithDefaultTimeout()
		infraContainerID := podInspect.OutputToString()

		conInspect := podmanTest.Podman([]string{"inspect", infraContainerID, "--format", "{{ .ImageName }}"})
		conInspect.WaitWithDefaultTimeout()
		infraContainerImage := conInspect.OutputToString()
		Expect(infraContainerImage).To(Equal(infraImage))
	})

	It("podman play kube should share ipc,net,uts when shareProcessNamespace is set", func() {
		SkipIfRootless("Requires root privileges for sharing few namespaces")
		err := writeYaml(sharedNamespacePodYaml, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", "testpod1", "--format", "'{{ .SharedNamespaces }}'"})
		inspect.WaitWithDefaultTimeout()
		sharednamespaces := inspect.OutputToString()
		Expect(sharednamespaces).To(ContainSubstring("ipc"))
		Expect(sharednamespaces).To(ContainSubstring("net"))
		Expect(sharednamespaces).To(ContainSubstring("uts"))
		Expect(sharednamespaces).To(ContainSubstring("pid"))
	})

	It("podman play kube should be able to run image where workdir is a symlink", func() {
		session := podmanTest.Podman([]string{
			"build", "-f", "build/workdir-symlink/Dockerfile", "-t", "test-symlink",
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		err := writeYaml(workdirSymlinkPodYaml, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		logs := podmanTest.Podman([]string{"pod", "logs", "-c", "test-symlink-test-symlink", "test-symlink"})
		logs.WaitWithDefaultTimeout()
		Expect(logs).Should(Exit(0))
		Expect(logs.OutputToString()).To(ContainSubstring("hello"))
	})

	It("podman play kube should not rename pod if container in pod has same name", func() {
		err := writeYaml(podnameEqualsContainerNameYaml, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		testPodCreated := podmanTest.Podman([]string{"pod", "exists", "podnameEqualsContainerNameYaml"})
		testPodCreated.WaitWithDefaultTimeout()
		Expect(testPodCreated).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", "podnameEqualsContainerNameYaml"})
		inspect.WaitWithDefaultTimeout()
		podInspect := inspect.InspectPodArrToJSON()
		Expect(podInspect).Should(HaveLen(1))
		var containerNames []string
		for _, container := range podInspect[0].Containers {
			containerNames = append(containerNames, container.Name)
		}
		Expect(containerNames).To(ContainElement("podnameEqualsContainerNameYaml-podnameEqualsContainerNameYaml"))
	})

	It("podman play kube should error if pod dont have a name", func() {
		err := writeYaml(podWithoutAName, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(125))

	})

	It("podman play kube support container liveness probe", func() {
		err := writeYaml(livenessProbePodYaml, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", "liveness-probe-pod-alpine", "--format", "'{{ .Config.Healthcheck }}'"})
		inspect.WaitWithDefaultTimeout()
		healthcheckcmd := inspect.OutputToString()
		// check if CMD-SHELL based equivalent health check is added to container
		Expect(healthcheckcmd).To(ContainSubstring("[CMD echo hello]"))
	})

	It("podman play kube liveness probe should fail", func() {
		err := writeYaml(livenessProbeUnhealthyPodYaml, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		time.Sleep(2 * time.Second)
		hc := podmanTest.Podman([]string{"healthcheck", "run", "liveness-unhealthy-probe-pod-alpine"})
		hc.WaitWithDefaultTimeout()
		hcoutput := hc.OutputToString()
		Expect(hcoutput).To(ContainSubstring(define.HealthCheckUnhealthy))
	})

	It("podman play kube support container startup probe", func() {
		ctrName := "startup-healthy-probe-pod-alpine"
		err := writeYaml(startupProbePodYaml, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		time.Sleep(2 * time.Second)
		inspect := podmanTest.InspectContainer(ctrName)
		Expect(inspect[0].State.Health).To(HaveField("Status", "starting"))

		hc := podmanTest.Podman([]string{"healthcheck", "run", ctrName})
		hc.WaitWithDefaultTimeout()
		Expect(hc).Should(Exit(1))

		exec := podmanTest.Podman([]string{"exec", ctrName, "sh", "-c", "echo 'startup probe success' > /testfile"})
		exec.WaitWithDefaultTimeout()
		Expect(exec).Should(Exit(0))

		hc = podmanTest.Podman([]string{"healthcheck", "run", ctrName})
		hc.WaitWithDefaultTimeout()
		Expect(hc).Should(Exit(0))

		inspect = podmanTest.InspectContainer(ctrName)
		Expect(inspect[0].State.Health).To(HaveField("Status", define.HealthCheckHealthy))
	})

	It("podman play kube fail with nonexistent authfile", func() {
		err := generateKubeYaml("pod", getPod(), kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", "--authfile", "/tmp/nonexistent", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).To(ExitWithError())

	})

	It("podman play kube test correct command", func() {
		pod := getPod()
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod), "--format", "'{{ .Config.Cmd }}'"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		cmd := inspect.OutputToString()

		inspect = podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod), "--format", "'{{ .Config.Entrypoint }}'"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		ep := inspect.OutputToString()

		// Use the defined command to override the image's command
		Expect(ep).To(ContainSubstring(strings.Join(defaultCtrCmd, " ")))
		Expect(cmd).To(ContainSubstring(strings.Join(defaultCtrArg, " ")))
	})

	// If you do not supply command or args for a Container, the defaults defined in the Docker image are used.
	It("podman play kube test correct args and cmd when not specified", func() {
		pod := getPod(withCtr(getCtr(withImage(REGISTRY_IMAGE), withCmd(nil), withArg(nil))))
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		// this image's ENTRYPOINT is `/entrypoint.sh`
		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod), "--format", "'{{ .Config.Entrypoint }}'"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(ContainSubstring(`/entrypoint.sh`))

		// and its COMMAND is `/etc/docker/registry/config.yml`
		inspect = podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod), "--format", "'{{ .Config.Cmd }}'"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(ContainSubstring(`[/etc/docker/registry/config.yml]`))
	})

	// If you supply a command but no args for a Container, only the supplied command is used.
	// The default EntryPoint and the default Cmd defined in the Docker image are ignored.
	It("podman play kube test correct command with only set command in yaml file", func() {
		pod := getPod(withCtr(getCtr(withImage(REGISTRY_IMAGE), withCmd([]string{"echo", "hello"}), withArg(nil))))
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		// Use the defined command to override the image's command, and don't set the args
		// so the full command in result should not contains the image's command
		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod), "--format", "'{{ .Config.Entrypoint }}'"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(ContainSubstring(`echo hello`))

		inspect = podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod), "--format", "'{{ .Config.Cmd }}'"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		// an empty command is reported as '[]'
		Expect(inspect.OutputToString()).To(ContainSubstring(`[]`))
	})

	// If you have an init container in the pod yaml, podman should create and run the init container with play kube
	// With annotation set to always
	It("podman play kube test with init containers and annotation set", func() {
		// With the init container type annotation set to always
		pod := getPod(withAnnotation("io.podman.annotations.init.container.type", "always"), withPodInitCtr(getCtr(withImage(ALPINE), withCmd([]string{"echo", "hello"}), withInitCtr(), withName("init-test"))), withCtr(getCtr(withImage(ALPINE), withCmd([]string{"top"}))))
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		// Expect the number of containers created to be 3, one init, infra, and regular container
		numOfCtrs := podmanTest.NumberOfContainers()
		Expect(numOfCtrs).To(Equal(3))

		// Init container should have exited after running
		inspect := podmanTest.Podman([]string{"inspect", "--format", "{{.State.Status}}", "testPod-init-test"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(ContainSubstring("exited"))

		// Regular container should be in running state
		inspect = podmanTest.Podman([]string{"inspect", "--format", "{{.State.Status}}", "testPod-" + defaultCtrName})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(ContainSubstring("running"))
	})

	// If you have an init container in the pod yaml, podman should create and run the init container with play kube
	// Using default init container type (once)
	It("podman play kube test with init container type set to default value", func() {
		// Using the default init container type (once)
		pod := getPod(withPodInitCtr(getCtr(withImage(ALPINE), withCmd([]string{"echo", "hello"}), withInitCtr(), withName("init-test"))), withCtr(getCtr(withImage(ALPINE), withCmd([]string{"top"}))))
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		// Expect the number of containers created to be 2, infra and regular container
		numOfCtrs := podmanTest.NumberOfContainers()
		Expect(numOfCtrs).To(Equal(2))

		// Regular container should be in running state
		inspect := podmanTest.Podman([]string{"inspect", "--format", "{{.State.Status}}", "testPod-" + defaultCtrName})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(ContainSubstring("running"))
	})

	// If you supply only args for a Container, the default Entrypoint defined in the Docker image is run with the args that you supplied.
	It("podman play kube test correct command with only set args in yaml file", func() {
		pod := getPod(withCtr(getCtr(withImage(REGISTRY_IMAGE), withCmd(nil), withArg([]string{"echo", "hello"}))))
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		// this image's ENTRYPOINT is `/entrypoint.sh`
		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod), "--format", "'{{ .Config.Entrypoint }}'"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(ContainSubstring(`/entrypoint.sh`))

		inspect = podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod), "--format", "'{{ .Config.Cmd }}'"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(ContainSubstring(`[echo hello]`))
	})

	// If you supply a command and args,
	// the default Entrypoint and the default Cmd defined in the Docker image are ignored.
	// Your command is run with your args.
	It("podman play kube test correct command with both set args and cmd in yaml file", func() {
		pod := getPod(withCtr(getCtr(withImage(REGISTRY_IMAGE), withCmd([]string{"echo"}), withArg([]string{"hello"}))))
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod), "--format", "'{{ .Config.Entrypoint }}'"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(ContainSubstring(`echo`))

		inspect = podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod), "--format", "'{{ .Config.Cmd }}'"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(ContainSubstring(`[hello]`))
	})

	It("podman play kube test correct output", func() {
		p := getPod(withCtr(getCtr(withCmd([]string{"echo", "hello"}), withArg([]string{"world"}))))

		err := generateKubeYaml("pod", p, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		wait := podmanTest.Podman([]string{"wait", getCtrNameInPod(p)})
		wait.WaitWithDefaultTimeout()
		Expect(wait).Should(Exit(0))

		logs := podmanTest.Podman([]string{"logs", getCtrNameInPod(p)})
		logs.WaitWithDefaultTimeout()
		Expect(logs).Should(Exit(0))
		Expect(logs.OutputToString()).To(ContainSubstring("hello world"))
	})

	It("podman pod logs test", func() {
		SkipIfRemote("podman-remote pod logs -c is mandatory for remote machine")
		p := getPod(withCtr(getCtr(withCmd([]string{"echo", "hello"}), withArg([]string{"world"}))))

		err := generateKubeYaml("pod", p, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		wait := podmanTest.Podman([]string{"wait", getCtrNameInPod(p)})
		wait.WaitWithDefaultTimeout()
		Expect(wait).Should(Exit(0))

		logs := podmanTest.Podman([]string{"pod", "logs", p.Name})
		logs.WaitWithDefaultTimeout()
		Expect(logs).Should(Exit(0))
		Expect(logs.OutputToString()).To(ContainSubstring("hello world"))
	})

	It("podman-remote pod logs test", func() {
		// -c or --container is required in podman-remote due to api limitation.
		p := getPod(withCtr(getCtr(withCmd([]string{"echo", "hello"}), withArg([]string{"world"}))))

		err := generateKubeYaml("pod", p, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		wait := podmanTest.Podman([]string{"wait", getCtrNameInPod(p)})
		wait.WaitWithDefaultTimeout()
		Expect(wait).Should(Exit(0))

		logs := podmanTest.Podman([]string{"pod", "logs", "-c", getCtrNameInPod(p), p.Name})
		logs.WaitWithDefaultTimeout()
		Expect(logs).Should(Exit(0))
		Expect(logs.OutputToString()).To(ContainSubstring("hello world"))
	})

	It("podman play kube test restartPolicy", func() {
		// podName,  set,  expect
		testSli := [][]string{
			{"testPod1", "", "always"}, // Default equal to always
			{"testPod2", "Always", "always"},
			{"testPod3", "OnFailure", "on-failure"},
			{"testPod4", "Never", "no"},
		}
		for _, v := range testSli {
			pod := getPod(withPodName(v[0]), withRestartPolicy(v[1]))
			err := generateKubeYaml("pod", pod, kubeYaml)
			Expect(err).ToNot(HaveOccurred())

			kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
			kube.WaitWithDefaultTimeout()
			Expect(kube).Should(Exit(0))

			inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod), "--format", "{{.HostConfig.RestartPolicy.Name}}"})
			inspect.WaitWithDefaultTimeout()
			Expect(inspect).Should(Exit(0))
			Expect(inspect.OutputToString()).To(Equal(v[2]))
		}
	})

	It("podman play kube test env value from configmap", func() {
		SkipIfRemote("configmap list is not supported as a param")
		cmYamlPathname := filepath.Join(podmanTest.TempDir, "foo-cm.yaml")
		cm := getConfigMap(withConfigMapName("foo"), withConfigMapData("FOO", "foo"))
		err := generateKubeYaml("configmap", cm, cmYamlPathname)
		Expect(err).ToNot(HaveOccurred())

		pod := getPod(withCtr(getCtr(withEnv("FOO", "", "configmap", "foo", "FOO", false))))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml, "--configmap", cmYamlPathname})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod), "--format", "'{{ .Config.Env }}'"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(ContainSubstring(`FOO=foo`))
	})

	It("podman play kube test env value from configmap and --replace should reuse the configmap volume", func() {
		SkipIfRemote("configmap list is not supported as a param")
		cmYamlPathname := filepath.Join(podmanTest.TempDir, "foo-cm.yaml")
		cm := getConfigMap(withConfigMapName("foo"), withConfigMapData("FOO", "foo"))
		err := generateKubeYaml("configmap", cm, cmYamlPathname)
		Expect(err).ToNot(HaveOccurred())

		pod := getPod(withCtr(getCtr(withEnv("FOO", "", "configmap", "foo", "FOO", false))))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml, "--configmap", cmYamlPathname})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		// create pod again with --replace
		kube = podmanTest.Podman([]string{"play", "kube", "--replace", kubeYaml, "--configmap", cmYamlPathname})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod), "--format", "'{{ .Config.Env }}'"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(ContainSubstring(`FOO=foo`))
	})

	It("podman play kube test required env value from configmap with missing key", func() {
		SkipIfRemote("configmap list is not supported as a param")
		cmYamlPathname := filepath.Join(podmanTest.TempDir, "foo-cm.yaml")
		cm := getConfigMap(withConfigMapName("foo"), withConfigMapData("FOO", "foo"))
		err := generateKubeYaml("configmap", cm, cmYamlPathname)
		Expect(err).ToNot(HaveOccurred())

		pod := getPod(withCtr(getCtr(withEnv("FOO", "", "configmap", "foo", "MISSING_KEY", false))))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml, "--configmap", cmYamlPathname})
		kube.WaitWithDefaultTimeout()
		Expect(kube).To(ExitWithError())
	})

	It("podman play kube test required env value from missing configmap", func() {
		pod := getPod(withCtr(getCtr(withEnv("FOO", "", "configmap", "missing_cm", "FOO", false))))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).To(ExitWithError())
	})

	It("podman play kube test optional env value from configmap with missing key", func() {
		SkipIfRemote("configmap list is not supported as a param")
		cmYamlPathname := filepath.Join(podmanTest.TempDir, "foo-cm.yaml")
		cm := getConfigMap(withConfigMapName("foo"), withConfigMapData("FOO", "foo"))
		err := generateKubeYaml("configmap", cm, cmYamlPathname)
		Expect(err).ToNot(HaveOccurred())

		pod := getPod(withCtr(getCtr(withEnv("FOO", "", "configmap", "foo", "MISSING_KEY", true))))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml, "--configmap", cmYamlPathname})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod), "--format", "'{{ range .Config.Env }}[{{ . }}]{{end}}'"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(Not(ContainSubstring(`[FOO=]`)))
	})

	It("podman play kube test optional env value from missing configmap", func() {
		pod := getPod(withCtr(getCtr(withEnv("FOO", "", "configmap", "missing_cm", "FOO", true))))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod), "--format", "'{{ range .Config.Env }}[{{ . }}]{{end}}'"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(Not(ContainSubstring(`[FOO=]`)))
	})

	It("podman play kube test get all key-value pairs from configmap as envs", func() {
		SkipIfRemote("configmap list is not supported as a param")
		cmYamlPathname := filepath.Join(podmanTest.TempDir, "foo-cm.yaml")
		cm := getConfigMap(withConfigMapName("foo"), withConfigMapData("FOO1", "foo1"), withConfigMapData("FOO2", "foo2"))
		err := generateKubeYaml("configmap", cm, cmYamlPathname)
		Expect(err).ToNot(HaveOccurred())

		pod := getPod(withCtr(getCtr(withEnvFrom("foo", "configmap", false))))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml, "--configmap", cmYamlPathname})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod), "--format", "'{{ .Config.Env }}'"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(ContainSubstring(`FOO1=foo1`))
		Expect(inspect.OutputToString()).To(ContainSubstring(`FOO2=foo2`))
	})

	It("podman play kube test get all key-value pairs from required configmap as envs", func() {
		pod := getPod(withCtr(getCtr(withEnvFrom("missing_cm", "configmap", false))))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).To(ExitWithError())
	})

	It("podman play kube test get all key-value pairs from optional configmap as envs", func() {
		pod := getPod(withCtr(getCtr(withEnvFrom("missing_cm", "configmap", true))))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))
	})

	It("podman play kube test env value from secret", func() {
		createSecret(podmanTest, "foo", defaultSecret)
		pod := getPod(withCtr(getCtr(withEnv("FOO", "", "secret", "foo", "FOO", false))))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod), "--format", "'{{ .Config.Env }}'"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(ContainSubstring(`FOO=foo`))
	})

	It("podman play kube test required env value from missing secret", func() {
		pod := getPod(withCtr(getCtr(withEnv("FOO", "", "secret", "foo", "FOO", false))))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).To(ExitWithError())
	})

	It("podman play kube test required env value from secret with missing key", func() {
		createSecret(podmanTest, "foo", defaultSecret)
		pod := getPod(withCtr(getCtr(withEnv("FOO", "", "secret", "foo", "MISSING", false))))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).To(ExitWithError())
	})

	It("podman play kube test optional env value from missing secret", func() {
		pod := getPod(withCtr(getCtr(withEnv("FOO", "", "secret", "foo", "FOO", true))))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod), "--format", "'{{ range .Config.Env }}[{{ . }}]{{end}}'"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(Not(ContainSubstring(`[FOO=]`)))
	})

	It("podman play kube test optional env value from secret with missing key", func() {
		createSecret(podmanTest, "foo", defaultSecret)
		pod := getPod(withCtr(getCtr(withEnv("FOO", "", "secret", "foo", "MISSING", true))))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod), "--format", "'{{ range .Config.Env }}[{{ . }}]{{end}}'"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(Not(ContainSubstring(`[FOO=]`)))
	})

	It("podman play kube test get all key-value pairs from secret as envs", func() {
		createSecret(podmanTest, "foo", defaultSecret)
		pod := getPod(withCtr(getCtr(withEnvFrom("foo", "secret", false))))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod), "--format", "'{{ .Config.Env }}'"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(ContainSubstring(`FOO=foo`))
		Expect(inspect.OutputToString()).To(ContainSubstring(`BAR=bar`))
	})

	It("podman play kube test get all key-value pairs from required secret as envs", func() {
		pod := getPod(withCtr(getCtr(withEnvFrom("missing_secret", "secret", false))))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).To(ExitWithError())
	})

	It("podman play kube test get all key-value pairs from optional secret as envs", func() {
		pod := getPod(withCtr(getCtr(withEnvFrom("missing_secret", "secret", true))))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))
	})

	It("podman play kube test duplicate container name", func() {
		p := getPod(withCtr(getCtr(withName("testctr"), withCmd([]string{"echo", "hello"}))), withCtr(getCtr(withName("testctr"), withCmd([]string{"echo", "world"}))))

		err := generateKubeYaml("pod", p, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).To(ExitWithError())

		p = getPod(withPodInitCtr(getCtr(withImage(ALPINE), withCmd([]string{"echo", "hello"}), withInitCtr(), withName("initctr"))), withCtr(getCtr(withImage(ALPINE), withName("initctr"), withCmd([]string{"top"}))))

		err = generateKubeYaml("pod", p, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube = podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).To(ExitWithError())
	})

	It("podman play kube test hostname", func() {
		pod := getPod()
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod), "--format", "{{ .Config.Hostname }}"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(Equal(defaultPodName))
	})

	It("podman play kube test with customized hostname", func() {
		hostname := "myhostname"
		pod := getPod(withHostname(hostname))
		err := generateKubeYaml("pod", getPod(withHostname(hostname)), kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod), "--format", "{{ .Config.Hostname }}"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(Equal(hostname))

		hostnameInCtr := podmanTest.Podman([]string{"exec", getCtrNameInPod(pod), "hostname"})
		hostnameInCtr.WaitWithDefaultTimeout()
		Expect(hostnameInCtr).Should(Exit(0))
		Expect(hostnameInCtr.OutputToString()).To(Equal(hostname))
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
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", pod.Name, "--format", "{{ .InfraConfig.HostAdd}}"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).
			To(Equal("[test1.podman.io:192.168.1.2 test2.podman.io:192.168.1.2 test3.podman.io:192.168.1.3 test4.podman.io:192.168.1.3]"))
	})

	It("podman play kube cap add", func() {
		capAdd := "CAP_SYS_ADMIN"
		ctr := getCtr(withCapAdd([]string{capAdd}), withCmd([]string{"cat", "/proc/self/status"}), withArg(nil))

		pod := getPod(withCtr(ctr))
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod)})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(ContainSubstring(capAdd))
	})

	It("podman play kube cap drop", func() {
		capDrop := "CAP_CHOWN"
		ctr := getCtr(withCapDrop([]string{capDrop}))

		pod := getPod(withCtr(ctr))
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod)})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(ContainSubstring(capDrop))
	})

	It("podman play kube no security context", func() {
		// expect play kube to not fail if no security context is specified
		pod := getPod(withCtr(getCtr(withSecurityContext(false))))
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod)})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
	})

	It("podman play kube seccomp container level", func() {
		SkipIfRemote("podman-remote does not support --seccomp-profile-root flag")
		// expect play kube is expected to set a seccomp label if it's applied as an annotation
		jsonFile, err := podmanTest.CreateSeccompJSON(seccompPwdEPERM)
		if err != nil {
			fmt.Println(err)
			Skip("Failed to prepare seccomp.json for test.")
		}

		ctrAnnotation := "container.seccomp.security.alpha.kubernetes.io/" + defaultCtrName
		ctr := getCtr(withCmd([]string{"pwd"}), withArg(nil))

		pod := getPod(withCtr(ctr), withAnnotation(ctrAnnotation, "localhost/"+filepath.Base(jsonFile)))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		// CreateSeccompJSON will put the profile into podmanTest.TempDir. Use --seccomp-profile-root to tell play kube where to look
		kube := podmanTest.Podman([]string{"play", "kube", "--seccomp-profile-root", podmanTest.TempDir, kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		logs := podmanTest.Podman([]string{"logs", getCtrNameInPod(pod)})
		logs.WaitWithDefaultTimeout()
		Expect(logs).Should(Exit(0))
		Expect(logs.ErrorToString()).To(ContainSubstring("Operation not permitted"))
	})

	It("podman play kube seccomp pod level", func() {
		SkipIfRemote("podman-remote does not support --seccomp-profile-root flag")
		// expect play kube is expected to set a seccomp label if it's applied as an annotation
		jsonFile, err := podmanTest.CreateSeccompJSON(seccompPwdEPERM)
		if err != nil {
			fmt.Println(err)
			Skip("Failed to prepare seccomp.json for test.")
		}
		defer os.Remove(jsonFile)

		ctr := getCtr(withCmd([]string{"pwd"}), withArg(nil))

		pod := getPod(withCtr(ctr), withAnnotation("seccomp.security.alpha.kubernetes.io/pod", "localhost/"+filepath.Base(jsonFile)))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		// CreateSeccompJSON will put the profile into podmanTest.TempDir. Use --seccomp-profile-root to tell play kube where to look
		kube := podmanTest.Podman([]string{"play", "kube", "--seccomp-profile-root", podmanTest.TempDir, kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		logs := podmanTest.Podman([]string{"logs", getCtrNameInPod(pod)})
		logs.WaitWithDefaultTimeout()
		Expect(logs).Should(Exit(0))
		Expect(logs.ErrorToString()).To(ContainSubstring("Operation not permitted"))
	})

	It("podman play kube with pull policy of never should be 125", func() {
		ctr := getCtr(withPullPolicy("never"), withImage(BB_GLIBC))
		err := generateKubeYaml("pod", getPod(withCtr(ctr)), kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(125))
	})

	It("podman play kube with pull policy of missing", func() {
		ctr := getCtr(withPullPolicy("Missing"), withImage(BB))
		err := generateKubeYaml("pod", getPod(withCtr(ctr)), kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))
	})

	It("podman play kube with pull always", func() {
		oldBB := "quay.io/libpod/busybox:1.30.1"
		pull := podmanTest.Podman([]string{"pull", oldBB})
		pull.WaitWithDefaultTimeout()

		tag := podmanTest.Podman([]string{"tag", oldBB, BB})
		tag.WaitWithDefaultTimeout()
		Expect(tag).Should(Exit(0))

		rmi := podmanTest.Podman([]string{"rmi", oldBB})
		rmi.WaitWithDefaultTimeout()
		Expect(rmi).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", BB})
		inspect.WaitWithDefaultTimeout()
		oldBBinspect := inspect.InspectImageJSON()

		ctr := getCtr(withPullPolicy("always"), withImage(BB))
		err := generateKubeYaml("pod", getPod(withCtr(ctr)), kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		inspect = podmanTest.Podman([]string{"inspect", BB})
		inspect.WaitWithDefaultTimeout()
		newBBinspect := inspect.InspectImageJSON()
		Expect(oldBBinspect[0].Digest).To(Not(Equal(newBBinspect[0].Digest)))
	})

	It("podman play kube with latest image should always pull", func() {
		oldBB := "quay.io/libpod/busybox:1.30.1"
		pull := podmanTest.Podman([]string{"pull", oldBB})
		pull.WaitWithDefaultTimeout()

		tag := podmanTest.Podman([]string{"tag", oldBB, BB})
		tag.WaitWithDefaultTimeout()
		Expect(tag).Should(Exit(0))

		rmi := podmanTest.Podman([]string{"rmi", oldBB})
		rmi.WaitWithDefaultTimeout()
		Expect(rmi).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", BB})
		inspect.WaitWithDefaultTimeout()
		oldBBinspect := inspect.InspectImageJSON()

		ctr := getCtr(withImage(BB))
		err := generateKubeYaml("pod", getPod(withCtr(ctr)), kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

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
		Expect(pull).Should(Exit(0))

		c := podmanTest.Podman([]string{"commit", "-c", "STOPSIGNAL=51", "newBB", "demo"})
		c.WaitWithDefaultTimeout()
		Expect(c).Should(Exit(0))

		conffile := filepath.Join(podmanTest.TempDir, "kube.yaml")
		tempdir, err = CreateTempDirInTempDir()
		Expect(err).ToNot(HaveOccurred())

		err := os.WriteFile(conffile, []byte(testyaml), 0755)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", conffile})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", "demo_pod-demo_kube"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))

		ctr := inspect.InspectContainerToJSON()
		Expect(ctr[0].Config.WorkingDir).To(ContainSubstring("/etc"))
		Expect(ctr[0].Config.Labels).To(HaveKeyWithValue("key1", ContainSubstring("value1")))
		Expect(ctr[0].Config.Labels).To(HaveKeyWithValue("key1", ContainSubstring("value1")))
		Expect(ctr[0].Config).To(HaveField("StopSignal", uint(51)))
	})

	// Deployment related tests
	It("podman play kube deployment 1 replica test correct command", func() {
		deployment := getDeployment()
		err := generateKubeYaml("deployment", deployment, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		podName := getPodNameInDeployment(deployment)
		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(&podName), "--format", "'{{ .Config.Entrypoint }}'"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		// yaml's command should override the image's Entrypoint
		Expect(inspect.OutputToString()).To(ContainSubstring(strings.Join(defaultCtrCmd, " ")))
	})

	It("podman play kube deployment more than 1 replica test correct command", func() {
		var numReplicas int32 = 5
		deployment := getDeployment(withReplicas(numReplicas))
		err := generateKubeYaml("deployment", deployment, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		podName := getPodNameInDeployment(deployment)

		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(&podName), "--format", "'{{ .Config.Entrypoint }}'"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(ContainSubstring(strings.Join(defaultCtrCmd, " ")))

	})

	It("podman play kube --ip and --mac-address", func() {
		var i, numReplicas int32
		numReplicas = 3
		deployment := getDeployment(withReplicas(numReplicas))
		err := generateKubeYaml("deployment", deployment, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		net := "playkube" + stringid.GenerateRandomID()
		session := podmanTest.Podman([]string{"network", "create", "--subnet", "10.25.31.0/24", net})
		session.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(net)
		Expect(session).Should(Exit(0))

		ips := []string{"10.25.31.5", "10.25.31.10", "10.25.31.15"}
		playArgs := []string{"play", "kube", "--network", net}
		for _, ip := range ips {
			playArgs = append(playArgs, "--ip", ip)
		}
		macs := []string{"e8:d8:82:c9:80:40", "e8:d8:82:c9:80:50", "e8:d8:82:c9:80:60"}
		for _, mac := range macs {
			playArgs = append(playArgs, "--mac-address", mac)
		}

		kube := podmanTest.Podman(append(playArgs, kubeYaml))
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		podName := getPodNameInDeployment(deployment)

		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(&podName), "--format", "{{ .NetworkSettings.Networks." + net + ".IPAddress }}"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(Equal(ips[i]))

		inspect = podmanTest.Podman([]string{"inspect", getCtrNameInPod(&podName), "--format", "{{ .NetworkSettings.Networks." + net + ".MacAddress }}"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(Equal(macs[i]))

	})

	It("podman play kube with multiple networks", func() {
		ctr := getCtr(withImage(ALPINE))
		pod := getPod(withCtr(ctr))
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		net1 := "net1" + stringid.GenerateRandomID()
		net2 := "net2" + stringid.GenerateRandomID()

		net := podmanTest.Podman([]string{"network", "create", "--subnet", "10.0.11.0/24", net1})
		net.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(net1)
		Expect(net).Should(Exit(0))

		net = podmanTest.Podman([]string{"network", "create", "--subnet", "10.0.12.0/24", net2})
		net.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(net2)
		Expect(net).Should(Exit(0))

		ip1 := "10.0.11.5"
		ip2 := "10.0.12.10"

		kube := podmanTest.Podman([]string{"play", "kube", "--network", net1 + ":ip=" + ip1, "--network", net2 + ":ip=" + ip2, kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"exec", getCtrNameInPod(pod), "ip", "addr"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(ContainSubstring(ip1))
		Expect(inspect.OutputToString()).To(ContainSubstring(ip2))
		Expect(inspect.OutputToString()).To(ContainSubstring("eth0"))
		Expect(inspect.OutputToString()).To(ContainSubstring("eth1"))
	})

	It("podman play kube test with network portbindings", func() {
		ip := "127.0.0.100"
		port := "8087"
		ctr := getCtr(withHostIP(ip, port), withImage(BB))

		pod := getPod(withCtr(ctr))
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"port", getCtrNameInPod(pod)})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(Equal("8087/tcp -> 127.0.0.100:8087"))
	})

	It("podman play kube test with nonexistent empty HostPath type volume", func() {
		hostPathLocation := filepath.Join(tempdir, "file")

		pod := getPod(withVolume(getHostPathVolume(`""`, hostPathLocation)))
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).To(ExitWithError())
		Expect(kube.ErrorToString()).To(ContainSubstring(defaultVolName))
	})

	It("podman play kube test with empty HostPath type volume", func() {
		hostPathLocation := filepath.Join(tempdir, "file")
		f, err := os.Create(hostPathLocation)
		Expect(err).ToNot(HaveOccurred())
		f.Close()

		pod := getPod(withVolume(getHostPathVolume(`""`, hostPathLocation)))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))
	})

	It("podman play kube test with nonexistent File HostPath type volume", func() {
		hostPathLocation := filepath.Join(tempdir, "file")

		pod := getPod(withVolume(getHostPathVolume("File", hostPathLocation)))
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).To(ExitWithError())
	})

	It("podman play kube test with File HostPath type volume", func() {
		hostPathLocation := filepath.Join(tempdir, "file")
		f, err := os.Create(hostPathLocation)
		Expect(err).ToNot(HaveOccurred())
		f.Close()

		pod := getPod(withVolume(getHostPathVolume("File", hostPathLocation)))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))
	})

	It("podman play kube test with FileOrCreate HostPath type volume", func() {
		hostPathLocation := filepath.Join(tempdir, "file")

		pod := getPod(withVolume(getHostPathVolume("FileOrCreate", hostPathLocation)))
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		// the file should have been created
		_, err = os.Stat(hostPathLocation)
		Expect(err).ToNot(HaveOccurred())
	})

	It("podman play kube test with DirectoryOrCreate HostPath type volume", func() {
		hostPathLocation := filepath.Join(tempdir, "file")

		pod := getPod(withVolume(getHostPathVolume("DirectoryOrCreate", hostPathLocation)))
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		// the file should have been created
		st, err := os.Stat(hostPathLocation)
		Expect(err).ToNot(HaveOccurred())
		Expect(st.Mode().IsDir()).To(BeTrue())
	})

	It("podman play kube test with DirectoryOrCreate HostPath type volume and non-existent directory path", func() {
		hostPathLocation := filepath.Join(filepath.Join(tempdir, "dir1"), "dir2")

		pod := getPod(withVolume(getHostPathVolume("DirectoryOrCreate", hostPathLocation)))
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		// the full path should have been created
		st, err := os.Stat(hostPathLocation)
		Expect(err).ToNot(HaveOccurred())
		Expect(st.Mode().IsDir()).To(BeTrue())
	})

	It("podman play kube test with DirectoryOrCreate HostPath type volume and existent directory path", func() {
		hostPathLocation := filepath.Join(filepath.Join(tempdir, "dir1"), "dir2")
		Expect(os.MkdirAll(hostPathLocation, os.ModePerm)).To(Succeed())

		pod := getPod(withVolume(getHostPathVolume("DirectoryOrCreate", hostPathLocation)))
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))
	})

	It("podman play kube test with Socket HostPath type volume should fail if not socket", func() {
		hostPathLocation := filepath.Join(tempdir, "file")
		f, err := os.Create(hostPathLocation)
		Expect(err).ToNot(HaveOccurred())
		f.Close()

		pod := getPod(withVolume(getHostPathVolume("Socket", hostPathLocation)))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).To(ExitWithError())
	})

	It("podman play kube test with read-only HostPath volume", func() {
		hostPathLocation := filepath.Join(tempdir, "file")
		f, err := os.Create(hostPathLocation)
		Expect(err).ToNot(HaveOccurred())
		f.Close()

		ctr := getCtr(withVolumeMount(hostPathLocation, "", true), withImage(BB))
		pod := getPod(withVolume(getHostPathVolume("File", hostPathLocation)), withCtr(ctr))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod), "--format", "'{{.HostConfig.Binds}}'"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))

		correct := fmt.Sprintf("%s:%s:%s", hostPathLocation, hostPathLocation, "ro")
		Expect(inspect.OutputToString()).To(ContainSubstring(correct))
	})

	It("podman play kube test duplicate volume destination between host path and image volumes", func() {
		// Create host test directory and file
		testdir := "testdir"
		testfile := "testfile"

		hostPathDir := filepath.Join(tempdir, testdir)
		err := os.Mkdir(hostPathDir, 0755)
		Expect(err).ToNot(HaveOccurred())
		defer os.RemoveAll(hostPathDir)

		hostPathDirFile := filepath.Join(hostPathDir, testfile)
		f, err := os.Create(hostPathDirFile)
		Expect(err).ToNot(HaveOccurred())
		f.Close()

		if selinux.GetEnabled() {
			label := SystemExec("chcon", []string{"-t", "container_file_t", hostPathDirFile})
			Expect(label).Should(Exit(0))
		}

		// Create container image with named volume
		containerfile := fmt.Sprintf(`
FROM  %s
VOLUME %s`, ALPINE, hostPathDir+"/")

		image := "podman-kube-test:podman"
		podmanTest.BuildImage(containerfile, image, "false")

		// Create and play kube pod
		ctr := getCtr(withVolumeMount(hostPathDir+"/", "", false), withImage(image))
		pod := getPod(withCtr(ctr), withVolume(getHostPathVolume("Directory", hostPathDir+"/")))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		result := podmanTest.Podman([]string{"exec", getCtrNameInPod(pod), "ls", hostPathDir + "/" + testfile})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod)})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))

		// If two volumes are specified and share the same destination,
		// only one will be mounted. Host path volumes take precedence.
		ctrJSON := inspect.InspectContainerToJSON()
		Expect(ctrJSON[0].Mounts).To(HaveLen(1))
		Expect(ctrJSON[0].Mounts[0]).To(HaveField("Type", "bind"))

	})

	It("podman play kube test with PersistentVolumeClaim volume", func() {
		volumeName := "namedVolume"

		ctr := getCtr(withVolumeMount("/test", "", false), withImage(BB))
		pod := getPod(withVolume(getPersistentVolumeClaimVolume(volumeName)), withCtr(ctr))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod), "--format", "{{ (index .Mounts 0).Type }}:{{ (index .Mounts 0).Name }}"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))

		correct := fmt.Sprintf("volume:%s", volumeName)
		Expect(inspect.OutputToString()).To(Equal(correct))
	})

	It("podman play kube ConfigMap volume with no items", func() {
		volumeName := "cmVol"
		cm := getConfigMap(withConfigMapName(volumeName), withConfigMapData("FOO", "foobar"))
		cmYaml, err := getKubeYaml("configmap", cm)
		Expect(err).ToNot(HaveOccurred())

		ctr := getCtr(withVolumeMount("/test", "", false), withImage(BB))
		pod := getPod(withVolume(getConfigMapVolume(volumeName, []map[string]string{}, false)), withCtr(ctr))
		podYaml, err := getKubeYaml("pod", pod)
		Expect(err).ToNot(HaveOccurred())
		yamls := []string{cmYaml, podYaml}
		err = generateMultiDocKubeYaml(yamls, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		cmData := podmanTest.Podman([]string{"exec", getCtrNameInPod(pod), "cat", "/test/FOO"})
		cmData.WaitWithDefaultTimeout()
		Expect(cmData).Should(Exit(0))
		Expect(cmData.OutputToString()).To(Equal("foobar"))
	})

	It("podman play kube ConfigMap volume with items", func() {
		volumeName := "cmVol"
		cm := getConfigMap(withConfigMapName(volumeName), withConfigMapData("FOO", "foobar"))
		cmYaml, err := getKubeYaml("configmap", cm)
		Expect(err).ToNot(HaveOccurred())
		volumeContents := []map[string]string{{
			"key":  "FOO",
			"path": "BAR",
		}}

		ctr := getCtr(withVolumeMount("/test", "", false), withImage(BB))
		pod := getPod(withVolume(getConfigMapVolume(volumeName, volumeContents, false)), withCtr(ctr))
		podYaml, err := getKubeYaml("pod", pod)
		Expect(err).ToNot(HaveOccurred())
		yamls := []string{cmYaml, podYaml}
		err = generateMultiDocKubeYaml(yamls, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		cmData := podmanTest.Podman([]string{"exec", getCtrNameInPod(pod), "cat", "/test/BAR"})
		cmData.WaitWithDefaultTimeout()
		Expect(cmData).Should(Exit(0))
		Expect(cmData.OutputToString()).To(Equal("foobar"))

		cmData = podmanTest.Podman([]string{"exec", getCtrNameInPod(pod), "cat", "/test/FOO"})
		cmData.WaitWithDefaultTimeout()
		Expect(cmData).Should(Not(Exit(0)))
	})

	It("podman play kube with a missing optional ConfigMap volume", func() {
		volumeName := "cmVol"

		ctr := getCtr(withVolumeMount("/test", "", false), withImage(BB))
		pod := getPod(withVolume(getConfigMapVolume(volumeName, []map[string]string{}, true)), withCtr(ctr))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))
	})

	It("podman play kube with emptyDir volume", func() {
		podName := "test-pod"
		ctrName1 := "vol-test-ctr"
		ctrName2 := "vol-test-ctr-2"
		ctr1 := getCtr(withVolumeMount("/test-emptydir", "", false), withImage(BB), withName(ctrName1))
		ctr2 := getCtr(withVolumeMount("/test-emptydir-2", "", false), withImage(BB), withName(ctrName2))
		pod := getPod(withPodName(podName), withVolume(getEmptyDirVolume()), withCtr(ctr1), withCtr(ctr2))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		emptyDirCheck1 := podmanTest.Podman([]string{"exec", podName + "-" + ctrName1, "ls", "/test-emptydir"})
		emptyDirCheck1.WaitWithDefaultTimeout()
		Expect(emptyDirCheck1).Should(Exit(0))

		emptyDirCheck2 := podmanTest.Podman([]string{"exec", podName + "-" + ctrName2, "ls", "/test-emptydir-2"})
		emptyDirCheck2.WaitWithDefaultTimeout()
		Expect(emptyDirCheck2).Should(Exit(0))

		volList1 := podmanTest.Podman([]string{"volume", "ls", "-q"})
		volList1.WaitWithDefaultTimeout()
		Expect(volList1).Should(Exit(0))
		Expect(volList1.OutputToString()).To(Equal(defaultVolName))

		remove := podmanTest.Podman([]string{"pod", "rm", "-f", podName})
		remove.WaitWithDefaultTimeout()
		Expect(remove).Should(Exit(0))

		volList2 := podmanTest.Podman([]string{"volume", "ls", "-q"})
		volList2.WaitWithDefaultTimeout()
		Expect(volList2).Should(Exit(0))
		Expect(volList2.OutputToString()).To(Equal(""))
	})

	It("podman play kube applies labels to pods", func() {
		var numReplicas int32 = 5
		expectedLabelKey := "key1"
		expectedLabelValue := "value1"
		deployment := getDeployment(
			withReplicas(numReplicas),
			withPod(getPod(withLabel(expectedLabelKey, expectedLabelValue))),
		)
		err := generateKubeYaml("deployment", deployment, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		correctLabels := expectedLabelKey + ":" + expectedLabelValue
		pod := getPodNameInDeployment(deployment)
		inspect := podmanTest.Podman([]string{"pod", "inspect", pod.Name, "--format", "'{{ .Labels }}'"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(ContainSubstring(correctLabels))
	})

	It("podman play kube allows setting resource limits", func() {
		SkipIfContainerized("Resource limits require a running systemd")
		SkipIfRootless("CPU limits require root")
		podmanTest.CgroupManager = "systemd"

		var (
			numReplicas           int32  = 3
			expectedCPURequest    string = "100m"
			expectedCPULimit      string = "200m"
			expectedMemoryRequest string = "10000000"
			expectedMemoryLimit   string = "20000000"
		)

		expectedCPUQuota := milliCPUToQuota(expectedCPULimit)

		deployment := getDeployment(
			withReplicas(numReplicas),
			withPod(getPod(withCtr(getCtr(
				withCPURequest(expectedCPURequest),
				withCPULimit(expectedCPULimit),
				withMemoryRequest(expectedMemoryRequest),
				withMemoryLimit(expectedMemoryLimit),
			)))))
		err := generateKubeYaml("deployment", deployment, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		pod := getPodNameInDeployment(deployment)
		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(&pod), "--format", `
CpuPeriod: {{ .HostConfig.CpuPeriod }}
CpuQuota: {{ .HostConfig.CpuQuota }}
Memory: {{ .HostConfig.Memory }}
MemoryReservation: {{ .HostConfig.MemoryReservation }}`})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(ContainSubstring(fmt.Sprintf("%s: %d", "CpuQuota", expectedCPUQuota)))
		Expect(inspect.OutputToString()).To(ContainSubstring("MemoryReservation: " + expectedMemoryRequest))
		Expect(inspect.OutputToString()).To(ContainSubstring("Memory: " + expectedMemoryLimit))

	})

	It("podman play kube allows setting resource limits with --cpus 1", func() {
		SkipIfContainerized("Resource limits require a running systemd")
		SkipIfRootless("CPU limits require root")
		podmanTest.CgroupManager = "systemd"

		var (
			expectedCPULimit string = "1"
		)

		deployment := getDeployment(
			withPod(getPod(withCtr(getCtr(
				withCPULimit(expectedCPULimit),
			)))))
		err := generateKubeYaml("deployment", deployment, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		pod := getPodNameInDeployment(deployment)
		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(&pod), "--format", `{{ .HostConfig.CpuPeriod }}:{{ .HostConfig.CpuQuota }}`})

		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))

		parts := strings.Split(strings.Trim(inspect.OutputToString(), "\n"), ":")
		Expect(parts).To(HaveLen(2))

		Expect(parts[0]).To(Equal(parts[1]))

	})

	It("podman play kube reports invalid image name", func() {
		invalidImageName := "./myimage"

		pod := getPod(
			withCtr(
				getCtr(
					withImage(invalidImageName),
				),
			),
		)
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(125))
		Expect(kube.ErrorToString()).To(ContainSubstring("invalid reference format"))
	})

	It("podman play kube applies log driver to containers", func() {
		SkipIfInContainer("journald inside a container doesn't work")
		pod := getPod()
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", "--log-opt=max-size=10k", "--log-driver", "journald", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		cid := getCtrNameInPod(pod)
		inspect := podmanTest.Podman([]string{"inspect", cid, "--format", "'{{ .HostConfig.LogConfig.Type }}'"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(ContainSubstring("journald"))
		inspect = podmanTest.Podman([]string{"container", "inspect", "--format", "{{.HostConfig.LogConfig.Size}}", cid})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).To(Exit(0))
		Expect(inspect.OutputToString()).To(Equal("10kB"))
	})

	It("podman play kube test only creating the containers", func() {
		pod := getPod()
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", "--start=false", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod), "--format", "{{ .State.Running }}"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(Equal("false"))
	})

	It("podman play kube test with HostNetwork", func() {
		pod := getPod(withHostNetwork(), withCtr(getCtr(withCmd([]string{"readlink", "/proc/self/ns/net"}), withArg(nil))))
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", pod.Name, "--format", "{{ .InfraConfig.HostNetwork }}"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(Equal("true"))

		ns := SystemExec("readlink", []string{"/proc/self/ns/net"})
		ns.WaitWithDefaultTimeout()
		Expect(ns).Should(Exit(0))
		netns := ns.OutputToString()
		Expect(netns).ToNot(BeEmpty())

		logs := podmanTest.Podman([]string{"logs", getCtrNameInPod(pod)})
		logs.WaitWithDefaultTimeout()
		Expect(logs).Should(Exit(0))
		Expect(logs.OutputToString()).To(Equal(netns))
	})

	It("podman play kube test with kube default network", func() {
		pod := getPod()
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", pod.Name, "--format", "{{ .InfraConfig.Networks }}"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(Equal("[podman-default-kube-network]"))
	})

	It("podman play kube persistentVolumeClaim", func() {
		volName := "myvol"
		volDevice := "tmpfs"
		volType := "tmpfs"
		volOpts := "nodev,noexec"

		pvc := getPVC(withPVCName(volName),
			withPVCAnnotations(util.VolumeDeviceAnnotation, volDevice),
			withPVCAnnotations(util.VolumeTypeAnnotation, volType),
			withPVCAnnotations(util.VolumeMountOptsAnnotation, volOpts))
		err = generateKubeYaml("persistentVolumeClaim", pvc, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", volName, "--format", `
Name: {{ .Name }}
Device: {{ .Options.device }}
Type: {{ .Options.type }}
o: {{ .Options.o }}`})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(ContainSubstring("Name: " + volName))
		Expect(inspect.OutputToString()).To(ContainSubstring("Device: " + volDevice))
		Expect(inspect.OutputToString()).To(ContainSubstring("Type: " + volType))
		Expect(inspect.OutputToString()).To(ContainSubstring("o: " + volOpts))
	})

	It("podman play kube persistentVolumeClaim with source", func() {
		fileName := "data"
		expectedFileContent := "Test"
		tarFilePath := filepath.Join(os.TempDir(), "podmanVolumeSource.tgz")
		err := createSourceTarFile(fileName, expectedFileContent, tarFilePath)
		Expect(err).ToNot(HaveOccurred())

		volName := "myVolWithStorage"
		pvc := getPVC(withPVCName(volName),
			withPVCAnnotations(util.VolumeImportSourceAnnotation, tarFilePath),
		)
		err = generateKubeYaml("persistentVolumeClaim", pvc, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		if IsRemote() {
			Expect(kube).Error()
			Expect(kube.ErrorToString()).To(ContainSubstring("importing volumes is not supported for remote requests"))
			return
		}
		Expect(kube).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", volName, "--format", `
{
	"Name": "{{ .Name }}",
	"Mountpoint": "{{ .Mountpoint }}"
}`})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		mp := make(map[string]string)
		err = json.Unmarshal([]byte(inspect.OutputToString()), &mp)
		Expect(err).ToNot(HaveOccurred())
		Expect(mp["Name"]).To(Equal(volName))
		files, err := os.ReadDir(mp["Mountpoint"])
		Expect(err).ToNot(HaveOccurred())
		Expect(files).To(HaveLen(1))
		Expect(files[0].Name()).To(Equal(fileName))
	})

	// Multi doc related tests
	It("podman play kube multi doc yaml with persistentVolumeClaim, service and deployment", func() {
		yamlDocs := []string{}

		serviceTemplate := `apiVersion: v1
kind: Service
metadata:
  name: %s
spec:
  ports:
  - port: 80
    protocol: TCP
    targetPort: 9376
  selector:
    app: %s
`
		// generate persistentVolumeClaim
		volName := "multiFoo"
		pvc := getPVC(withPVCName(volName))

		// generate deployment
		deploymentName := "multiFoo"
		podName := "multiFoo"
		ctrName := "ctr-01"
		ctr := getCtr(withVolumeMount("/test", "", false))
		ctr.Name = ctrName
		pod := getPod(withPodName(podName), withVolume(getPersistentVolumeClaimVolume(volName)), withCtr(ctr))
		deployment := getDeployment(withPod(pod))
		deployment.Name = deploymentName

		// add pvc
		k, err := getKubeYaml("persistentVolumeClaim", pvc)
		Expect(err).ToNot(HaveOccurred())
		yamlDocs = append(yamlDocs, k)

		// add service
		yamlDocs = append(yamlDocs, fmt.Sprintf(serviceTemplate, deploymentName, deploymentName))

		// add deployment
		k, err = getKubeYaml("deployment", deployment)
		Expect(err).ToNot(HaveOccurred())
		yamlDocs = append(yamlDocs, k)

		// generate multi doc yaml
		err = generateMultiDocKubeYaml(yamlDocs, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		inspectVolume := podmanTest.Podman([]string{"inspect", volName, "--format", "'{{ .Name }}'"})
		inspectVolume.WaitWithDefaultTimeout()
		Expect(inspectVolume).Should(Exit(0))
		Expect(inspectVolume.OutputToString()).To(ContainSubstring(volName))

		inspectPod := podmanTest.Podman([]string{"inspect", podName + "-pod", "--format", "'{{ .State }}'"})
		inspectPod.WaitWithDefaultTimeout()
		Expect(inspectPod).Should(Exit(0))
		Expect(inspectPod.OutputToString()).To(ContainSubstring(`Running`))

		inspectMounts := podmanTest.Podman([]string{"inspect", podName + "-pod-" + ctrName, "--format", "{{ (index .Mounts 0).Type }}:{{ (index .Mounts 0).Name }}"})
		inspectMounts.WaitWithDefaultTimeout()
		Expect(inspectMounts).Should(Exit(0))

		correct := fmt.Sprintf("volume:%s", volName)
		Expect(inspectMounts.OutputToString()).To(Equal(correct))
	})

	It("podman play kube multi doc yaml with multiple services, pods and deployments", func() {
		yamlDocs := []string{}
		podNames := []string{}

		serviceTemplate := `apiVersion: v1
kind: Service
metadata:
  name: %s
spec:
  ports:
  - port: 80
    protocol: TCP
    targetPort: 9376
  selector:
    app: %s
`
		// generate services, pods and deployments
		for i := 0; i < 2; i++ {
			podName := fmt.Sprintf("testPod%d", i)
			deploymentName := fmt.Sprintf("testDeploy%d", i)
			deploymentPodName := fmt.Sprintf("%s-pod", deploymentName)

			podNames = append(podNames, podName)
			podNames = append(podNames, deploymentPodName)

			pod := getPod(withPodName(podName))
			podDeployment := getPod(withPodName(deploymentName))
			deployment := getDeployment(withPod(podDeployment))
			deployment.Name = deploymentName

			// add services
			yamlDocs = append([]string{
				fmt.Sprintf(serviceTemplate, podName, podName),
				fmt.Sprintf(serviceTemplate, deploymentPodName, deploymentPodName)}, yamlDocs...)

			// add pods
			k, err := getKubeYaml("pod", pod)
			Expect(err).ToNot(HaveOccurred())
			yamlDocs = append(yamlDocs, k)

			// add deployments
			k, err = getKubeYaml("deployment", deployment)
			Expect(err).ToNot(HaveOccurred())
			yamlDocs = append(yamlDocs, k)
		}

		// generate multi doc yaml
		err = generateMultiDocKubeYaml(yamlDocs, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		for _, n := range podNames {
			inspect := podmanTest.Podman([]string{"inspect", n, "--format", "'{{ .State }}'"})
			inspect.WaitWithDefaultTimeout()
			Expect(inspect).Should(Exit(0))
			Expect(inspect.OutputToString()).To(ContainSubstring(`Running`))
		}
	})

	It("podman play kube invalid multi doc yaml", func() {
		yamlDocs := []string{}

		serviceTemplate := `apiVersion: v1
kind: Service
metadata:
  name: %s
spec:
  ports:
  - port: 80
    protocol: TCP
    targetPort: 9376
  selector:
	app: %s
---
invalid kube kind
`
		// add invalid multi doc yaml
		yamlDocs = append(yamlDocs, fmt.Sprintf(serviceTemplate, "foo", "foo"))

		// add pod
		pod := getPod()
		k, err := getKubeYaml("pod", pod)
		Expect(err).ToNot(HaveOccurred())
		yamlDocs = append(yamlDocs, k)

		// generate multi doc yaml
		err = generateMultiDocKubeYaml(yamlDocs, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).To(ExitWithError())
	})

	It("podman play kube with auto update annotations for all containers", func() {
		ctr01Name := "ctr01"
		ctr02Name := "infra"
		podName := "foo"
		autoUpdateRegistry := "io.containers.autoupdate"
		autoUpdateRegistryValue := "registry"
		autoUpdateAuthfile := "io.containers.autoupdate.authfile"
		autoUpdateAuthfileValue := "/some/authfile.json"

		ctr01 := getCtr(withName(ctr01Name))
		ctr02 := getCtr(withName(ctr02Name))

		pod := getPod(
			withPodName(podName),
			withCtr(ctr01),
			withCtr(ctr02),
			withAnnotation(autoUpdateRegistry, autoUpdateRegistryValue),
			withAnnotation(autoUpdateAuthfile, autoUpdateAuthfileValue))

		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		for _, ctr := range []string{podName + "-" + ctr01Name, podName + "-" + ctr02Name} {
			inspect := podmanTest.Podman([]string{"inspect", ctr, "--format", "'{{.Config.Labels}}'"})
			inspect.WaitWithDefaultTimeout()
			Expect(inspect).Should(Exit(0))

			Expect(inspect.OutputToString()).To(ContainSubstring(autoUpdateRegistry + ":" + autoUpdateRegistryValue))
			Expect(inspect.OutputToString()).To(ContainSubstring(autoUpdateAuthfile + ":" + autoUpdateAuthfileValue))
		}
	})

	It("podman play kube with auto update annotations for first container only", func() {
		ctr01Name := "ctr01"
		ctr02Name := "ctr02"
		autoUpdateRegistry := "io.containers.autoupdate"
		autoUpdateRegistryValue := "local"

		ctr01 := getCtr(withName(ctr01Name))
		ctr02 := getCtr(withName(ctr02Name))

		pod := getPod(
			withCtr(ctr01),
			withCtr(ctr02),
		)

		deployment := getDeployment(
			withPod(pod),
			withDeploymentAnnotation(autoUpdateRegistry+"/"+ctr01Name, autoUpdateRegistryValue),
		)

		err = generateKubeYaml("deployment", deployment, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		podName := getPodNameInDeployment(deployment).Name

		inspect := podmanTest.Podman([]string{"inspect", podName + "-" + ctr01Name, "--format", "'{{.Config.Labels}}'"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(ContainSubstring(autoUpdateRegistry + ":" + autoUpdateRegistryValue))

		inspect = podmanTest.Podman([]string{"inspect", podName + "-" + ctr02Name, "--format", "'{{.Config.Labels}}'"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).NotTo(ContainSubstring(autoUpdateRegistry + ":" + autoUpdateRegistryValue))
	})

	It("podman play kube teardown", func() {
		pod := getPod()
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		ls := podmanTest.Podman([]string{"pod", "ps", "--format", "'{{.ID}}'"})
		ls.WaitWithDefaultTimeout()
		Expect(ls).Should(Exit(0))
		Expect(ls.OutputToStringArray()).To(HaveLen(1))

		//	 teardown
		teardown := podmanTest.Podman([]string{"play", "kube", "--down", kubeYaml})
		teardown.WaitWithDefaultTimeout()
		Expect(teardown).Should(Exit(0))

		checkls := podmanTest.Podman([]string{"pod", "ps", "--format", "'{{.ID}}'"})
		checkls.WaitWithDefaultTimeout()
		Expect(checkls).Should(Exit(0))
		Expect(checkls.OutputToStringArray()).To(BeEmpty())
	})

	It("podman play kube teardown pod does not exist", func() {
		//	 teardown
		teardown := podmanTest.Podman([]string{"play", "kube", "--down", kubeYaml})
		teardown.WaitWithDefaultTimeout()
		Expect(teardown).Should(Exit(125))
	})

	It("podman play kube teardown with volume without force delete", func() {

		volName := RandomString(12)
		volDevice := "tmpfs"
		volType := "tmpfs"
		volOpts := "nodev,noexec"

		pvc := getPVC(withPVCName(volName),
			withPVCAnnotations(util.VolumeDeviceAnnotation, volDevice),
			withPVCAnnotations(util.VolumeTypeAnnotation, volType),
			withPVCAnnotations(util.VolumeMountOptsAnnotation, volOpts))
		err = generateKubeYaml("persistentVolumeClaim", pvc, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		exists := podmanTest.Podman([]string{"volume", "exists", volName})
		exists.WaitWithDefaultTimeout()
		Expect(exists).To(Exit(0))

		teardown := podmanTest.Podman([]string{"play", "kube", "--down", kubeYaml})
		teardown.WaitWithDefaultTimeout()
		Expect(teardown).To(Exit(0))

		// volume should not be deleted on teardown
		exists = podmanTest.Podman([]string{"volume", "exists", volName})
		exists.WaitWithDefaultTimeout()
		Expect(exists).To(Exit(0))
	})

	It("podman play kube teardown with volume force delete", func() {

		volName := RandomString(12)
		volDevice := "tmpfs"
		volType := "tmpfs"
		volOpts := "nodev,noexec"

		pvc := getPVC(withPVCName(volName),
			withPVCAnnotations(util.VolumeDeviceAnnotation, volDevice),
			withPVCAnnotations(util.VolumeTypeAnnotation, volType),
			withPVCAnnotations(util.VolumeMountOptsAnnotation, volOpts))
		err = generateKubeYaml("persistentVolumeClaim", pvc, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		exists := podmanTest.Podman([]string{"volume", "exists", volName})
		exists.WaitWithDefaultTimeout()
		Expect(exists).To(Exit(0))

		teardown := podmanTest.Podman([]string{"play", "kube", "--down", "--force", kubeYaml})
		teardown.WaitWithDefaultTimeout()
		Expect(teardown).To(Exit(0))

		// volume should not be deleted on teardown
		exists = podmanTest.Podman([]string{"volume", "exists", volName})
		exists.WaitWithDefaultTimeout()
		Expect(exists).To(Exit(1))
	})

	It("podman play kube after teardown with volume reuse", func() {

		volName := RandomString(12)
		volDevice := "tmpfs"
		volType := "tmpfs"
		volOpts := "nodev,noexec"

		pvc := getPVC(withPVCName(volName),
			withPVCAnnotations(util.VolumeDeviceAnnotation, volDevice),
			withPVCAnnotations(util.VolumeTypeAnnotation, volType),
			withPVCAnnotations(util.VolumeMountOptsAnnotation, volOpts))
		err = generateKubeYaml("persistentVolumeClaim", pvc, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		exists := podmanTest.Podman([]string{"volume", "exists", volName})
		exists.WaitWithDefaultTimeout()
		Expect(exists).To(Exit(0))

		teardown := podmanTest.Podman([]string{"play", "kube", "--down", kubeYaml})
		teardown.WaitWithDefaultTimeout()
		Expect(teardown).To(Exit(0))

		// volume should not be deleted on teardown
		exists = podmanTest.Podman([]string{"volume", "exists", volName})
		exists.WaitWithDefaultTimeout()
		Expect(exists).To(Exit(0))

		restart := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		restart.WaitWithDefaultTimeout()
		Expect(restart).To(Exit(0))
	})

	It("podman play kube use network mode from config", func() {
		confPath, err := filepath.Abs("config/containers-netns2.conf")
		Expect(err).ToNot(HaveOccurred())
		os.Setenv("CONTAINERS_CONF", confPath)
		defer os.Unsetenv("CONTAINERS_CONF")
		if IsRemote() {
			podmanTest.RestartRemoteService()
		}

		pod := getPod()
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		podInspect := podmanTest.Podman([]string{"pod", "inspect", pod.Name, "--format", "{{.InfraContainerID}}"})
		podInspect.WaitWithDefaultTimeout()
		Expect(podInspect).To(Exit(0))
		infraID := podInspect.OutputToString()

		inspect := podmanTest.Podman([]string{"inspect", "--format", "{{.HostConfig.NetworkMode}}", infraID})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).To(Exit(0))
		Expect(inspect.OutputToString()).To(Equal("bridge"))
	})

	It("podman play kube replace", func() {
		pod := getPod()
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		ls := podmanTest.Podman([]string{"pod", "ps", "--format", "'{{.ID}}'"})
		ls.WaitWithDefaultTimeout()
		Expect(ls).Should(Exit(0))
		Expect(ls.OutputToStringArray()).To(HaveLen(1))

		containerLen := podmanTest.Podman([]string{"pod", "inspect", pod.Name, "--format", "{{len .Containers}}"})
		containerLen.WaitWithDefaultTimeout()
		Expect(containerLen).Should(Exit(0))
		Expect(containerLen.OutputToString()).To(Equal("2"))
		ctr01Name := "ctr01"
		ctr02Name := "ctr02"

		ctr01 := getCtr(withName(ctr01Name))
		ctr02 := getCtr(withName(ctr02Name))

		newPod := getPod(
			withCtr(ctr01),
			withCtr(ctr02),
		)
		err = generateKubeYaml("pod", newPod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		replace := podmanTest.Podman([]string{"play", "kube", "--replace", kubeYaml})
		replace.WaitWithDefaultTimeout()
		Expect(replace).Should(Exit(0))

		newContainerLen := podmanTest.Podman([]string{"pod", "inspect", newPod.Name, "--format", "{{len .Containers}}"})
		newContainerLen.WaitWithDefaultTimeout()
		Expect(newContainerLen).Should(Exit(0))
		Expect(newContainerLen.OutputToString()).NotTo(Equal(containerLen.OutputToString()))
	})

	It("podman play kube replace non-existing pod", func() {
		pod := getPod()
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		replace := podmanTest.Podman([]string{"play", "kube", "--replace", kubeYaml})
		replace.WaitWithDefaultTimeout()
		Expect(replace).Should(Exit(0))

		ls := podmanTest.Podman([]string{"pod", "ps", "--format", "'{{.ID}}'"})
		ls.WaitWithDefaultTimeout()
		Expect(ls).Should(Exit(0))
		Expect(ls.OutputToStringArray()).To(HaveLen(1))
	})

	It("podman play kube RunAsUser", func() {
		ctr1Name := "ctr1"
		ctr2Name := "ctr2"
		ctr1 := getCtr(withName(ctr1Name), withSecurityContext(true), withRunAsUser("101"), withRunAsGroup("102"))
		ctr2 := getCtr(withName(ctr2Name), withSecurityContext(true))

		pod := getPod(
			withCtr(ctr1),
			withCtr(ctr2),
			withPodSecurityContext(true),
			withPodRunAsUser("103"),
			withPodRunAsGroup("104"),
		)

		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		cmd := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		cmd.WaitWithDefaultTimeout()
		Expect(cmd).Should(Exit(0))

		// we expect the user:group as configured for the container
		inspect := podmanTest.Podman([]string{"container", "inspect", "--format", "'{{.Config.User}}'", makeCtrNameInPod(pod, ctr1Name)})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.OutputToString()).To(Equal("'101:102'"))

		// we expect the user:group as configured for the pod
		inspect = podmanTest.Podman([]string{"container", "inspect", "--format", "'{{.Config.User}}'", makeCtrNameInPod(pod, ctr2Name)})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.OutputToString()).To(Equal("'103:104'"))
	})

	Describe("verify environment variables", func() {
		var maxLength int
		BeforeEach(func() {
			maxLength = format.MaxLength
			format.MaxLength = 0
		})
		AfterEach(func() {
			format.MaxLength = maxLength
		})

		It("values containing equal sign", func() {
			javaToolOptions := `-XX:+IgnoreUnrecognizedVMOptions -XX:+IdleTuningGcOnIdle -Xshareclasses:name=openj9_system_scc,cacheDir=/opt/java/.scc,readonly,nonFatal`
			openj9JavaOptions := `-XX:+IgnoreUnrecognizedVMOptions -XX:+IdleTuningGcOnIdle -Xshareclasses:name=openj9_system_scc,cacheDir=/opt/java/.scc,readonly,nonFatal -Dosgi.checkConfiguration=false`

			containerfile := fmt.Sprintf(`FROM %s
ENV JAVA_TOOL_OPTIONS=%q
ENV OPENJ9_JAVA_OPTIONS=%q
`,
				ALPINE, javaToolOptions, openj9JavaOptions)

			image := "podman-kube-test:env"
			podmanTest.BuildImage(containerfile, image, "false")
			ctnr := getCtr(withImage(image))
			pod := getPod(withCtr(ctnr))
			Expect(generateKubeYaml("pod", pod, kubeYaml)).Should(Succeed())

			play := podmanTest.Podman([]string{"play", "kube", "--start", kubeYaml})
			play.WaitWithDefaultTimeout()
			Expect(play).Should(Exit(0))

			inspect := podmanTest.Podman([]string{"container", "inspect", "--format=json", getCtrNameInPod(pod)})
			inspect.WaitWithDefaultTimeout()
			Expect(inspect).Should(Exit(0))

			contents := string(inspect.Out.Contents())
			Expect(contents).To(ContainSubstring(javaToolOptions))
			Expect(contents).To(ContainSubstring(openj9JavaOptions))
		})
	})

	Context("with configmap in multi-doc yaml", func() {
		It("podman play kube uses env value", func() {
			cm := getConfigMap(withConfigMapName("foo"), withConfigMapData("FOO", "foo"))

			cmYaml, err := getKubeYaml("configmap", cm)
			Expect(err).ToNot(HaveOccurred())

			pod := getPod(withCtr(getCtr(withEnv("FOO", "", "configmap", "foo", "FOO", false))))

			podYaml, err := getKubeYaml("pod", pod)
			Expect(err).ToNot(HaveOccurred())

			yamls := []string{cmYaml, podYaml}
			err = generateMultiDocKubeYaml(yamls, kubeYaml)
			Expect(err).ToNot(HaveOccurred())

			kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
			kube.WaitWithDefaultTimeout()
			Expect(kube).Should(Exit(0))

			inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod), "--format", "'{{ .Config.Env }}'"})
			inspect.WaitWithDefaultTimeout()
			Expect(inspect).Should(Exit(0))
			Expect(inspect.OutputToString()).To(ContainSubstring(`FOO=foo`))
		})

		It("podman play kube fails for required env value with missing key", func() {
			cm := getConfigMap(withConfigMapName("foo"), withConfigMapData("FOO", "foo"))

			cmYaml, err := getKubeYaml("configmap", cm)
			Expect(err).ToNot(HaveOccurred())

			pod := getPod(withCtr(getCtr(withEnv("FOO", "", "configmap", "foo", "MISSING_KEY", false))))

			podYaml, err := getKubeYaml("pod", pod)
			Expect(err).ToNot(HaveOccurred())

			yamls := []string{cmYaml, podYaml}
			err = generateMultiDocKubeYaml(yamls, kubeYaml)
			Expect(err).ToNot(HaveOccurred())

			kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
			kube.WaitWithDefaultTimeout()
			Expect(kube).To(ExitWithError())
		})

		It("podman play kube succeeds for optional env value with missing key", func() {
			cm := getConfigMap(withConfigMapName("foo"), withConfigMapData("FOO", "foo"))

			cmYaml, err := getKubeYaml("configmap", cm)
			Expect(err).ToNot(HaveOccurred())

			pod := getPod(withCtr(getCtr(withEnv("FOO", "", "configmap", "foo", "MISSING_KEY", true))))

			podYaml, err := getKubeYaml("pod", pod)
			Expect(err).ToNot(HaveOccurred())

			yamls := []string{cmYaml, podYaml}
			err = generateMultiDocKubeYaml(yamls, kubeYaml)
			Expect(err).ToNot(HaveOccurred())

			kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
			kube.WaitWithDefaultTimeout()
			Expect(kube).Should(Exit(0))

			inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod), "--format", "'{{ range .Config.Env }}[{{ . }}]{{end}}'"})
			inspect.WaitWithDefaultTimeout()
			Expect(inspect).Should(Exit(0))
			Expect(inspect.OutputToString()).To(Not(ContainSubstring(`[FOO=]`)))
		})

		It("podman play kube uses all key-value pairs as envs", func() {
			cm := getConfigMap(withConfigMapName("foo"), withConfigMapData("FOO1", "foo1"), withConfigMapData("FOO2", "foo2"))
			cmYaml, err := getKubeYaml("configmap", cm)
			Expect(err).ToNot(HaveOccurred())

			pod := getPod(withCtr(getCtr(withEnvFrom("foo", "configmap", false))))

			podYaml, err := getKubeYaml("pod", pod)
			Expect(err).ToNot(HaveOccurred())

			yamls := []string{cmYaml, podYaml}
			err = generateMultiDocKubeYaml(yamls, kubeYaml)
			Expect(err).ToNot(HaveOccurred())

			kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
			kube.WaitWithDefaultTimeout()
			Expect(kube).Should(Exit(0))

			inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod), "--format", "'{{ .Config.Env }}'"})
			inspect.WaitWithDefaultTimeout()
			Expect(inspect).Should(Exit(0))
			Expect(inspect.OutputToString()).To(ContainSubstring(`FOO1=foo1`))
			Expect(inspect.OutputToString()).To(ContainSubstring(`FOO2=foo2`))
		})

		It("podman play kube deployment uses variable from config map", func() {
			cm := getConfigMap(withConfigMapName("foo"), withConfigMapData("FOO", "foo"))

			cmYaml, err := getKubeYaml("configmap", cm)
			Expect(err).ToNot(HaveOccurred())

			pod := getPod(withCtr(getCtr(withEnv("FOO", "", "configmap", "foo", "FOO", false))))

			deployment := getDeployment(withPod(pod))
			deploymentYaml, err := getKubeYaml("deployment", deployment)
			Expect(err).ToNot(HaveOccurred(), "getKubeYaml(deployment)")
			yamls := []string{cmYaml, deploymentYaml}
			err = generateMultiDocKubeYaml(yamls, kubeYaml)
			Expect(err).ToNot(HaveOccurred())

			kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
			kube.WaitWithDefaultTimeout()
			Expect(kube).Should(Exit(0))

			inspect := podmanTest.Podman([]string{"inspect", fmt.Sprintf("%s-%s-%s", deployment.Name, "pod", defaultCtrName), "--format", "'{{ .Config }}'"})
			inspect.WaitWithDefaultTimeout()
			Expect(inspect).Should(Exit(0))
			Expect(inspect.OutputToString()).To(ContainSubstring(`FOO=foo`))

		})

		It("podman play kube uses env value from configmap for HTTP API client", func() {
			SkipIfRemote("cannot run in a remote setup")
			address := url.URL{
				Scheme: "tcp",
				Host:   net.JoinHostPort("localhost", "8080"),
			}

			session := podmanTest.Podman([]string{
				"system", "service", "--log-level=debug", "--time=0", address.String(),
			})
			defer session.Kill()

			WaitForService(address)

			cm := getConfigMap(withConfigMapName("foo"), withConfigMapData("FOO", "foo"))
			cmYaml, err := getKubeYaml("configmap", cm)
			Expect(err).ToNot(HaveOccurred())

			pod := getPod(withCtr(getCtr(withEnv("FOO", "", "configmap", "foo", "FOO", false))))
			podYaml, err := getKubeYaml("pod", pod)
			Expect(err).ToNot(HaveOccurred())
			yamls := []string{cmYaml, podYaml}
			err = generateMultiDocKubeYaml(yamls, kubeYaml)
			Expect(err).ToNot(HaveOccurred())

			podmanConnection, err := bindings.NewConnection(context.Background(), address.String())
			Expect(err).ToNot(HaveOccurred())

			_, err = play.Kube(podmanConnection, kubeYaml, nil)
			Expect(err).ToNot(HaveOccurred())

			inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod), "--format", "'{{ .Config.Env }}'"})
			inspect.WaitWithDefaultTimeout()
			Expect(inspect).Should(Exit(0))
			Expect(inspect.OutputToString()).To(ContainSubstring(`FOO=foo`))
		})
	})

	Context("with configmap in multi-doc yaml and files", func() {
		It("podman play kube uses env values from both sources", func() {
			SkipIfRemote("--configmaps is not supported for remote")

			fsCmYamlPathname := filepath.Join(podmanTest.TempDir, "foo-cm.yaml")
			fsCm := getConfigMap(withConfigMapName("fooFs"), withConfigMapData("FOO_FS", "fooFS"))
			err := generateKubeYaml("configmap", fsCm, fsCmYamlPathname)
			Expect(err).ToNot(HaveOccurred())

			cm := getConfigMap(withConfigMapName("foo"), withConfigMapData("FOO", "foo"))

			cmYaml, err := getKubeYaml("configmap", cm)
			Expect(err).ToNot(HaveOccurred())

			pod := getPod(withCtr(getCtr(
				withEnv("FOO_FS", "", "configmap", "fooFs", "FOO_FS", false),
				withEnv("FOO", "", "configmap", "foo", "FOO", false),
			)))

			podYaml, err := getKubeYaml("pod", pod)
			Expect(err).ToNot(HaveOccurred())

			yamls := []string{cmYaml, podYaml}
			err = generateMultiDocKubeYaml(yamls, kubeYaml)
			Expect(err).ToNot(HaveOccurred())

			kube := podmanTest.Podman([]string{"play", "kube", kubeYaml, "--configmap", fsCmYamlPathname})
			kube.WaitWithDefaultTimeout()
			Expect(kube).Should(Exit(0))

			inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod), "--format", "'{{ .Config.Env }}'"})
			inspect.WaitWithDefaultTimeout()
			Expect(inspect).Should(Exit(0))
			Expect(inspect.OutputToString()).To(And(
				ContainSubstring(`FOO=foo`),
				ContainSubstring(`FOO_FS=fooFS`),
			))
		})

		It("podman play kube uses all env values from both sources", func() {
			SkipIfRemote("--configmaps is not supported for remote")

			fsCmYamlPathname := filepath.Join(podmanTest.TempDir, "foo-cm.yaml")
			fsCm := getConfigMap(withConfigMapName("fooFs"),
				withConfigMapData("FOO_FS_1", "fooFS1"),
				withConfigMapData("FOO_FS_2", "fooFS2"))
			err := generateKubeYaml("configmap", fsCm, fsCmYamlPathname)
			Expect(err).ToNot(HaveOccurred())

			cm := getConfigMap(withConfigMapName("foo"),
				withConfigMapData("FOO_1", "foo1"),
				withConfigMapData("FOO_2", "foo2"),
			)

			cmYaml, err := getKubeYaml("configmap", cm)
			Expect(err).ToNot(HaveOccurred())

			pod := getPod(withCtr(getCtr(
				withEnvFrom("foo", "configmap", false),
				withEnvFrom("fooFs", "configmap", false),
			)))

			podYaml, err := getKubeYaml("pod", pod)
			Expect(err).ToNot(HaveOccurred())

			yamls := []string{cmYaml, podYaml}
			err = generateMultiDocKubeYaml(yamls, kubeYaml)
			Expect(err).ToNot(HaveOccurred())

			kube := podmanTest.Podman([]string{"play", "kube", kubeYaml, "--configmap", fsCmYamlPathname})
			kube.WaitWithDefaultTimeout()
			Expect(kube).Should(Exit(0))

			inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod), "--format", "'{{ .Config.Env }}'"})
			inspect.WaitWithDefaultTimeout()
			Expect(inspect).Should(Exit(0))
			Expect(inspect.OutputToString()).To(And(
				ContainSubstring(`FOO_1=foo1`),
				ContainSubstring(`FOO_2=foo2`),
				ContainSubstring(`FOO_FS_1=fooFS1`),
				ContainSubstring(`FOO_FS_2=fooFS2`),
			))
		})

		It("podman play kube reports error when the same configmap name is present in both sources", func() {
			SkipIfRemote("--configmaps is not supported for remote")

			fsCmYamlPathname := filepath.Join(podmanTest.TempDir, "foo-cm.yaml")
			fsCm := getConfigMap(withConfigMapName("foo"), withConfigMapData("FOO", "fooFS"))
			err := generateKubeYaml("configmap", fsCm, fsCmYamlPathname)
			Expect(err).ToNot(HaveOccurred())

			cm := getConfigMap(withConfigMapName("foo"), withConfigMapData("FOO", "foo"))

			cmYaml, err := getKubeYaml("configmap", cm)
			Expect(err).ToNot(HaveOccurred())

			pod := getPod(withCtr(getCtr(
				withEnv("FOO", "", "configmap", "foo", "FOO", false),
			)))

			podYaml, err := getKubeYaml("pod", pod)
			Expect(err).ToNot(HaveOccurred())

			yamls := []string{cmYaml, podYaml}
			err = generateMultiDocKubeYaml(yamls, kubeYaml)
			Expect(err).ToNot(HaveOccurred())

			kube := podmanTest.Podman([]string{"play", "kube", kubeYaml, "--configmap", fsCmYamlPathname})
			kube.WaitWithDefaultTimeout()
			Expect(kube).Should(Exit(125))
			Expect(kube.ErrorToString()).To(ContainSubstring("ambiguous configuration: the same config map foo is present in YAML and in --configmaps"))
		})
	})

	It("podman play kube --log-opt = tag test", func() {
		SkipIfContainerized("journald does not work inside the container")
		pod := getPod()
		err := generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml, "--log-driver", "journald", "--log-opt", "tag={{.ImageName}}"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		start := podmanTest.Podman([]string{"start", getCtrNameInPod(pod)})
		start.WaitWithDefaultTimeout()
		Expect(start).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", getCtrNameInPod(pod)})
		inspect.WaitWithDefaultTimeout()
		Expect(start).Should(Exit(0))
		Expect((inspect.InspectContainerToJSON()[0]).HostConfig.LogConfig.Tag).To(Equal("{{.ImageName}}"))
	})

	It("podman play kube using a user namespace", func() {
		u, err := user.Current()
		Expect(err).ToNot(HaveOccurred())
		name := u.Name
		if name == "root" {
			name = "containers"
		}
		content, err := os.ReadFile("/etc/subuid")
		if err != nil {
			Skip("cannot read /etc/subuid")
		}
		if !strings.Contains(string(content), name) {
			Skip("cannot find mappings for the current user")
		}

		initialUsernsConfig, err := os.ReadFile("/proc/self/uid_map")
		Expect(err).ToNot(HaveOccurred())
		if isRootless() {
			unshare := podmanTest.Podman([]string{"unshare", "cat", "/proc/self/uid_map"})
			unshare.WaitWithDefaultTimeout()
			Expect(unshare).Should(Exit(0))
			initialUsernsConfig = unshare.Out.Contents()
		}

		pod := getPod()
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		usernsInCtr := podmanTest.Podman([]string{"exec", getCtrNameInPod(pod), "cat", "/proc/self/uid_map"})
		usernsInCtr.WaitWithDefaultTimeout()
		Expect(usernsInCtr).Should(Exit(0))
		// the conversion to string is needed for better error messages
		Expect(string(usernsInCtr.Out.Contents())).To(Equal(string(initialUsernsConfig)))

		// PodmanNoCache is a workaround for https://github.com/containers/storage/issues/1232
		kube = podmanTest.PodmanNoCache([]string{"play", "kube", "--replace", "--userns=auto", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		usernsInCtr = podmanTest.Podman([]string{"exec", getCtrNameInPod(pod), "cat", "/proc/self/uid_map"})
		usernsInCtr.WaitWithDefaultTimeout()
		Expect(usernsInCtr).Should(Exit(0))
		Expect(string(usernsInCtr.Out.Contents())).To(Not(Equal(string(initialUsernsConfig))))

		// Now try with hostUsers in the pod spec
		for _, hostUsers := range []bool{true, false} {
			pod = getPod(withHostUsers(hostUsers))
			err = generateKubeYaml("pod", pod, kubeYaml)
			Expect(err).ToNot(HaveOccurred())

			kube = podmanTest.PodmanNoCache([]string{"play", "kube", "--replace", kubeYaml})
			kube.WaitWithDefaultTimeout()
			Expect(kube).Should(Exit(0))

			usernsInCtr = podmanTest.Podman([]string{"exec", getCtrNameInPod(pod), "cat", "/proc/self/uid_map"})
			usernsInCtr.WaitWithDefaultTimeout()
			Expect(usernsInCtr).Should(Exit(0))
			if hostUsers {
				Expect(string(usernsInCtr.Out.Contents())).To(Equal(string(initialUsernsConfig)))
			} else {
				Expect(string(usernsInCtr.Out.Contents())).To(Not(Equal(string(initialUsernsConfig))))
			}
		}
	})

	// Check the block devices are exposed inside container
	It("podman play kube expose block device inside container", func() {
		SkipIfRootless("It needs root access to create devices")

		// randomize the folder name to avoid error when running tests with multiple nodes
		uuid, err := uuid.NewUUID()
		Expect(err).ToNot(HaveOccurred())
		devFolder := fmt.Sprintf("/dev/foodev%x", uuid[:6])
		Expect(os.MkdirAll(devFolder, os.ModePerm)).To(Succeed())
		defer os.RemoveAll(devFolder)

		devicePath := fmt.Sprintf("%s/blockdevice", devFolder)
		mknod := SystemExec("mknod", []string{devicePath, "b", "7", "0"})
		mknod.WaitWithDefaultTimeout()
		Expect(mknod).Should(Exit(0))

		blockVolume := getHostPathVolume("BlockDevice", devicePath)

		pod := getPod(withVolume(blockVolume), withCtr(getCtr(withImage(REGISTRY_IMAGE), withCmd(nil), withArg(nil), withVolumeMount(devicePath, "", false))))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		// Container should be in running state
		inspect := podmanTest.Podman([]string{"inspect", "--format", "{{.State.Status}}", "testPod-" + defaultCtrName})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(ContainSubstring("running"))

		// Container should have a block device /dev/loop1
		inspect = podmanTest.Podman([]string{"inspect", "--format", "{{.HostConfig.Devices}}", "testPod-" + defaultCtrName})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(ContainSubstring(devicePath))
	})

	// Check the char devices are exposed inside container
	It("podman play kube expose character device inside container", func() {
		SkipIfRootless("It needs root access to create devices")

		// randomize the folder name to avoid error when running tests with multiple nodes
		uuid, err := uuid.NewUUID()
		Expect(err).ToNot(HaveOccurred())
		devFolder := fmt.Sprintf("/dev/foodev%x", uuid[:6])
		Expect(os.MkdirAll(devFolder, os.ModePerm)).To(Succeed())
		defer os.RemoveAll(devFolder)

		devicePath := fmt.Sprintf("%s/chardevice", devFolder)
		mknod := SystemExec("mknod", []string{devicePath, "c", "3", "1"})
		mknod.WaitWithDefaultTimeout()
		Expect(mknod).Should(Exit(0))

		charVolume := getHostPathVolume("CharDevice", devicePath)

		pod := getPod(withVolume(charVolume), withCtr(getCtr(withImage(REGISTRY_IMAGE), withCmd(nil), withArg(nil), withVolumeMount(devicePath, "", false))))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		// Container should be in running state
		inspect := podmanTest.Podman([]string{"inspect", "--format", "{{.State.Status}}", "testPod-" + defaultCtrName})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(ContainSubstring("running"))

		// Container should have a block device /dev/loop1
		inspect = podmanTest.Podman([]string{"inspect", "--format", "{{.HostConfig.Devices}}", "testPod-" + defaultCtrName})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(ContainSubstring(devicePath))
	})

	It("podman play kube reports error when the device does not exists", func() {
		SkipIfRootless("It needs root access to create devices")

		devicePath := "/dev/foodevdir/baddevice"

		blockVolume := getHostPathVolume("BlockDevice", devicePath)

		pod := getPod(withVolume(blockVolume), withCtr(getCtr(withImage(REGISTRY_IMAGE), withCmd(nil), withArg(nil), withVolumeMount(devicePath, "", false))))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(125))
	})

	It("podman play kube reports error when we try to expose char device as block device", func() {
		SkipIfRootless("It needs root access to create devices")

		// randomize the folder name to avoid error when running tests with multiple nodes
		uuid, err := uuid.NewUUID()
		Expect(err).ToNot(HaveOccurred())
		devFolder := fmt.Sprintf("/dev/foodev%x", uuid[:6])
		Expect(os.MkdirAll(devFolder, os.ModePerm)).To(Succeed())
		defer os.RemoveAll(devFolder)

		devicePath := fmt.Sprintf("%s/chardevice", devFolder)
		mknod := SystemExec("mknod", []string{devicePath, "c", "3", "1"})
		mknod.WaitWithDefaultTimeout()
		Expect(mknod).Should(Exit(0))

		charVolume := getHostPathVolume("BlockDevice", devicePath)

		pod := getPod(withVolume(charVolume), withCtr(getCtr(withImage(REGISTRY_IMAGE), withCmd(nil), withArg(nil), withVolumeMount(devicePath, "", false))))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(125))
	})

	It("podman play kube reports error when we try to expose block device as char device", func() {
		SkipIfRootless("It needs root access to create devices")

		// randomize the folder name to avoid error when running tests with multiple nodes
		uuid, err := uuid.NewUUID()
		Expect(err).ToNot(HaveOccurred())
		devFolder := fmt.Sprintf("/dev/foodev%x", uuid[:6])
		Expect(os.MkdirAll(devFolder, os.ModePerm)).To(Succeed())

		devicePath := fmt.Sprintf("%s/blockdevice", devFolder)
		mknod := SystemExec("mknod", []string{devicePath, "b", "7", "0"})
		mknod.WaitWithDefaultTimeout()
		Expect(mknod).Should(Exit(0))

		charVolume := getHostPathVolume("CharDevice", devicePath)

		pod := getPod(withVolume(charVolume), withCtr(getCtr(withImage(REGISTRY_IMAGE), withCmd(nil), withArg(nil), withVolumeMount(devicePath, "", false))))
		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(125))
	})

	It("podman play kube secret as volume support - simple", func() {
		createAndTestSecret(podmanTest, secretYaml, "newsecret", kubeYaml)
		testPodWithSecret(podmanTest, secretPodYaml, kubeYaml, true, true)
		deleteAndTestSecret(podmanTest, "newsecret")
	})

	It("podman play kube secret as volume support - multiple volumes", func() {
		yamls := []string{secretYaml, secretPodYaml}
		err = generateMultiDocKubeYaml(yamls, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		// do not remove newsecret to test that we auto remove on collision

		yamls = []string{secretYaml, complexSecretYaml}
		err = generateMultiDocKubeYaml(yamls, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube = podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		err = writeYaml(secretPodYamlTwo, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube = podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		exec := podmanTest.Podman([]string{"exec", "-it", "mypod2-myctr", "cat", "/etc/foo/username"})
		exec.WaitWithDefaultTimeout()
		Expect(exec).Should(Exit(0))
		username, _ := base64.StdEncoding.DecodeString("dXNlcg==")
		Expect(exec.OutputToString()).Should(ContainSubstring(string(username)))

		exec = podmanTest.Podman([]string{"exec", "-it", "mypod2-myctr", "cat", "/etc/bar/username"})
		exec.WaitWithDefaultTimeout()
		Expect(exec).Should(Exit(0))
		username, _ = base64.StdEncoding.DecodeString("Y2RvZXJu")
		Expect(exec.OutputToString()).Should(ContainSubstring(string(username)))

		exec = podmanTest.Podman([]string{"exec", "-it", "mypod2-myctr", "cat", "/etc/baz/plain_note"})
		exec.WaitWithDefaultTimeout()
		Expect(exec).Should(Exit(0))
		Expect(exec.OutputToString()).Should(ContainSubstring("This is a test"))

	})

	It("podman play kube secret as volume support - optional field", func() {
		createAndTestSecret(podmanTest, secretYaml, "newsecret", kubeYaml)

		testPodWithSecret(podmanTest, optionalExistingSecretPodYaml, kubeYaml, true, true)
		testPodWithSecret(podmanTest, optionalNonExistingSecretPodYaml, kubeYaml, true, false)
		testPodWithSecret(podmanTest, noOptionalExistingSecretPodYaml, kubeYaml, true, true)
		testPodWithSecret(podmanTest, noOptionalNonExistingSecretPodYaml, kubeYaml, false, false)

		deleteAndTestSecret(podmanTest, "newsecret")
	})

	It("podman play kube with disabled cgroup", func() {
		conffile := filepath.Join(podmanTest.TempDir, "container.conf")
		// Disabled ipcns and cgroupfs in the config file
		// Since shmsize (Inherit from infra container) cannot be set if ipcns is "host", we should remove the default value.
		// Also, cgroupfs config should be loaded into SpecGenerator when playing kube.
		err := os.WriteFile(conffile, []byte(`
[containers]
ipcns="host"
cgroups="disabled"`), 0644)
		Expect(err).ToNot(HaveOccurred())
		defer os.Unsetenv("CONTAINERS_CONF")
		os.Setenv("CONTAINERS_CONF", conffile)
		err = writeYaml(simplePodYaml, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))
	})

	It("podman kube --quiet with error", func() {
		SkipIfNotRootless("We need to create an error trying to bind to port 80")
		yaml := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: frontend
spec:
  replicas: 2
  template:
    metadata:
      labels:
        app: guestbook
        tier: frontend
    spec:
      containers:
      - name: php-redis
        image: quay.io/libpod/alpine_nginx:latest
        ports:
        - containerPort: 1234
          hostPort: 80
`

		err = writeYaml(yaml, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"kube", "play", "--quiet", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).To(ExitWithError())
		// The ugly format-error exited once in Podman. The test makes
		// sure it's not coming back.
		Expect(kube.ErrorToString()).To(Not(ContainSubstring("Error: %!s(<nil>)")))
	})

	It("podman kube play invalid yaml should clean up pod that was created before failure", func() {
		podTemplate := `---
apiVersion: v1
kind: Pod
metadata:
	creationTimestamp: "2022-08-02T04:05:53Z"
	labels:
	app: vol-test-3-pod
	name: vol-test-3
spec:
	containers:
	- command:
	- sleep
	- "1000"
	image: non-existing-image
	name: vol-test-3
`

		// the image is incorrect so the kube play will fail, but it will clean up the pod that was created for it before the failure happened
		kube := podmanTest.Podman([]string{"kube", "play", podTemplate})
		kube.WaitWithDefaultTimeout()
		Expect(kube).To(ExitWithError())

		ps := podmanTest.Podman([]string{"pod", "ps", "-q"})
		ps.WaitWithDefaultTimeout()
		Expect(ps).Should(Exit(0))
		Expect(ps.OutputToStringArray()).To(HaveLen(0))
	})

	It("podman play kube with named volume subpaths", func() {
		SkipIfRemote("volume export does not exist on remote")
		volumeCreate := podmanTest.Podman([]string{"volume", "create", "testvol1"})
		volumeCreate.WaitWithDefaultTimeout()
		Expect(volumeCreate).Should(Exit(0))

		session := podmanTest.Podman([]string{"run", "--volume", "testvol1:/data", ALPINE, "sh", "-c", "mkdir -p /data/testing/onlythis && touch /data/testing/onlythis/123.txt && echo hi >> /data/testing/onlythis/123.txt"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		tar := filepath.Join(podmanTest.TempDir, "out.tar")
		session = podmanTest.Podman([]string{"volume", "export", "--output", tar, "testvol1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		volumeCreate = podmanTest.Podman([]string{"volume", "create", "testvol"})
		volumeCreate.WaitWithDefaultTimeout()
		Expect(volumeCreate).Should(Exit(0))

		volumeImp := podmanTest.Podman([]string{"volume", "import", "testvol", filepath.Join(podmanTest.TempDir, "out.tar")})
		volumeImp.WaitWithDefaultTimeout()
		Expect(volumeImp).Should(Exit(0))

		err = writeYaml(subpathTestNamedVolume, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		playKube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		playKube.WaitWithDefaultTimeout()
		Expect(playKube).Should(Exit(0))

		exec := podmanTest.Podman([]string{"exec", "-it", "testpod-testctr", "cat", "/var/123.txt"})
		exec.WaitWithDefaultTimeout()
		Expect(exec).Should(Exit(0))
		Expect(exec.OutputToString()).Should(Equal("hi"))
	})

	It("podman play kube with hostPath subpaths", func() {
		if !Containerized() {
			Skip("something is wrong with file permissions in CI or in the yaml creation. cannot ls or cat the fs unless in a container")
		}

		hostPathLocation := podmanTest.TempDir
		Expect(os.MkdirAll(filepath.Join(hostPathLocation, "testing", "onlythis"), 0755)).To(Succeed())
		file, err := os.Create(filepath.Join(hostPathLocation, "testing", "onlythis", "123.txt"))
		Expect(err).ToNot(HaveOccurred())

		_, err = file.Write([]byte("hi"))
		Expect(err).ToNot(HaveOccurred())

		err = file.Close()
		Expect(err).ToNot(HaveOccurred())

		pod := getPod(withPodName("testpod"), withCtr(getCtr(withImage(ALPINE), withName("testctr"), withCmd([]string{"top"}), withVolumeMount("/var", "testing/onlythis", false))), withVolume(getHostPathVolume("DirectoryOrCreate", hostPathLocation)))

		err = generateKubeYaml("pod", pod, kubeYaml)
		Expect(err).To(Not(HaveOccurred()))
		playKube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		playKube.WaitWithDefaultTimeout()
		Expect(playKube).Should(Exit(0))
		exec := podmanTest.Podman([]string{"exec", "-it", "testpod-testctr", "ls", "/var"})
		exec.WaitWithDefaultTimeout()
		Expect(exec).Should(Exit(0))
		Expect(exec.OutputToString()).Should(ContainSubstring("123.txt"))
	})

	It("podman play kube with configMap subpaths", func() {
		volumeName := "cmVol"
		cm := getConfigMap(withConfigMapName(volumeName), withConfigMapData("FOO", "foobar"))
		cmYaml, err := getKubeYaml("configmap", cm)
		Expect(err).ToNot(HaveOccurred())
		volumeContents := []map[string]string{{
			"key":  "FOO",
			"path": "BAR",
		}}

		ctr := getCtr(withPullPolicy("always"), withName("testctr"), withCmd([]string{"top"}), withVolumeMount("/etc/BAR", "BAR", false), withImage(ALPINE))
		pod := getPod(withPodName("testpod"), withVolume(getConfigMapVolume(volumeName, volumeContents, false)), withCtr(ctr))

		podYaml, err := getKubeYaml("pod", pod)
		Expect(err).ToNot(HaveOccurred())

		yamls := []string{cmYaml, podYaml}
		err = generateMultiDocKubeYaml(yamls, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		out, _ := os.ReadFile(kubeYaml)
		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0), string(out))

		exec := podmanTest.Podman([]string{"exec", "-it", "testpod-testctr", "ls", "/etc/"})
		exec.WaitWithDefaultTimeout()
		Expect(exec).Should(Exit(0))
		Expect(exec.OutputToString()).ShouldNot(HaveLen(3))
		Expect(exec.OutputToString()).Should(ContainSubstring("BAR"))
		// we want to check that we can mount a subpath but not replace the entire dir
	})

	It("podman play kube without Ports - curl should fail", func() {
		err := writeYaml(publishPortsPodWithoutPorts, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		curlTest := podmanTest.Podman([]string{"run", "--network", "host", NGINX_IMAGE, "curl", "-s", "localhost:19000"})
		curlTest.WaitWithDefaultTimeout()
		Expect(curlTest).Should(Exit(7))
	})

	It("podman play kube without Ports, publish in command line - curl should succeed", func() {
		err := writeYaml(publishPortsPodWithoutPorts, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", "--publish", "19002:80", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		testHTTPServer("19002", false, "podman rulez")
	})

	It("podman play kube with privileged container ports - should fail", func() {
		SkipIfNotRootless("rootlessport can expose privileged port 80, no point in checking for failure")
		err := writeYaml(publishPortsPodWithContainerPort, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(125))
		// The error message is printed only on local call
		if !IsRemote() {
			Expect(kube.OutputToString()).Should(ContainSubstring("rootlessport cannot expose privileged port 80"))
		}
	})

	It("podman play kube with privileged containers ports and publish in command line - curl should succeed", func() {
		err := writeYaml(publishPortsPodWithContainerPort, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", "--publish", "19003:80", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		testHTTPServer("19003", false, "podman rulez")
	})

	It("podman play kube with Host Ports - curl should succeed", func() {
		err := writeYaml(publishPortsPodWithContainerHostPort, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", "--publish", "19004:80", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		testHTTPServer("19004", false, "podman rulez")
	})

	It("podman play kube with Host Ports and publish in command line - curl should succeed only on overriding port", func() {
		err := writeYaml(publishPortsPodWithContainerHostPort, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", "--publish", "19005:80", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		testHTTPServer("19001", true, "connection refused")
		testHTTPServer("19005", false, "podman rulez")
	})

	It("podman play kube multiple publish ports", func() {
		err := writeYaml(publishPortsPodWithoutPorts, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", "--publish", "19006:80", "--publish", "19007:80", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		testHTTPServer("19006", false, "podman rulez")
		testHTTPServer("19007", false, "podman rulez")
	})

	It("podman play kube override with tcp should keep udp from YAML file", func() {
		err := writeYaml(publishPortsEchoWithHostPortUDP, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", "--publish", "19010:19008/tcp", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		verifyPodPorts(podmanTest, "network-echo", "19008/tcp:[{ 19010}]", "19008/udp:[{ 19009}]")
	})

	It("podman play kube override with udp should keep tcp from YAML file", func() {
		err := writeYaml(publishPortsEchoWithHostPortTCP, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", "--publish", "19012:19008/udp", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		verifyPodPorts(podmanTest, "network-echo", "19008/tcp:[{ 19011}]", "19008/udp:[{ 19012}]")
	})

	It("podman play kube with replicas limits the count to 1 and emits a warning", func() {
		deployment := getDeployment(withReplicas(10))
		err := generateKubeYaml("deployment", deployment, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		// warnings are only propagated to local clients
		if !IsRemote() {
			Expect(kube.ErrorToString()).Should(ContainSubstring("Limiting replica count to 1, more than one replica is not supported by Podman"))
		}

		Expect(strings.Count(kube.OutputToString(), "Pod:")).To(Equal(1))
		Expect(strings.Count(kube.OutputToString(), "Container:")).To(Equal(1))
	})

	It("podman play kube test with hostPID", func() {
		err := writeYaml(podWithHostPIDDefined, kubeYaml)
		Expect(err).ToNot(HaveOccurred())

		kube := podmanTest.Podman([]string{"play", "kube", kubeYaml})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(Exit(0))

		logs := podmanTest.Podman([]string{"pod", "logs", "-c", "test-hostpid-alpine", "test-hostpid"})
		logs.WaitWithDefaultTimeout()
		Expect(logs).Should(Exit(0))
		Expect(logs.OutputToString()).To(Not(Equal("1")), "PID should never be 1 because of host pidns")

		inspect := podmanTest.Podman([]string{"inspect", "test-hostpid-alpine", "--format", "{{ .HostConfig.PidMode }}"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(Equal("host"))
	})

})
