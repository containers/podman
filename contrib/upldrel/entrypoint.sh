#!/bin/bash

set -e

source /usr/local/bin/lib_entrypoint.sh

req_env_var GCPJSON_FILEPATH GCPNAME GCPPROJECT BUCKET FROM_FILEPATH TO_FILENAME ALSO_FILENAME

[[ -r "$FROM_FILEPATH" ]] || \
    die 2 ERROR Cannot read release archive file: "$REL_ARC_FILEPATH"

[[ -r "$GCPJSON_FILEPATH" ]] || \
    die 3 ERROR Cannot read GCP credentials file: "$GCPJSON_FILEPATH"

echo "Authenticating to google cloud for upload"
gcloud_init "$GCPJSON_FILEPATH"

echo "Uploading archive as $TO_FILENAME"
gsutil cp "$FROM_FILEPATH" "gs://$BUCKET/$TO_FILENAME"
gsutil cp "$FROM_FILEPATH" "gs://$BUCKET/$ALSO_FILENAME"

echo "."
echo "Release now available for download at:"
echo "    https://storage.cloud.google.com/$BUCKET/$TO_FILENAME"
echo "    https://storage.cloud.google.com/$BUCKET/$ALSO_FILENAME"
