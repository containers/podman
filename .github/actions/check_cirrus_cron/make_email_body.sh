#!/bin/bash

set -eo pipefail

# Intended to be executed from a github action workflow step.
# Input: File listing space separated failed cron build names and IDs
# Output: $GITHUB_WORKSPACE/artifacts/email_body.txt file

source $(dirname "${BASH_SOURCE[0]}")/lib.sh

_errfmt="Expecting %s value to not be empty"
# NAME_ID_FILEPATH is defined by workflow YAML
# shellcheck disable=SC2154
if [[ -z "$GITHUB_REPOSITORY" ]]; then
    err $(printf "$_errfmt" "\$GITHUB_REPOSITORY")
elif [[ ! -r "$NAME_ID_FILEPATH" ]]; then
    err "Expecting \$NAME_ID_FILEPATH value ($NAME_ID_FILEPATH) to be a readable file"
fi

confirm_gha_environment

mkdir -p artifacts
(
    echo "Detected one or more Cirrus-CI cron-triggered jobs have failed recently:"
    echo ""

    while read -r NAME BID; do
        echo "Cron build '$NAME' Failed: https://cirrus-ci.com/build/$BID"
    done < "$NAME_ID_FILEPATH"

    echo ""
    # Defined by github-actions
    # shellcheck disable=SC2154
    echo "# Source: ${GITHUB_WORKFLOW} workflow on ${GITHUB_REPOSITORY}."
    # Separate content from sendgrid.com automatic footer.
    echo ""
    echo ""
) > ./artifacts/email_body.txt
