TAGS ?= seccomp
BUILDFLAGS := -tags "$(AUTOTAGS) $(TAGS)"
GO := go
PACKAGE := github.com/seccomp/containers-golang

sources := $(wildcard *.go)

default.json: $(sources)
	$(GO) build -compiler gc $(BUILDFLAGS) ./cmd/generate.go
	$(GO) build -compiler gc ./cmd/generate.go
	$(GO) run ${BUILDFLAGS} cmd/generate.go

all: default.json 

.PHONY: test-unit
test-unit:
	$(GO) test $(BUILDFLAGS) $(shell $(GO) list ./... | grep -v ^$(PACKAGE)/vendor)
	$(GO) test $(shell $(GO) list ./... | grep -v ^$(PACKAGE)/vendor)
