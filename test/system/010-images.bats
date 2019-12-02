#!/usr/bin/env bats

load helpers

@test "podman images - basic output" {
    run_podman images -a

    is "${lines[0]}" "REPOSITORY *TAG *IMAGE ID *CREATED *SIZE" "header line"
    is "${lines[1]}" "$PODMAN_TEST_IMAGE_REGISTRY/$PODMAN_TEST_IMAGE_USER/$PODMAN_TEST_IMAGE_NAME *$PODMAN_TEST_IMAGE_TAG *[0-9a-f]\+" "podman images output"
}

@test "podman images - custom formats" {
    tests="
--format {{.ID}}                  |        [0-9a-f]\\\{12\\\}
--format {{.ID}} --no-trunc       | sha256:[0-9a-f]\\\{64\\\}
--format {{.Repository}}:{{.Tag}} | $PODMAN_TEST_IMAGE_FQN
"

    parse_table "$tests" | while read fmt expect; do
        run_podman images $fmt
        is "$output" "$expect\$" "podman images $fmt"
    done

}


@test "podman images - json" {
    # 'created': podman includes fractional seconds, podman-remote does not
    tests="
names[0]   | $PODMAN_TEST_IMAGE_FQN
id         |        [0-9a-f]\\\{64\\\}
digest     | sha256:[0-9a-f]\\\{64\\\}
created    | [0-9-]\\\+T[0-9:.]\\\+Z
size       | [0-9]\\\+
"

    run_podman images -a --format json

    parse_table "$tests" | while read field expect; do
        actual=$(echo "$output" | jq -r ".[0].$field")
        dprint "# actual=<$actual> expect=<$expect}>"
        is "$actual" "$expect" "jq .$field"
    done

}

@test "podman images - history output" {
    run_podman images --format json
    actual=$(echo $output | jq -r '.[0].history | length')
    is "$actual" "0"

    run_podman tag $PODMAN_TEST_IMAGE_REGISTRY/$PODMAN_TEST_IMAGE_USER/$PODMAN_TEST_IMAGE_NAME:$PODMAN_TEST_IMAGE_TAG test-image
    run_podman images --format json
    actual=$(echo $output | jq -r '.[1].history | length')
    is "$actual" "0"
    actual=$(echo $output | jq -r '.[0].history | length')
    is "$actual" "1"
    actual=$(echo $output | jq -r '.[0].history[0]')
    is "$actual" "$PODMAN_TEST_IMAGE_REGISTRY/$PODMAN_TEST_IMAGE_USER/$PODMAN_TEST_IMAGE_NAME:$PODMAN_TEST_IMAGE_TAG"
}

# vim: filetype=sh
