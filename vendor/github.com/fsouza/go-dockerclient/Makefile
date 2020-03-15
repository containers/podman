.PHONY: \
	all \
	lint \
	pretest \
	test \
	integration

all: test

lint:
	cd /tmp && GO111MODULE=on go get github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	golangci-lint run

pretest: lint

gotest:
	go test -race -vet all ./...

test: pretest gotest

integration:
	go test -tags docker_integration -run TestIntegration -v
