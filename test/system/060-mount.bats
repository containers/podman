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

    # Get full image ID, to verify umount
    run_podman image inspect --format '{{.ID}}' $IMAGE
    iid="$output"

    # Mount, and make sure the mount point exists
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
    is "$output" "$IMAGE *$mount_path" "podman image mount with no args"

    # Clean up
    run_podman image umount $IMAGE
    is "$output" "$iid" "podman image umount: image ID of what was umounted"

    run_podman image umount $IMAGE
    is "$output" "" "podman image umount: does not re-umount"

    run_podman 125 image umount no-such-container
    is "$output" "Error: no-such-container: image not known" \
       "error message from image umount no-such-container"

    run_podman image mount
    is "$output" "" "podman image mount, no args, after umount"
}

@test "podman run --mount image" {
    skip_if_rootless "too hard to test rootless"

    # Run a container with an image mount
    run_podman run --rm --mount type=image,src=$IMAGE,dst=/image-mount $IMAGE diff /etc/os-release /image-mount/etc/os-release

    # Make sure the mount is read only
    run_podman 1 run --rm --mount type=image,src=$IMAGE,dst=/image-mount $IMAGE touch /image-mount/read-only
    is "$output" "touch: /image-mount/read-only: Read-only file system"

    # Make sure that rw,readwrite work
    run_podman run --rm --mount type=image,src=$IMAGE,dst=/image-mount,rw=true $IMAGE touch /image-mount/readwrite
    run_podman run --rm --mount type=image,src=$IMAGE,dst=/image-mount,readwrite=true $IMAGE touch /image-mount/readwrite

    skip_if_remote "mounting remote is meaningless"

    # The mount should be cleaned up during container removal as no other entity mounted the image
    run_podman image umount $IMAGE
    is "$output" "" "image mount should have been cleaned up during container removal"

    # Now make sure that the image mount is not cleaned up during container removal when another entity mounted the image
    run_podman image mount $IMAGE
    run_podman run --rm --mount type=image,src=$IMAGE,dst=/image-mount $IMAGE diff /etc/os-release /image-mount/etc/os-release

    run_podman image inspect --format '{{.ID}}' $IMAGE
    iid="$output"

    run_podman image umount $IMAGE
    is "$output" "$iid" "podman image umount: image ID of what was umounted"

    run_podman image umount $IMAGE
    is "$output" "" "image mount should have been cleaned up via 'image umount'"

    # Run a container in the background (source is the ID instead of name)
    run_podman run -d --mount type=image,src=$iid,dst=/image-mount,readwrite=true $IMAGE sleep infinity
    cid="$output"

    # Unmount the image
    run_podman image umount $IMAGE
    is "$output" "$iid" "podman image umount: image ID of what was umounted"
    run_podman image umount $IMAGE
    is "$output" "" "image mount should have been cleaned up via 'image umount'"

    # Make sure that the mount in the container is unaffected
    run_podman exec $cid diff /etc/os-release /image-mount/etc/os-release
    run_podman exec $cid find /image-mount/etc/

    # Clean up
    run_podman rm -t 0 -f $cid
}

@test "podman run --mount image inspection" {
    skip_if_rootless "too hard to test rootless"

    # Run a container in the background
    run_podman run -d --mount type=image,src=$IMAGE,dst=/image-mount,rw=true $IMAGE sleep infinity
    cid="$output"

    run_podman inspect --format "{{(index .Mounts 0).Type}}" $cid
    is "$output" "image" "inspect data includes image mount type"

    run_podman inspect --format "{{(index .Mounts 0).Source}}" $cid
    is "$output" "$IMAGE" "inspect data includes image mount source"

    run_podman inspect --format "{{(index .Mounts 0).Destination}}" $cid
    is "$output" "/image-mount" "inspect data includes image mount source"

    run_podman inspect --format "{{(index .Mounts 0).RW}}" $cid
    is "$output" "true" "inspect data includes image mount source"

    run_podman rm -t 0 -f $cid
}

@test "podman mount external container - basic test" {
    # Only works with root (FIXME: does it work with rootless + vfs?)
    skip_if_rootless "mount does not work rootless"
    skip_if_remote "mounting remote is meaningless"

    # Create a container that podman does not know about
    external_cid=$(buildah from $IMAGE)

    run_podman mount $external_cid
    mount_path=$output

    # Test image will always have this file, and will always have the tag
    test -d $mount_path
    is $(< "$mount_path/home/podman/testimage-id") "$PODMAN_TEST_IMAGE_TAG"  \
       "Contents of well-known file in image"

    # Make sure that 'podman mount' (no args) returns the expected path
    run_podman mount --notruncate

    reported_mountpoint=$(echo "$output" | awk '{print $2}')
    is $reported_mountpoint $mount_path "mountpoint reported by 'podman mount'"

    # umount, and make sure files are gone
    run_podman umount $external_cid
    if [ -d "$mount_path" ]; then
        die "'podman umount' did not umount"
    fi
    buildah rm $external_cid
}

# vim: filetype=sh
