#!/bin/bash

set -e

source $(dirname $0)/lib.sh

# mIRC "escape" codes are the most standard, for a non-standard client-side interpretation.
ETX="$(echo -n -e '\x03')"
RED="${ETX}4"
NOR="$(echo -n -e '\x0f')"

if [[ "$CIRRUS_BRANCH" =~ "master" ]]
then
    BURL="https://cirrus-ci.com/build/$CIRRUS_BUILD_ID"
    ircmsg "${RED}[Action Recommended]: ${NOR}Post-merge testing ${RED}$CIRRUS_BRANCH failed${NOR} in $CIRRUS_TASK_NAME on $(OS_RELEASE_ID)-$(OS_RELEASE_VER): $BURL.  Please investigate, and re-run if appropriate."
fi

# This script assumed to be executed on failure
die 1 "Testing Failed"
