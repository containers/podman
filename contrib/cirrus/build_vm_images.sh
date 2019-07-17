#!/bin/bash

set -e
source $(dirname $0)/lib.sh

BASE_IMAGE_VARS='FEDORA_BASE_IMAGE PRIOR_FEDORA_BASE_IMAGE UBUNTU_BASE_IMAGE'
ENV_VARS="PACKER_BUILDS BUILT_IMAGE_SUFFIX $BASE_IMAGE_VARS SERVICE_ACCOUNT GCE_SSH_USERNAME GCP_PROJECT_ID PACKER_VER SCRIPT_BASE PACKER_BASE CIRRUS_BUILD_ID CIRRUS_CHANGE_IN_REPO"
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
# Add/update labels on base-images used in this build to prevent premature deletion
ARGS="
"
for base_image_var in $BASE_IMAGE_VARS
do
    # See entrypoint.sh in contrib/imgts and contrib/imgprune
    # These updates can take a while, run them in the background, check later
    gcloud compute images update "$image" \
        --update-labels=last-used=$(date +%s) \
        --update-labels=build-id=$CIRRUS_BUILD_ID \
        --update-labels=repo-ref=$CIRRUS_CHANGE_IN_REPO \
        --update-labels=project=$GCP_PROJECT_ID \
        ${!base_image_var} &
done

make libpod_images \
    PACKER_BUILDS=$PACKER_BUILDS \
    PACKER_VER=$PACKER_VER \
    GOSRC=$GOSRC \
    SCRIPT_BASE=$SCRIPT_BASE \
    PACKER_BASE=$PACKER_BASE \
    BUILT_IMAGE_SUFFIX=$BUILT_IMAGE_SUFFIX

# Separate PR-produced images from those produced on master.
if [[ "${CIRRUS_BRANCH:-}" == "master" ]]
then
    POST_MERGE_BUCKET_SUFFIX="-master"
else
    POST_MERGE_BUCKET_SUFFIX=""
fi

# When successful, upload manifest of produced images using a filename unique
# to this build.
URI="gs://packer-import${POST_MERGE_BUCKET_SUFFIX}/manifest${BUILT_IMAGE_SUFFIX}.json"
gsutil cp packer-manifest.json "$URI"

# Ensure any background 'gcloud compute images update' processes finish
set +e  # need 'wait' exit code to avoid race
while [[ -n "$(jobs)" ]]
do
    wait -n
    RET=$?
    if [[ "$RET" -eq "127" ]] || \   # Avoid TOCTOU race w/ jobs + wait
       [[ "$RET" -eq "0" ]]
    then
        continue
    fi
    die $RET "Required base-image metadata update failed"
done

echo "Finished. A JSON manifest of produced images is available at $URI"
