#!/usr/bin/env bats   -*- bats -*-
#
# All tests in here perform nasty manipulations on image storage.
#

load helpers

###############################################################################
# BEGIN setup/teardown

# Create a scratch directory; this is what we'll use for image store and cache
if [ -z "${PODMAN_CORRUPT_TEST_WORKDIR}" ]; then
    export PODMAN_CORRUPT_TEST_WORKDIR=$(mktemp -d --tmpdir=${BATS_TMPDIR:-${TMPDIR:-/tmp}} podman_corrupt_test.XXXXXX)
fi

# All tests in this file (and ONLY in this file) run with a custom rootdir
function setup() {
    skip_if_remote "none of these tests run under podman-remote"
    _PODMAN_TEST_OPTS="--storage-driver=vfs --root ${PODMAN_CORRUPT_TEST_WORKDIR}/root"
}

function teardown() {
    # No other tests should ever run with this custom rootdir
    unset _PODMAN_TEST_OPTS

    is_remote && return

    # Clean up
    umount ${PODMAN_CORRUPT_TEST_WORKDIR}/root/overlay || true
    if is_rootless; then
        run_podman unshare rm -rf ${PODMAN_CORRUPT_TEST_WORKDIR}/root
    else
        rm -rf ${PODMAN_CORRUPT_TEST_WORKDIR}/root
    fi
}

# END   setup/teardown
###############################################################################
# BEGIN primary test helper

# This is our main action, invoked by every actual test. It:
#    - creates a new empty rootdir
#    - populates it with our crafted test image
#    - removes [ manifest, blob ]
#    - confirms that "podman images" throws an error
#    - runs the specified command (rmi -a -f, prune, reset, etc)
#    - confirms that it succeeds, and also emits expected warnings
function _corrupt_image_test() {
    run_podman info --format "{{.Store.GraphRoot}}"
    indexPath=$output/*-layers/layers.json

    run_podman pull $IMAGE
    run_podman image exists $IMAGE

    indexTmp="$(realpath $indexPath).tmp.$$"
    jq 'del(.[0])' $indexPath > $indexTmp
    mv -f $indexTmp $indexPath
    # A corrupted image won't be marked as existing.
    run_podman 1 image exists $IMAGE

    # Run the requested command. Confirm it succeeds, with suitable warnings
    run_podman $*
    is "$output" ".*Image $IMAGE exists in local storage but may be corrupted: layer not known.*"
    run_podman rmi -f $IMAGE
}

# END   primary test helper
###############################################################################
# BEGIN actual tests

@test "podman corrupt images - run --rm" {
    _corrupt_image_test "run --rm $IMAGE ls"
}

@test "podman corrupt images - create" {
    _corrupt_image_test "create $IMAGE ls"
}

# END   actual tests
###############################################################################
# BEGIN final cleanup

@test "podman corrupt images - cleanup" {
    rm -rf ${PODMAN_CORRUPT_TEST_WORKDIR}
}

# END   final cleanup
###############################################################################

# vim: filetype=sh
