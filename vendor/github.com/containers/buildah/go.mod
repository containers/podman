module github.com/containers/buildah

go 1.16

require (
	github.com/containerd/containerd v1.6.6
	github.com/containernetworking/cni v1.1.2
	github.com/containers/common v0.49.1
	github.com/containers/image/v5 v5.22.0
	github.com/containers/ocicrypt v1.1.5
	github.com/containers/storage v1.42.0
	github.com/docker/distribution v2.8.1+incompatible
	github.com/docker/docker v20.10.17+incompatible
	github.com/docker/go-units v0.4.0
	github.com/docker/libnetwork v0.8.0-dev.2.0.20190625141545-5a177b73e316
	github.com/fsouza/go-dockerclient v1.7.11
	github.com/ghodss/yaml v1.0.0
	github.com/hashicorp/go-multierror v1.1.1
	github.com/ishidawataru/sctp v0.0.0-20210226210310-f2269e66cdee // indirect
	github.com/konsorten/go-windows-terminal-sequences v1.0.3 // indirect
	github.com/mattn/go-shellwords v1.0.12
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.20.0
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.0.3-0.20220114050600-8b9d41f48198
	github.com/opencontainers/runc v1.1.3
	github.com/opencontainers/runtime-spec v1.0.3-0.20210326190908-1c3f411f0417
	github.com/opencontainers/runtime-tools v0.9.1-0.20220714195903-17b3287fafb7
	github.com/opencontainers/selinux v1.10.1
	github.com/openshift/imagebuilder v1.2.4-0.20220502172744-009dbc6cb805
	github.com/pkg/errors v0.9.1
	github.com/seccomp/libseccomp-golang v0.10.0
	github.com/sirupsen/logrus v1.9.0
	github.com/spf13/cobra v1.5.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.8.0
	github.com/syndtr/gocapability v0.0.0-20200815063812-42c35b437635
	go.etcd.io/bbolt v1.3.6
	golang.org/x/crypto v0.0.0-20220411220226-7b82a4e95df4
	golang.org/x/sync v0.0.0-20220601150217-0de741cfad7f
	golang.org/x/sys v0.0.0-20220715151400-c0bba94af5f8
	golang.org/x/term v0.0.0-20210927222741-03fcf44c2211
)

replace github.com/sirupsen/logrus => github.com/sirupsen/logrus v1.4.2

replace github.com/opencontainers/image-spec => github.com/opencontainers/image-spec v1.0.2-0.20211123152302-43a7dee1ec31
