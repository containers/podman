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
    is "$reported_mountpoint" "$mount_path" "mountpoint reported by 'podman mount'"

    # umount, and make sure files are gone
    run_podman umount $c_name
    if [[ -e "$mount_path/$f_path" ]]; then
        # With vfs, umount is a NOP: the path always exists as long as the
        # container exists. But with overlay, umount should truly remove.
        if [[ "$(podman_storage_driver)" != "vfs" ]]; then
            die "Mounted file exists even after umount: $mount_path/$f_path"
        fi
    fi

    # Remove the container. Now even with vfs the file must be gone.
    run_podman rm $c_name
    if [[ -e "$mount_path/$f_path" ]]; then
        die "Mounted file exists even after container rm: $mount_path/$f_path"
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
    is "$reported_mountpoint" "$mount_path" "mountpoint reported by 'podman mount'"

    # umount, and make sure files are gone
    run_podman umount $external_cid
    if [ -d "$mount_path" ]; then
        # Under VFS, mountpoint always exists even despite umount
        if [[ "$(podman_storage_driver)" != "vfs" ]]; then
            die "'podman umount' did not umount $mount_path"
        fi
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

    mkdir $PODMAN_TMPDIR/foo1 $PODMAN_TMPDIR/foo2 $PODMAN_TMPDIR/foo3
    touch $PODMAN_TMPDIR/foo1/bar $PODMAN_TMPDIR/foo2/bar $PODMAN_TMPDIR/foo3/bar
    touch $PODMAN_TMPDIR/foo1/bar1 $PODMAN_TMPDIR/foo2/bar2 $PODMAN_TMPDIR/foo3/bar3
    run_podman 125 run --rm --mount type=glob,source=${PODMAN_TMPDIR}/foo?/bar,destination=/tmp $IMAGE ls -l /tmp
    is "$output" "Error: /tmp/bar: duplicate mount destination" "Should report conflict on destination directory"
    run_podman run --rm --mount type=glob,source=${PODMAN_TMPDIR}/foo?/bar?,destination=/tmp,ro $IMAGE ls /tmp
    is "$output" "bar1.*bar2.*bar3" "Should match multiple source files on single destination directory"
}

@test "podman mount noswap memory mounts" {
    # if volumes source and dest match then pass
    run_podman run --rm --mount type=ramfs,destination=${PODMAN_TMPDIR} $IMAGE stat -f -c "%T" ${PODMAN_TMPDIR}
    is "$output" "ramfs" "ramfs mounted"

    if is_rootless; then
        run_podman 125 run --rm --mount type=tmpfs,destination=${PODMAN_TMPDIR},noswap  $IMAGE stat -f -c "%T" ${PODMAN_TMPDIR}
        is "$output" "Error: the 'noswap' option is only allowed with rootful tmpfs mounts: must provide an argument for option" "noswap not supported in rootless mode"
    else
        run_podman run --rm --mount type=tmpfs,destination=${PODMAN_TMPDIR},noswap  $IMAGE sh -c "mount| grep ${PODMAN_TMPDIR}"
        is "$output" ".*noswap" "tmpfs noswap mounted"
    fi
}

