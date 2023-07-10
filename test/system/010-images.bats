#!/usr/bin/env bats

load helpers

@test "podman images - basic output" {
    headings="REPOSITORY *TAG *IMAGE ID *CREATED *SIZE"

    run_podman images -a
    is "${lines[0]}" "$headings" "header line"
    is "${lines[1]}" "$PODMAN_TEST_IMAGE_REGISTRY/$PODMAN_TEST_IMAGE_USER/$PODMAN_TEST_IMAGE_NAME *$PODMAN_TEST_IMAGE_TAG *[0-9a-f]\+" "podman images output"

    # 'podman images' should emit headings even if there are no images
    # (but --root only works locally)
    if ! is_remote; then
        run_podman --storage-driver=vfs --root ${PODMAN_TMPDIR}/nothing-here-move-along images
        is "$output" "$headings" "'podman images' emits headings even w/o images"
    fi
}

@test "podman images - custom formats" {
    tests="
{{.ID}}                  |        [0-9a-f]\\\{12\\\}\\\$
{{.ID| upper}}           |        [0-9A-F]\\\{12\\\}\\\$
{{.Repository}}:{{.Tag}} | $PODMAN_TEST_IMAGE_FQN
{{.Labels.created_by}}   | test/system/build-testimage
{{.Labels.created_at}}   | 20[0-9-]\\\+T[0-9:]\\\+Z
"

    parse_table "$tests" | while read fmt expect; do
        run_podman images --format "$fmt"
        is "$output" "$expect" "podman images --format '$fmt'"
    done

    run_podman images --format "{{.ID}}" --no-trunc
    is "$output" "sha256:[0-9a-f]\\{64\\}\$" "podman images --no-trunc"
}

@test "podman images - json" {
    # 'created': podman includes fractional seconds, podman-remote does not
    tests="
Names[0]          | $PODMAN_TEST_IMAGE_FQN
Id                |        [0-9a-f]\\\{64\\\}
Digest            | sha256:[0-9a-f]\\\{64\\\}
CreatedAt         | [0-9-]\\\+T[0-9:.]\\\+Z
Size              | [0-9]\\\+
Labels.created_by | test/system/build-testimage
Labels.created_at | 20[0-9-]\\\+T[0-9:]\\\+Z
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
    # Multiple --name options confirm command-line override (last one wins)
    run_podman run --name ignore-me --name my-container $IMAGE true
    run_podman commit my-container my-test-image

    run_podman images my-test-image --format '{{ .History }}'
    is "$output" "localhost/my-test-image:latest" "image history with initial name"

    # Generate two randomish tags; 'tr' because they must be all lower-case
    rand_name1="test-image-history-$(random_string 10 | tr A-Z a-z)"
    rand_name2="test-image-history-$(random_string 10 | tr A-Z a-z)"

    # Tag once, rmi, and make sure the tag name appears in history
    run_podman tag my-test-image $rand_name1
    run_podman rmi $rand_name1
    run_podman images my-test-image --format '{{ .History }}'
    is "$output" "localhost/my-test-image:latest, localhost/${rand_name1}:latest" "image history after one tag"

    # Repeat with second tag. Now both tags should be in history
    run_podman tag my-test-image $rand_name2
    run_podman rmi $rand_name2
    run_podman images my-test-image --format '{{ .History }}'
    is "$output" "localhost/my-test-image:latest, localhost/${rand_name2}:latest, localhost/${rand_name1}:latest" \
       "image history after two tags"

    run_podman rmi my-test-image
    run_podman rm my-container
}

@test "podman images - filter" {
    # Multiple --format options confirm command-line override (last one wins)
    run_podman inspect --format '{{.XYZ}}' --format '{{.ID}}' $IMAGE
    iid=$output

    run_podman images --noheading --filter=after=$iid
    is "$output" "" "baseline: empty results from filter (after)"

    run_podman images --noheading --filter=before=$iid
    is "$output" "" "baseline: empty results from filter (before)"

    # Create a dummy container, then commit that as an image. We will
    # now be able to use before/after/since queries
    run_podman run --name mytinycontainer $IMAGE true
    run_podman commit -q  mytinycontainer mynewimage
    new_iid=$output

    # (refactor common options for legibility)
    opts='--noheading --no-trunc --format={{.ID}}--{{.Repository}}:{{.Tag}}'

    run_podman images ${opts} --filter=after=$iid
    is "$output" "sha256:$new_iid--localhost/mynewimage:latest" "filter: after"

    # Same thing, with 'since' instead of 'after'
    run_podman images ${opts} --filter=since=$iid
    is "$output" "sha256:$new_iid--localhost/mynewimage:latest" "filter: since"

    run_podman images ${opts} --filter=before=mynewimage
    is "$output" "sha256:$iid--$IMAGE" "filter: before"

    # Clean up
    run_podman rmi mynewimage
    run_podman rm  mytinycontainer
}

