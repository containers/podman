module github.com/containers/buildah

go 1.12

require (
	github.com/containernetworking/cni v0.7.2-0.20190904153231-83439463f784
	github.com/containers/common v0.11.2
	github.com/containers/image/v5 v5.4.4
	github.com/containers/ocicrypt v1.0.2
	github.com/containers/storage v1.19.2
	github.com/cyphar/filepath-securejoin v0.2.2
	github.com/docker/distribution v2.7.1+incompatible
	github.com/docker/go-units v0.4.0
	github.com/docker/libnetwork v0.8.0-dev.2.0.20190625141545-5a177b73e316
	github.com/etcd-io/bbolt v1.3.3
	github.com/fsouza/go-dockerclient v1.6.5
	github.com/ghodss/yaml v1.0.0
	github.com/hashicorp/go-multierror v1.0.0
	github.com/ishidawataru/sctp v0.0.0-20191218070446-00ab2ac2db07 // indirect
	github.com/mattn/go-shellwords v1.0.10
	github.com/onsi/ginkgo v1.12.1
	github.com/onsi/gomega v1.10.0
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.0.2-0.20190823105129-775207bd45b6
	github.com/opencontainers/runc v1.0.0-rc9
	github.com/opencontainers/runtime-spec v1.0.3-0.20200520003142-237cc4f519e2
	github.com/opencontainers/runtime-tools v0.9.0
	github.com/opencontainers/selinux v1.5.1
	github.com/openshift/imagebuilder v1.1.4
	github.com/pkg/errors v0.9.1
	github.com/seccomp/containers-golang v0.4.1
	github.com/seccomp/libseccomp-golang v0.9.1
	github.com/sirupsen/logrus v1.6.0
	github.com/spf13/cobra v0.0.7
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.5.1
	github.com/syndtr/gocapability v0.0.0-20180916011248-d98352740cb2
	github.com/vishvananda/netlink v1.1.0 // indirect
	golang.org/x/crypto v0.0.0-20200423211502-4bdfaf469ed5
	golang.org/x/sys v0.0.0-20200420163511-1957bb5e6d1f
)

replace github.com/sirupsen/logrus => github.com/sirupsen/logrus v1.4.2
