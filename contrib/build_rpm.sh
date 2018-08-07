#!/bin/bash
set -x
dnf -y install device-mapper-devel \
		git \
		glib2-devel \
		glibc-static \
		golang \
		golang-github-cpuguy83-go-md2man \
		gpgme-devel \
		libassuan-devel \
        libseccomp-devel \
        libselinux-devel \
        make \
        ostree-devel \
        golang-github-cpuguy83-go-md2man \
        rpm-build \
        btrfs-progs-devel \
        python3-devel \
        python3-varlink \
        go-compilers-golang-compiler

make -f .copr/Makefile
rpmbuild --rebuild podman-*.src.rpm
