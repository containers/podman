#!/usr/bin/env bats

load helpers

@test "podman run - basic tests" {
    rand=$(random_string 30)

    # 2019-09 Fedora 31 and rawhide (32) are switching from runc to crun
    # because of cgroups v2; crun emits different error messages.
    # Default to runc:
    err_no_such_cmd="Error: .*: starting container process caused .*exec:.*stat /no/such/command: no such file or directory"
    err_no_exec_dir="Error: .*: starting container process caused .*exec:.* permission denied"

    # ...but check the configured runtime engine, and switch to crun as needed
    run_podman info --format '{{ .host.OCIRuntime.path }}'
    if expr "$output" : ".*/crun"; then
        err_no_such_cmd="Error: executable file not found in \$PATH: No such file or directory: OCI runtime command not found error"
        err_no_exec_dir="Error: open executable: Operation not permitted: OCI runtime permission denied error"
    fi

    tests="
true              |   0 |
false             |   1 |
sh -c 'exit 32'   |  32 |
echo $rand        |   0 | $rand
/no/such/command  | 127 | $err_no_such_cmd
/etc              | 126 | $err_no_exec_dir
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
    skip_if_rootless "cannot umount as rootless"

    run_podman run --rm --uidmap 0:100:10000 $IMAGE mount
    run grep /sys/kernel <(echo "$output")
    is "$output" "" "unwanted /sys/kernel in 'mount' output"

    run_podman run --rm --net host --uidmap 0:100:10000 $IMAGE mount
    run grep /sys/kernel <(echo "$output")
    is "$output" "" "unwanted /sys/kernel in 'mount' output (with --net=host)"
}

# 'run --rm' goes through different code paths and may lose exit status.
# See https://github.com/containers/libpod/issues/3795
@test "podman run --rm" {

    run_podman 0 run --rm $IMAGE /bin/true
    run_podman 1 run --rm $IMAGE /bin/false

    # Believe it or not, 'sh -c' resulted in different behavior
    run_podman 0 run --rm $IMAGE sh -c /bin/true
    run_podman 1 run --rm $IMAGE sh -c /bin/false
}

# vim: filetype=sh
