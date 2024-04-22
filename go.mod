module github.com/containers/podman/v4

go 1.17

require (
	github.com/BurntSushi/toml v1.0.0
	github.com/blang/semver v3.5.1+incompatible
	github.com/buger/goterm v1.0.4
	github.com/checkpoint-restore/checkpointctl v0.0.0-20211204171957-54b4ebfdb681
	github.com/checkpoint-restore/go-criu/v5 v5.3.0
	github.com/container-orchestrated-devices/container-device-interface v0.0.0-20220111162300-46367ec063fd
	github.com/containernetworking/cni v1.0.1
	github.com/containernetworking/plugins v1.0.1
	github.com/containers/buildah v1.24.7
	github.com/containers/common v0.47.6
	github.com/containers/conmon v2.0.20+incompatible
	github.com/containers/image/v5 v5.19.4
	github.com/containers/ocicrypt v1.1.4
	github.com/containers/psgo v1.7.2
	github.com/containers/storage v1.38.5
	github.com/coreos/go-systemd/v22 v22.3.2
	github.com/coreos/stream-metadata-go v0.0.0-20210225230131-70edb9eb47b3
	github.com/cyphar/filepath-securejoin v0.2.3
	github.com/davecgh/go-spew v1.1.1
	github.com/digitalocean/go-qemu v0.0.0-20210326154740-ac9e0b687001
	github.com/docker/distribution v2.8.0+incompatible
	github.com/docker/docker v20.10.12+incompatible
	github.com/docker/go-connections v0.4.1-0.20210727194412-58542c764a11
	github.com/docker/go-plugins-helpers v0.0.0-20211224144127-6eecb7beb651
	github.com/docker/go-units v0.4.0
	github.com/dtylman/scp v0.0.0-20181017070807-f3000a34aef4
	github.com/fsnotify/fsnotify v1.5.1
	github.com/ghodss/yaml v1.0.0
	github.com/godbus/dbus/v5 v5.1.0
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510
	github.com/google/uuid v1.3.0
	github.com/gorilla/handlers v1.5.1
	github.com/gorilla/mux v1.8.0
	github.com/gorilla/schema v1.2.0
	github.com/hashicorp/go-multierror v1.1.1
	github.com/hpcloud/tail v1.0.0
	github.com/json-iterator/go v1.1.12
	github.com/mattn/go-isatty v0.0.14
	github.com/moby/term v0.0.0-20210619224110-3f7ff695adc6
	github.com/mrunalp/fileutils v0.5.0
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.18.1
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.0.3-0.20220114050600-8b9d41f48198
	github.com/opencontainers/runc v1.1.2
	github.com/opencontainers/runtime-spec v1.0.3-0.20211214071223-8958f93039ab
	github.com/opencontainers/runtime-tools v0.9.1-0.20220110225228-7e2d60f1e41f
	github.com/opencontainers/selinux v1.10.0
	github.com/pkg/errors v0.9.1
	github.com/pmezard/go-difflib v1.0.0
	github.com/rootless-containers/rootlesskit v0.14.6
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/cobra v1.3.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.0
	github.com/syndtr/gocapability v0.0.0-20200815063812-42c35b437635
	github.com/uber/jaeger-client-go v2.30.0+incompatible
	github.com/ulikunitz/xz v0.5.10
	github.com/vbauerster/mpb/v6 v6.0.4
	github.com/vishvananda/netlink v1.1.1-0.20220115184804-dd687eb2f2d4
	go.etcd.io/bbolt v1.3.6
	golang.org/x/crypto v0.17.0
	golang.org/x/sync v0.1.0
	golang.org/x/sys v0.15.0
	golang.org/x/text v0.14.0
	google.golang.org/protobuf v1.27.1
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b
	k8s.io/api v0.22.4
	k8s.io/apimachinery v0.22.4
)

