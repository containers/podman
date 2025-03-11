module github.com/containers/podman/v4

go 1.23.0

require (
	github.com/BurntSushi/toml v1.2.0
	github.com/blang/semver/v4 v4.0.0
	github.com/buger/goterm v1.0.4
	github.com/checkpoint-restore/checkpointctl v0.0.0-20220321135231-33f4a66335f0
	github.com/checkpoint-restore/go-criu/v5 v5.3.0
	github.com/container-orchestrated-devices/container-device-interface v0.4.0
	github.com/containernetworking/cni v1.1.2
	github.com/containernetworking/plugins v1.1.1
	github.com/containers/buildah v1.27.6
	github.com/containers/common v0.49.3
	github.com/containers/conmon v2.0.20+incompatible
	github.com/containers/image/v5 v5.22.1
	github.com/containers/ocicrypt v1.1.5
	github.com/containers/psgo v1.7.2
	github.com/containers/storage v1.42.1-0.20221223141133-5351738c4dba
	github.com/coreos/go-systemd/v22 v22.3.2
	github.com/coreos/stream-metadata-go v0.0.0-20210225230131-70edb9eb47b3
	github.com/cyphar/filepath-securejoin v0.2.3
	github.com/digitalocean/go-qemu v0.0.0-20210326154740-ac9e0b687001
	github.com/docker/distribution v2.8.1+incompatible
	github.com/docker/docker v20.10.17+incompatible
	github.com/docker/go-connections v0.4.1-0.20210727194412-58542c764a11
	github.com/docker/go-plugins-helpers v0.0.0-20211224144127-6eecb7beb651
	github.com/docker/go-units v0.4.0
	github.com/dtylman/scp v0.0.0-20181017070807-f3000a34aef4
	github.com/fsnotify/fsnotify v1.5.4
	github.com/ghodss/yaml v1.0.0
	github.com/godbus/dbus/v5 v5.1.0
	github.com/google/gofuzz v1.2.0
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510
	github.com/google/uuid v1.3.0
	github.com/gorilla/handlers v1.5.1
	github.com/gorilla/mux v1.8.0
	github.com/gorilla/schema v1.4.1
	github.com/hashicorp/go-multierror v1.1.1
	github.com/json-iterator/go v1.1.12
	github.com/mattn/go-isatty v0.0.14
	github.com/moby/term v0.0.0-20210619224110-3f7ff695adc6
	github.com/nxadm/tail v1.4.8
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.20.0
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.0.3-0.20220114050600-8b9d41f48198
	github.com/opencontainers/runc v1.1.3
	github.com/opencontainers/runtime-spec v1.0.3-0.20211214071223-8958f93039ab
	github.com/opencontainers/runtime-tools v0.9.1-0.20220714195903-17b3287fafb7
	github.com/opencontainers/selinux v1.10.2
	github.com/rootless-containers/rootlesskit v1.0.1
	github.com/sirupsen/logrus v1.9.0
	github.com/spf13/cobra v1.5.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.8.0
	github.com/syndtr/gocapability v0.0.0-20200815063812-42c35b437635
	github.com/uber/jaeger-client-go v2.30.0+incompatible
	github.com/ulikunitz/xz v0.5.10
	github.com/vbauerster/mpb/v7 v7.4.2
	github.com/vishvananda/netlink v1.1.1-0.20220115184804-dd687eb2f2d4
	go.etcd.io/bbolt v1.3.6
	golang.org/x/crypto v0.36.0
	golang.org/x/sync v0.12.0
	golang.org/x/sys v0.31.0
	golang.org/x/term v0.30.0
	golang.org/x/text v0.23.0
	google.golang.org/protobuf v1.33.0
	gopkg.in/inf.v0 v0.9.1
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/Azure/go-ansiterm v0.0.0-20210617225240-d185dfc1b5a1 // indirect
	github.com/Microsoft/go-winio v0.5.2 // indirect
	github.com/Microsoft/hcsshim v0.9.3 // indirect
	github.com/VividCortex/ewma v1.2.0 // indirect
	github.com/acarl005/stripansi v0.0.0-20180116102854-5a71ef0e047d // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/blang/semver v3.5.1+incompatible // indirect
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/chzyer/readline v0.0.0-20180603132655-2972be24d48e // indirect
	github.com/containerd/cgroups v1.0.3 // indirect
	github.com/containerd/containerd v1.6.6 // indirect
	github.com/containerd/stargz-snapshotter/estargz v0.12.0 // indirect
	github.com/containers/libtrust v0.0.0-20200511145503-9c3a6c22cd9a // indirect
	github.com/coreos/go-systemd v0.0.0-20190620071333-e64a0ec8b42a // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/digitalocean/go-libvirt v0.0.0-20201209184759-e2a69bcd5bd1 // indirect
	github.com/disiqueira/gotree/v3 v3.0.2 // indirect
	github.com/docker/docker-credential-helpers v0.6.4 // indirect
	github.com/docker/go-metrics v0.0.1 // indirect
	github.com/docker/libnetwork v0.8.0-dev.2.0.20190625141545-5a177b73e316 // indirect
	github.com/felixge/httpsnoop v1.0.1 // indirect
	github.com/fsouza/go-dockerclient v1.8.1 // indirect
	github.com/go-task/slim-sprig v0.0.0-20210107165309-348f09dbbbc0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/google/go-containerregistry v0.10.0 // indirect
	github.com/google/go-intervals v0.0.2 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/imdario/mergo v0.3.13 // indirect
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/jinzhu/copier v0.3.5 // indirect
	github.com/klauspost/compress v1.15.9 // indirect
	github.com/klauspost/pgzip v1.2.5 // indirect
	github.com/letsencrypt/boulder v0.0.0-20220331220046-b23ab962616e // indirect
	github.com/manifoldco/promptui v0.9.0 // indirect
	github.com/mattn/go-runewidth v0.0.13 // indirect
	github.com/mattn/go-shellwords v1.0.12 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.2-0.20181231171920-c182affec369 // indirect
	github.com/miekg/pkcs11 v1.1.1 // indirect
	github.com/mistifyio/go-zfs v2.1.2-0.20190413222219-f784269be439+incompatible // indirect
	github.com/moby/sys/mount v0.3.3 // indirect
	github.com/moby/sys/mountinfo v0.6.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/openshift/imagebuilder v1.2.4-0.20220711175835-4151e43600df // indirect
	github.com/ostreedev/ostree-go v0.0.0-20210805093236-719684c64e4f // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/proglottis/gpgme v0.1.3 // indirect
	github.com/prometheus/client_golang v1.12.1 // indirect
	github.com/prometheus/client_model v0.2.0 // indirect
	github.com/prometheus/common v0.32.1 // indirect
	github.com/prometheus/procfs v0.7.3 // indirect
	github.com/rivo/uniseg v0.2.0 // indirect
	github.com/seccomp/libseccomp-golang v0.10.0 // indirect
	github.com/sigstore/sigstore v1.3.1-0.20220629021053-b95fc0d626c1 // indirect
	github.com/stefanberger/go-pkcs11uri v0.0.0-20201008174630-78d3cae3a980 // indirect
	github.com/sylabs/sif/v2 v2.7.1 // indirect
	github.com/tchap/go-patricia v2.3.0+incompatible // indirect
	github.com/theupdateframework/go-tuf v0.3.1 // indirect
	github.com/titanous/rocacheck v0.0.0-20171023193734-afe73141d399 // indirect
	github.com/vbatts/tar-split v0.11.2 // indirect
	github.com/vishvananda/netns v0.0.0-20210104183010-2eb08e3e575f // indirect
	github.com/xeipuuv/gojsonpointer v0.0.0-20190905194746-02993c407bfb // indirect
	github.com/xeipuuv/gojsonreference v0.0.0-20180127040603-bd5ef7bd5415 // indirect
	github.com/xeipuuv/gojsonschema v1.2.0 // indirect
	go.mozilla.org/pkcs7 v0.0.0-20200128120323-432b2356ecb1 // indirect
	go.opencensus.io v0.23.0 // indirect
	golang.org/x/net v0.25.0 // indirect
	golang.org/x/tools v0.21.1-0.20240508182429-e35e4ccd0d2d // indirect
	google.golang.org/genproto v0.0.0-20220624142145-8cd45d7dbd1f // indirect
	google.golang.org/grpc v1.47.0 // indirect
	gopkg.in/square/go-jose.v2 v2.6.0 // indirect
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	sigs.k8s.io/yaml v1.3.0 // indirect
)

// See https://github.com/projectatomic/runc/blob/podman-v4.2.0-rhel/README.branch for details.
replace github.com/opencontainers/runc => github.com/projectatomic/runc v0.0.0-20240307021259-c0428046ced8
