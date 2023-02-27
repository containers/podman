module github.com/containers/podman/v4

go 1.18

require (
	github.com/BurntSushi/toml v1.2.1
	github.com/blang/semver/v4 v4.0.0
	github.com/buger/goterm v1.0.4
	github.com/checkpoint-restore/checkpointctl v0.0.0-20220321135231-33f4a66335f0
	github.com/checkpoint-restore/go-criu/v6 v6.3.0
	github.com/container-orchestrated-devices/container-device-interface v0.5.4
	github.com/containernetworking/cni v1.1.2
	github.com/containernetworking/plugins v1.2.0
	github.com/containers/buildah v1.29.1-0.20230201192322-e56eb25575c7
	github.com/containers/common v0.51.1-0.20230221111605-852ca05a1fbb
	github.com/containers/conmon v2.0.20+incompatible
	github.com/containers/image/v5 v5.24.2-0.20230221092641-10858b2058d8
	github.com/containers/ocicrypt v1.1.7
	github.com/containers/psgo v1.8.0
	github.com/containers/storage v1.45.4
	github.com/coreos/go-systemd/v22 v22.5.0
	github.com/coreos/stream-metadata-go v0.4.1
	github.com/cyphar/filepath-securejoin v0.2.3
	github.com/digitalocean/go-qemu v0.0.0-20210326154740-ac9e0b687001
	github.com/docker/docker v23.0.1+incompatible
	github.com/docker/go-connections v0.4.1-0.20210727194412-58542c764a11
	github.com/docker/go-plugins-helpers v0.0.0-20211224144127-6eecb7beb651
	github.com/docker/go-units v0.5.0
	github.com/fsnotify/fsnotify v1.6.0
	github.com/ghodss/yaml v1.0.0
	github.com/godbus/dbus/v5 v5.1.1-0.20221029134443-4b691ce883d5
	github.com/google/gofuzz v1.2.0
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510
	github.com/google/uuid v1.3.0
	github.com/gorilla/handlers v1.5.1
	github.com/gorilla/mux v1.8.0
	github.com/gorilla/schema v1.2.0
	github.com/hashicorp/go-multierror v1.1.1
	github.com/json-iterator/go v1.1.12
	github.com/mattn/go-shellwords v1.0.12
	github.com/mattn/go-sqlite3 v1.14.16
	github.com/moby/term v0.0.0-20221120202655-abb19827d345
	github.com/nxadm/tail v1.4.8
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.27.1
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.1.0-rc2
	github.com/opencontainers/runc v1.1.4
	github.com/opencontainers/runtime-spec v1.1.0-rc.1
	github.com/opencontainers/runtime-tools v0.9.1-0.20221107090550-2e043c6bd626
	github.com/opencontainers/selinux v1.11.0
	github.com/openshift/imagebuilder v1.2.4-0.20230207193036-6e08c897da73
	github.com/rootless-containers/rootlesskit v1.1.0
	github.com/sirupsen/logrus v1.9.0
	github.com/spf13/cobra v1.6.1
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.8.1
	github.com/syndtr/gocapability v0.0.0-20200815063812-42c35b437635
	github.com/uber/jaeger-client-go v2.30.0+incompatible
	github.com/ulikunitz/xz v0.5.11
	github.com/vbauerster/mpb/v8 v8.2.0
	github.com/vishvananda/netlink v1.2.1-beta.2
	go.etcd.io/bbolt v1.3.7
	golang.org/x/net v0.7.0
	golang.org/x/sync v0.1.0
	golang.org/x/sys v0.5.0
	golang.org/x/term v0.5.0
	golang.org/x/text v0.7.0
	google.golang.org/protobuf v1.28.1
	gopkg.in/inf.v0 v0.9.1
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/Azure/go-ansiterm v0.0.0-20210617225240-d185dfc1b5a1 // indirect
	github.com/Microsoft/go-winio v0.6.0 // indirect
	github.com/Microsoft/hcsshim v0.9.7 // indirect
	github.com/VividCortex/ewma v1.2.0 // indirect
	github.com/acarl005/stripansi v0.0.0-20180116102854-5a71ef0e047d // indirect
	github.com/asaskevich/govalidator v0.0.0-20210307081110-f21760c49a8d // indirect
	github.com/chzyer/readline v1.5.1 // indirect
	github.com/containerd/cgroups v1.0.4 // indirect
	github.com/containerd/containerd v1.6.18 // indirect
	github.com/containerd/stargz-snapshotter/estargz v0.14.1 // indirect
	github.com/containers/libtrust v0.0.0-20230121012942-c1716e8a8d01 // indirect
	github.com/coreos/go-oidc/v3 v3.5.0 // indirect
	github.com/coreos/go-systemd v0.0.0-20190719114852-fd7a80b32e1f // indirect
	github.com/cyberphone/json-canonicalization v0.0.0-20220623050100-57a0ce2678a7 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/digitalocean/go-libvirt v0.0.0-20201209184759-e2a69bcd5bd1 // indirect
	github.com/disiqueira/gotree/v3 v3.0.2 // indirect
	github.com/docker/distribution v2.8.1+incompatible // indirect
	github.com/docker/docker-credential-helpers v0.7.0 // indirect
	github.com/felixge/httpsnoop v1.0.3 // indirect
	github.com/fsouza/go-dockerclient v1.9.3 // indirect
	github.com/go-jose/go-jose/v3 v3.0.0 // indirect
	github.com/go-openapi/analysis v0.21.4 // indirect
	github.com/go-openapi/errors v0.20.3 // indirect
	github.com/go-openapi/jsonpointer v0.19.5 // indirect
	github.com/go-openapi/jsonreference v0.20.0 // indirect
	github.com/go-openapi/loads v0.21.2 // indirect
	github.com/go-openapi/runtime v0.24.1 // indirect
	github.com/go-openapi/spec v0.20.7 // indirect
	github.com/go-openapi/strfmt v0.21.3 // indirect
	github.com/go-openapi/swag v0.22.3 // indirect
	github.com/go-openapi/validate v0.22.0 // indirect
	github.com/go-playground/locales v0.14.0 // indirect
	github.com/go-playground/universal-translator v0.18.0 // indirect
	github.com/go-playground/validator/v10 v10.11.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/google/go-cmp v0.5.9 // indirect
	github.com/google/go-containerregistry v0.12.1 // indirect
	github.com/google/go-intervals v0.0.2 // indirect
	github.com/google/trillian v1.5.1-0.20220819043421-0a389c4bb8d9 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.2 // indirect
	github.com/imdario/mergo v0.3.13 // indirect
	github.com/inconshreveable/mousetrap v1.0.1 // indirect
	github.com/jinzhu/copier v0.3.5 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/klauspost/compress v1.15.15 // indirect
	github.com/klauspost/pgzip v1.2.6-0.20220930104621-17e8dac29df8 // indirect
	github.com/kr/fs v0.1.0 // indirect
	github.com/leodido/go-urn v1.2.1 // indirect
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
	github.com/rivo/uniseg v0.4.3 // indirect
	github.com/seccomp/libseccomp-golang v0.10.0 // indirect
	github.com/segmentio/ksuid v1.0.4 // indirect
	github.com/sigstore/fulcio v1.1.0 // indirect
	github.com/sigstore/rekor v1.0.1 // indirect
	github.com/sigstore/sigstore v1.5.1 // indirect
	github.com/skratchdot/open-golang v0.0.0-20200116055534-eef842397966 // indirect
	github.com/stefanberger/go-pkcs11uri v0.0.0-20201008174630-78d3cae3a980 // indirect
	github.com/sylabs/sif/v2 v2.9.2 // indirect
	github.com/tchap/go-patricia/v2 v2.3.1 // indirect
	github.com/theupdateframework/go-tuf v0.5.2 // indirect
	github.com/titanous/rocacheck v0.0.0-20171023193734-afe73141d399 // indirect
	github.com/vbatts/tar-split v0.11.2 // indirect
	github.com/vishvananda/netns v0.0.0-20210104183010-2eb08e3e575f // indirect
	go.mongodb.org/mongo-driver v1.11.1 // indirect
	go.mozilla.org/pkcs7 v0.0.0-20210826202110-33d05740a352 // indirect
	go.opencensus.io v0.24.0 // indirect
	golang.org/x/crypto v0.6.0 // indirect
	golang.org/x/exp v0.0.0-20230206171751-46f607a40771 // indirect
	golang.org/x/mod v0.8.0 // indirect
	golang.org/x/oauth2 v0.5.0 // indirect
	golang.org/x/tools v0.6.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20230209215440-0dfe4f8abfcc // indirect
	google.golang.org/grpc v1.53.0 // indirect
	gopkg.in/go-jose/go-jose.v2 v2.6.1 // indirect
	gopkg.in/square/go-jose.v2 v2.6.0 // indirect
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	sigs.k8s.io/yaml v1.3.0 // indirect
)

replace github.com/opencontainers/runc => github.com/opencontainers/runc v1.1.1-0.20220617142545-8b9452f75cbc
