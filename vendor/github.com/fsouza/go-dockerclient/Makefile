.PHONY: \
	all \
	staticcheck \
	fmt \
	fmtcheck \
	pretest \
	test \
	integration

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
	go mod download

pretest: staticcheck fmtcheck

gotest:
	go test -race -vet all ./...

test: testdeps pretest gotest

integration:
	go test -tags docker_integration -run TestIntegration -v
