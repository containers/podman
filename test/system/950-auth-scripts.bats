#!/usr/bin/env bats
#
# Tests for podman auth scripts
#

load helpers
load helpers.network

function setup() {
    basic_setup
}

function teardown() {
    basic_teardown
}

@test "podman auth script" {
    auth_dir=$PODMAN_TMPDIR/auth
    mkdir -p $auth_dir
    auth_script=$auth_dir/pull_check.sh

    cat > $auth_script <<EOF
#!/bin/sh
if echo \$@ | grep "pull foobar"; then
    exit 42
fi
exit 43
EOF
    chmod +x $auth_script

    PODMAN_AUTH_SCRIPTS_DIR=$auth_dir run_podman 42 pull foobar
    PODMAN_AUTH_SCRIPTS_DIR=$auth_dir run_podman 43 pull barfoo
}
