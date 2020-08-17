#!/usr/bin/env bash
BASE_PATH="/usr/libexec/podman"
CATATONIT_PATH="${BASE_PATH}/catatonit"
CATATONIT_VERSION="v0.1.4"
set -e

if [ -f $CATATONIT_PATH ]; then
	echo "skipping ... catatonit is already installed"
else
	echo "installing catatonit to $CATATONIT_PATH"
	buildDir=$(mktemp -d)
	git clone https://github.com/openSUSE/catatonit.git $buildDir

	pushd $buildDir
	echo `pwd`
	git reset --hard ${CATATONIT_VERSION}
	autoreconf -fi
	./configure
	make
	install ${SELINUXOPT} -d -m 755 $BASE_PATH
	install ${SELINUXOPT} -m 755 catatonit $CATATONIT_PATH
	popd

	rm -rf $buildDir
fi
