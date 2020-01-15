#!/bin/bash -e
$GOBIN/golangci-lint --version | grep $VERSION
if [ $?  -ne 0 ]; then
	curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | sh -s -- -b $GOBIN v$VERSION
fi
