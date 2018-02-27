#!/usr/bin/make -f

spec := contrib/spec/podman.spec
outdir := $(CURDIR)
tmpdir := build
gitdir := $(PWD)/.git

rev := $(shell git rev-list HEAD | head -n1 | cut -c -7)
date := $(shell date +%Y%m%d.%H%M)

version := $(shell sed -n '/Version:/{s/.* //;p}' $(spec))
release := $(date).git.$(rev)

srpm: $(outdir)/podman-$(version)-$(release).src.rpm

$(tmpdir)/podman.spec: $(spec)
	@mkdir -p $(tmpdir)
	sed '/^Release:/s/\(: *\).*/\1$(release)%{?dist}/' $< >$@

$(tmpdir)/$(version).tar.gz: $(gitdir)/..
	@mkdir -p $(tmpdir)
	tar c --exclude-vcs --exclude-vcs-ignores -C $< --transform 's|^\.|podman-$(version)|' . | gzip -9 >$@

$(outdir)/podman-$(version)-$(release).src.rpm: $(tmpdir)/podman.spec $(tmpdir)/$(version).tar.gz
	@mkdir -p $(outdir)
	rpmbuild -D'_srcrpmdir $(outdir)' -D'_sourcedir $(tmpdir)' -bs $(tmpdir)/podman.spec

.PHONY: srpm
