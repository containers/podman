#!/bin/bash

set -e
source $(dirname $0)/lib.sh

ENV_VARS='PACKER_BUILDS BUILT_IMAGE_SUFFIX UBUNTU_BASE_IMAGE FEDORA_BASE_IMAGE PRIOR_FEDORA_BASE_IMAGE SERVICE_ACCOUNT GCE_SSH_USERNAME GCP_PROJECT_ID PACKER_VER SCRIPT_BASE PACKER_BASE'
req_env_var $ENV_VARS
# Must also be made available through make, into packer process
export $ENV_VARS

# Everything here is running on the 'image-builder-image' GCE image
# Assume basic dependencies are all met, but there could be a newer version
# of the packer binary
PACKER_FILENAME="packer_${PACKER_VER}_linux_amd64.zip"
if [[ -d "$HOME/packer" ]]
then
    cd "$HOME/packer"
    # image_builder_image has packer pre-installed, check if same version requested
    if [[ -r "$PACKER_FILENAME" ]]
    then
        cp $PACKER_FILENAME "$GOSRC/$PACKER_BASE/"
        cp packer "$GOSRC/$PACKER_BASE/"
    fi
fi

cd "$GOSRC/$PACKER_BASE"

make libpod_images \
    PACKER_BUILDS=$PACKER_BUILDS \
    PACKER_VER=$PACKER_VER \
    GOSRC=$GOSRC \
    SCRIPT_BASE=$SCRIPT_BASE \
    PACKER_BASE=$PACKER_BASE \
    BUILT_IMAGE_SUFFIX=$BUILT_IMAGE_SUFFIX

# When successful, upload manifest of produced images using a filename unique
# to this build.
URI="gs://packer-import${POST_MERGE_BUCKET_SUFFIX}/manifest${BUILT_IMAGE_SUFFIX}.json"
gsutil cp packer-manifest.json "$URI"

echo "Finished. A JSON manifest of produced images is available at $URI"
