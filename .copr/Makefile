#!/usr/bin/make -f
mkfile_path := $(abspath $(lastword $(MAKEFILE_LIST)))
current_dir := $(notdir $(patsubst %/,%,$(dir $(mkfile_path))))
outdir := $(CURDIR)

srpm:
	sh $(current_dir)/prepare.sh
	rpmbuild -bs -D "dist %{nil}" -D "_sourcedir build/" -D "_srcrpmdir $(outdir)" --nodeps contrib/spec/podman.spec
