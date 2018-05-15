#!/bin/bash

# podman needs to play some games with resources
if [[ $(id -u) != 0 ]]; then
  echo >&2 $0 must be run as root.
  exit 2
fi

while getopts "vh" arg; do
  case $arg in
    v ) VERBOSE='-v' ;;
    h ) echo >2 $0 [-v] [-h] [test.TestCase|test.TestCase.step] ; exit 2 ;;
  esac
done
shift $((OPTIND-1))

# Create temporary directory for storage
export TMPDIR=`mktemp -d /tmp/podman.XXXXXXXXXX`

function umount {
  # xargs -r always ran once, so write any mount points to file first
  mount |awk "/$1/"' { print $3 }' >${TMPDIR}/mounts
  if [[ -s ${TMPDIR}/mounts ]]; then
    xargs <${TMPDIR}/mounts -t umount
  fi
}

function cleanup {
  umount '^(shm|nsfs)'
  umount '\/run\/netns'
  rm -fr ${TMPDIR}
}
trap cleanup EXIT

# setup path to find new binaries _NOT_ system binaries
if [[ ! -x ../../bin/podman ]]; then
  echo 1>&2 Cannot find podman binary from libpod root directory, Or, run \"make binaries\"
  exit 1
fi
export PATH=../../bin:$PATH

function showlog {
  [ -s "$1" ] && (echo $1 =====; cat "$1")
}

# Need a location to store the podman socket
mkdir -p ${TMPDIR}/{podman,crio,crio-run,cni/net.d}

# Cannot be done in python unittest fixtures.  EnvVar not picked up.
export REGISTRIES_CONFIG_PATH=${TMPDIR}/registry.conf
cat >$REGISTRIES_CONFIG_PATH <<-EOT
  [registries.search]
    registries = ['docker.io']
  [registries.insecure]
    registries = []
  [registries.block]
    registries = []
EOT

export CNI_CONFIG_PATH=${TMPDIR}/cni/net.d
cat >$CNI_CONFIG_PATH/87-podman-bridge.conflist <<-EOT
{
  "cniVersion": "0.3.0",
  "name": "podman",
  "plugins": [{
      "type": "bridge",
      "bridge": "cni0",
      "isGateway": true,
      "ipMasq": true,
      "ipam": {
        "type": "host-local",
        "subnet": "10.88.0.0/16",
        "routes": [{
          "dst": "0.0.0.0/0"
        }]
      }
    },
    {
      "type": "portmap",
      "capabilities": {
        "portMappings": true
      }
    }
  ]
}
EOT

export PODMAN_HOST="unix:${TMPDIR}/podman/io.projectatomic.podman"
PODMAN_ARGS="--storage-driver=vfs\
  --root=${TMPDIR}/crio\
  --runroot=${TMPDIR}/crio-run\
  --cni-config-dir=$CNI_CONFIG_PATH\
  "
PODMAN="podman $PODMAN_ARGS"

# document what we're about to do...
$PODMAN --version

set -x
# Run podman in background without systemd for test purposes
$PODMAN varlink ${PODMAN_HOST} >/tmp/test_runner.output 2>&1 &

if [[ -z $1 ]]; then
  export PYTHONPATH=.
  python3 -m unittest discover -s . $VERBOSE
else
  export PYTHONPATH=.:./test
  python3 -m unittest $1 $VERBOSE
fi

set +x
pkill podman
pkill -9 conmon

showlog /tmp/alpine.log
showlog /tmp/busybox.log
