#!/bin/bash

if [ -z "$VERSION" ]; then
	echo \$VERSION is empty
	exit 1
fi

if [ -z "$GOBIN" ]; then
	echo \$GOBIN is empty
	exit 1
fi

$GOBIN/golangci-lint --version | grep $VERSION
if [ $?  -ne 0 ]; then
	set -e
	curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | sh -s -- -b $GOBIN v$VERSION
fi
