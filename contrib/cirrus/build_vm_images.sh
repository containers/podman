#!/bin/bash

set -e
source $(dirname $0)/lib.sh

req_env_var "
CNI_COMMIT $CNI_COMMIT
CRIO_COMMIT $CRIO_COMMIT
RUNC_COMMIT $RUNC_COMMIT
PACKER_BUILDS $PACKER_BUILDS
CENTOS_BASE_IMAGE $CENTOS_BASE_IMAGE
UBUNTU_BASE_IMAGE $UBUNTU_BASE_IMAGE
FEDORA_BASE_IMAGE $FEDORA_BASE_IMAGE
FAH_BASE_IMAGE $FAH_BASE_IMAGE
RHEL_BASE_IMAGE $RHEL_BASE_IMAGE
RHSM_COMMAND $RHSM_COMMAND
SERVICE_ACCOUNT $SERVICE_ACCOUNT
GCE_SSH_USERNAME $GCE_SSH_USERNAME
GCP_PROJECT_ID $GCP_PROJECT_ID
PACKER_VER $PACKER_VER
SCRIPT_BASE $SCRIPT_BASE
PACKER_BASE $PACKER_BASE
"

show_env_vars

# Everything here is running on the 'image-builder-image' GCE image
# Assume basic dependencies are all met, but there could be a newer version
# of the packer binary
PACKER_FILENAME="packer_${PACKER_VER}_linux_amd64.zip"
mkdir -p "$HOME/packer"
cd "$HOME/packer"
# image_builder_image has packer pre-installed, check if same version requested
if ! [[ -r "$PACKER_FILENAME" ]]
then
    curl -L -O https://releases.hashicorp.com/packer/$PACKER_VER/$PACKER_FILENAME
    curl -L https://releases.hashicorp.com/packer/${PACKER_VER}/packer_${PACKER_VER}_SHA256SUMS | \
        grep 'linux_amd64' > ./sha256sums
    sha256sum --check ./sha256sums
    unzip -o $PACKER_FILENAME
    ./packer --help &> /dev/null # verify exit(0)
fi

set -x

cd "$GOSRC/$PACKER_BASE"
make libpod_images \
    PACKER_BUILDS=$PACKER_BUILDS \
    PACKER_VER=$PACKER_VER \
    GOSRC=$GOSRC \
    SCRIPT_BASE=$SCRIPT_BASE \
    PACKER_BASE=$PACKER_BASE
