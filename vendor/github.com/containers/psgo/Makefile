export GO111MODULE=off
export GOPROXY=https://proxy.golang.org

SHELL= /bin/bash
GO ?= go
BUILD_DIR := ./bin
BIN_DIR := /usr/local/bin
NAME := psgo
PROJECT := github.com/containers/psgo
BATS_TESTS := *.bats
GO_SRC=$(shell find . -name \*.go)

GO_BUILD=$(GO) build
# Go module support: set `-mod=vendor` to use the vendored sources
ifeq ($(shell go help mod >/dev/null 2>&1 && echo true), true)
	GO_BUILD=GO111MODULE=on $(GO) build -mod=vendor
endif

all: validate build

.PHONY: build
build: $(GO_SRC)
	 $(GO_BUILD) -buildmode=pie -o $(BUILD_DIR)/$(NAME) $(PROJECT)/sample

.PHONY: clean
clean:
	rm -rf $(BUILD_DIR)

.PHONY: vendor
vendor:
	GO111MODULE=on go mod tidy
	GO111MODULE=on go mod vendor
	GO111MODULE=on go mod verify

.PHONY: validate
validate: .install.lint
	@which gofmt >/dev/null 2>/dev/null || (echo "ERROR: gofmt not found." && false)
	test -z "$$(gofmt -s -l . | grep -vE 'vendor/' | tee /dev/stderr)"
	@which golangci-lint >/dev/null 2>/dev/null|| (echo "ERROR: golangci-lint not found." && false)
	test -z "$$(golangci-lint run)"
	@go doc cmd/vet >/dev/null 2>/dev/null|| (echo "ERROR: go vet not found." && false)
	test -z "$$($(GO) vet $$($(GO) list $(PROJECT)/...) 2>&1 | tee /dev/stderr)"

.PHONY: test
test: test-unit test-integration

.PHONY: test-integration
test-integration:
	bats test/$(BATS_TESTS)

.PHONY: test-unit
test-unit:
	go test -v $(PROJECT)
	go test -v $(PROJECT)/internal/...

.PHONY: install
install:
	sudo install -D -m755 $(BUILD_DIR)/$(NAME) $(BIN_DIR)

.PHONY: .install.lint
.install.lint:
	# Workaround for https://github.com/golangci/golangci-lint/issues/523
	go get -u github.com/golangci/golangci-lint/cmd/golangci-lint

.PHONY: uninstall
uninstall:
	sudo rm $(BIN_DIR)/$(NAME)
