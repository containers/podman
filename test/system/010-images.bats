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
    # podman history is persistent: it permanently alters our base image.
    # Create a dummy image here so we leave our setup as we found it.
    run_podman run --name my-container $IMAGE true
    run_podman commit my-container my-test-image

    run_podman images my-test-image --format '{{ .History }}'
    is "$output" "" "Image has empty history to begin with"

    # Generate two randomish tags; 'tr' because they must be all lower-case
    rand_name1="test-image-history-$(random_string 10 | tr A-Z a-z)"
    rand_name2="test-image-history-$(random_string 10 | tr A-Z a-z)"

    # Tag once, rmi, and make sure the tag name appears in history
    run_podman tag my-test-image $rand_name1
    run_podman rmi $rand_name1
    run_podman images my-test-image --format '{{ .History }}'
    is "$output" "localhost/${rand_name1}:latest" "image history after one tag"

    # Repeat with second tag. Now both tags should be in history
    run_podman tag my-test-image $rand_name2
    run_podman rmi $rand_name2
    run_podman images my-test-image --format '{{ .History }}'
    is "$output" "localhost/${rand_name2}:latest, localhost/${rand_name1}:latest" \
       "image history after two tags"

    run_podman rmi my-test-image
    run_podman rm my-container
}

# vim: filetype=sh
