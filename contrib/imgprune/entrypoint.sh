#!/bin/bash

set -e

source /usr/local/bin/lib_entrypoint.sh

req_env_var GCPJSON GCPNAME GCPPROJECT IMGNAMES

gcloud_init

# For safety's sake + limit nr background processes
PRUNE_LIMIT=10
THEFUTURE=$(date --date='+1 hour' +%s)
TOO_OLD='90 days ago'
THRESHOLD=$(date --date="$TOO_OLD" +%s)
# Format Ref: https://cloud.google.com/sdk/gcloud/reference/topic/formats
FORMAT='value[quote](name,selfLink,creationTimestamp,labels)'
PROJRE="/v1/projects/$GCPPROJECT/global/"
BASE_IMAGE_RE='cloud-base'
RECENTLY=$(date --date='30 days ago' --iso-8601=date)
EXCLUDE="$IMGNAMES $IMAGE_BUILDER_CACHE_IMAGE_NAME" # whitespace separated values
# Filter Ref: https://cloud.google.com/sdk/gcloud/reference/topic/filters
FILTER="selfLink~$PROJRE AND creationTimestamp<$RECENTLY AND NOT name=($EXCLUDE)"
TODELETE=$(mktemp -p '' todelete.XXXXXX)

echo "Searching images for pruning candidates older than $TOO_OLD ($THRESHOLD):"
$GCLOUD compute images list --format="$FORMAT" --filter="$FILTER" | \
    while read name selfLink creationTimestamp labels
    do
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

        echo "NOT $markmsgpfx: last used on $last_used_ymd)"
    done

echo "Pruning up to $PRUNE_LIMIT images that were marked for deletion:"
for image_name in $(tail -$PRUNE_LIMIT $TODELETE | sort --random-sort)
do
    # This can take quite some time (minutes), run in parallel disconnected from terminal
    echo "TODO: Would have: $GCLOUD compute images delete $image_name &"
    sleep "$[1+RANDOM/1000]s" &  # Simlate background operation
done

wait || true  # Nothing to delete: No background jobs
