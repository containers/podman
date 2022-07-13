#!/bin/bash

set -eo pipefail

# Intended to be executed from a github action workflow step.
# Outputs the Cirrus cron names and IDs of any failed builds

err() {
    # Ref: https://docs.github.com/en/free-pro-team@latest/actions/reference/workflow-commands-for-github-actions
    echo "::error file=${BASH_SOURCE[0]},line=${BASH_LINENO[0]}::${1:-No error message given}"
    exit 1
}

_errfmt="Expecting %s value to not be empty"
if [[ -z "$GITHUB_REPOSITORY" ]]; then
    err $(printf "$_errfmt" "\$GITHUB_REPOSITORY")
elif [[ -z "$NAME_ID_FILEPATH" ]]; then
    err $(printf "$_errfmt" "\$NAME_ID_FILEPATH")
fi

mkdir -p artifacts
cat > ./artifacts/query_raw.json << "EOF"
{"query":"
  query CronNameStatus($owner: String!, $repo: String!) {
    ownerRepository(platform: \"LINUX\", owner: $owner, name: $repo) {
      cronSettings {
        name
        lastInvocationBuild {
          id
          status
        }
      }
    }
  }
",
"variables":"{
  \"owner\": \"@@OWNER@@\",
  \"repo\": \"@@REPO@@\"
}"}
EOF
# Makes for easier copy/pasting query to/from
# https://cirrus-ci.com/explorer
owner=$(cut -d '/' -f 1 <<<"$GITHUB_REPOSITORY")
repo=$(cut -d '/' -f 2 <<<"$GITHUB_REPOSITORY")
sed -i -r -e "s/@@OWNER@@/$owner/g" -e "s/@@REPO@@/$repo/g" ./artifacts/query_raw.json

echo "::group::Posting GraphQL Query"
# Easier to debug in error-reply when query is compacted
tr -d '\n' < ./artifacts/query_raw.json | tr -s ' ' | tee ./artifacts/query.json | \
    jq --indent 4 --color-output .

if grep -q '@@' ./artifacts/query.json; then
    err "Found unreplaced substitution token in raw query JSON"
fi
curl \
  --request POST \
  --silent \
  --location \
  --header 'content-type: application/json' \
  --url 'https://api.cirrus-ci.com/graphql' \
  --data @./artifacts/query.json \
  --output ./artifacts/reply.json
echo "::endgroup::"

echo "::group::Received GraphQL Reply"
jq --indent 4 --color-output . <./artifacts/reply.json || \
    cat ./artifacts/reply.json
echo "::endgroup::"

# e.x. reply.json
# {
#   "data": {
#     "ownerRepository": {
#       "cronSettings": [
#         {
#           "name": "Keepalive_v2.0",
#           "lastInvocationBuild": {
#             "id": "5776050544181248",
#             "status": "EXECUTING"
#           }
#         },
#         {
#           "name": "Keepalive_v1.9",
#           "lastInvocationBuild": {
#             "id": "5962921081569280",
#             "status": "COMPLETED"
#           }
#         },
#         {
#           "name": "Keepalive_v2.0.5-rhel",
#           "lastInvocationBuild": {
#             "id": "5003065549914112",
#             "status": "FAILED"
#           }
#         }
#       ]
#     }
#   }
# }

# This should never ever return an empty-list, unless there are no cirrus-cron
# jobs defined for the repository.  In that case, this monitoring script shouldn't
# be running anyway.
filt_head='.data.ownerRepository.cronSettings'
if ! jq -e "$filt_head" ./artifacts/reply.json &> /dev/null
then
    # Actual colorized JSON reply was printed above
    err "Null/empty result filtering reply with '$filt_head'"
fi

filt="$filt_head | map(select(.lastInvocationBuild.status==\"FAILED\") | { name:.name, id:.lastInvocationBuild.id} | join(\" \")) | join(\"\n\")"
jq --raw-output "$filt" ./artifacts/reply.json > "$NAME_ID_FILEPATH"

echo "<Cron Name> <Failed Build ID>"
cat "$NAME_ID_FILEPATH"

# Don't rely on a newline present for zero/one output line, always count words
records=$(wc --words "$NAME_ID_FILEPATH" | cut -d ' ' -f 1)
# Always two words per record
failures=$((records/2))
echo "::set-output name=failures::$failures"
echo "Total failed Cirrus-CI cron builds: $failures"
