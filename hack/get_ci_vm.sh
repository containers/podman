#!/usr/bin/env bash

#
# For help and usage information, simply execute the script w/o any arguments.
#
# This script is intended to be run by Red Hat podman developers who need
# to debug problems specifically related to Cirrus-CI automated testing.
# It requires that you have been granted prior access to create VMs in
# google-cloud.  For non-Red Hat contributors, VMs are available as-needed,
# with supervision upon request.

set -e

SCRIPT_FILEPATH=$(realpath "${BASH_SOURCE[0]}")
SCRIPT_DIRPATH=$(dirname "$SCRIPT_FILEPATH")
REPO_DIRPATH=$(realpath "$SCRIPT_DIRPATH/../")

# Help detect if we were called by get_ci_vm container
GET_CI_VM="${GET_CI_VM:-0}"
in_get_ci_vm() {
    if ((GET_CI_VM==0)); then
        echo "Error: $1 is not intended for use in this context"
        exit 2
    fi
}

# get_ci_vm APIv1 container entrypoint calls into this script
# to obtain required repo. specific configuration options.
if [[ "$1" == "--config" ]]; then
    in_get_ci_vm "$1"
    cat <<EOF
DESTDIR="/var/tmp/go/src/github.com/containers/podman"
UPSTREAM_REPO="https://github.com/containers/podman.git"
CI_ENVFILE="/etc/ci_environment"
GCLOUD_PROJECT="libpod-218412"
GCLOUD_IMGPROJECT="libpod-218412"
GCLOUD_CFG="libpod"
GCLOUD_ZONE="${GCLOUD_ZONE:-us-central1-a}"
GCLOUD_CPUS="2"
GCLOUD_MEMORY="4Gb"
GCLOUD_DISK="200"
EOF
elif [[ "$1" == "--setup" ]]; then
    in_get_ci_vm "$1"
    # get_ci_vm container entrypoint calls us with this option on the
    # Cirrus-CI environment instance, to perform repo.-specific setup.
    cd $REPO_DIRPATH
    echo "+ Loading ./contrib/cirrus/lib.sh" > /dev/stderr
    source ./contrib/cirrus/lib.sh
    echo "+ Mimicking .cirrus.yml build_task" > /dev/stderr
    make install.tools
    make binaries
    make docs
    echo "+ Running environment setup" > /dev/stderr
    ./contrib/cirrus/setup_environment.sh
else
    # Create and access VM for specified Cirrus-CI task
    mkdir -p $HOME/.config/gcloud/ssh
    podman run -it --rm \
        --tz=local \
        -e NAME="$USER" \
        -e SRCDIR=/src \
        -e GCLOUD_ZONE="$GCLOUD_ZONE" \
        -e A_DEBUG="${A_DEBUG:-0}" \
        -v $REPO_DIRPATH:/src:O \
        -v $HOME/.config/gcloud:/root/.config/gcloud:z \
        -v $HOME/.config/gcloud/ssh:/root/.ssh:z \
        quay.io/libpod/get_ci_vm:latest "$@"
fi
