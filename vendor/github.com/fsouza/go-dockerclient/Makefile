.PHONY: \
	all \
	staticcheck \
	fmt \
	fmtcheck \
	pretest \
	test \
	integration

DEP_TOOL ?= mod

all: test

staticcheck:
	GO111MODULE=off go get honnef.co/go/tools/cmd/staticcheck
	staticcheck ./...

fmtcheck:
	if [ -z "$${SKIP_FMT_CHECK}" ]; then [ -z "$$(gofumpt -s -d . | tee /dev/stderr)" ]; fi

fmt:
	GO111MODULE=off go get mvdan.cc/gofumpt
	gofumpt -s -w .

testdeps:
ifeq ($(DEP_TOOL), dep)
	GO111MODULE=off go get -u github.com/golang/dep/cmd/dep
	dep ensure -v
else
	go mod download
endif

pretest: staticcheck fmtcheck

gotest:
	go test -race -vet all ./...

test: testdeps pretest gotest

integration:
	go test -tags docker_integration -run TestIntegration -v
