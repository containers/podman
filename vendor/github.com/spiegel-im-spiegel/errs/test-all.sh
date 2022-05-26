#!/bin/bash
go mod verify || exit 1
go mod tidy -v || exit 1
go list -json -m all | nancy sleuth -n || exit 1
golangci-lint run ./... || exit 1
go test ./...
