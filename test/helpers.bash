#!/bin/bash

# Root directory of integration tests.
INTEGRATION_ROOT=$(dirname "$(readlink -f "$BASH_SOURCE")")

# Test data path.
TESTDATA="${INTEGRATION_ROOT}/testdata"

# Root directory of the repository.
if [[ ! -z "$CRIO_ROOT" ]]; then
    CRIO_ROOT=${CRIO_ROOT}
elif [[ ! -z "$TRAVIS" ]]; then
    CRIO_ROOT="/go/src/github.com/projectatomic/libpod"
elif [[ ! -z "$PAPR" ]]; then
    CRIO_ROOT="/var/tmp/checkout"
else
    CRIO_ROOT=$(cd "$INTEGRATION_ROOT/.."; pwd -P)
fi

PODMAN_BINARY=${PODMAN_BINARY:-${CRIO_ROOT}/bin/podman}
# Path of the conmon binary.
CONMON_BINARY=${CONMON_BINARY:-${CRIO_ROOT}/bin/conmon}
# Path of the default seccomp profile.
SECCOMP_PROFILE=${SECCOMP_PROFILE:-${CRIO_ROOT}/seccomp.json}
# Name of the default apparmor profile.
APPARMOR_PROFILE=${APPARMOR_PROFILE:-crio-default}
# Runtime
RUNTIME=${RUNTIME:-runc}
RUNTIME_PATH=$(command -v $RUNTIME || true)
RUNTIME_BINARY=${RUNTIME_PATH:-/usr/local/sbin/runc}
# Path of the apparmor_parser binary.
APPARMOR_PARSER_BINARY=${APPARMOR_PARSER_BINARY:-/sbin/apparmor_parser}
# Path of the apparmor profile for test.
APPARMOR_TEST_PROFILE_PATH=${APPARMOR_TEST_PROFILE_PATH:-${TESTDATA}/apparmor_test_deny_write}
# Path of the apparmor profile for unloading crio-default.
FAKE_CRIO_DEFAULT_PROFILE_PATH=${FAKE_CRIO_DEFAULT_PROFILE_PATH:-${TESTDATA}/fake_crio_default}
# Name of the apparmor profile for test.
APPARMOR_TEST_PROFILE_NAME=${APPARMOR_TEST_PROFILE_NAME:-apparmor-test-deny-write}
# Path of boot config.
BOOT_CONFIG_FILE_PATH=${BOOT_CONFIG_FILE_PATH:-/boot/config-`uname -r`}
# Path of apparmor parameters file.
APPARMOR_PARAMETERS_FILE_PATH=${APPARMOR_PARAMETERS_FILE_PATH:-/sys/module/apparmor/parameters/enabled}
# Path of the bin2img binary.
BIN2IMG_BINARY=${BIN2IMG_BINARY:-${CRIO_ROOT}/test/bin2img/bin2img}
# Path of the copyimg binary.
COPYIMG_BINARY=${COPYIMG_BINARY:-${CRIO_ROOT}/test/copyimg/copyimg}
# Path of tests artifacts.
ARTIFACTS_PATH=${ARTIFACTS_PATH:-${CRIO_ROOT}/.artifacts}
# Path of the checkseccomp binary.
CHECKSECCOMP_BINARY=${CHECKSECCOMP_BINARY:-${CRIO_ROOT}/test/checkseccomp/checkseccomp}
# XXX: This is hardcoded inside cri-o at the moment.
DEFAULT_LOG_PATH=/var/log/crio/pods
# Cgroup manager to be used
CGROUP_MANAGER=${CGROUP_MANAGER:-cgroupfs}
# Image volumes handling
IMAGE_VOLUMES=${IMAGE_VOLUMES:-mkdir}
# Container pids limit
PIDS_LIMIT=${PIDS_LIMIT:-1024}
# Log size max limit
LOG_SIZE_MAX_LIMIT=${LOG_SIZE_MAX_LIMIT:--1}

if [[ ! -d "/test.dir" ]]; then
    mkdir /test.dir
fi

TESTDIR=$(mktemp -p /test.dir -d)

declare -A -g IMAGES
IMAGES+=(["alpine"]=docker.io/library/alpine:latest ["busybox"]=docker.io/library/busybox:latest)

