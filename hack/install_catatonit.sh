#!/bin/bash -e
BASE_PATH="/usr/libexec/podman"
CATATONIT_PATH="${BASE_PATH}/catatonit"
CATATONIT_VERSION="v0.1.3"

if [ -f $CATATONIT_PATH ]; then
	echo "skipping ... catatonit is already installed"
else
	echo "downloading catatonit to $CATATONIT_PATH"
	curl -o catatonit -L https://github.com/openSUSE/catatonit/releases/download/$CATATONIT_VERSION/catatonit.x86_64
	chmod +x catatonit
	install ${SELINUXOPT} -d -m 755 $BASE_PATH
	install ${SELINUXOPT} -m 755 catatonit $CATATONIT_PATH
	rm catatonit
fi
