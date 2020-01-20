#!/bin/bash

set -e

source /usr/local/bin/lib_entrypoint.sh

req_env_var GCPJSON GCPNAME GCPPROJECT IMGNAMES BUILDID REPOREF

gcloud_init

ARGS="
    --update-labels=last-used=$(date +%s)
    --update-labels=build-id=$BUILDID
    --update-labels=repo-ref=$REPOREF
    --update-labels=project=$GCPPROJECT
"

for image in $IMGNAMES; do
    $GCLOUD compute images update "$image" $ARGS &
done

wait || echo "Warning: No \$IMGNAMES were specified."
