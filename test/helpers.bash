#!/bin/bash

# Root directory of integration tests.
INTEGRATION_ROOT=$(dirname "$(readlink -f "$BASH_SOURCE")")

# Test data path.
TESTDATA="${INTEGRATION_ROOT}/testdata"

# Root directory of the repository.
CRIO_ROOT=${CRIO_ROOT:-$(cd "$INTEGRATION_ROOT/../.."; pwd -P)}

# Path of the crio binary.
CRIO_BINARY=${CRIO_BINARY:-${CRIO_ROOT}/cri-o/bin/crio}
# Path of the crictl binary.
CRICTL_PATH=$(command -v crictl || true)
CRICTL_BINARY=${CRICTL_PATH:-/usr/bin/crictl}
# Path to kpod binary.
KPOD_BINARY=${KPOD_BINARY:-${CRIO_ROOT}/cri-o/bin/kpod}
# Path of the conmon binary.
CONMON_BINARY=${CONMON_BINARY:-${CRIO_ROOT}/cri-o/bin/conmon}
# Path of the pause binary.
PAUSE_BINARY=${PAUSE_BINARY:-${CRIO_ROOT}/cri-o/bin/pause}
# Path of the default seccomp profile.
SECCOMP_PROFILE=${SECCOMP_PROFILE:-${CRIO_ROOT}/cri-o/seccomp.json}
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
BIN2IMG_BINARY=${BIN2IMG_BINARY:-${CRIO_ROOT}/cri-o/test/bin2img/bin2img}
# Path of the copyimg binary.
COPYIMG_BINARY=${COPYIMG_BINARY:-${CRIO_ROOT}/cri-o/test/copyimg/copyimg}
# Path of tests artifacts.
ARTIFACTS_PATH=${ARTIFACTS_PATH:-${CRIO_ROOT}/cri-o/.artifacts}
# Path of the checkseccomp binary.
CHECKSECCOMP_BINARY=${CHECKSECCOMP_BINARY:-${CRIO_ROOT}/cri-o/test/checkseccomp/checkseccomp}
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

TESTDIR=$(mktemp -d)

# kpod pull needs a configuration file for shortname pulls
export REGISTRIES_CONFIG_PATH="$INTEGRATION_ROOT/registries.conf"

# Setup default hooks dir
HOOKSDIR=$TESTDIR/hooks
mkdir ${HOOKSDIR}
HOOKS_OPTS="--hooks-dir-path=$HOOKSDIR"

# Setup default secrets mounts
MOUNT_PATH="$TESTDIR/secrets"
mkdir ${MOUNT_PATH}
MOUNT_FILE="${MOUNT_PATH}/test.txt"
touch ${MOUNT_FILE}
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
CRIO_SOCKET="$TESTDIR/crio.sock"
CRIO_CONFIG="$TESTDIR/crio.conf"
CRIO_CNI_CONFIG="$TESTDIR/cni/net.d/"
CRIO_CNI_PLUGIN=${CRIO_CNI_PLUGIN:-/opt/cni/bin/}
POD_CIDR="10.88.0.0/16"
POD_CIDR_MASK="10.88.*.*"

KPOD_OPTIONS="--root $TESTDIR/crio $STORAGE_OPTIONS --runroot $TESTDIR/crio-run --runtime ${RUNTIME_BINARY}"

cp "$CONMON_BINARY" "$TESTDIR/conmon"

PATH=$PATH:$TESTDIR

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

# Make sure we have a copy of the mrunalp/oom:latest image.
if ! [ -d "$ARTIFACTS_PATH"/oom-image ]; then
    mkdir -p "$ARTIFACTS_PATH"/oom-image
    if ! "$COPYIMG_BINARY" --import-from=docker://mrunalp/oom --export-to=dir:"$ARTIFACTS_PATH"/oom-image --signature-policy="$INTEGRATION_ROOT"/policy.json ; then
        echo "Error pulling docker://mrunalp/oom"
        rm -fr "$ARTIFACTS_PATH"/oom-image
        exit 1
    fi
fi

# Make sure we have a copy of the mrunalp/image-volume-test:latest image.
if ! [ -d "$ARTIFACTS_PATH"/image-volume-test-image ]; then
    mkdir -p "$ARTIFACTS_PATH"/image-volume-test-image
    if ! "$COPYIMG_BINARY" --import-from=docker://mrunalp/image-volume-test --export-to=dir:"$ARTIFACTS_PATH"/image-volume-test-image --signature-policy="$INTEGRATION_ROOT"/policy.json ; then
        echo "Error pulling docker://mrunalp/image-volume-test-image"
        rm -fr "$ARTIFACTS_PATH"/image-volume-test-image
        exit 1
    fi
fi
# Run crio using the binary specified by $CRIO_BINARY.
# This must ONLY be run on engines created with `start_crio`.
function crio() {
	"$CRIO_BINARY" --listen "$CRIO_SOCKET" "$@"
}

