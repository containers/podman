#!/usr/bin/env bats
#
# Tests for podman preexec hooks
#

load helpers
load helpers.network

function setup() {
    basic_setup
}

function teardown() {
    basic_teardown
}

@test "podman preexec hook" {
    preexec_hook_dir=$PODMAN_TMPDIR/auth
    mkdir -p $preexec_hook_dir
    preexec_hook_script=$preexec_hook_dir/pull_check.sh

    cat > $preexec_hook_script <<EOF
#!/bin/sh
if echo \$@ | grep "pull foobar"; then
    exit 42
fi
exit 43
EOF
    chmod +x $preexec_hook_script

    PODMAN_PREEXEC_HOOKS_DIR=$preexec_hook_dir run_podman 42 pull foobar
    PODMAN_PREEXEC_HOOKS_DIR=$preexec_hook_dir run_podman 43 pull barfoo
}
