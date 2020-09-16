#!/usr/bin/env bats

load helpers


@test "podman mount - basic test" {
    # Only works with root (FIXME: does it work with rootless + vfs?)
    skip_if_rootless "mount does not work rootless"
    skip_if_remote "mounting remote is meaningless"

    f_path=/tmp/tmpfile_$(random_string 8)
    f_content=$(random_string 30)

    c_name=mount_test_$(random_string 5)
    run_podman run --name $c_name $IMAGE \
               sh -c "echo $f_content > $f_path"

    run_podman mount $c_name
    mount_path=$output

    test -d $mount_path
    test -e "$mount_path/$f_path"
    is $(< "$mount_path/$f_path") "$f_content" "contents of file, as read via fs"

    # Make sure that 'podman mount' (no args) returns the expected path
    run_podman mount --notruncate
    # FIXME: is it worth the effort to validate the CID ($1) ?
    reported_mountpoint=$(echo "$output" | awk '{print $2}')
    is $reported_mountpoint $mount_path "mountpoint reported by 'podman mount'"

    # umount, and make sure files are gone
    run_podman umount $c_name
    if [ -e "$mount_path/$f_path" ]; then
        die "Mounted file exists even after umount: $mount_path/$f_path"
    fi
}


@test "podman image mount" {
    skip_if_remote "mounting remote is meaningless"
    skip_if_rootless "too hard to test rootless"

    # Start with clean slate
    run_podman image umount -a

    run_podman image mount $IMAGE
    mount_path="$output"

    test -d $mount_path

    # Image is custom-built and has a file containing the YMD tag. Check it.
    testimage_file="/home/podman/testimage-id"
    test -e "$mount_path$testimage_file"
    is $(< "$mount_path$testimage_file") "$PODMAN_TEST_IMAGE_TAG"  \
       "Contents of $testimage_file in image"

    # 'image mount', no args, tells us what's mounted
    run_podman image mount
    is "$output" "$IMAGE $mount_path" "podman image mount with no args"

    # Clean up
    run_podman image umount $IMAGE

    run_podman image mount
    is "$output" "" "podman image mount, no args, after umount"
}

# vim: filetype=sh