# Regression test for https://github.com/containers/podman/issues/7651
# in which "podman pull image-with-sha" causes "images -a" to crash
@test "podman images -a, after pulling by sha " {
    # This test requires that $IMAGE be 100% the same as the registry one
    run_podman rmi -a -f
    _prefetch $IMAGE

    # Get a baseline for 'images -a'
    run_podman images -a
    local images_baseline="$output"

    # Get the digest of our local test image. We need to do this in two steps
    # because 'podman inspect' only works reliably on *IMAGE ID*, not name.
    # See https://github.com/containers/podman/issues/3761
    run_podman inspect --format '{{.Id}}' $IMAGE
    local iid="$output"
    run_podman inspect --format '{{.Digest}}' $iid
    local sha="$output"

    local imgbase="${PODMAN_TEST_IMAGE_REGISTRY}/${PODMAN_TEST_IMAGE_USER}/${PODMAN_TEST_IMAGE_NAME}"
    local fqin="${imgbase}@$sha"

    # This will always pull, because even though it's the same image we
    # already have, podman doesn't actually know that.
    run_podman pull $fqin
    is "$output" "Trying to pull ${fqin}\.\.\..*" "output of podman pull"

    # Prior to #7654, this would crash and burn. Now podman recognizes it
    # as the same image and, even though it internally tags it with the
    # sha, still only shows us one image (which should be our baseline)
    #
    # WARNING! If this test fails, we're going to see a lot of failures
    # in subsequent tests due to 'podman ps' showing the '@sha' tag!
    # I choose not to add a complicated teardown() (with 'rmi @sha')
    # because the failure window here is small, and if it fails it
    # needs attention anyway. So if you see lots of failures, but
    # start here because this is the first one, fix this problem.
    # You can (probably) ignore any subsequent failures showing '@sha'
    # in the error output.
    #
    # WARNING! This test is likely to fail for an hour or so after
    # building a new testimage (via build-testimage script), because
    # two consecutive 'podman images' may result in a one-minute
    # difference in the "XX minutes ago" output. This is OK to ignore.
    run_podman images -a
    is "$output" "$images_baseline" "images -a, after pull: same as before"

    # Clean up: this should simply untag, not remove
    run_podman rmi $fqin
    is "$output" "Untagged: $fqin" "podman rmi untags, does not remove"

    # ...and now we should still have our same image.
    run_podman images -a
    is "$output" "$images_baseline" "after podman rmi @sha, still the same"
}

# Tests #7199 (Restore "table" --format from V1)
#
# Tag our image with different-length strings; confirm table alignment
@test "podman images - table format" {
    # Craft two tags such that they will bracket $IMAGE on either side (above
    # and below). This assumes that $IMAGE is quay.io or foo.com or simply
    # not something insane that will sort before 'aaa' or after 'zzz'.
    local aaa_name=a.b/c
    local aaa_tag=d
    local zzz_name=zzzzzzzzzz.yyyyyyyyy/xxxxxxxxx
    local zzz_tag=$(random_string 15)

    # Helper function to check one line of tabular output; all this does is
    # generate a line with the given repo/tag, formatted to the width of the
    # widest image, which is the zzz one. Fields are separated by TWO spaces.
    function _check_line() {
        local lineno=$1
        local name=$2
        local tag=$3

        is "${lines[$lineno]}" \
           "$(printf '%-*s  %-*s  %s' ${#zzz_name} ${name} ${#zzz_tag} ${tag} $iid)" \
           "podman images, $testname, line $lineno"
    }

    function _run_format_test() {
        local testname=$1
        local format=$2

        run_podman images --sort repository --format "$format"

        line_no=0
        if [[ $format == table* ]]; then
            # skip headers from table command
            line_no=1
        fi

        _check_line $line_no ${aaa_name} ${aaa_tag}
        _check_line $((line_no+1)) "${PODMAN_TEST_IMAGE_REGISTRY}/${PODMAN_TEST_IMAGE_USER}/${PODMAN_TEST_IMAGE_NAME}" "${PODMAN_TEST_IMAGE_TAG}"
        _check_line $((line_no+2)) ${zzz_name} ${zzz_tag}
    }

    # Begin the test: tag $IMAGE with both the given names
    run_podman tag $IMAGE ${aaa_name}:${aaa_tag}
    run_podman tag $IMAGE ${zzz_name}:${zzz_tag}

    # Get the image ID, used to verify output below (all images share same IID)
    run_podman inspect --format '{{.ID}}' $IMAGE
    iid=${output:0:12}

    # Run the test: this will output three column-aligned rows. Test them.
    _run_format_test 'table' 'table {{.Repository}} {{.Tag}} {{.ID}}'

    # Clean up.
    run_podman rmi ${aaa_name}:${aaa_tag} ${zzz_name}:${zzz_tag}
}

