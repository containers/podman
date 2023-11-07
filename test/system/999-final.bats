#!/usr/bin/env bats
#
# Final set of tests to run.
#

load helpers

# Confirm that we're still using the same database we started with.
#
# This should never fail! If it does, it means that some test somewhere
# has run podman with --db-backend, which is known to wreak havoc.
#
# See  https://github.com/containers/podman/issues/20563
@test "podman database backend has not changed" {
    # File is always written in 005-info.bats. It must always exist
    # by the time we get here...
    db_backend_file=$BATS_SUITE_TMPDIR/db-backend

    if [[ ! -e "$db_backend_file" ]]; then
        # ...except in a manual run like "hack/bats 999"
        if [[ $BATS_SUITE_TEST_NUMBER -le 5 ]]; then
            skip "$db_backend_file missing, but this is a short manual bats run, so, ok"
        fi

        die "Internal error: $db_backend_file does not exist! (check 005-*.bats)"
    fi

    run_podman info --format '{{.Host.DatabaseBackend}}'
    assert "$output" = "$(<$db_backend_file)" ".Host.DatabaseBackend has changed!"
}
