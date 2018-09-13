#!/usr/bin/make -f
mkfile_path := $(abspath $(lastword $(MAKEFILE_LIST)))
current_dir := $(notdir $(patsubst %/,%,$(dir $(mkfile_path))))
outdir := $(CURDIR)
topdir := $(CURDIR)/rpmbuild
SHORT_COMMIT ?= $(shell git rev-parse --short=8 HEAD)

srpm:
	mkdir -p $(topdir)
	sh $(current_dir)/prepare.sh
	rpmbuild -bs -D "dist %{nil}" -D "_sourcedir build/" -D "_srcrpmdir $(outdir)" -D "_topdir $(topdir)" --nodeps contrib/spec/podman.spec

build_binary:
	mkdir -p $(topdir)
	rpmbuild --rebuild -D "_rpmdir $(outdir)" -D "_topdir $(topdir)" $(outdir)/podman-*.git$(SHORT_COMMIT).src.rpm

clean:
	rm -fr rpms
	rm -fr cri-o
