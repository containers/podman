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

# Regression test for #8931
@test "podman images - bare manifest list" {
    # Create an empty manifest list and list images.

    run_podman inspect --format '{{.ID}}' $IMAGE
    iid=$output

    run_podman manifest create test:1.0
    run_podman images --format '{{.ID}}' --no-trunc
    [[ "$output" == *"sha256:$iid"* ]]

    run_podman rmi test:1.0
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
.** Image used by .*: image is in use by a container
.** Image used by .*: image is in use by a container"

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

    run_podman version --format "{{.Server.Version}}-{{.Server.Built}}"
    pauseImage=localhost/podman-pause:$output
    run_podman inspect --format '{{.ID}}' $pauseImage
    pauseID=$output

    run_podman 2 rmi $pauseImage
    is "$output" "Error: Image used by .* image is in use by a container"

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

# vim: filetype=sh
