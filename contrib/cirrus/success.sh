#!/bin/bash

set -e

source $(dirname $0)/lib.sh

req_env_var "
    CIRRUS_BRANCH $CIRRUS_BRANCH
    CIRRUS_BUILD_ID $CIRRUS_BUILD_ID
"

REF=$(basename $CIRRUS_BRANCH)  # PR number or branch named
URL="https://cirrus-ci.com/build/$CIRRUS_BUILD_ID"

if [[ "$CIRRUS_BRANCH" =~ "pull" ]]
then
    ircmsg "Cirrus-CI testing successful for PR #$REF: $URL"
else
    ircmsg "Cirrus-CI testing branch $REF successful: $URL"
fi
