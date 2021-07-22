#!/usr/bin/env bats   -*- bats -*-
#
# tests for podman import
#

load helpers

@test "podman import" {
    local archive=$PODMAN_TMPDIR/archive.tar
    local random_content=$(random_string 12)
    # Generate a random name and tag (must be lower-case)
    local random_name=x0$(random_string 12 | tr A-Z a-z)
    local random_tag=t0$(random_string 7 | tr A-Z a-z)
    local fqin=localhost/$random_name:$random_tag

    run_podman run --name import $IMAGE sh -c "echo ${random_content} > /random.txt"
    run_podman export import -o $archive
    run_podman rm -f import

    # Simple import
    run_podman import -q $archive
    iid="$output"
    run_podman run -t --rm $iid cat /random.txt
    is "$output" "$random_content" "simple import"
    run_podman rmi -f $iid

    # Simple import via stdin
    run_podman import -q - < <(cat $archive)
    iid="$output"
    run_podman run -t --rm $iid cat /random.txt
    is "$output" "$random_content" "simple import via stdin"
    run_podman rmi -f $iid

    # Tagged import
    run_podman import -q $archive $fqin
    run_podman run -t --rm $fqin cat /random.txt
    is "$output" "$random_content" "tagged import"
    run_podman rmi -f $fqin

    # Tagged import via stdin
    run_podman import -q - $fqin < <(cat $archive)
    run_podman run -t --rm $fqin cat /random.txt
    is "$output" "$random_content" "tagged import via stdin"
    run_podman rmi -f $fqin
}
