module github.com/containers/podman/v4

go 1.18

require (
	github.com/BurntSushi/toml v1.3.1
	github.com/Microsoft/go-winio v0.6.1
	github.com/blang/semver/v4 v4.0.0
	github.com/buger/goterm v1.0.4
	github.com/checkpoint-restore/checkpointctl v0.1.0
	github.com/checkpoint-restore/go-criu/v6 v6.3.0
	github.com/container-orchestrated-devices/container-device-interface v0.5.4
	github.com/containernetworking/cni v1.1.2
	github.com/containernetworking/plugins v1.3.0
	github.com/containers/buildah v1.30.1-0.20230504052500-e925b5852e07
	github.com/containers/common v0.53.1-0.20230506101404-3e93a76d461c
	github.com/containers/conmon v2.0.20+incompatible
	github.com/containers/image/v5 v5.25.1-0.20230505072505-dc4a4be9cc1e
	github.com/containers/libhvee v0.0.5
	github.com/containers/ocicrypt v1.1.7
	github.com/containers/psgo v1.8.0
	github.com/containers/storage v1.46.1
	github.com/coreos/go-systemd/v22 v22.5.0
	github.com/coreos/stream-metadata-go v0.4.2
	github.com/crc-org/vfkit v0.0.5-0.20230427143911-8117c28876bc
	github.com/cyphar/filepath-securejoin v0.2.3
	github.com/digitalocean/go-qemu v0.0.0-20221209210016-f035778c97f7
	github.com/docker/docker v24.0.0+incompatible
	github.com/docker/go-connections v0.4.1-0.20210727194412-58542c764a11
	github.com/docker/go-plugins-helpers v0.0.0-20211224144127-6eecb7beb651
	github.com/docker/go-units v0.5.0
	github.com/fsnotify/fsnotify v1.6.0
	github.com/godbus/dbus/v5 v5.1.1-0.20230522191255-76236955d466
	github.com/google/gofuzz v1.2.0
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510
	github.com/google/uuid v1.3.0
	github.com/gorilla/handlers v1.5.1
	github.com/gorilla/mux v1.8.0
	github.com/gorilla/schema v1.2.0
	github.com/hashicorp/go-multierror v1.1.1
	github.com/json-iterator/go v1.1.12
	github.com/mattn/go-shellwords v1.0.12
	github.com/mattn/go-sqlite3 v1.14.17
	github.com/moby/term v0.5.0
	github.com/nxadm/tail v1.4.8
	github.com/onsi/ginkgo/v2 v2.9.6
	github.com/onsi/gomega v1.27.7
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.1.0-rc3
	github.com/opencontainers/runc v1.1.7
	github.com/opencontainers/runtime-spec v1.1.0-rc.2
	github.com/opencontainers/runtime-tools v0.9.1-0.20230317050512-e931285f4b69
	github.com/opencontainers/selinux v1.11.0
	github.com/openshift/imagebuilder v1.2.5
	github.com/rootless-containers/rootlesskit v1.1.1
	github.com/sirupsen/logrus v1.9.3
	github.com/spf13/cobra v1.7.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.8.4
	github.com/syndtr/gocapability v0.0.0-20200815063812-42c35b437635
	github.com/ulikunitz/xz v0.5.11
	github.com/vbauerster/mpb/v8 v8.4.0
	github.com/vishvananda/netlink v1.2.1-beta.2
	go.etcd.io/bbolt v1.3.7
	golang.org/x/net v0.10.0
	golang.org/x/sync v0.2.0
	golang.org/x/sys v0.8.0
	golang.org/x/term v0.8.0
	golang.org/x/text v0.9.0
	google.golang.org/protobuf v1.30.0
	gopkg.in/inf.v0 v0.9.1
	gopkg.in/yaml.v3 v3.0.1
	sigs.k8s.io/yaml v1.3.0
)

