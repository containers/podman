#!/bin/bash

set -e

source /usr/local/bin/lib_entrypoint.sh

req_env_var GCPJSON GCPNAME GCPPROJECT IMGNAMES

for env in $(sed -ne 's/^.*BASE_IMAGE=/img=/p' contrib/cirrus/lib.sh);do
    eval $env
    BASE_IMAGES="$BASE_IMAGES $img"
done
# When executing under Cirrus-CI, have access to current source
if [[ "$CI" == "true" ]] && [[ -r "$CIRRUS_WORKING_DIR/$SCRIPT_BASE" ]]
then
    # Avoid importing anything that might conflict
    eval "$(egrep -sh '^export .+BASE_IMAGE=' < $CIRRUS_WORKING_DIR/$SCRIPT_BASE/lib.sh)"
    BASE_IMAGES="$UBUNTU_BASE_IMAGE $PRIOR_UBUNTU_BASE_IMAGE $FEDORA_BASE_IMAGE $PRIOR_FEDORA_BASE_IMAGE"
else
    # metadata labeling may have broken for some reason in the future
    echo "Warning: Running outside of Cirrus-CI, very minor-risk of base-image deletion."
fi

gcloud_init

# For safety's sake + limit nr background processes
PRUNE_LIMIT=5
THEFUTURE=$(date --date='+1 hour' +%s)
TOO_OLD='30 days ago'
THRESHOLD=$(date --date="$TOO_OLD" +%s)
# Format Ref: https://cloud.google.com/sdk/gcloud/reference/topic/formats
FORMAT='value[quote](name,selfLink,creationTimestamp,labels)'
PROJRE="/v1/projects/$GCPPROJECT/global/"
RECENTLY=$(date --date='3 days ago' --iso-8601=date)
# Filter Ref: https://cloud.google.com/sdk/gcloud/reference/topic/filters
FILTER="selfLink~$PROJRE AND creationTimestamp<$RECENTLY AND NOT name=($IMGNAMES $BASE_IMAGES)"
TODELETE=$(mktemp -p '' todelete.XXXXXX)
IMGCOUNT=$(mktemp -p '' imgcount.XXXXXX)

# Search-loop runs in a sub-process, must store count in file
echo "0" > "$IMGCOUNT"
count_image() {
    local count
    count=$(<"$IMGCOUNT")
    let 'count+=1'
    echo "$count" > "$IMGCOUNT"
}

echo "Using filter: $FILTER"
echo "Searching images for pruning candidates older than $TOO_OLD ($(date --date="$TOO_OLD" --iso-8601=date)):"
$GCLOUD compute images list --format="$FORMAT" --filter="$FILTER" | \
    while read name selfLink creationTimestamp labels
    do
        count_image
        created_ymd=$(date --date=$creationTimestamp --iso-8601=date)
        last_used=$(egrep --only-matching --max-count=1 'last-used=[[:digit:]]+' <<< $labels || true)
        markmsgpfx="Marking $name (created $created_ymd) for deletion"
        if [[ -z "$last_used" ]]
        then # image pre-dates addition of tracking labels
            echo "$markmsgpfx: Missing 'last-used' metadata, labels: '$labels'"
            echo "$name" >> $TODELETE
            continue
        fi

        last_used_timestamp=$(date --date=@$(cut -d= -f2 <<< $last_used || true) +%s || true)
        last_used_ymd=$(date --date=@$last_used_timestamp --iso-8601=date)
        if [[ -z "$last_used_timestamp" ]] || [[ "$last_used_timestamp" -ge "$THEFUTURE" ]]
        then
            echo "$markmsgpfx: Missing or invalid last-used timestamp: '$last_used_timestamp'"
            echo "$name" >> $TODELETE
            continue
        fi

        if [[ "$last_used_timestamp" -le "$THRESHOLD" ]]
        then
            echo "$markmsgpfx: Used over $TOO_OLD on $last_used_ymd"
            echo "$name" >> $TODELETE
            continue
        fi
    done

COUNT=$(<"$IMGCOUNT")
echo "########################################################################"
echo "Deleting up to $PRUNE_LIMIT images marked ($(wc -l < $TODELETE)) of all searched ($COUNT):"

# Require a minimum number of images to exist
NEED="$[$PRUNE_LIMIT*2]"
if [[ "$COUNT" -lt "$NEED" ]]
then
    die 0 Safety-net Insufficient images \($COUNT\) to process deletions \($NEED\)
    exit 0
fi

for image_name in $(sort --random-sort $TODELETE | tail -$PRUNE_LIMIT)
do
    if echo "$IMGNAMES $BASE_IMAGES" | grep -q "$image_name"
    then
        # double-verify in-use images were filtered out in search loop above
        die 8 FATAL ATTEMPT TO DELETE IN-USE IMAGE \'$image_name\' - THIS SHOULD NEVER HAPPEN
    fi
    echo "Deleting $image_name in parallel..."
    $GCLOUD compute images delete $image_name &
done

wait || true  # Nothing to delete: No background jobs