@test "podman mount no-dereference" {
    # Test how bind and glob-mounts behave with respect to relative (rel) and
    # absolute (abs) symlinks.

    if [ $(podman_runtime) != "crun" ]; then
        # Requires crun >= 1.11.0
        skip "only crun supports the no-dereference (copy-symlink) mount option"
    fi

    # One directory for testing relative symlinks, another for absolute ones.
    rel_dir=$PODMAN_TMPDIR/rel-dir
    abs_dir=$PODMAN_TMPDIR/abs-dir
    mkdir $rel_dir $abs_dir

    # Create random values to discrimate data in the rel/abs directory and the
    # one from the image.
    rel_random_host="rel_on_the_host_$(random_string 15)"
    abs_random_host="abs_on_the_host_$(random_string 15)"
    random_img="on_the_image_$(random_string 15)"

    # Relative symlink
    echo "$rel_random_host" > $rel_dir/data
    ln -r -s $rel_dir/data $rel_dir/link
    # Absolute symlink
    echo "$abs_random_host" > $abs_dir/data
    ln -s $abs_dir/data $abs_dir/link

    dockerfile=$PODMAN_TMPDIR/Dockerfile
    cat >$dockerfile <<EOF
FROM $IMAGE
RUN echo $random_img > /tmp/data
EOF

    img="localhost/preserve:symlinks"
    run_podman build -t $img -f $dockerfile

    link_path="/tmp/link"
    create_path="/tmp/i/do/not/exist/link"

    tests="
0 | bind | $rel_dir/link | /tmp/link |  | /tmp/link | $rel_random_host | $link_path | bind mount relative symlink: mounts target from the host
0 | bind | $abs_dir/link | /tmp/link |  | /tmp/link | $abs_random_host | $link_path | bind mount absolute symlink: mounts target from the host
0 | glob | $rel_dir/lin* | /tmp/     |  | /tmp/link | $rel_random_host | $link_path | glob mount relative symlink: mounts target from the host
0 | glob | $abs_dir/lin* | /tmp/     |  | /tmp/link | $abs_random_host | $link_path | glob mount absolute symlink: mounts target from the host
0 | glob | $rel_dir/*    | /tmp/     |  | /tmp/link | $rel_random_host | $link_path | glob mount entire directory: mounts relative target from the host
0 | glob | $abs_dir/*    | /tmp/     |  | /tmp/link | $abs_random_host | $link_path | glob mount entire directory: mounts absolute target from the host
0 | bind | $rel_dir/link | /tmp/link | ,no-dereference | '/tmp/link' -> 'data' | $random_img      | $link_path | no_deref: bind mount relative symlink: points to file on the image
0 | glob | $rel_dir/lin* | /tmp/     | ,no-dereference | '/tmp/link' -> 'data' | $random_img      | $link_path | no_deref: glob mount relative symlink: points to file on the image
0 | bind | $rel_dir/     | /tmp/     | ,no-dereference | '/tmp/link' -> 'data' | $rel_random_host | $link_path | no_deref: bind mount the entire directory: preserves symlink automatically
0 | glob | $rel_dir/*    | /tmp/     | ,no-dereference | '/tmp/link' -> 'data' | $rel_random_host | $link_path | no_deref: glob mount the entire directory: preserves symlink automatically
1 | bind | $abs_dir/link | /tmp/link | ,no-dereference | '/tmp/link' -> '$abs_dir/data' | cat: can't open '/tmp/link': No such file or directory | $link_path | bind mount *preserved* absolute symlink: now points to a non-existent file on the container
1 | glob | $abs_dir/lin* | /tmp/     | ,no-dereference | '/tmp/link' -> '$abs_dir/data' | cat: can't open '/tmp/link': No such file or directory | $link_path | glob mount *preserved* absolute symlink: now points to a non-existent file on the container
0 | bind | $rel_dir/link | $create_path |  | $create_path | $rel_random_host | $create_path | bind mount relative symlink: creates dirs and mounts target from the host
1 | bind | $rel_dir/link | $create_path | ,no-dereference | '$create_path' -> 'data' | cat: can't open '$create_path': No such file or directory | $create_path | no_deref: bind mount relative symlink: creates dirs and mounts target from the host
"

    while read exit_code mount_type mount_src mount_dst mount_opts line_0 line_1 path description; do
        if [[ $mount_opts == "''" ]];then
            unset mount_opts
        fi
        run_podman $exit_code run \
            --mount type=$mount_type,src=$mount_src,dst=$mount_dst$mount_opts \
            --rm --privileged $img sh -c "stat -c '%N' $path; cat $path"
        assert "${lines[0]}" = "$line_0" "$description"
        assert "${lines[1]}" = "$line_1" "$description"
    done < <(parse_table "$tests")

    # Make sure that it's presvered across starts and stops
    run_podman create --mount type=glob,src=$rel_dir/*,dst=/tmp/,no-dereference --privileged $img sh -c "stat -c '%N' /tmp/link; cat /tmp/link"
    cid="$output"
    run_podman start -a $cid
    assert "${lines[0]}" = "'/tmp/link' -> 'data'" "symlink is preserved"
    assert "${lines[1]}" = "$rel_random_host"      "glob macthes symlink and host 'data' file"
    run_podman start -a $cid
    assert "${lines[0]}" = "'/tmp/link' -> 'data'" "symlink is preserved"
    assert "${lines[1]}" = "$rel_random_host"      "glob macthes symlink and host 'data' file"
    run_podman rm -f -t=0 $cid

    run_podman rmi -f $img
}
