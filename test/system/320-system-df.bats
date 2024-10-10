#!/usr/bin/env bats   -*- bats -*-
#
# tests for podman system df
#
# DO NOT PARALLELIZE. All of these tests require complete control of images.
#

load helpers

function setup_file() {
    # Pristine setup: no pods, containers, volumes, images
    run_podman pod rm -a -f
    run_podman rm -f -a -t0
    run_podman volume rm -a
    run_podman image rm -f -a

    _prefetch $IMAGE
}

@test "podman system df - basic functionality" {
    run_podman system df
    is "$output" ".*Images  *1 *0 "       "Exactly one image"
    is "$output" ".*Containers *0 *0 "    "No containers"
    is "$output" ".*Local Volumes *0 *0 " "No volumes"
}

@test "podman system df --format {{ json . }} functionality" {
    run_podman system df --format '{{json .}}'
    is "$output" '.*"TotalCount":1'       "Exactly one image"
    is "$output" '.*"RawSize".*"Size"' "RawSize and Size reported"
    is "$output" '.*"RawReclaimable".*"Reclaimable"' "RawReclaimable and Reclaimable reported"
    is "$output" '.*"Containers".*"Total":0' "Total containers reported"
    is "$output" '.*"Local Volumes".*"Size":"0B"' "Total containers reported"
    is "$output" '.*"Local Volumes".*"Size":"0B"' "Total containers reported"
}

@test "podman system df --format json functionality" {
    # Run two dummy containers, one which exits, one which stays running
    cname_stopped=c-stopped-$(safename)
    cname_running=c-running-$(safename)

    run_podman run    --name $cname_stopped $IMAGE true
    run_podman run -d --name $cname_running $IMAGE top
    run_podman system df --format json
    local results="$output"

    # FIXME! This needs to be fiddled with every time we bump testimage.
    local size=12
    if [[ "$(uname -m)" = "aarch64" ]]; then
        size=14
    fi

    # FIXME: we can't check exact RawSize or Size because every CI system
    # computes a different value: 12701526, 12702113, 12706209... and
    # those are all amd64. aarch64 gets 12020148, 12019561.
    #
    # WARNING: RawSize and Size tests may fail if $IMAGE is updated. Since
    # that tends to be done yearly or less, and only by Ed, that's OK.
    local tests="
Type           | Images         | Containers | Local Volumes
Total          |              1 |          2 |             0
Active         |              1 |          1 |             0
RawSize        | ~${size}...... |         !0 |             0
RawReclaimable |              0 |         !0 |             0
Reclaimable    |        ~\(0%\) |   ~\(50%\) |       ~\(0%\)
TotalCount     |              1 |          2 |             0
Size           |   ~${size}.*MB |        !0B |            0B
"
    while read -a fields; do
        for i in 0 1 2;do
            expect="${fields[$((i+1))]}"
            actual=$(jq -r ".[$i].${fields[0]}" <<<"$results")

            # Do exact-match check, unless the expect term starts with ~ or !
            op='='
            if [[ "$expect" =~ ^\! ]]; then
                op='!='
                expect=${expect##\!}
            fi
            if [[ "$expect" =~ ^~ ]]; then
                op='=~'
                expect=${expect##\~}
            fi

            assert "$actual" "$op" "$expect" "system df[$i].${fields[0]}"
        done
    done < <(parse_table "$tests")

    # Clean up
    run_podman rm -f -t 0 $cname_stopped $cname_running
}

@test "podman system df - with active containers and volumes" {
    c1=c1-$(safename)
    c2=c2-$(safename)
    run_podman run    -v /myvol1 --name $c1 $IMAGE true
    run_podman run -d -v /myvol2 --name $c2 $IMAGE top

    run_podman system df --format '{{ .Type }}:{{ .Total }}:{{ .Active }}'
    is "${lines[0]}" "Images:1:1"        "system df : Images line"
    is "${lines[1]}" "Containers:2:1"    "system df : Containers line"
    is "${lines[2]}" "Local Volumes:2:2" "system df : Volumes line"

    # Try -v. (Grrr. No way to specify individual formats)
    #
    # Yes, I know this would be more elegant as a separate @test, but
    # container/volume setup/teardown costs ~3 seconds and that matters.
    run_podman system df -v
    is "${lines[2]}" \
       "${PODMAN_TEST_IMAGE_REGISTRY}/${PODMAN_TEST_IMAGE_USER}/${PODMAN_TEST_IMAGE_NAME} * ${PODMAN_TEST_IMAGE_TAG} [0-9a-f]* .* 2" \
       "system df -v: the 'Images' line"

    # Containers are listed in random order. Just check that each has 1 volume
    is "${lines[5]}" \
       "[0-9a-f]\{12\} *[0-9a-f]\{12\} .* 1 .* c[12]-$(safename)" \
       "system df -v, 'Containers', first line"
    is "${lines[6]}" \
       "[0-9a-f]\{12\} *[0-9a-f]\{12\} .* 1 .* c[12]-$(safename)" \
       "system df -v, 'Containers', second line"

    # Volumes, likewise: random order.
    is "${lines[9]}" "[0-9a-f]\{64\} *[01] * 0B" \
       "system df -v, 'Volumes', first line"
    is "${lines[10]}" "[0-9a-f]\{64\} *[01] * 0B" \
       "system df -v, 'Volumes', second line"

    # Make sure that the return image "raw" size is correct
    run_podman image inspect $IMAGE --format "{{.Size}}"
    expectedSize="$output"

    run_podman system df --format "{{.RawSize}}"
    is "${lines[0]}" "$expectedSize" "raw image size is correct"

    # Clean up and check reclaimable image data
    run_podman system df --format '{{.Reclaimable}}'
    is "${lines[0]}" "0B (0%)" "cannot reclaim image data as it's still used by the containers"

    run_podman stop $c2

    # Create a second image by committing a container.
    run_podman container commit -q $c1
    image="$output"

    run_podman system df --format '{{.Reclaimable}}'
    is "${lines[0]}" ".* (100%)" "100 percent of image data is reclaimable because $IMAGE has unique size of 0"

    # Make sure the unique size is now really 0.  We cannot use --format for
    # that unfortunately but we can exploit the fact that $IMAGE is used by
    # two containers.
    run_podman system df -v
    is "$output" ".*0B\\s\\+2.*"

    run_podman rm $c1 $c2

    run_podman system df --format '{{.Reclaimable}}'
    is "${lines[0]}" ".* (100%)" "100 percent of image data is reclaimable because all containers are gone"

    run_podman rmi $image
    run_podman volume rm -a
}

# vim: filetype=sh
