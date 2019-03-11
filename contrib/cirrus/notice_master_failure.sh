#!/bin/bash

set -e

source $(dirname $0)/lib.sh

ETX="$(echo -n -e '\x03')"
RED="${ETX}4"
NOR="$(echo -n -e '\x0f')"

[[ -n "$1" ]] || die 46 "Required command and arguments to wrap"

if [[ "$CIRRUS_BRANCH" =~ "master" ]]
then
    # TURL="https://cirrus-ci.com/task/$CIRRUS_TASK_ID"
    BURL="https://cirrus-ci.com/build/$CIRRUS_BUILD_ID"
    echo "Monitoring execution of $1 and notifying on failure"
    set +e
    "$@"
    RET=$?
    MSG="Action Required: Post-merge testing on $(os_release_id) $(os_release_ver), $(basename $1) failed (exit $RET): $BURL"
    ((RET)) && echo "$MSG"
    ((RET)) && ircmsg "${RED}$MSG"
    exit $RET
else
    "$@"
fi
