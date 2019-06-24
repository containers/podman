module github.com/containers/libpod

go 1.12

require (
	github.com/BurntSushi/toml v0.3.1
	github.com/DataDog/zstd v1.4.0 // indirect
	github.com/Microsoft/hcsshim v0.8.3 // indirect
	github.com/Nvveen/Gotty v0.0.0-20120604004816-cd527374f1e5 // indirect
	github.com/VividCortex/ewma v1.1.1 // indirect
	github.com/blang/semver v3.5.0+incompatible // indirect
	github.com/buger/goterm v0.0.0-20181115115552-c206103e1f37
	github.com/checkpoint-restore/go-criu v0.0.0-20181120144056-17b0214f6c48
	github.com/containerd/cgroups v0.0.0-20190328223300-4994991857f9
	github.com/containerd/continuity v0.0.0-20190426062206-aaeac12a7ffc // indirect
	github.com/containernetworking/cni v0.7.0-rc2
	github.com/containernetworking/plugins v0.7.4
	github.com/containers/buildah v1.9.0
	github.com/containers/image v2.0.0+incompatible
	github.com/containers/psgo v1.3.0
	github.com/containers/storage v1.12.11
	github.com/coreos/go-iptables v0.4.0
	github.com/coreos/go-systemd v0.0.0-20180511133405-39ca1b05acc7
	github.com/coreos/pkg v0.0.0-20180108230652-97fdf19511ea // indirect
	github.com/cri-o/ocicni v0.0.0-20190328132530-0c180f981b27
	github.com/cyphar/filepath-securejoin v0.2.1
	github.com/davecgh/go-spew v1.1.1
	github.com/docker/distribution v0.0.0-20170817175659-5f6282db7d65
	github.com/docker/docker v0.7.3-0.20190309235953-33c3200e0d16
	github.com/docker/docker-credential-helpers v0.6.1
	github.com/docker/go-connections v0.4.0
	github.com/docker/go-units v0.4.0
	github.com/docker/libnetwork v0.8.0-dev.2.0.20180608203834-19279f049241 // indirect
	github.com/docker/libtrust v0.0.0-20160708172513-aabc10ec26b7 // indirect
	github.com/docker/spdystream v0.0.0-20181023171402-6480d4af844c // indirect
	github.com/etcd-io/bbolt v1.3.2
	github.com/fatih/camelcase v1.0.0
	github.com/fsnotify/fsnotify v1.4.7
	github.com/fsouza/go-dockerclient v1.4.1 // indirect
	github.com/ghodss/yaml v1.0.0
	github.com/godbus/dbus v4.1.0+incompatible
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b // indirect
	github.com/golang/protobuf v1.3.1 // indirect
	github.com/google/gofuzz v0.0.0-20170612174753-24818f796faf // indirect
	github.com/google/shlex v0.0.0-20181106134648-c34317bd91bf
	github.com/gorilla/context v1.1.1 // indirect
	github.com/hashicorp/go-multierror v1.0.0
	github.com/hpcloud/tail v1.0.0
	github.com/imdario/mergo v0.3.6 // indirect
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/ishidawataru/sctp v0.0.0-20180213033435-07191f837fed // indirect
	github.com/json-iterator/go v1.1.5
	github.com/kisielk/errcheck v1.2.0 // indirect
	github.com/klauspost/compress v1.4.1 // indirect
	github.com/klauspost/cpuid v1.2.0 // indirect
	github.com/klauspost/pgzip v1.2.1 // indirect
	github.com/konsorten/go-windows-terminal-sequences v1.0.2 // indirect
	github.com/mattn/go-isatty v0.0.4 // indirect
	github.com/mattn/go-shellwords v1.0.5 // indirect
	github.com/mistifyio/go-zfs v2.1.1+incompatible // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.1 // indirect
	github.com/mrunalp/fileutils v0.0.0-20171103030105-7d4729fb3618
	github.com/mtrmac/gpgme v0.0.0-20170102180018-b2432428689c // indirect
	github.com/onsi/ginkgo v1.8.0
	github.com/onsi/gomega v1.4.3
	github.com/opencontainers/go-digest v1.0.0-rc1
	github.com/opencontainers/image-spec v1.0.1
	github.com/opencontainers/runc v1.0.0-rc6
	github.com/opencontainers/runtime-spec v0.0.0-20181111125026-1722abf79c2f
	github.com/opencontainers/runtime-tools v0.8.0
	github.com/opencontainers/selinux v0.0.0-20190118194635-b707dfcb00a1
	github.com/openshift/imagebuilder v1.1.0 // indirect
	github.com/opentracing/opentracing-go v0.0.0-20190218023034-25a84ff92183
	github.com/ostreedev/ostree-go v0.0.0-20181213164143-d0388bd827cf // indirect
	github.com/pkg/errors v0.8.1
	github.com/pkg/profile v1.2.1
	github.com/pmezard/go-difflib v1.0.0
	github.com/pquerna/ffjson v0.0.0-20181028064349-e517b90714f7 // indirect
	github.com/seccomp/containers-golang v0.0.0-20180629143253-cdfdaa7543f4 // indirect
	github.com/seccomp/libseccomp-golang v0.9.0 // indirect
	github.com/sirupsen/logrus v1.4.2
	github.com/spf13/cobra v0.0.3
	github.com/spf13/pflag v1.0.3
	github.com/stretchr/objx v0.2.0 // indirect
	github.com/stretchr/testify v1.3.0
	github.com/syndtr/gocapability v0.0.0-20180916011248-d98352740cb2 // indirect
	github.com/tchap/go-patricia v2.2.6+incompatible // indirect
	github.com/uber/jaeger-client-go v0.0.0-20190214182810-64f57863bf63
	github.com/uber/jaeger-lib v0.0.0-20190122222657-d036253de8f5 // indirect
	github.com/ulikunitz/xz v0.5.5 // indirect
	github.com/varlink/go v0.0.0-20190502142041-0f1d566d194b
	github.com/vbatts/tar-split v0.11.1 // indirect
	github.com/vbauerster/mpb v3.3.4+incompatible // indirect
	github.com/vishvananda/netlink v1.0.0
	github.com/vishvananda/netns v0.0.0-20180720170159-13995c7128cc // indirect
	github.com/xeipuuv/gojsonpointer v0.0.0-20180127040702-4e3ac2762d5f // indirect
	github.com/xeipuuv/gojsonreference v0.0.0-20180127040603-bd5ef7bd5415 // indirect
	github.com/xeipuuv/gojsonschema v1.1.0 // indirect
	golang.org/x/crypto v0.0.0-20190621222207-cc06ce4a13d4
	golang.org/x/net v0.0.0-20190620200207-3b0461eec859 // indirect
	golang.org/x/sync v0.0.0-20190423024810-112230192c58
	golang.org/x/sys v0.0.0-20190621203818-d432491b9138
	golang.org/x/text v0.3.2 // indirect
	golang.org/x/time v0.0.0-20181108054448-85acf8d2951c // indirect
	golang.org/x/tools v0.0.0-20190621195816-6e04913cbbac // indirect
	gopkg.in/airbrake/gobrake.v2 v2.0.9 // indirect
	gopkg.in/gemnasium/logrus-airbrake-hook.v2 v2.1.2 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/mgo.v2 v2.0.0-20180705113604-9856a29383ce // indirect
	gopkg.in/yaml.v2 v2.2.2
	k8s.io/api v0.0.0-20181219151656-a146d628c217
	k8s.io/apimachinery v0.0.0-20181212193828-688d82452747
	k8s.io/client-go v0.0.0-20181219152756-3dd551c0f083
)
