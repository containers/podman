GO := go
GO_BUILD=$(GO) build

BUILDDIR := build

SOURCES := $(shell find . -name '*.go')

all: $(BUILDDIR)

.PHONY: vendor
vendor:
	$(GO) mod tidy
	$(GO) mod vendor
	$(GO) mod verify

.PHONY: clean
clean:
	rm -rf $(BUILDDIR)

.PHONY: $(BUILDDIR)
$(BUILDDIR): \
	$(BUILDDIR)/goimports \
	$(BUILDDIR)/go-md2man \
	$(BUILDDIR)/git-validation \
	$(BUILDDIR)/ginkgo

$(BUILDDIR)/goimports: $(SOURCES)
	$(GO_BUILD) -o $@ ./vendor/golang.org/x/tools/cmd/goimports

$(BUILDDIR)/go-md2man: $(SOURCES)
	$(GO_BUILD) -o $@ ./vendor/github.com/cpuguy83/go-md2man/v2

$(BUILDDIR)/git-validation: $(SOURCES)
	$(GO_BUILD) -o $@ ./vendor/github.com/vbatts/git-validation

$(BUILDDIR)/ginkgo: $(SOURCES)
	$(GO_BUILD) -o $@ ./vendor/github.com/onsi/ginkgo/v2/ginkgo
