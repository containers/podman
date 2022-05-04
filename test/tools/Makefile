GO := go
GO_BUILD=$(GO) build

BUILDDIR := build

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
	$(BUILDDIR)/git-validation

$(BUILDDIR)/goimports:
	$(GO_BUILD) -o $@ ./vendor/golang.org/x/tools/cmd/goimports

$(BUILDDIR)/go-md2man:
	$(GO_BUILD) -o $@ ./vendor/github.com/cpuguy83/go-md2man/v2

$(BUILDDIR)/git-validation:
	$(GO_BUILD) -o $@ ./vendor/github.com/vbatts/git-validation
