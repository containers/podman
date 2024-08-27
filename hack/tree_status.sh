#!/usr/bin/env bash
set -e

sed -i -e 's/, "security.ima"//' \
    vendor/github.com/containers/buildah/copier/xattrs.go \
    vendor/github.com/containers/storage/pkg/archive/archive.go

SUGGESTION="${SUGGESTION:-run \"make vendor\" and commit all changes.}"

STATUS=$(git status --porcelain)
if [[ -z $STATUS ]]
then
	echo "tree is clean"
else
	echo "tree is dirty, please $SUGGESTION"
	echo ""
	echo "$STATUS"
	echo ""
	echo "---------------------- Diff below ----------------------"
	echo ""
	git --no-pager diff
	exit 0
fi
