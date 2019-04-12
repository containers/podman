#!/usr/bin/env bats

load helpers

@test "podman run - basic tests" {
    skip_if_remote

    rand=$(random_string 30)
    tests="
true              |   0 |
false             |   1 |
sh -c 'exit 32'   |  32 |
echo $rand        |   0 | $rand
/no/such/command  | 127 | Error: container create failed:.*exec:.* no such file or dir
/etc              | 126 | Error: container create failed:.*exec:.* permission denied
"

    while read cmd expected_rc expected_output; do
        if [ "$expected_output" = "''" ]; then expected_output=""; fi

        # THIS IS TRICKY: this is what lets us handle a quoted command.
        # Without this incantation (and the "$@" below), the cmd string
        # gets passed on as individual tokens: eg "sh" "-c" "'exit" "32'"
        # (note unmatched opening and closing single-quotes in the last 2).
        # That results in a bizarre and hard-to-understand failure
        # in the BATS 'run' invocation.
        # This should really be done inside parse_table; I can't find
        # a way to do so.
        eval set "$cmd"

        run_podman $expected_rc run $IMAGE "$@"
        is "$output" "$expected_output" "podman run $cmd - output"
    done < <(parse_table "$tests")
}

@test "podman run - uidmapping has no /sys/kernel mounts" {
      run_podman $expected_rc run --uidmapping 0:100:10000 $IMAGE mount | grep /sys/kernel
      is "$output" "" "podman run $cmd - output"

      run_podman $expected_rc run --net host --uidmapping 0:100:10000 $IMAGE mount | grep /sys/kernel
      is "$output" "" "podman run $cmd - output"
}

# vim: filetype=sh
