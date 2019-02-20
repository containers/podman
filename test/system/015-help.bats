#!/usr/bin/env bats
#
# Tests based on 'podman help'
#
# Find all commands listed by 'podman --help'. Run each one, make sure it
# provides its own --help output. If the usage message ends in '[command]',
# treat it as a subcommand, and recurse into its own list of sub-subcommands.
#
# Any usage message that ends in '[flags]' is interpreted as a command
# that takes no further arguments; we confirm by running with 'invalid-arg'
# and confirming that it exits with error status and message.
#
load helpers

# run 'podman help', parse the output looking for 'Available Commands';
# return that list.
function podman_commands() {
    dprint "$@"
    run_podman help "$@" |\
        awk '/^Available Commands:/{ok=1;next}/^Flags:/{ok=0}ok { print $1 }' |\
        grep .
    "$output"
}


function check_help() {
    count=0
    for cmd in $(podman_commands "$@"); do
        dprint "podman $@ $cmd --help"
        run_podman "$@" $cmd --help

        # FIXME FIXME FIXME
        usage=$(echo "$output" | grep -A2 '^Usage:' | grep . | tail -1)
        #        dprint "$usage"
        [ -n "$usage" ] || die "podman $cmd: no Usage message found"

        # if ends in '[command]', recurse into subcommands
        if expr "$usage" : '.*\[command\]$' >/dev/null; then
            check_help "$@" $cmd
            continue
        fi

        # if ends in '[flag]' FIXME
        if expr "$usage" : '.*\[flags\]$' >/dev/null; then
            if [ "$cmd" != "help" ]; then
                run_podman 125 "$@" $cmd invalid-arg
                is "$output" "Error: .* takes no arguments" \
                   "'podman $@ $cmd' with extra (invalid) arguments"
            fi
        fi

        count=$(expr $count + 1)
    done

    [ $count -gt 0 ] || \
        die "Internal error: no commands found in 'podman help $@' list"
}


@test "podman help - basic tests" {
    check_help
}
