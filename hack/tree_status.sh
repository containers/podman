#!/usr/bin/env bash
set -e

SUGGESTION="${SUGGESTION:-sync the vendor.conf and commit all changes.}"

STATUS=$(git status --porcelain)
if [[ -z $STATUS ]]
then
	echo "tree is clean"
else
	echo "tree is dirty, please $SUGGESTION"
	echo ""
	echo "$STATUS"
	exit 1
fi
