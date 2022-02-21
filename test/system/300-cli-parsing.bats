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

@test "podman flag error" {
    local name="podman"
    if is_remote; then
        name="podman-remote"
    fi
    run_podman 125 run -h
    is "$output" "Error: flag needs an argument: 'h' in -h
See '$name run --help'" "expected error output"

    run_podman 125 bad --invalid
    is "$output" "Error: unknown flag: --invalid
See '$name --help'" "expected error output"
}

# vim: filetype=sh