# DEPRECATED
OCIC_BINARY=${OCIC_BINARY:-${CRIO_ROOT}/cri-o/bin/crioctl}
# Run crioctl using the binary specified by $OCIC_BINARY.
function crioctl() {
	"$OCIC_BINARY" --connect "$CRIO_SOCKET" "$@"
}

# Run crictl using the binary specified by $CRICTL_BINARY.
function crictl() {
	"$CRICTL_BINARY" -r "$CRIO_SOCKET" -i "$CRIO_SOCKET" "$@"
}

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

# Start crio.
function start_crio() {
	if [[ -n "$1" ]]; then
		seccomp="$1"
	else
		seccomp="$SECCOMP_PROFILE"
	fi

	if [[ -n "$2" ]]; then
		apparmor="$2"
	else
		apparmor="$APPARMOR_PROFILE"
	fi

	# Don't forget: bin2img, copyimg, and crio have their own default drivers, so if you override any, you probably need to override them all
	if ! [ "$3" = "--no-pause-image" ] ; then
		"$BIN2IMG_BINARY" --root "$TESTDIR/crio" $STORAGE_OPTIONS --runroot "$TESTDIR/crio-run" --source-binary "$PAUSE_BINARY"
	fi
	"$COPYIMG_BINARY" --root "$TESTDIR/crio" $STORAGE_OPTIONS --runroot "$TESTDIR/crio-run" --image-name=docker.io/library/redis:alpine --import-from=dir:"$ARTIFACTS_PATH"/redis-image --signature-policy="$INTEGRATION_ROOT"/policy.json
# TODO: remove the code below for copying redis:alpine in using a canonical reference once
#       https://github.com/kubernetes-incubator/cri-o/issues/531 is complete and we can
#       copy the image using a tagged reference and then subsequently find the image without
#       having to explicitly record the canonical reference as one of the image's names
	"$COPYIMG_BINARY" --root "$TESTDIR/crio" $STORAGE_OPTIONS --runroot "$TESTDIR/crio-run" --image-name=docker.io/library/redis@sha256:03789f402b2ecfb98184bf128d180f398f81c63364948ff1454583b02442f73b --import-from=dir:"$ARTIFACTS_PATH"/redis-image-digest --signature-policy="$INTEGRATION_ROOT"/policy.json
	"$COPYIMG_BINARY" --root "$TESTDIR/crio" $STORAGE_OPTIONS --runroot "$TESTDIR/crio-run" --image-name=mrunalp/oom --import-from=dir:"$ARTIFACTS_PATH"/oom-image --signature-policy="$INTEGRATION_ROOT"/policy.json
	"$COPYIMG_BINARY" --root "$TESTDIR/crio" $STORAGE_OPTIONS --runroot "$TESTDIR/crio-run" --image-name=docker.io/library/mrunalp/image-volume-test --import-from=dir:"$ARTIFACTS_PATH"/image-volume-test-image --signature-policy="$INTEGRATION_ROOT"/policy.json
	"$COPYIMG_BINARY" --root "$TESTDIR/crio" $STORAGE_OPTIONS --runroot "$TESTDIR/crio-run" --image-name=docker.io/library/busybox:latest --import-from=dir:"$ARTIFACTS_PATH"/busybox-image --signature-policy="$INTEGRATION_ROOT"/policy.json
	"$COPYIMG_BINARY" --root "$TESTDIR/crio" $STORAGE_OPTIONS --runroot "$TESTDIR/crio-run" --image-name=docker.io/library/runcom/stderr-test:latest --import-from=dir:"$ARTIFACTS_PATH"/stderr-test --signature-policy="$INTEGRATION_ROOT"/policy.json
	"$CRIO_BINARY" ${DEFAULT_MOUNTS_OPTS} ${HOOKS_OPTS} --conmon "$CONMON_BINARY" --listen "$CRIO_SOCKET" --cgroup-manager "$CGROUP_MANAGER" --registry "docker.io" --runtime "$RUNTIME_BINARY" --root "$TESTDIR/crio" --runroot "$TESTDIR/crio-run" $STORAGE_OPTIONS --seccomp-profile "$seccomp" --apparmor-profile "$apparmor" --cni-config-dir "$CRIO_CNI_CONFIG" --cni-plugin-dir "$CRIO_CNI_PLUGIN" --signature-policy "$INTEGRATION_ROOT"/policy.json --image-volumes "$IMAGE_VOLUMES" --pids-limit "$PIDS_LIMIT" --log-size-max "$LOG_SIZE_MAX_LIMIT" --config /dev/null config >$CRIO_CONFIG

	# Prepare the CNI configuration files, we're running with non host networking by default
	if [[ -n "$4" ]]; then
		netfunc="$4"
	else
		netfunc="prepare_network_conf"
	fi
	${netfunc} $POD_CIDR

	"$CRIO_BINARY" --log-level debug --config "$CRIO_CONFIG" & CRIO_PID=$!
	wait_until_reachable

	run crictl inspecti redis:alpine
	if [ "$status" -ne 0 ] ; then
		crictl pull redis:alpine
	fi
	REDIS_IMAGEID=$(crictl inspecti redis:alpine | head -1 | sed -e "s/ID: //g")
	run crictl inspecti redis@sha256:03789f402b2ecfb98184bf128d180f398f81c63364948ff1454583b02442f73b
	if [ "$status" -ne 0 ] ; then
		crictl pull redis@sha256:03789f402b2ecfb98184bf128d180f398f81c63364948ff1454583b02442f73b
	fi
	REDIS_IMAGEID_DIGESTED=$(crictl inspecti redis@sha256:03789f402b2ecfb98184bf128d180f398f81c63364948ff1454583b02442f73b | head -1 | sed -e "s/ID: //g")
	run crictl inspecti mrunalp/oom
	if [ "$status" -ne 0 ] ; then
		  crictl pull mrunalp/oom
	fi
	OOM_IMAGEID=$(crictl inspecti mrunalp/oom | head -1 | sed -e "s/ID: //g")
	run crioctl image status --id=runcom/stderr-test
	if [ "$status" -ne 0 ] ; then
		crictl pull runcom/stderr-test:latest
	fi
	STDERR_IMAGEID=$(crictl inspecti runcom/stderr-test | head -1 | sed -e "s/ID: //g")
	run crictl inspecti busybox
	if [ "$status" -ne 0 ] ; then
		crictl pull busybox:latest
	fi
	BUSYBOX_IMAGEID=$(crictl inspecti busybox | head -1 | sed -e "s/ID: //g")
	run crictl inspecti mrunalp/image-volume-test
	if [ "$status" -ne 0 ] ; then
		  crictl pull mrunalp/image-volume-test:latest
	fi
	VOLUME_IMAGEID=$(crictl inspecti mrunalp/image-volume-test | head -1 | sed -e "s/ID: //g")
}

