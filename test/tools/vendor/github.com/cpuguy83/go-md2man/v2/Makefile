GO111MODULE ?= on
LINTER_BIN ?= golangci-lint

export GO111MODULE

.PHONY:
build: bin/go-md2man

.PHONY: clean
clean:
	@rm -rf bin/*

.PHONY: test
test:
	@go test $(TEST_FLAGS) ./...

bin/go-md2man: actual_build_flags := $(BUILD_FLAGS) -o bin/go-md2man
bin/go-md2man: bin
	@CGO_ENABLED=0 go build $(actual_build_flags)

bin:
	@mkdir ./bin

.PHONY: mod
mod:
	@go mod tidy

.PHONY: check-mod
check-mod: # verifies that module changes for go.mod and go.sum are checked in
	@hack/ci/check_mods.sh

.PHONY: vendor
vendor: mod
	@go mod vendor -v

