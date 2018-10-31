#!/bin/bash

# This script allows running 'packer build libpod_images.sh' manually,
# outside of CI.  It requires that you've properly set:
#
# GOOGLE_APPLICATION_CREDENTIALS to a json file for a service account.
# SERVICE_ACCOUNT to the e-mail of the service account (above)
# GCP_PROJECT_ID to the (unique) name for the project
# RHSM_COMMAND to the command needed to register a RHEL machine

set -e

getyaml() {
    egrep -m 1 '^[^#]+\s+'"$1"':\s+' $GOSRC/.cirrus.yml | cut -d : -f 2- | tr -d \" | tr -d '[[:blank:]]'
}

export GOSRC=$(realpath "$(dirname $0)/../../../")
export FEDORA_CNI_COMMIT="$(getyaml FEDORA_CNI_COMMIT)"
export CNI_COMMIT="$(getyaml CNI_COMMIT)"
export CRIO_COMMIT="$(getyaml CRIO_COMMIT)"
export CRIU_COMMIT="$(getyaml CRIU_COMMIT)"
export RUNC_COMMIT="$(getyaml RUNC_COMMIT)"
export ENVLIB=".bash_profile"
export CIRRUS_SHELL="/bin/bash"
export SCRIPT_BASE="$(getyaml SCRIPT_BASE)"
export PACKER_BASE="$(getyaml PACKER_BASE)"
export PACKER_VER="$(getyaml PACKER_VER)"

export PACKER_BUILDS="${PACKER_BUILDS:-$(getyaml PACKER_BUILDS)}"
export CENTOS_BASE_IMAGE="$(getyaml CENTOS_BASE_IMAGE)"
export UBUNTU_BASE_IMAGE="$(getyaml UBUNTU_BASE_IMAGE)"
export RHEL_BASE_IMAGE="$(getyaml RHEL_BASE_IMAGE)"
export FEDORA_IMAGE_PREFIX="$(getyaml FEDORA_IMAGE_PREFIX)"
export FEDORA_IMAGE_URL="$(getyaml FEDORA_IMAGE_URL)"
export FEDORA_CSUM_URL="$(getyaml FEDORA_CSUM_URL)"
export FAH_IMAGE_PREFIX="$(getyaml FAH_IMAGE_PREFIX)"
export FAH_IMAGE_URL="$(getyaml FAH_IMAGE_URL)"
export FAH_CSUM_URL="$(getyaml FAH_CSUM_URL)"

export CIRRUS_WORKING_DIR="$GOSRC"
export CIRRUS_BUILD_ID=deadbeef
export CIRRUS_REPO_NAME=libpod
export CIRRUS_CHANGE_IN_REPO=$(git rev-parse HEAD)
export BUILT_IMAGE_SUFFIX="-$CIRRUS_REPO_NAME-${CIRRUS_CHANGE_IN_REPO:0:8}"
export GCE_SSH_USERNAME=root  # in this context only
export PATH="$PATH:$GOSRC/$SCRIPT_BASE"

source $GOSRC/$SCRIPT_BASE/lib.sh

req_env_var "
    GOOGLE_APPLICATION_CREDENTIALS $GOOGLE_APPLICATION_CREDENTIALS
    SERVICE_ACCOUNT $SERVICE_ACCOUNT
    GCP_PROJECT_ID $GCP_PROJECT_ID
    RHSM_COMMAND $RHSM_COMMAND
"

TMPDIRPATH=$(mktemp -d)
trap "rm -rf $TMPDIRPATH" EXIT

# Fedora and FAH need cloud-init w/ ssh keys
nocloud_floppy $TMPDIRPATH
export CIDATA_IMAGE=$TMPDIRPATH.img
export CIDATA_SSH_KEY=$TMPDIRPATH.ssh

cd "$GOSRC/$PACKER_BASE"
make PACKER_VER=$PACKER_VER PACKER_BUILDS=$PACKER_BUILDS
ls -la $TMPDIRPATH