function cleanup_ctrs() {
	run crictl ps --quiet
	if [ "$status" -eq 0 ]; then
		if [ "$output" != "" ]; then
			printf '%s\n' "$output" | while IFS= read -r line
			do
			   crictl stop "$line"
			   crictl rm "$line"
			done
		fi
	fi
	rm -f /run/hookscheck
}

function cleanup_images() {
	run crictl images --quiet
	if [ "$status" -eq 0 ]; then
		if [ "$output" != "" ]; then
			printf '%s\n' "$output" | while IFS= read -r line
			do
			   crictl rmi "$line"
			done
		fi
	fi
}

function cleanup_pods() {
	run crictl sandboxes --quiet
	if [ "$status" -eq 0 ]; then
		if [ "$output" != "" ]; then
			printf '%s\n' "$output" | while IFS= read -r line
			do
			   crictl stops "$line"
			   crictl rms "$line"
			done
		fi
	fi
}

# Stop crio.
function stop_crio() {
	if [ "$CRIO_PID" != "" ]; then
		kill "$CRIO_PID" >/dev/null 2>&1
		wait "$CRIO_PID"
		rm -f "$CRIO_CONFIG"
	fi

	cleanup_network_conf
}

function restart_crio() {
	if [ "$CRIO_PID" != "" ]; then
		kill "$CRIO_PID" >/dev/null 2>&1
		wait "$CRIO_PID"
		start_crio
	else
		echo "you must start crio first"
		exit 1
	fi
}

function cleanup_test() {
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

function prepare_network_conf() {
	mkdir -p $CRIO_CNI_CONFIG
	cat >$CRIO_CNI_CONFIG/10-crio.conf <<-EOF
{
    "cniVersion": "0.2.0",
    "name": "crionet",
    "type": "bridge",
    "bridge": "cni0",
    "isGateway": true,
    "ipMasq": true,
    "ipam": {
        "type": "host-local",
        "subnet": "$1",
        "routes": [
            { "dst": "0.0.0.0/0"  }
        ]
    }
}
EOF

	cat >$CRIO_CNI_CONFIG/99-loopback.conf <<-EOF
{
    "cniVersion": "0.2.0",
    "type": "loopback"
}
EOF

	echo 0
}

function prepare_plugin_test_args_network_conf() {
	mkdir -p $CRIO_CNI_CONFIG
	cat >$CRIO_CNI_CONFIG/10-plugin-test-args.conf <<-EOF
{
    "cniVersion": "0.2.0",
    "name": "crionet_test_args",
    "type": "bridge-custom",
    "bridge": "cni0",
    "isGateway": true,
    "ipMasq": true,
    "ipam": {
        "type": "host-local",
        "subnet": "$1",
        "routes": [
            { "dst": "0.0.0.0/0"  }
        ]
    }
}
EOF

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
	rm -rf $CRIO_CNI_CONFIG

	echo 0
}

function temp_sandbox_conf() {
	sed -e s/\"namespace\":.*/\"namespace\":\ \"$1\",/g "$TESTDATA"/sandbox_config.json > $TESTDIR/sandbox_config_$1.json
}
