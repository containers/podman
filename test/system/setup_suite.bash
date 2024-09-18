# -*- bash -*-
#
# global setup/teardown for the entire system test suite
#
bats_require_minimum_version 1.8.0

load helpers
load helpers.network
load helpers.registry

# Create common environment just in case we end up needing a registry.
# These environment variables will be available to all tests.
function setup_suite() {
    # FIXME: 2023-12-13: https://github.com/bats-core/bats-core/issues/812
    # Running 'bats --filter-tags' sets IFS=',' which ... ugh. Not fun to debug.
    # The line below is newline, space, tab.
    IFS="
 	"

    export PODMAN_LOGIN_WORKDIR="$BATS_SUITE_TMPDIR/podman-bats-registry"
    mkdir "$PODMAN_LOGIN_WORKDIR"

    export PODMAN_LOGIN_USER="user$(random_string 4)"
    export PODMAN_LOGIN_PASS="pw$(random_string 15)"

    # FIXME: racy! It could be many minutes between now and when we start it.
    # To mitigate, we use a range not used anywhere else in system tests.
    export PODMAN_LOGIN_REGISTRY_PORT=$(random_free_port 27000-27999)

    # The above does not handle errors. Do a final confirmation.
    assert "$PODMAN_LOGIN_REGISTRY_PORT" != "" \
           "Unable to set PODMAN_LOGIN_REGISTRY_PORT"

    clean_setup

    # Canary file. Will be removed if any individual test fails.
    touch "$BATS_SUITE_TMPDIR/all-tests-passed"

    # Track network namespaces, so we can check for leaks at test end
    check_netns_files > $BATS_SUITE_TMPDIR/netns-pre
}

# Run at the very end of all tests. Useful for cleanup of non-BATS tmpdirs.
function teardown_suite() {
    stop_registry
    local exit_code=$?

    # At end, if all tests have passed, check for leaks.
    # Don't do this if there were errors: failing tests may not clean up.
    if [[ -e "$BATS_SUITE_TMPDIR/all-tests-passed" ]]; then
        leak_check
        if [ $? -gt 0 ]; then
            exit_code=$((exit_code + 1))
        fi

        # Network namespace leak check. List should match what we saw above.
        # When they leak we indefinitely leak resources which is bad.
        echo
        check_netns_files > $BATS_SUITE_TMPDIR/netns-post
        if ! diff -u $BATS_SUITE_TMPDIR/netns-{pre,post}; then
            echo
            echo "^^^^^ Leaks found in $NETNS_DIR ^^^^^"
            exit_code=$((exit_code + 1))
        fi
    fi

    return $exit_code
}

NETNS_DIR=
# List a files in the common netns dir that is used to bind the netns files.
function check_netns_files() {
    if is_rootless; then
        NETNS_DIR=$XDG_RUNTIME_DIR/netns
    else
        NETNS_DIR=/run/netns
    fi

    # The dir may not exists which is fine
    if [ -d "$NETNS_DIR" ]; then
        ls -1 "$NETNS_DIR"
    fi
}
