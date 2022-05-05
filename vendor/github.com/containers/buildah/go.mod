module github.com/containers/buildah

go 1.16

require (
	github.com/containerd/containerd v1.6.4
	github.com/containernetworking/cni v1.1.0
	github.com/containers/common v0.48.0
	github.com/containers/image/v5 v5.21.1
	github.com/containers/ocicrypt v1.1.4-0.20220428134531-566b808bdf6f
	github.com/containers/storage v1.40.2
	github.com/docker/distribution v2.8.1+incompatible
	github.com/docker/docker v20.10.14+incompatible
	github.com/docker/go-units v0.4.0
	github.com/docker/libnetwork v0.8.0-dev.2.0.20190625141545-5a177b73e316
	github.com/fsouza/go-dockerclient v1.7.11
	github.com/ghodss/yaml v1.0.0
	github.com/hashicorp/go-multierror v1.1.1
	github.com/ishidawataru/sctp v0.0.0-20210226210310-f2269e66cdee // indirect
	github.com/konsorten/go-windows-terminal-sequences v1.0.3 // indirect
	github.com/mattn/go-shellwords v1.0.12
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.19.0
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.0.3-0.20211202193544-a5463b7f9c84
	github.com/opencontainers/runc v1.1.1
	github.com/opencontainers/runtime-spec v1.0.3-0.20210326190908-1c3f411f0417
	github.com/opencontainers/runtime-tools v0.9.0
	github.com/opencontainers/selinux v1.10.1
	github.com/openshift/imagebuilder v1.2.4-0.20220502172744-009dbc6cb805
	github.com/pkg/errors v0.9.1
	github.com/seccomp/libseccomp-golang v0.9.2-0.20210429002308-3879420cc921
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/cobra v1.4.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.1
	github.com/syndtr/gocapability v0.0.0-20200815063812-42c35b437635
	go.etcd.io/bbolt v1.3.6
	golang.org/x/crypto v0.0.0-20220411220226-7b82a4e95df4
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	golang.org/x/sys v0.0.0-20220422013727-9388b58f7150
	golang.org/x/term v0.0.0-20210927222741-03fcf44c2211
)

replace github.com/sirupsen/logrus => github.com/sirupsen/logrus v1.4.2

replace github.com/opencontainers/image-spec => github.com/opencontainers/image-spec v1.0.2-0.20211123152302-43a7dee1ec31
