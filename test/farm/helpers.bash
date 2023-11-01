# -*- bash -*-

load ../system/helpers.bash

function setup(){
    basic_setup

    # Always create the same containerfile
    cat >$PODMAN_TMPDIR/Containerfile <<EOF
FROM $IMAGE
RUN arch | tee /arch.txt
RUN date | tee /built.txt
EOF
}

function teardown(){
    basic_teardown
}