BB_GLIBC="docker.io/library/busybox:glibc"
BB="docker.io/library/busybox:latest"
ALPINE="docker.io/library/alpine:latest"
FEDORA_MINIMAL="registry.fedoraproject.org/fedora-minimal:latest"

# podman pull needs a configuration file for shortname pulls
export REGISTRIES_CONFIG_PATH="$INTEGRATION_ROOT/registries.conf"

# Setup default hooks dir
HOOKSDIR=$TESTDIR/hooks
mkdir ${HOOKSDIR}
HOOKS_OPTS="--hooks-dir-path=$HOOKSDIR"

# Setup default secrets mounts
MOUNT_PATH="$TESTDIR/secrets"
mkdir ${MOUNT_PATH}
MOUNT_FILE="${MOUNT_PATH}/test.txt"
touch ${MOUNT_FILE}}
echo "Testing secrets mounts!" > ${MOUNT_FILE}

DEFAULT_MOUNTS_OPTS="--default-mounts=${MOUNT_PATH}:/container/path1"

# We may need to set some default storage options.
case "$(stat -f -c %T ${TESTDIR})" in
    aufs)
        # None of device mapper, overlay, or aufs can be used dependably over aufs, and of course btrfs and zfs can't,
        # and we have to explicitly specify the "vfs" driver in order to use it, so do that now.
        STORAGE_OPTIONS=${STORAGE_OPTIONS:---storage-driver vfs}
        ;;
esac

if [ -e /usr/sbin/selinuxenabled ] && /usr/sbin/selinuxenabled; then
    . /etc/selinux/config
    filelabel=$(awk -F'"' '/^file.*=.*/ {print $2}' /etc/selinux/${SELINUXTYPE}/contexts/lxc_contexts)
    chcon -R ${filelabel} $TESTDIR
fi

LIBPOD_CNI_CONFIG="$TESTDIR/etc/cni/net.d/"
LIBPOD_CNI_PLUGIN=${LIBPOD_CNI_PLUGIN:-/opt/cni/bin/}
POD_CIDR="10.88.0.0/16"
POD_CIDR_MASK="10.88.*.*"

