#!/bin/bash

set -e

source /usr/local/bin/lib_entrypoint.sh

req_env_var GCPJSON_FILEPATH GCPNAME GCPPROJECT REL_ARC_FILEPATH PR_OR_BRANCH BUCKET

[[ -r "$REL_ARC_FILEPATH" ]] || \
    die 2 ERROR Cannot read release archive file: "$REL_ARC_FILEPATH"

[[ -r "$GCPJSON_FILEPATH" ]] || \
    die 3 ERROR Cannot read GCP credentials file: "$GCPJSON_FILEPATH"

cd $TMPDIR
echo "Attempting to extract release.txt from tar or zip $REL_ARC_FILEPATH"
unset SFX
if tar xzf "$REL_ARC_FILEPATH" "./release.txt"
then
    echo "It's a tarball"
    SFX="tar.gz"
elif unzip "$REL_ARC_FILEPATH" release.txt
then
    echo "It's a zip"
    SFX="zip"
else
    die 5 ERROR Could not extract release.txt from $REL_ARC_FILEPATH
fi

echo "Parsing release.txt contents"
RELEASETXT=$(<release.txt)
cd -
[[ -n "$RELEASETXT" ]] || \
    die 3 ERROR Could not obtain metadata from release.txt in $REL_ARC_FILEPATH

RELEASE_INFO=$(echo "$RELEASETXT" | grep -m 1 'X-RELEASE-INFO:' | sed -r -e 's/X-RELEASE-INFO:\s*(.+)/\1/')
if [[ "$?" -ne "0" ]] || [[ -z "$RELEASE_INFO" ]]
then
    die 4 ERROR Metadata is empty or invalid: '$RELEASETXT'
fi

# e.g. libpod v1.3.1-166-g60df124e fedora 29 amd64
# or   libpod v1.3.1-166-g60df124e   amd64
FIELDS="RELEASE_BASENAME RELEASE_VERSION RELEASE_DIST RELEASE_DIST_VER RELEASE_ARCH"
read $FIELDS <<< $RELEASE_INFO
for f in $FIELDS
do
    [[ -n "${!f}" ]] || \
        die 5 ERROR Expecting $f to be non-empty in metadata: '$RELEASE_INFO'
done

gcloud_init "$GCPJSON_FILEPATH"

# Drop version number to enable "latest" representation
# (version available w/in zip-file comment)
RELEASE_ARCHIVE_NAME="${RELEASE_BASENAME}-${PR_OR_BRANCH}-${RELEASE_DIST}-${RELEASE_DIST_VER}-${RELEASE_ARCH}.${SFX}"

echo "Uploading archive as $RELEASE_ARCHIVE_NAME"
gsutil cp "$REL_ARC_FILEPATH" "gs://$BUCKET/$RELEASE_ARCHIVE_NAME"

echo "Release now available at:"
echo "    https://storage.cloud.google.com/$BUCKET/$RELEASE_ARCHIVE_NAME"
