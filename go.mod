module github.com/containers/libpod

go 1.12

require (
	cloud.google.com/go v0.40.0 // indirect
	github.com/Azure/go-autorest v12.1.0+incompatible // indirect
	github.com/BurntSushi/toml v0.3.1
	github.com/DataDog/zstd v1.4.0 // indirect
	github.com/Microsoft/hcsshim v0.8.6 // indirect
	github.com/NYTimes/gziphandler v1.1.1 // indirect
	github.com/Nvveen/Gotty v0.0.0-20120604004816-cd527374f1e5 // indirect
	github.com/VividCortex/ewma v1.1.1 // indirect
	github.com/blang/semver v3.5.1+incompatible // indirect
	github.com/buger/goterm v0.0.0-20181115115552-c206103e1f37
	github.com/checkpoint-restore/go-criu v0.0.0-20190109184317-bdb7599cd87b
	github.com/containerd/continuity v0.0.0-20190426062206-aaeac12a7ffc // indirect
	github.com/containernetworking/cni v0.7.1
	github.com/containernetworking/plugins v0.8.1
	github.com/containers/buildah v1.9.0
	github.com/containers/image v2.0.0+incompatible
	github.com/containers/psgo v1.3.0
	github.com/containers/storage v1.12.13
	github.com/coreos/bbolt v1.3.3 // indirect
	github.com/coreos/etcd v3.3.13+incompatible // indirect
	github.com/coreos/go-iptables v0.4.1
	github.com/coreos/go-semver v0.3.0 // indirect
	github.com/coreos/go-systemd v0.0.0-20190620071333-e64a0ec8b42a
	github.com/cri-o/ocicni v0.0.0-20190328132530-0c180f981b27
	github.com/cyphar/filepath-securejoin v0.2.2
	github.com/davecgh/go-spew v1.1.1
	github.com/docker/distribution v2.7.1+incompatible
	github.com/docker/docker v0.7.3-0.20190309235953-33c3200e0d16
	github.com/docker/docker-credential-helpers v0.6.2
	github.com/docker/go-connections v0.4.0
	github.com/docker/go-metrics v0.0.0-20181218153428-b84716841b82 // indirect
	github.com/docker/go-units v0.4.0
	github.com/docker/libnetwork v0.8.0-dev.2.0.20180608203834-19279f049241 // indirect
	github.com/docker/libtrust v0.0.0-20160708172513-aabc10ec26b7 // indirect
	github.com/docker/spdystream v0.0.0-20181023171402-6480d4af844c // indirect
	github.com/elazarl/goproxy v0.0.0-20190421051319-9d40249d3c2f // indirect
	github.com/emicklei/go-restful v2.9.6+incompatible // indirect
	github.com/etcd-io/bbolt v1.3.3
	github.com/evanphx/json-patch v4.5.0+incompatible // indirect
	github.com/fatih/camelcase v1.0.0
	github.com/fsnotify/fsnotify v1.4.7
	github.com/fsouza/go-dockerclient v1.4.1 // indirect
	github.com/ghodss/yaml v1.0.0
	github.com/go-openapi/spec v0.19.2 // indirect
	github.com/godbus/dbus v0.0.0-20181101234600-2ff6f7ffd60f
	github.com/golang/mock v1.3.1 // indirect
	github.com/google/pprof v0.0.0-20190515194954-54271f7e092f // indirect
	github.com/google/shlex v0.0.0-20181106134648-c34317bd91bf
	github.com/googleapis/gax-go/v2 v2.0.5 // indirect
	github.com/googleapis/gnostic v0.3.0 // indirect
	github.com/gophercloud/gophercloud v0.2.0 // indirect
	github.com/gorilla/context v1.1.1 // indirect
	github.com/gregjones/httpcache v0.0.0-20190611155906-901d90724c79 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.9.2 // indirect
	github.com/hashicorp/go-multierror v1.0.0
	github.com/hpcloud/tail v1.0.0
	github.com/imdario/mergo v0.3.7 // indirect
	github.com/ishidawataru/sctp v0.0.0-20180213033435-07191f837fed // indirect
	github.com/json-iterator/go v1.1.6
	github.com/kisielk/errcheck v1.2.0 // indirect
	github.com/klauspost/compress v1.7.1 // indirect
	github.com/klauspost/cpuid v1.2.1 // indirect
	github.com/klauspost/pgzip v1.2.1 // indirect
	github.com/konsorten/go-windows-terminal-sequences v1.0.2 // indirect
	github.com/magiconair/properties v1.8.1 // indirect
	github.com/mailru/easyjson v0.0.0-20190620125010-da37f6c1e481 // indirect
	github.com/mattn/go-isatty v0.0.8 // indirect
	github.com/mattn/go-shellwords v1.0.5 // indirect
	github.com/mistifyio/go-zfs v2.1.1+incompatible // indirect
	github.com/mrunalp/fileutils v0.0.0-20171103030105-7d4729fb3618
	github.com/mtrmac/gpgme v0.0.0-20170102180018-b2432428689c // indirect
	github.com/munnerz/goautoneg v0.0.0-20190414153302-2ae31c8b6b30 // indirect
	github.com/onsi/ginkgo v1.8.0
	github.com/onsi/gomega v1.5.0
	github.com/opencontainers/go-digest v1.0.0-rc1
	github.com/opencontainers/image-spec v1.0.1
	github.com/opencontainers/runc v1.0.0-rc6
	github.com/opencontainers/runtime-spec v0.0.0-20181111125026-1722abf79c2f
	github.com/opencontainers/runtime-tools v0.9.0
	github.com/opencontainers/selinux v1.2.2
	github.com/openshift/imagebuilder v1.1.0 // indirect
	github.com/opentracing/opentracing-go v1.1.0
	github.com/ostreedev/ostree-go v0.0.0-20181213164143-d0388bd827cf // indirect
	github.com/pelletier/go-toml v1.4.0 // indirect
	github.com/pkg/errors v0.8.1
	github.com/pkg/profile v1.3.0
	github.com/pmezard/go-difflib v1.0.0
	github.com/pquerna/ffjson v0.0.0-20181028064349-e517b90714f7 // indirect
	github.com/prometheus/common v0.6.0 // indirect
	github.com/rogpeppe/fastuuid v1.1.0 // indirect
	github.com/seccomp/containers-golang v0.0.0-20190312124753-8ca8945ccf5f // indirect
	github.com/seccomp/libseccomp-golang v0.9.1 // indirect
	github.com/sirupsen/logrus v1.4.2
	github.com/spf13/cobra v0.0.5
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/pflag v1.0.3
	github.com/spf13/viper v1.4.0 // indirect
	github.com/stretchr/testify v1.3.0
	github.com/syndtr/gocapability v0.0.0-20180916011248-d98352740cb2 // indirect
	github.com/tchap/go-patricia v2.3.0+incompatible // indirect
	github.com/uber/jaeger-client-go v2.16.0+incompatible
	github.com/uber/jaeger-lib v0.0.0-20190122222657-d036253de8f5 // indirect
	github.com/ugorji/go v1.1.5-pre // indirect
	github.com/ulikunitz/xz v0.5.6 // indirect
	github.com/varlink/go v0.0.0-20190502142041-0f1d566d194b
	github.com/vbatts/tar-split v0.11.1 // indirect
	github.com/vbauerster/mpb v3.4.0+incompatible // indirect
	github.com/vishvananda/netlink v1.0.0
	github.com/vishvananda/netns v0.0.0-20180720170159-13995c7128cc // indirect
	github.com/xeipuuv/gojsonpointer v0.0.0-20180127040702-4e3ac2762d5f // indirect
	github.com/xeipuuv/gojsonreference v0.0.0-20180127040603-bd5ef7bd5415 // indirect
	github.com/xeipuuv/gojsonschema v1.1.0 // indirect
	go.etcd.io/bbolt v1.3.3 // indirect
	go.opencensus.io v0.22.0 // indirect
	golang.org/x/crypto v0.0.0-20190621222207-cc06ce4a13d4
	golang.org/x/exp v0.0.0-20190510132918-efd6b22b2522 // indirect
	golang.org/x/image v0.0.0-20190622003408-7e034cad6442 // indirect
	golang.org/x/mobile v0.0.0-20190607214518-6fa95d984e88 // indirect
	golang.org/x/net v0.0.0-20190620200207-3b0461eec859 // indirect
	golang.org/x/sync v0.0.0-20190423024810-112230192c58
	golang.org/x/sys v0.0.0-20190624142023-c5567b49c5d0
	golang.org/x/tools v0.0.0-20190624190245-7f2218787638 // indirect
	google.golang.org/appengine v1.6.1 // indirect
	google.golang.org/genproto v0.0.0-20190620144150-6af8c5fc6601 // indirect
	google.golang.org/grpc v1.21.1 // indirect
	gopkg.in/airbrake/gobrake.v2 v2.0.9 // indirect
	gopkg.in/gemnasium/logrus-airbrake-hook.v2 v2.1.2 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/mgo.v2 v2.0.0-20180705113604-9856a29383ce // indirect
	gopkg.in/yaml.v2 v2.2.2
	k8s.io/api v0.0.0-20190624085159-95846d7ef82a
	k8s.io/apimachinery v0.0.0-20190624085041-961b39a1baa0
	k8s.io/client-go v0.0.0-20190620085101-78d2af792bab
	k8s.io/gengo v0.0.0-20190327210449-e17681d19d3a // indirect
	k8s.io/klog v0.3.3 // indirect
	k8s.io/kube-openapi v0.0.0-20190603182131-db7b694dc208 // indirect
	k8s.io/utils v0.0.0-20190607212802-c55fbcfc754a // indirect
)
