module github.com/containers/buildah

go 1.13

require (
	github.com/containerd/containerd v1.5.5
	github.com/containernetworking/cni v0.8.1
	github.com/containers/common v0.44.0
	github.com/containers/image/v5 v5.16.0
	github.com/containers/ocicrypt v1.1.2
	github.com/containers/storage v1.36.0
	github.com/docker/distribution v2.7.1+incompatible
	github.com/docker/go-units v0.4.0
	github.com/docker/libnetwork v0.8.0-dev.2.0.20190625141545-5a177b73e316
	github.com/fsouza/go-dockerclient v1.7.4
	github.com/ghodss/yaml v1.0.0
	github.com/hashicorp/go-multierror v1.1.1
	github.com/ishidawataru/sctp v0.0.0-20210226210310-f2269e66cdee // indirect
	github.com/konsorten/go-windows-terminal-sequences v1.0.3 // indirect
	github.com/mattn/go-shellwords v1.0.12
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.16.0
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.0.2-0.20210819154149-5ad6f50d6283
	github.com/opencontainers/runc v1.0.2
	github.com/opencontainers/runtime-spec v1.0.3-0.20210326190908-1c3f411f0417
	github.com/opencontainers/runtime-tools v0.9.0
	github.com/opencontainers/selinux v1.8.5
	github.com/openshift/imagebuilder v1.2.2-0.20210415181909-87f3e48c2656
	github.com/pkg/errors v0.9.1
	github.com/seccomp/libseccomp-golang v0.9.2-0.20200616122406-847368b35ebf
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/cobra v1.2.1
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.0
	github.com/syndtr/gocapability v0.0.0-20200815063812-42c35b437635
	go.etcd.io/bbolt v1.3.6
	golang.org/x/crypto v0.0.0-20210711020723-a769d52b0f97
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	golang.org/x/sys v0.0.0-20210820121016-41cdb8703e55
	golang.org/x/term v0.0.0-20210615171337-6886f2dfbf5b
	k8s.io/klog v1.0.0 // indirect
)

replace github.com/sirupsen/logrus => github.com/sirupsen/logrus v1.4.2
