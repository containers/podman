.PHONY: \
	all \
	lint \
	pretest \
	test \
	integration


ifeq "$(strip $(shell go env GOARCH))" "amd64"
RACE_FLAG := -race
endif

all: test

lint:
	cd /tmp && GO111MODULE=on go get github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	golangci-lint run

pretest: lint

gotest:
	go test $(RACE_FLAG) -vet all ./...

test: pretest gotest

integration:
	go test -tags docker_integration -run TestIntegration -v
