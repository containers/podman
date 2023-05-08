SHELL := /bin/bash
RAGEL := ragel
GOFMT := go fmt

export GO_TEST=env GOTRACEBACK=all go test $(GO_ARGS)

.PHONY: build
build: machine.go

.PHONY: clean
clean:
	@rm -rf docs
	@rm -f machine.go

.PHONY: images
images: docs/urn.png

.PHONY: removecomments
removecomments:
	@cd ./tools/removecomments; go build -o ../../removecomments ./main.go

machine.go: machine.go.rl

machine.go: removecomments

machine.go:
	$(RAGEL) -Z -G2 -e -o $@ $<
	@./removecomments $@
	$(MAKE) -s file=$@ snake2camel
	$(GOFMT) $@

docs/urn.dot: machine.go.rl
	@mkdir -p docs
	$(RAGEL) -Z -e -Vp $< -o $@

docs/urn.png: docs/urn.dot
	dot $< -Tpng -o $@

.PHONY: bench
bench: *_test.go machine.go
	go test -bench=. -benchmem -benchtime=5s ./...

.PHONY: tests
tests: *_test.go 
	$(GO_TEST) ./...

.PHONY: snake2camel
snake2camel:
	@awk -i inplace '{ \
	while ( match($$0, /(.*)([a-z]+[0-9]*)_([a-zA-Z0-9])(.*)/, cap) ) \
	$$0 = cap[1] cap[2] toupper(cap[3]) cap[4]; \
	print \
	}' $(file)