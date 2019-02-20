#!/usr/bin/env bats

load helpers


@test "podman mount - basic test" {
    # Only works with root (FIXME: does it work with rootless + vfs?)
    skip_if_rootless

    f_path=/tmp/tmpfile_$(random_string 8)
    f_content=$(random_string 30)

    c_name=mount_test_$(random_string 5)
    run_podman run --name $c_name $PODMAN_TEST_IMAGE_FQN \
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
