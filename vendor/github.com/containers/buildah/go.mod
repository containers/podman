module github.com/containers/buildah

go 1.12

require (
	github.com/containernetworking/cni v0.7.2-0.20190904153231-83439463f784
	github.com/containers/common v0.8.4
	github.com/containers/image/v5 v5.4.3
	github.com/containers/storage v1.18.2
	github.com/cyphar/filepath-securejoin v0.2.2
	github.com/docker/distribution v2.7.1+incompatible
	github.com/docker/go-units v0.4.0
	github.com/docker/libnetwork v0.8.0-dev.2.0.20190625141545-5a177b73e316
	github.com/etcd-io/bbolt v1.3.3
	github.com/fsouza/go-dockerclient v1.6.3
	github.com/ghodss/yaml v1.0.0
	github.com/hashicorp/go-multierror v1.0.0
	github.com/ishidawataru/sctp v0.0.0-20191218070446-00ab2ac2db07 // indirect
	github.com/mattn/go-shellwords v1.0.10
	github.com/onsi/ginkgo v1.12.0
	github.com/onsi/gomega v1.9.0
	github.com/opencontainers/go-digest v1.0.0-rc1
	github.com/opencontainers/image-spec v1.0.2-0.20190823105129-775207bd45b6
	github.com/opencontainers/runc v1.0.0-rc9
	github.com/opencontainers/runtime-spec v0.1.2-0.20190618234442-a950415649c7
	github.com/opencontainers/runtime-tools v0.9.0
	github.com/opencontainers/selinux v1.5.1
	github.com/openshift/api v0.0.0-20200106203948-7ab22a2c8316
	github.com/openshift/imagebuilder v1.1.4
	github.com/pkg/errors v0.9.1
	github.com/seccomp/containers-golang v0.0.0-20190312124753-8ca8945ccf5f
	github.com/seccomp/libseccomp-golang v0.9.1
	github.com/sirupsen/logrus v1.5.0
	github.com/spf13/cobra v0.0.7
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.5.1
	github.com/syndtr/gocapability v0.0.0-20180916011248-d98352740cb2
	github.com/vishvananda/netlink v1.1.0 // indirect
	golang.org/x/crypto v0.0.0-20200323165209-0ec3e9974c59
	golang.org/x/sys v0.0.0-20200327173247-9dae0f8f5775
)

replace github.com/sirupsen/logrus => github.com/sirupsen/logrus v1.4.2