# Make sure the cni config dirs are created and populate them with the default configs
mkdir -p ${LIBPOD_CNI_CONFIG}
cp ${CRIO_ROOT}/cni/* ${LIBPOD_CNI_CONFIG}

PODMAN_OPTIONS="--root $TESTDIR/crio $STORAGE_OPTIONS --runroot $TESTDIR/crio-run --runtime ${RUNTIME_BINARY} --conmon ${CONMON_BINARY} --cni-config-dir ${LIBPOD_CNI_CONFIG}"

cp "$CONMON_BINARY" "$TESTDIR/conmon"

PATH=$PATH:$TESTDIR

for key in ${!IMAGES[@]}; do
    if ! [ -d "$ARTIFACTS_PATH"/${key} ]; then
        mkdir -p "$ARTIFACTS_PATH"/${key}
        if ! "$COPYIMG_BINARY" --import-from=docker://${IMAGES[${key}]} --export-to=dir:"$ARTIFACTS_PATH"/${key} --signature-policy="$INTEGRATION_ROOT"/policy.json ; then
            echo "Error pulling docker://${IMAGES[${key}]}"
            rm -fr "$ARTIFACTS_PATH"/${key}
            exit 1
        fi
    fi

done

###
# Buildah related variables
###
BUILDAH_BINARY=${BUILDAH_BINARY:-$(dirname ${BASH_SOURCE})/../buildah}
BUILDAH_IMGTYPE_BINARY=${BUILDAH_IMGTYPE_BINARY:-$(dirname ${BASH_SOURCE})/../imgtype}
BUILDAH_TESTSDIR=${BUILDAH_TESTSDIR:-$(dirname ${BASH_SOURCE})}
BUILDAH_STORAGE_DRIVER=${BUILDAH_STORAGE_DRIVER:-vfs}
#BUILDAH_PATH=$(dirname ${BASH_SOURCE})/..:${BUILDAH_PATH}

# Make sure we have a copy of the redis:alpine image.
if ! [ -d "$ARTIFACTS_PATH"/redis-image ]; then
    mkdir -p "$ARTIFACTS_PATH"/redis-image
    if ! "$COPYIMG_BINARY" --import-from=docker://redis:alpine --export-to=dir:"$ARTIFACTS_PATH"/redis-image --signature-policy="$INTEGRATION_ROOT"/policy.json ; then
        echo "Error pulling docker://redis"
        rm -fr "$ARTIFACTS_PATH"/redis-image
        exit 1
    fi
fi

# TODO: remove the code below for pulling redis:alpine using a canonical reference once
#       https://github.com/kubernetes-incubator/cri-o/issues/531 is complete and we can
#       pull the image using a tagged reference and then subsequently find the image without
#       having to explicitly record the canonical reference as one of the image's names
if ! [ -d "$ARTIFACTS_PATH"/redis-image-digest ]; then
    mkdir -p "$ARTIFACTS_PATH"/redis-image-digest
    if ! "$COPYIMG_BINARY" --import-from=docker://redis@sha256:03789f402b2ecfb98184bf128d180f398f81c63364948ff1454583b02442f73b --export-to=dir:"$ARTIFACTS_PATH"/redis-image-digest --signature-policy="$INTEGRATION_ROOT"/policy.json ; then
        echo "Error pulling docker://redis@sha256:03789f402b2ecfb98184bf128d180f398f81c63364948ff1454583b02442f73b"
        rm -fr "$ARTIFACTS_PATH"/redis-image-digest
        exit 1
    fi
fi

# Make sure we have a copy of the runcom/stderr-test image.
if ! [ -d "$ARTIFACTS_PATH"/stderr-test ]; then
    mkdir -p "$ARTIFACTS_PATH"/stderr-test
    if ! "$COPYIMG_BINARY" --import-from=docker://runcom/stderr-test:latest --export-to=dir:"$ARTIFACTS_PATH"/stderr-test --signature-policy="$INTEGRATION_ROOT"/policy.json ; then
        echo "Error pulling docker://stderr-test"
        rm -fr "$ARTIFACTS_PATH"/stderr-test
        exit 1
    fi
fi

# Make sure we have a copy of the busybox:latest image.
if ! [ -d "$ARTIFACTS_PATH"/busybox-image ]; then
    mkdir -p "$ARTIFACTS_PATH"/busybox-image
    if ! "$COPYIMG_BINARY" --import-from=docker://busybox --export-to=dir:"$ARTIFACTS_PATH"/busybox-image --signature-policy="$INTEGRATION_ROOT"/policy.json ; then
        echo "Error pulling docker://busybox"
        rm -fr "$ARTIFACTS_PATH"/busybox-image
        exit 1
    fi
fi

# Communicate with Docker on the host machine.
# Should rarely use this.
function docker_host() {
	command docker "$@"
}

# Retry a command $1 times until it succeeds. Wait $2 seconds between retries.
function retry() {
	local attempts=$1
	shift
	local delay=$1
	shift
	local i

	for ((i=0; i < attempts; i++)); do
		run "$@"
		if [[ "$status" -eq 0 ]] ; then
			return 0
		fi
		sleep $delay
	done

	echo "Command \"$@\" failed $attempts times. Output: $output"
	false
}

# Waits until the given crio becomes reachable.
function wait_until_reachable() {
	retry 15 1 crictl status
}

function cleanup_test() {
	run bash -c "${PODMAN_BINARY} ${PODMAN_OPTIONS} rm --force --all"
	echo "$output"
	rm -rf "$TESTDIR"
}


function load_apparmor_profile() {
	"$APPARMOR_PARSER_BINARY" -r "$1"
}

function remove_apparmor_profile() {
	"$APPARMOR_PARSER_BINARY" -R "$1"
}

function is_seccomp_enabled() {
	if ! "$CHECKSECCOMP_BINARY" ; then
		echo 0
		return
	fi
	echo 1
}

function is_apparmor_enabled() {
	if [[ -f "$APPARMOR_PARAMETERS_FILE_PATH" ]]; then
		out=$(cat "$APPARMOR_PARAMETERS_FILE_PATH")
		if [[ "$out" =~ "Y" ]]; then
			echo 1
			return
		fi
	fi
	echo 0
}

function check_pod_cidr() {
	run crioctl ctr execsync --id $1 ip addr show dev eth0 scope global 2>&1
	echo "$output"
	[ "$status" -eq 0  ]
	[[ "$output" =~ $POD_CIDR_MASK  ]]
}

function parse_pod_ip() {
	for arg
	do
		cidr=`echo "$arg" | grep $POD_CIDR_MASK`
		if [ "$cidr" == "$arg" ]
		then
			echo `echo "$arg" | sed "s/\/[0-9][0-9]//"`
		fi
	done
}

function get_host_ip() {
	gateway_dev=`ip -o route show default 0.0.0.0/0 | sed 's/.*dev \([^[:space:]]*\).*/\1/'`
	[ "$gateway_dev" ]
	host_ip=`ip -o -4 addr show dev $gateway_dev scope global | sed 's/.*inet \([0-9.]*\).*/\1/'`
}

