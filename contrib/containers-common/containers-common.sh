#!/usr/bin/bash -e
#
# Script used for downloading man pages and config files from
# github.com/containers libraries (storage, common, image)
#
# Must be run from directory containing input specfile
#

die() {
    echo "$(basename $0): $*" >&2
    exit 1
}

branchversion() {
    gomod=$(git rev-parse --show-toplevel)/go.mod
    v=$(awk -v X=github.com/containers/$1 '$1 ~ X { print $2 }' <$gomod)
    hash=$(expr "$v" : "v.*-[0-9.]\+-\([0-9a-f]\+\)")
    if [[ -n "$hash" ]]; then
        v="$hash"
    fi
    echo "$v"
}


SPECFILE=containers-common.spec
if [[ ! -e $SPECFILE.in ]]; then
    die "Please run me from the same directory as $SPECFILE.in"
fi

declare -A moduleversion
for module in common image storage; do
    v=$(branchversion $module)
    if [[ -z "$v" ]]; then
        die "Could not find version for module '$v'"
    fi
    moduleversion[$module]=$v
done

builddir=containers-common-${moduleversion[common]}
mkdir -p $builddir

sed -e "s/COMMON_BRANCH/${moduleversion[common]}/g" \
    -e "s/IMAGE_BRANCH/${moduleversion[image]}/g"  \
    -e "s/STORAGE_BRANCH/${moduleversion[storage]}/g"  \
    <$SPECFILE.in >$builddir/$SPECFILE

cd $builddir
spectool -fg $SPECFILE

if [[ ! -e storage.conf ]]; then
    die "spectool did not pull storage.conf"
fi

echo "Changing storage.conf..."
sed -i -e 's/^driver.*=.*/driver = "overlay"/' -e 's/^mountopt.*=.*/mountopt = "nodev,metacopy=on"/' \
        storage.conf
