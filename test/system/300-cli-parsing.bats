#!/usr/bin/env bats   -*- bats -*-
#
# Various command-line parsing regression tests that don't fit in elsewhere
#

load helpers

@test "podman cli parsing - quoted args - #2574" {
    # 1.1.2 fails with:
    #   Error: invalid argument "true=\"false\"" for "-l, --label" \
    #      flag: parse error on line 1, column 5: bare " in non-quoted-field
    run_podman run --rm --label 'true="false"' $IMAGE true
}

# vim: filetype=sh
