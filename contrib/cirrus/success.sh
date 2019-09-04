#!/bin/bash

set -e

source $(dirname $0)/lib.sh

req_env_var CIRRUS_BRANCH CIRRUS_BUILD_ID CIRRUS_REPO_FULL_NAME CIRRUS_BASE_SHA CIRRUS_CHANGE_IN_REPO

cd $CIRRUS_WORKING_DIR

if [[ "$CIRRUS_BRANCH" =~ "pull" ]]
then
    echo "Retrieving latest HEADS and tags"
    git fetch --all --tags
    echo "Finding commit authors for PR $CIRRUS_PR"
    unset NICKS
    if [[ -r "$AUTHOR_NICKS_FILEPATH" ]]
    then
        SHARANGE="${CIRRUS_BASE_SHA}..${CIRRUS_CHANGE_IN_REPO}"
        EXCLUDE_RE='merge-robot'
        AUTHOR_NICKS=$(egrep -v '(^[[:space:]]*$)|(^[[:space:]]*#)' "$AUTHOR_NICKS_FILEPATH" | sort -u)
        # Depending on branch-state, it's possible SHARANGE could be _WAY_ too big
        MAX_NICKS=10
        # newline separated
        GITLOG="git log --format='%ae'"
        COMMIT_AUTHORS=$($GITLOGt $SHARANGE || $GITLOG -1 HEAD | \
                         sort -u | \
                         egrep -v "$EXCLUDE_RE" | \
                         tail -$MAX_NICKS)

        for c_email in $COMMIT_AUTHORS
        do
            echo -e "\tExamining $c_email"
            NICK=$(echo "$AUTHOR_NICKS" | grep -m 1 "$c_email" | \
                   awk --field-separator ',' '{print $2}' | tr -d '[[:blank:]]')
            if [[ -n "$NICK" ]]
            then
                echo -e "\t\tFound $c_email -> $NICK in $(basename $AUTHOR_NICKS_FILEPATH)"
            else
                echo -e "\t\tNot found in $(basename $AUTHOR_NICKS_FILEPATH), using e-mail username."
                NICK=$(echo "$c_email" | cut -d '@' -f 1)
            fi
            echo -e "\tUsing nick $NICK"
            NICKS="${NICKS:+$NICKS, }$NICK"
        done
    fi

    unset MENTION_PREFIX
    [[ -z "$NICKS" ]] || \
        MENTION_PREFIX="$NICKS: "

    URL="https://github.com/$CIRRUS_REPO_FULL_NAME/pull/$CIRRUS_PR"
    PR_SUBJECT=$(echo "$CIRRUS_CHANGE_MESSAGE" | head -1)
    ircmsg "${MENTION_PREFIX}Cirrus-CI testing successful for PR '$PR_SUBJECT': $URL"
else
    URL="https://cirrus-ci.com/github/containers/libpod/$CIRRUS_BRANCH"
    ircmsg "Cirrus-CI testing branch $(basename $CIRRUS_BRANCH) successful: $URL"
fi
