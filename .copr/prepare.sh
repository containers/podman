#!/bin/sh -euf
set -x
OS_TEST=${OS_TEST:=0}

if [ ! -e /usr/bin/git ]; then
    dnf -y install git-core
fi

git fetch --unshallow || :

COMMIT=$(git rev-parse HEAD)
COMMIT_SHORT=$(git rev-parse --short=8 HEAD)
COMMIT_NUM=$(git rev-list HEAD --count)
COMMIT_DATE=$(date +%s)
BR="#BuildRequires: golang-bin"
NEWBR="BuildRequires: golang-bin"


sed "s,#COMMIT#,${COMMIT},;
     s,#SHORTCOMMIT#,${COMMIT_SHORT},;
     s,#COMMITNUM#,${COMMIT_NUM},;
     s,#COMMITDATE#,${COMMIT_DATE}," \
         contrib/spec/podman.spec.in > contrib/spec/podman.spec

if [ ${OS_TEST} -eq 0 ]; then
    sed -i "s/${BR}/${NEWBR}/g" contrib/spec/podman.spec
fi

mkdir build/
git archive --prefix "libpod-${COMMIT_SHORT}/" --format "tar.gz" HEAD -o "build/libpod-${COMMIT_SHORT}.tar.gz"
git clone https://github.com/kubernetes-incubator/cri-o
cd cri-o && git checkout 4cd5a7c60349be0678d9f1b0657683324c1a2726 && git archive --prefix "crio/" --format "tar.gz" HEAD -o "../build/crio.tar.gz"
