#!/bin/bash

set -eo pipefail

source $(dirname $0)/lib.sh

req_env_var GOSRC CIRRUS_BUILD_ID IN_PODMAN_IMAGE UPLDREL_IMAGE GATE_IMAGE TIMESTAMP_IMAGE PRUNE_IMAGE TEST_IMAGE_CACHE_DIRPATH

[[ -n "$(type -P podman || false)" ]] || \
    die 1 "$(basename $0) requires a working podman"

cd $GOSRC

podman build -t ${IN_PODMAN_IMAGE%:*}:$CIRRUS_TASK_ID -f Dockerfile.fedora ./

# Cirrus-CI Artifacts storage-URL includes all components of source-path
mkdir -p $TEST_IMAGE_CACHE_DIRPATH

echo "Building test container images"
for name in $IN_PODMAN_IMAGE $GATE_IMAGE $TIMESTAMP_IMAGE $PRUNE_IMAGE $UPLDREL_IMAGE
do
    image_name="$(basename ${name%:*})"
    fqin="${name%:*}:$CIRRUS_BUILD_ID"  # avoid clobbering any existing
    echo -n "===== $fqin from "
    podman_build="podman build -t $fqin -f"
    case "$image_name" in
        in_podman)
            dockerfile="Dockerfile.fedora"
            ;;
        *)
            dockerfile="contrib/$image_name/Dockerfile"
    esac
    # Last echo had no \n
        echo "$dockerfile"
    $podman_build $dockerfile ./
    artifact=$TEST_IMAGE_CACHE_DIRPATH/${image_name}.tar
    echo "===== saving image to $artifact"
    podman save --output "$artifact"
    echo
done

# Cirrus-CI will cache /test_images for use by subsequent testing tasks
# ref: https://cirrus-ci.org/guide/writing-tasks/#cache-instruction
