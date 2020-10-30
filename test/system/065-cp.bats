#!/usr/bin/env bats   -*- bats -*-
#
# Tests for 'podman cp'
#
# ASSUMPTION FOR ALL THESE TESTS: /tmp in the container starts off empty
#

load helpers

# Create two random-name random-content files in /tmp in the container
# podman-cp them into the host using '/tmp/*', i.e. asking podman to
# perform wildcard expansion in the container. We should get both
# files copied into the host.
@test "podman cp * - wildcard copy multiple files from container to host" {
    skip_if_remote "podman-remote does not yet handle cp"

    srcdir=$PODMAN_TMPDIR/cp-test-in
    dstdir=$PODMAN_TMPDIR/cp-test-out
    mkdir -p $srcdir $dstdir

    rand_filename1=$(random_string 20)
    rand_content1=$(random_string 50)
    rand_filename2=$(random_string 20)
    rand_content2=$(random_string 50)

    run_podman run --name cpcontainer $IMAGE sh -c \
               "echo $rand_content1 >/tmp/$rand_filename1;
                echo $rand_content2 >/tmp/$rand_filename2"

    # cp no longer supports wildcarding
    run_podman 125 cp 'cpcontainer:/tmp/*' $dstdir

    run_podman rm cpcontainer
}


# Create a file on the host; make a symlink in the container pointing
# into host-only space. Try to podman-cp that symlink. It should fail.
@test "podman cp - will not recognize symlink pointing into host space" {
    skip_if_remote "podman-remote does not yet handle cp"

    srcdir=$PODMAN_TMPDIR/cp-test-in
    dstdir=$PODMAN_TMPDIR/cp-test-out
    mkdir -p $srcdir $dstdir
    echo "this file is on the host" >$srcdir/hostfile

    run_podman run --name cpcontainer $IMAGE \
               sh -c "ln -s $srcdir/hostfile /tmp/badlink"
    # This should fail because, from the container's perspective, the symlink
    # points to a nonexistent file
    run_podman 125 cp 'cpcontainer:/tmp/*' $dstdir/

    # FIXME: this might not be the exactly correct error message
    is "$output" ".*error evaluating symlinks.*lstat.*no such file or dir" \
       "Expected error from copying invalid symlink"

    # make sure there are no files in dstdir
    is "$(/bin/ls -1 $dstdir)" "" "incorrectly copied symlink from host"

    run_podman rm cpcontainer
}


# Issue #3829 - like the above, but with a level of indirection in the
# wildcard expansion: create a file on the host; create a symlink in
# the container named 'file1' pointing to this file; then another symlink
# in the container pointing to 'file*' (file star). Try to podman-cp
# this invalid double symlink. It must fail.
@test "podman cp - will not expand globs in host space (#3829)" {
    skip_if_remote "podman-remote does not yet handle cp"

    srcdir=$PODMAN_TMPDIR/cp-test-in
    dstdir=$PODMAN_TMPDIR/cp-test-out
    mkdir -p $srcdir $dstdir
    echo "This file is on the host" > $srcdir/hostfile

    run_podman run --name cpcontainer $IMAGE \
               sh -c "ln -s $srcdir/hostfile file1;ln -s file\* copyme"
    run_podman 125 cp cpcontainer:copyme $dstdir

    is "$output" ".*error evaluating symlinks.*lstat.*no such file or dir" \
       "Expected error from copying invalid symlink"

    # make sure there are no files in dstdir
    is "$(/bin/ls -1 $dstdir)" "" "incorrectly copied symlink from host"

    run_podman rm cpcontainer
}


# Another symlink into host space, this one named '*' (star). cp should fail.
@test "podman cp - will not expand wildcard" {
    skip_if_remote "podman-remote does not yet handle cp"

    srcdir=$PODMAN_TMPDIR/cp-test-in
    dstdir=$PODMAN_TMPDIR/cp-test-out
    mkdir -p $srcdir $dstdir
    echo "This file lives on the host" > $srcdir/hostfile

    run_podman run --name cpcontainer $IMAGE \
               sh -c "ln -s $srcdir/hostfile /tmp/\*"
    run_podman 125 cp 'cpcontainer:/tmp/*' $dstdir

    is "$output" ".*error evaluating symlinks.*lstat.*no such file or dir" \
       "Expected error from copying invalid symlink"

    # dstdir must be empty
    is "$(/bin/ls -1 $dstdir)" "" "incorrectly copied symlink from host"

    run_podman rm cpcontainer
}

###############################################################################
# cp INTO container

