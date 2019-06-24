build:
	go build ./cmd/imagebuilder
.PHONY: build

test:
	go test $(go list ./... | grep -v /vendor/)
.PHONY: test

test-conformance:
	go test -v -tags conformance -timeout 10m ./dockerclient
.PHONY: test-conformance
