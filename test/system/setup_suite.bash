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
    # Can't use $BATS_SUITE_TMPDIR because podman barfs:
    #    Error: the specified runroot is longer than 50 characters
    export PODMAN_LOGIN_WORKDIR=$(mktemp -d --tmpdir=${BATS_TMPDIR:-${TMPDIR:-/tmp}} podman-bats-registry.XXXXXX)

    export PODMAN_LOGIN_USER="user$(random_string 4)"
    export PODMAN_LOGIN_PASS="pw$(random_string 15)"

    # FIXME: racy! It could be many minutes between now and when we start it.
    # To mitigate, we use a range not used anywhere else in system tests.
    export PODMAN_LOGIN_REGISTRY_PORT=$(random_free_port 42000-42999)
}

# Run at the very end of all tests. Useful for cleanup of non-BATS tmpdirs.
function teardown_suite() {
    stop_registry
}
