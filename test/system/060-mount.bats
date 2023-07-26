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

    # Clean up, and make sure nothing is mounted any more
    run_podman image umount -f $IMAGE
    is "$output" "$iid" "podman image umount: image ID of what was umounted"

    run_podman image umount $IMAGE
    is "$output" "" "podman image umount: does not re-umount"

    run_podman 125 image umount no-such-image
    is "$output" "Error: no-such-image: image not known" \
       "error message from image umount no-such-image"

    # Tests for mount -a. This may mount more than one image! (E.g. systemd)
    run_podman image mount -a
    is "$output" "$IMAGE .*$mount_path"

    run_podman image umount -a
    assert "$output" =~ "$iid" "Test image is unmounted"

    run_podman image mount
    is "$output" "" "podman image mount, no args, after umount"
}

@test "podman run --mount ro=false " {
    local volpath=/path/in/container
    local stdopts="type=volume,destination=$volpath"

    # Variations on a theme (not by Paganini). All of these should fail.
    for varopt in readonly readonly=true ro=true ro rw=false;do
        run_podman 1 run --rm -q --mount $stdopts,$varopt $IMAGE touch $volpath/a
        is "$output" "touch: $volpath/a: Read-only file system" "with $varopt"
    done

    # All of these should pass
    for varopt in rw rw=true ro=false readonly=false;do
        run_podman run --rm -q --mount $stdopts,$varopt $IMAGE touch $volpath/a
    done
}

@test "podman run --mount image" {
    skip_if_rootless "too hard to test rootless"

    # Run a container with an image mount
    run_podman run --rm --mount type=image,src=$IMAGE,dst=/image-mount $IMAGE diff /etc/os-release /image-mount/etc/os-release

    # Make sure the mount is read-only
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

@test "podman mount containers.conf" {
    skip_if_remote "remote does not support CONTAINERS_CONF*"

    dest=/$(random_string 30)
    tmpfile1=$PODMAN_TMPDIR/volume-test1
    random1=$(random_string 30)
    echo $random1 > $tmpfile1

    tmpfile2=$PODMAN_TMPDIR/volume-test2
    random2=$(random_string 30)
    echo $random2 > $tmpfile2
    bogus=$(random_string 10)

    mountStr1=type=bind,src=$tmpfile1,destination=$dest,ro,Z
    mountStr2=type=bind,src=$tmpfile2,destination=$dest,ro,Z
    containersconf=$PODMAN_TMPDIR/containers.conf
    cat >$containersconf <<EOF
[containers]
mounts=[ "$mountStr1", ]
EOF
    badcontainersconf=$PODMAN_TMPDIR/badcontainers.conf
    cat >$badcontainersconf <<EOF
[containers]
mounts=[ "type=$bogus,src=$tmpfile2,destination=$dest,ro", ]
EOF

    run_podman 1 run $IMAGE cat $dest
    is "$output" "cat: can't open '$dest': No such file or directory" "$dest does not exist"

    CONTAINERS_CONF_OVERRIDE="$containersconf" run_podman run $IMAGE cat $dest
    is "$output" "$random1" "file should contain $random1"

    CONTAINERS_CONF_OVERRIDE="$containersconf" run_podman run --mount $mountStr2 $IMAGE cat $dest
    is "$output" "$random2" "overridden file should contain $random2"

    CONTAINERS_CONF_OVERRIDE="$containersconf" run_podman 125 run --mount $mountStr1 --mount $mountStr2 $IMAGE cat $dest
    is "$output" "Error: $dest: duplicate mount destination" "Should through duplicate destination error for $dest"

    CONTAINERS_CONF_OVERRIDE="$badcontainersconf" run_podman 125 run $IMAGE cat $dest
    is "$output" "Error: parsing containers.conf mounts: invalid filesystem type \"$bogus\"" "containers.conf should fail with bad mounts entry"

    run_podman rm --all --force -t 0
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

@test "podman volume globs" {
    v1a=v1_$(random_string)
    v1b=v1_$(random_string)
    v2=v2_$(random_string)
    vol1a=${PODMAN_TMPDIR}/$v1a
    vol1b=${PODMAN_TMPDIR}/$v1b
    vol2=${PODMAN_TMPDIR}/$v2
    touch $vol1a $vol1b $vol2

    # if volumes source and dest match then pass
    run_podman run --rm --mount type=glob,src=${PODMAN_TMPDIR}/v1\*,ro $IMAGE ls $vol1a $vol1b
    run_podman 1 run --rm --mount source=${PODMAN_TMPDIR}/v1\*,type=glob,ro $IMAGE ls $vol2
    is "$output" ".*No such file or directory" "$vol2 should not be mounted in the container"

    run_podman 125 run --rm --mount source=${PODMAN_TMPDIR}/v3\*,type=glob,ro $IMAGE ls $vol2
    is "$output" "Error: no file paths matching glob \"${PODMAN_TMPDIR}/v3\*\"" "Glob does not match so should throw error"

    run_podman 1 run --rm --mount source=${PODMAN_TMPDIR}/v2\*,type=glob,ro,Z $IMAGE touch $vol2
    is "$output" "touch: $vol2: Read-only file system" "Mount should be read-only"

    run_podman run --rm --mount source=${PODMAN_TMPDIR}/v2\*,type=glob,ro=false,Z $IMAGE touch $vol2

    run_podman run --rm --mount type=glob,src=${PODMAN_TMPDIR}/v1\*,destination=/non/existing/directory,ro $IMAGE ls /non/existing/directory
    is "$output" ".*$v1a" "podman images --inspect should include $v1a"
    is "$output" ".*$v1b" "podman images --inspect should include $v1b"

    run_podman create --rm --mount type=glob,src=${PODMAN_TMPDIR}/v1\*,ro $IMAGE ls $vol1a $vol1b
    cid=$output
    run_podman container inspect $output
    is "$output" ".*$vol1a" "podman images --inspect should include $vol1a"
    is "$output" ".*$vol1b" "podman images --inspect should include $vol1b"

    run_podman 125 run --rm --mount source=${PODMAN_TMPDIR}/v2\*,type=bind,ro=false $IMAGE touch $vol2
    is "$output" "Error: must set volume destination" "Bind mounts require destination"

    run_podman 125 run --rm --mount source=${PODMAN_TMPDIR}/v2\*,destination=/tmp/foobar, ro=false $IMAGE touch $vol2
    is "$output" "Error: invalid reference format" "Default mounts don not support globs"
}

# vim: filetype=sh
