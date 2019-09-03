#!/bin/bash

set -e

source $(dirname $0)/lib.sh

req_env_var IN_PODMAN_IMAGE UPLDREL_IMAGE GATE_IMAGE TIMESTAMP_IMAGE PRUNE_IMAGE TEST_IMAGE_CACHE_DIRPATH

for fqin in $IN_PODMAN_IMAGE $GATE_IMAGE $TIMESTAMP_IMAGE $PRUNE_IMAGE $UPLDREL_IMAGE
do
    image_cache_file="$TEST_IMAGE_CACHE_DIRPATH/$(basename ${fqin%:*}).tar"
    [[ -r "$image_cache_file" ]] ||
        die 1 "Required cached container image missing: $image_cache_file"
    echo "Found $image_cache_file"
done