require (
	github.com/Azure/go-ansiterm v0.0.0-20210617225240-d185dfc1b5a1 // indirect
	github.com/Microsoft/hcsshim v0.10.0-rc.7 // indirect
	github.com/VividCortex/ewma v1.2.0 // indirect
	github.com/acarl005/stripansi v0.0.0-20180116102854-5a71ef0e047d // indirect
	github.com/asaskevich/govalidator v0.0.0-20230301143203-a9d515a09cc2 // indirect
	github.com/chzyer/readline v1.5.1 // indirect
	github.com/containerd/cgroups v1.1.0 // indirect
	github.com/containerd/containerd v1.7.0 // indirect
	github.com/containerd/stargz-snapshotter/estargz v0.14.3 // indirect
	github.com/containers/libtrust v0.0.0-20230121012942-c1716e8a8d01 // indirect
	github.com/coreos/go-oidc/v3 v3.5.0 // indirect
	github.com/coreos/go-systemd v0.0.0-20190719114852-fd7a80b32e1f // indirect
	github.com/cyberphone/json-canonicalization v0.0.0-20220623050100-57a0ce2678a7 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/digitalocean/go-libvirt v0.0.0-20220804181439-8648fbde413e // indirect
	github.com/disiqueira/gotree/v3 v3.0.2 // indirect
	github.com/docker/distribution v2.8.2+incompatible // indirect
	github.com/docker/docker-credential-helpers v0.7.0 // indirect
	github.com/felixge/httpsnoop v1.0.3 // indirect
	github.com/fsouza/go-dockerclient v1.9.7 // indirect
	github.com/go-jose/go-jose/v3 v3.0.0 // indirect
	github.com/go-logr/logr v1.2.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/go-openapi/analysis v0.21.4 // indirect
	github.com/go-openapi/errors v0.20.3 // indirect
	github.com/go-openapi/jsonpointer v0.19.5 // indirect
	github.com/go-openapi/jsonreference v0.20.0 // indirect
	github.com/go-openapi/loads v0.21.2 // indirect
	github.com/go-openapi/runtime v0.26.0 // indirect
	github.com/go-openapi/spec v0.20.9 // indirect
	github.com/go-openapi/strfmt v0.21.7 // indirect
	github.com/go-openapi/swag v0.22.3 // indirect
	github.com/go-openapi/validate v0.22.1 // indirect
	github.com/go-task/slim-sprig v0.0.0-20230315185526-52ccab3ef572 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/go-cmp v0.5.9 // indirect
	github.com/google/go-containerregistry v0.14.0 // indirect
	github.com/google/go-intervals v0.0.2 // indirect
	github.com/google/pprof v0.0.0-20230323073829-e72429f035bd // indirect
	github.com/google/trillian v1.5.1 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.2 // indirect
	github.com/imdario/mergo v0.3.15 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jinzhu/copier v0.3.5 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/klauspost/compress v1.16.5 // indirect
	github.com/klauspost/pgzip v1.2.6 // indirect
	github.com/kr/fs v0.1.0 // indirect
	github.com/letsencrypt/boulder v0.0.0-20230213213521-fdfea0d469b6 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/manifoldco/promptui v0.9.0 // indirect
	github.com/mattn/go-runewidth v0.0.14 // indirect
	github.com/miekg/pkcs11 v1.1.1 // indirect
	github.com/mistifyio/go-zfs/v3 v3.0.0 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/moby/patternmatcher v0.5.0 // indirect
	github.com/moby/sys/mountinfo v0.6.2 // indirect
	github.com/moby/sys/sequential v0.5.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/oklog/ulid v1.3.1 // indirect
	github.com/opentracing/opentracing-go v1.2.0 // indirect
	github.com/ostreedev/ostree-go v0.0.0-20210805093236-719684c64e4f // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pkg/sftp v1.13.5 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/proglottis/gpgme v0.1.3 // indirect
	github.com/rivo/uniseg v0.4.4 // indirect
	github.com/seccomp/libseccomp-golang v0.10.0 // indirect
	github.com/segmentio/ksuid v1.0.4 // indirect
	github.com/sigstore/fulcio v1.3.1 // indirect
	github.com/sigstore/rekor v1.1.2-0.20230508234306-ad288b385a44 // indirect
	github.com/sigstore/sigstore v1.6.4 // indirect
	github.com/skratchdot/open-golang v0.0.0-20200116055534-eef842397966 // indirect
	github.com/stefanberger/go-pkcs11uri v0.0.0-20201008174630-78d3cae3a980 // indirect
	github.com/sylabs/sif/v2 v2.11.3 // indirect
	github.com/tchap/go-patricia/v2 v2.3.1 // indirect
	github.com/theupdateframework/go-tuf v0.5.2 // indirect
	github.com/titanous/rocacheck v0.0.0-20171023193734-afe73141d399 // indirect
	github.com/vbatts/tar-split v0.11.3 // indirect
	github.com/vishvananda/netns v0.0.4 // indirect
	go.mongodb.org/mongo-driver v1.11.3 // indirect
	go.mozilla.org/pkcs7 v0.0.0-20210826202110-33d05740a352 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/otel v1.15.0 // indirect
	go.opentelemetry.io/otel/trace v1.15.0 // indirect
	golang.org/x/crypto v0.8.0 // indirect
	golang.org/x/exp v0.0.0-20230425010034-47ecfdc1ba53 // indirect
	golang.org/x/mod v0.10.0 // indirect
	golang.org/x/oauth2 v0.7.0 // indirect
	golang.org/x/tools v0.9.1 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20230410155749-daa745c078e1 // indirect
	google.golang.org/grpc v1.55.0 // indirect
	gopkg.in/go-jose/go-jose.v2 v2.6.1 // indirect
	gopkg.in/square/go-jose.v2 v2.6.0 // indirect
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
)

replace github.com/opencontainers/runc => github.com/opencontainers/runc v1.1.1-0.20220617142545-8b9452f75cbc
