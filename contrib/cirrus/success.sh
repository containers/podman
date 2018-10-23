#!/bin/bash

set -e
source $(dirname $0)/lib.sh

req_env_var "
    CIRRUS_TASK_NAME $CIRRUS_TASK_NAME
    CIRRUS_BRANCH $CIRRUS_BRANCH
    OS_RELEASE_ID $OS_RELEASE_ID
    OS_RELEASE_VER $OS_RELEASE_VER
    CIRRUS_REPO_CLONE_URL $CIRRUS_REPO_CLONE_URL
"

REF_URL="$(echo $CIRRUS_REPO_CLONE_URL | sed 's/.git$//g')"
if [[ "$CIRRUS_BRANCH" =~ "pull" ]]
then
    REF_URL="$REF_URL/$CIRRUS_BRANCH"  # pull request URL
else
    REF_URL="$REF_URL/commits/$CIRRUS_BRANCH"  # branch merge
fi

ircmsg "Cirrus-CI $CIRRUS_TASK_NAME on $OS_RELEASE_ID-$OS_RELEASE_VER successful for $REF_URL"
