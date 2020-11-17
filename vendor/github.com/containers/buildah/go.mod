module github.com/containers/buildah

go 1.12

require (
	github.com/containerd/containerd v1.4.1 // indirect
	github.com/containernetworking/cni v0.7.2-0.20190904153231-83439463f784
	github.com/containers/common v0.26.3
	github.com/containers/image/v5 v5.8.0
	github.com/containers/ocicrypt v1.0.3
	github.com/containers/storage v1.24.0
	github.com/docker/distribution v2.7.1+incompatible
	github.com/docker/docker v17.12.0-ce-rc1.0.20201020191947-73dc6a680cdd+incompatible // indirect
	github.com/docker/go-units v0.4.0
	github.com/docker/libnetwork v0.8.0-dev.2.0.20190625141545-5a177b73e316
	github.com/fsouza/go-dockerclient v1.6.6
	github.com/ghodss/yaml v1.0.0
	github.com/hashicorp/go-multierror v1.1.0
	github.com/ishidawataru/sctp v0.0.0-20191218070446-00ab2ac2db07 // indirect
	github.com/konsorten/go-windows-terminal-sequences v1.0.2 // indirect
	github.com/mattn/go-shellwords v1.0.10
	github.com/moby/sys/mount v0.1.1 // indirect
	github.com/moby/term v0.0.0-20200915141129-7f0af18e79f2 // indirect
	github.com/onsi/ginkgo v1.14.2
	github.com/onsi/gomega v1.10.3
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.0.2-0.20190823105129-775207bd45b6
	github.com/opencontainers/runc v1.0.0-rc91
	github.com/opencontainers/runtime-spec v1.0.3-0.20200710190001-3e4195d92445
	github.com/opencontainers/runtime-tools v0.9.0
	github.com/opencontainers/selinux v1.6.0
	github.com/openshift/imagebuilder v1.1.8
	github.com/pkg/errors v0.9.1
	github.com/seccomp/libseccomp-golang v0.9.2-0.20200616122406-847368b35ebf
	github.com/sirupsen/logrus v1.7.0
	github.com/spf13/cobra v1.1.1
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.6.1
	github.com/syndtr/gocapability v0.0.0-20180916011248-d98352740cb2
	go.etcd.io/bbolt v1.3.5
	golang.org/x/crypto v0.0.0-20200622213623-75b288015ac9
	golang.org/x/sync v0.0.0-20200317015054-43a5402ce75a
	golang.org/x/sys v0.0.0-20201018230417-eeed37f84f13
	gotest.tools/v3 v3.0.3 // indirect
	k8s.io/klog v1.0.0 // indirect
)

replace github.com/sirupsen/logrus => github.com/sirupsen/logrus v1.4.2
