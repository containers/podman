#!/bin/bash

set -e
source $(dirname $0)/lib.sh

req_env_var "
CNI_COMMIT $CNI_COMMIT
CRIO_COMMIT $CRIO_COMMIT
RUNC_COMMIT $RUNC_COMMIT
PACKER_BUILDS $PACKER_BUILDS
BUILT_IMAGE_SUFFIX $BUILT_IMAGE_SUFFIX
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

record_timestamp "cache-image build start"

show_env_vars

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

# Separate PR-produced images from those produced on master.
if [[ "${CIRRUS_BRANCH:-}" == "master" ]]
then
    POST_MERGE_BUCKET_SUFFIX="-master"
else
    POST_MERGE_BUCKET_SUFFIX=""
fi

make libpod_images \
    PACKER_BUILDS=$PACKER_BUILDS \
    PACKER_VER=$PACKER_VER \
    GOSRC=$GOSRC \
    SCRIPT_BASE=$SCRIPT_BASE \
    PACKER_BASE=$PACKER_BASE \
    POST_MERGE_BUCKET_SUFFIX=$POST_MERGE_BUCKET_SUFFIX \
    BUILT_IMAGE_SUFFIX=$BUILT_IMAGE_SUFFIX

record_timestamp "cache-image build end"

# When successful, upload manifest of produced images using a filename unique
# to this build.
URI="gs://packer-import${POST_MERGE_BUCKET_SUFFIX}/manifest${BUILT_IMAGE_SUFFIX}.json"
gsutil cp packer-manifest.json "$URI"

echo "Finished."
echo "Any tarball URI's referenced above at at $URI"
echo "may be used to create VM images suitable for use in"
echo ".cirrus.yml as values for the 'image_name' keys."