require (
	github.com/Azure/go-ansiterm v0.0.0-20210617225240-d185dfc1b5a1 // indirect
	github.com/Microsoft/go-winio v0.5.2 // indirect
	github.com/Microsoft/hcsshim v0.9.2 // indirect
	github.com/VividCortex/ewma v1.2.0 // indirect
	github.com/acarl005/stripansi v0.0.0-20180116102854-5a71ef0e047d // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/chzyer/readline v0.0.0-20180603132655-2972be24d48e // indirect
	github.com/containerd/cgroups v1.0.3 // indirect
	github.com/containerd/containerd v1.5.18 // indirect
	github.com/containerd/stargz-snapshotter/estargz v0.11.0 // indirect
	github.com/containers/libtrust v0.0.0-20190913040956-14b96171aa3b // indirect
	github.com/coreos/go-systemd v0.0.0-20190620071333-e64a0ec8b42a // indirect
	github.com/digitalocean/go-libvirt v0.0.0-20201209184759-e2a69bcd5bd1 // indirect
	github.com/disiqueira/gotree/v3 v3.0.2 // indirect
	github.com/docker/docker-credential-helpers v0.6.4 // indirect
	github.com/docker/go-metrics v0.0.1 // indirect
	github.com/docker/libnetwork v0.8.0-dev.2.0.20190625141545-5a177b73e316 // indirect
	github.com/felixge/httpsnoop v1.0.1 // indirect
	github.com/fsouza/go-dockerclient v1.7.8 // indirect
	github.com/go-logr/logr v0.4.0 // indirect
	github.com/go-task/slim-sprig v0.0.0-20210107165309-348f09dbbbc0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/google/go-cmp v0.5.7 // indirect
	github.com/google/go-intervals v0.0.2 // indirect
	github.com/google/gofuzz v1.1.0 // indirect
	github.com/hashicorp/errwrap v1.0.0 // indirect
	github.com/imdario/mergo v0.3.12 // indirect
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/ishidawataru/sctp v0.0.0-20210226210310-f2269e66cdee // indirect
	github.com/jinzhu/copier v0.3.5 // indirect
	github.com/klauspost/compress v1.14.2 // indirect
	github.com/klauspost/pgzip v1.2.6 // indirect
	github.com/manifoldco/promptui v0.9.0 // indirect
	github.com/mattn/go-runewidth v0.0.13 // indirect
	github.com/mattn/go-shellwords v1.0.12 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/miekg/pkcs11 v1.1.1 // indirect
	github.com/mistifyio/go-zfs v2.1.2-0.20190413222219-f784269be439+incompatible // indirect
	github.com/mitchellh/mapstructure v1.4.3 // indirect
	github.com/moby/sys/mount v0.2.0 // indirect
	github.com/moby/sys/mountinfo v0.5.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/nxadm/tail v1.4.8 // indirect
	github.com/openshift/imagebuilder v1.2.2 // indirect
	github.com/ostreedev/ostree-go v0.0.0-20190702140239-759a8c1ac913 // indirect
	github.com/proglottis/gpgme v0.1.1 // indirect
	github.com/prometheus/client_golang v1.11.1 // indirect
	github.com/prometheus/client_model v0.2.0 // indirect
	github.com/prometheus/common v0.26.0 // indirect
	github.com/prometheus/procfs v0.6.0 // indirect
	github.com/rivo/uniseg v0.2.0 // indirect
	github.com/seccomp/libseccomp-golang v0.9.2-0.20210429002308-3879420cc921 // indirect
	github.com/stefanberger/go-pkcs11uri v0.0.0-20201008174630-78d3cae3a980 // indirect
	github.com/sylabs/sif/v2 v2.3.1 // indirect
	github.com/tchap/go-patricia v2.3.0+incompatible // indirect
	github.com/vbatts/tar-split v0.11.2 // indirect
	github.com/vbauerster/mpb/v7 v7.3.2 // indirect
	github.com/vishvananda/netns v0.0.0-20210104183010-2eb08e3e575f // indirect
	github.com/xeipuuv/gojsonpointer v0.0.0-20190809123943-df4f5c81cb3b // indirect
	github.com/xeipuuv/gojsonreference v0.0.0-20180127040603-bd5ef7bd5415 // indirect
	github.com/xeipuuv/gojsonschema v1.2.0 // indirect
	go.mozilla.org/pkcs7 v0.0.0-20200128120323-432b2356ecb1 // indirect
	go.opencensus.io v0.23.0 // indirect
	golang.org/x/net v0.10.0 // indirect
	golang.org/x/term v0.15.0 // indirect
	golang.org/x/tools v0.6.0 // indirect
	google.golang.org/genproto v0.0.0-20211208223120-3a66f561d7aa // indirect
	google.golang.org/grpc v1.42.0 // indirect
	gopkg.in/fsnotify.v1 v1.4.7 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/square/go-jose.v2 v2.5.1 // indirect
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	k8s.io/klog/v2 v2.9.0 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.1.2 // indirect
	sigs.k8s.io/yaml v1.3.0 // indirect
)

replace github.com/onsi/gomega => github.com/onsi/gomega v1.16.0

// See https://github.com/projectatomic/runc/blob/podman-v4.0-rhel/README.branch for details.
replace github.com/opencontainers/runc => github.com/projectatomic/runc v0.0.0-20240307031208-38278c48667a
