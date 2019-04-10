#!/usr/bin/env bats

load helpers

@test "podman info - basic test" {
    skip_if_remote

    run_podman info

    expected_keys="
BuildahVersion: *[0-9.]\\\+
Conmon:\\\s\\\+package:
Distribution:
OCIRuntime:\\\s\\\+package:
os:
rootless:
insecure registries:
store:
GraphDriverName:
GraphRoot:
GraphStatus:
ImageStore:\\\s\\\+number: 1
RunRoot:
"
    while read expect; do
        is "$output" ".*$expect" "output includes '$expect'"
    done < <(parse_table "$expected_keys")
}

@test "podman info - json" {
    skip_if_remote

    run_podman info --format=json

    expr_nvr="[a-z0-9-]\\\+-[a-z0-9.]\\\+-[a-z0-9]\\\+\."
    expr_path="/[a-z0-9\\\/.]\\\+\\\$"

    tests="
host.BuildahVersion       | [0-9.]
host.Conmon.package       | $expr_nvr
host.Conmon.path          | $expr_path
host.OCIRuntime.package   | $expr_nvr
host.OCIRuntime.path      | $expr_path
store.ConfigFile          | $expr_path
store.GraphDriverName     | [a-z0-9]\\\+\\\$
store.GraphRoot           | $expr_path
store.ImageStore.number   | 1
"

    parse_table "$tests" | while read field expect; do
        actual=$(echo "$output" | jq -r ".$field")
        dprint "# actual=<$actual> expect=<$expect>"
        is "$actual" "$expect" "jq .$field"
    done

}

# vim: filetype=sh