@test "podman images - rmi -af removes all containers and pods" {
    pname=$(random_string)
    run_podman create --pod new:$pname $IMAGE

    run_podman inspect --format '{{.ID}}' $IMAGE
    imageID=$output

    pauseImage=$(pause_image)
    run_podman inspect --format '{{.ID}}' $pauseImage
    pauseID=$output

    run_podman 2 rmi -a
    is "$output" "Error: 2 errors occurred:
.** image used by .*: image is in use by a container: consider listing external containers and force-removing image
.** image used by .*: image is in use by a container: consider listing external containers and force-removing image"

    run_podman rmi -af
    is "$output" "Untagged: $IMAGE
Untagged: $pauseImage
Deleted: $imageID
Deleted: $pauseID" "infra images gets removed as well"

    run_podman images --noheading
    is "$output" ""
    run_podman ps --all --noheading
    is "$output" ""
    run_podman pod ps --noheading
    is "$output" ""

    run_podman create --pod new:$pname $IMAGE
    # Clean up
    run_podman rm "${lines[-1]}"
    run_podman pod rm -a
    run_podman rmi $pauseImage
}

@test "podman images - rmi -f can remove infra images" {
    pname=$(random_string)
    run_podman create --pod new:$pname $IMAGE

    pauseImage=$(pause_image)
    run_podman inspect --format '{{.ID}}' $pauseImage
    pauseID=$output

    run_podman 2 rmi $pauseImage
    is "$output" "Error: image used by .* image is in use by a container: consider listing external containers and force-removing image"

    run_podman rmi -f $pauseImage
    is "$output" "Untagged: $pauseImage
Deleted: $pauseID"

    # Force-removing the infra container removes the pod and all its containers.
    run_podman ps --all --noheading
    is "$output" ""
    run_podman pod ps --noheading
    is "$output" ""

    # Other images are still present.
    run_podman image exists $IMAGE
}

@test "podman rmi --ignore" {
    random_image_name=$(random_string)
    random_image_name=${random_image_name,,} # name must be lowercase
    run_podman 1 rmi $random_image_name
    is "$output" "Error: $random_image_name: image not known.*"
    run_podman rmi --ignore $random_image_name
    is "$output" ""
}

@test "podman image rm --force bogus" {
    run_podman 1 image rm bogus
    is "$output" "Error: bogus: image not known" "Should print error"
    run_podman image rm --force bogus
    is "$output" "" "Should print no output"
}

@test "podman images - commit docker with comment" {
    run_podman run --name my-container -d $IMAGE top
    run_podman 125 commit -m comment my-container my-test-image
    assert "$output" == "Error: messages are only compatible with the docker image format (-f docker)" "podman should fail unless docker format"

    # Without -q: verbose output, but only on podman-local, not remote
    run_podman commit my-container --format docker -m comment my-test-image1
    if ! is_remote; then
        assert "$output" =~ "Getting image.*Writing manif" \
               "Without -q, verbose output"
    fi

    # With -q, both local and remote: only an image ID
    run_podman commit -q my-container --format docker -m comment my-test-image2
    assert "$output" =~ "^[0-9a-f]{64}\$" \
           "With -q, output is a commit ID, no warnings or other output"

    run_podman rmi my-test-image1 my-test-image2
    run_podman rm my-container --force -t 0
}

@test "podman pull image with additional store" {
    skip_if_remote "only works on local"

    local imstore=$PODMAN_TMPDIR/imagestore
    local sconf=$PODMAN_TMPDIR/storage.conf
    cat >$sconf <<EOF
[storage]
driver="overlay"

[storage.options]
additionalimagestores = [ "$imstore/root" ]
EOF

    skopeo copy containers-storage:$IMAGE \
           containers-storage:\[overlay@$imstore/root+$imstore/runroot\]$IMAGE

    CONTAINERS_STORAGE_CONF=$sconf run_podman images -a -n --format "{{.Repository}}:{{.Tag}} {{.ReadOnly}}"
    is "${lines[0]}" "$IMAGE false" "image from readonly store"
    is "${lines[1]}" "$IMAGE true" "image from readwrite store"

    CONTAINERS_STORAGE_CONF=$sconf run_podman images -a -n --format "{{.Id}}"
    id=${lines[0]}

    CONTAINERS_STORAGE_CONF=$sconf run_podman pull -q $IMAGE
    is "$output" "$id" "Should only print one line"

    run_podman --root $imstore/root rmi --all
}

@test "podman images with concurrent removal" {
    skip_if_remote "following test is not supported for remote clients"
    local count=5

    # First build $count images
    for i in $(seq --format '%02g' 1 $count); do
        cat >$PODMAN_TMPDIR/Containerfile <<EOF
FROM $IMAGE
RUN echo $i
EOF
        run_podman build -q -t i$i $PODMAN_TMPDIR
    done

    run_podman images
    # Now remove all images in parallel and in the background and make sure
    # that listing all images does not fail (see BZ 2216700).
    for i in $(seq --format '%02g' 1 $count); do
        timeout --foreground -v --kill=10 60 \
                $PODMAN rmi i$i &
    done

    tries=100
    while [[ ${#lines[*]} -gt 1 ]] && [[ $tries -gt 0 ]]; do
        # Prior to #18980, 'podman images' during rmi could fail with 'image not known'
        run_podman images --format "{{.ID}} {{.Names}}"
        tries=$((tries - 1))
    done

    if [[ $tries -eq 0 ]]; then
        die "Timed out waiting for images to be removed"
    fi

    wait
}


# vim: filetype=sh