function ping_pod() {
	inet=`crioctl ctr execsync --id $1 ip addr show dev eth0 scope global 2>&1 | grep inet`

	IFS=" "
	ip=`parse_pod_ip $inet`

	ping -W 1 -c 5 $ip

	echo $?
}

function ping_pod_from_pod() {
	inet=`crioctl ctr execsync --id $1 ip addr show dev eth0 scope global 2>&1 | grep inet`

	IFS=" "
	ip=`parse_pod_ip $inet`

	run crioctl ctr execsync --id $2 ping -W 1 -c 2 $ip
	echo "$output"
	[ "$status" -eq 0   ]
}


function cleanup_network_conf() {
	rm -rf $LIBPOD_CNI_CONFIG

	echo 0
}

function temp_sandbox_conf() {
	sed -e s/\"namespace\":.*/\"namespace\":\ \"$1\",/g "$TESTDATA"/sandbox_config.json > $TESTDIR/sandbox_config_$1.json
}

function copy_images() {
    for key in ${!IMAGES[@]}; do
        "$COPYIMG_BINARY" --root "$TESTDIR/crio" $STORAGE_OPTIONS --runroot "$TESTDIR/crio-run" --image-name=${IMAGES[${key}]} --import-from=dir:"$ARTIFACTS_PATH"/${key} --add-name=${IMAGES[${key}]}
    done
}

###
# Buildah Functions
###
function setup() {
	suffix=$(dd if=/dev/urandom bs=12 count=1 status=none | base64 | tr +/ABCDEFGHIJKLMNOPQRSTUVWXYZ _.abcdefghijklmnopqrstuvwxyz)
	TESTDIR=${BATS_TMPDIR}/tmp.${suffix}
	rm -fr ${TESTDIR}
	mkdir -p ${TESTDIR}/{root,runroot}
}

function starthttpd() {
	pushd ${2:-${TESTDIR}} > /dev/null
	cp ${BUILDAH_TESTSDIR}/serve.go .
	go build serve.go
	HTTP_SERVER_PORT=$((RANDOM+32768))
	./serve ${HTTP_SERVER_PORT} ${1:-${BATS_TMPDIR}} &
	HTTP_SERVER_PID=$!
	popd > /dev/null
}

function stophttpd() {
	if test -n "$HTTP_SERVER_PID" ; then
		kill -HUP ${HTTP_SERVER_PID}
		unset HTTP_SERVER_PID
		unset HTTP_SERVER_PORT
	fi
	true
}

function teardown() {
	stophttpd
	rm -fr ${TESTDIR}
}

function createrandom() {
	dd if=/dev/urandom bs=1 count=${2:-256} of=${1:-${BATS_TMPDIR}/randomfile} status=none
}

function buildah() {
	${BUILDAH_BINARY} --debug --root ${TESTDIR}/root --runroot ${TESTDIR}/runroot --storage-driver ${BUILDAH_STORAGE_DRIVER} "$@"
}

function imgtype() {
        ${BUILDAH_IMGTYPE_BINARY} -root ${TESTDIR}/root -runroot ${TESTDIR}/runroot -storage-driver ${BUILDAH_STORAGE_DRIVER} "$@"
}