# THIS IS EXTREMELY WEIRD. Podman expands symlinks in weird ways.
@test "podman cp into container: weird symlink expansion" {
    skip_if_remote "podman-remote does not yet handle cp"

    srcdir=$PODMAN_TMPDIR/cp-test-in
    dstdir=$PODMAN_TMPDIR/cp-test-out
    mkdir -p $srcdir $dstdir

    rand_filename1=$(random_string 20)
    rand_content1=$(random_string 50)
    echo $rand_content1 > $srcdir/$rand_filename1

    rand_filename2=$(random_string 20)
    rand_content2=$(random_string 50)
    echo $rand_content2 > $srcdir/$rand_filename2

    rand_filename3=$(random_string 20)
    rand_content3=$(random_string 50)
    echo $rand_content3 > $srcdir/$rand_filename3

    # Create tmp subdirectories in container, most with an invalid 'x' symlink
    # Keep container running so we can exec into it.
    run_podman run -d --name cpcontainer $IMAGE \
               sh -c "mkdir /tmp/d1;ln -s /tmp/nonesuch1 /tmp/d1/x;
                      mkdir /tmp/d2;ln -s /tmp/nonesuch2 /tmp/d2/x;
                      mkdir /tmp/d3;
                      trap 'exit 0' 15;while :;do sleep 0.5;done"

    # Copy file from host into container, into a file named 'x'
    # Note that the second has a trailing slash, implying a directory.
    # Since that destination directory doesn't exist, the cp will fail
    run_podman cp --pause=false $srcdir/$rand_filename1 cpcontainer:/tmp/d1/x
    is "$output" "" "output from podman cp 1"

    run_podman 125 cp --pause=false $srcdir/$rand_filename2 cpcontainer:/tmp/d2/x/
    is "$output" ".*stat.* no such file or directory" "cp will not create nonexistent destination directory"

    run_podman cp --pause=false $srcdir/$rand_filename3 cpcontainer:/tmp/d3/x
    is "$output" "" "output from podman cp 3"

    # Read back.
    # In the first case, podman actually creates the file nonesuch1 (i.e.
    # podman expands 'x -> nonesuch1' and, instead of overwriting x,
    # creates an actual file).
    run_podman exec cpcontainer cat /tmp/nonesuch1
    is "$output" "$rand_content1" "cp creates destination file"

    # cp into nonexistent directory should not mkdir nonesuch2 directory
    run_podman 1 exec cpcontainer test -e /tmp/nonesuch2

    # In the third case, podman (correctly imo) creates a file named 'x'
    run_podman exec cpcontainer cat /tmp/d3/x
    is "$output" "$rand_content3" "cp creates file named x"

    run_podman rm -f cpcontainer


}


# rhbz1741718 : file copied into container:/var/lib/foo appears as /foo
# (docker only, never seems to have affected podman. Make sure it never does).
@test "podman cp into a subdirectory matching GraphRoot" {
    skip_if_remote "podman-remote does not yet handle cp"

    # Create tempfile with random name and content
    srcdir=$PODMAN_TMPDIR/cp-test-in
    mkdir -p $srcdir
    rand_filename=$(random_string 20)
    rand_content=$(random_string 50)
    echo $rand_content > $srcdir/$rand_filename
    chmod 644 $srcdir/$rand_filename

    # Determine path to podman storage (eg /var/lib/c/s, or $HOME/.local/...)
    run_podman info --format '{{.Store.GraphRoot}}'
    graphroot=$output

    # Create that directory in the container, and sleep (to keep container
    # running, so we can exec into it). The trap/while is so podman-rm will
    # run quickly instead of taking 10 seconds.
    run_podman run -d --name cpcontainer $IMAGE sh -c \
               "mkdir -p $graphroot; trap 'exit 0' 15;while :;do sleep 0.5;done"

    # Copy from host into container.
    run_podman cp --pause=false $srcdir/$rand_filename cpcontainer:$graphroot/$rand_filename

    # ls, and confirm it's there.
    run_podman exec cpcontainer ls -l $graphroot/$rand_filename
    is "$output" "-rw-r--r-- .* 1 .* root .* 51 .* $graphroot/$rand_filename" \
       "File is copied into container in the correct (full) path"

    # Confirm it has the expected content (this is unlikely to ever fail)
    run_podman exec cpcontainer cat $graphroot/$rand_filename
    is "$output" "$rand_content" "Contents of file copied into container"

    run_podman rm -f cpcontainer
}


function teardown() {
    # In case any test fails, clean up the container we left behind
    run_podman rm -f cpcontainer
    basic_teardown
}

# vim: filetype=sh
