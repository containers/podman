#!/usr/bin/env bats   -*- bats -*-
#
# Tests for 'podman cp'
#
# ASSUMPTION FOR ALL THESE TESTS: /tmp in the container starts off empty
#

load helpers

@test "podman cp file from host to container" {
    srcdir=$PODMAN_TMPDIR/cp-test-file-host-to-ctr
    mkdir -p $srcdir
    local -a randomcontent=(
        random-0-$(random_string 10)
        random-1-$(random_string 15)
        random-2-$(random_string 20)
    )
    echo "${randomcontent[0]}" > $srcdir/hostfile0
    echo "${randomcontent[1]}" > $srcdir/hostfile1
    echo "${randomcontent[2]}" > $srcdir/hostfile2

    run_podman run -d --name cpcontainer --workdir=/srv $IMAGE sleep infinity
    run_podman exec cpcontainer mkdir /srv/subdir

    # format is: <id> | <destination arg to cp> | <full dest path> | <test name>
    # where:
    #    id        is 0-2, one of the random strings/files
    #    dest arg  is the right-hand argument to 'podman cp' (may be implicit)
    #    dest path is the full explicit path we expect to see
    #    test name is a short description of what we're testing here
    tests="
0 | /                    | /hostfile0            | copy to root
0 | /anotherbase.txt     | /anotherbase.txt      | copy to root, new name
0 | /tmp                 | /tmp/hostfile0        | copy to /tmp
1 | /tmp/                | /tmp/hostfile1        | copy to /tmp/
2 | /tmp/.               | /tmp/hostfile2        | copy to /tmp/.
0 | /tmp/hostfile2       | /tmp/hostfile2        | overwrite previous copy
0 | /tmp/anotherbase.txt | /tmp/anotherbase.txt  | copy to /tmp, new name
0 | .                    | /srv/hostfile0        | copy to workdir (rel path), new name
1 | ./                   | /srv/hostfile1        | copy to workdir (rel path), new name
0 | anotherbase.txt      | /srv/anotherbase.txt  | copy to workdir (rel path), new name
0 | subdir               | /srv/subdir/hostfile0 | copy to workdir/subdir
"

    # Copy one of the files into container, exec+cat, confirm the file
    # is there and matches what we expect
    while read id dest dest_fullname description; do
        run_podman cp $srcdir/hostfile$id cpcontainer:$dest
        run_podman exec cpcontainer cat $dest_fullname
        is "$output" "${randomcontent[$id]}" "$description (cp -> ctr:$dest)"
    done < <(parse_table "$tests")

    # Host path does not exist.
    run_podman 125 cp $srcdir/IdoNotExist cpcontainer:/tmp
    is "$output" 'Error: ".*/IdoNotExist" could not be found on the host' \
       "copy nonexistent host path"

    # Container (parent) path does not exist.
    run_podman 125 cp $srcdir/hostfile0 cpcontainer:/IdoNotExist/
    is "$output" 'Error: "/IdoNotExist/" could not be found on container cpcontainer: No such file or directory' \
       "copy into nonexistent path in container"

    run_podman rm -f cpcontainer
}


@test "podman cp file from container to host" {
    srcdir=$PODMAN_TMPDIR/cp-test-file-ctr-to-host
    mkdir -p $srcdir

    # Create 3 files with random content in the container.
    local -a randomcontent=(
        random-0-$(random_string 10)
        random-1-$(random_string 15)
        random-2-$(random_string 20)
    )
    run_podman run -d --name cpcontainer --workdir=/srv $IMAGE sleep infinity
    run_podman exec cpcontainer sh -c "echo ${randomcontent[0]} > /tmp/containerfile"
    run_podman exec cpcontainer sh -c "echo ${randomcontent[1]} > /srv/containerfile1"
    run_podman exec cpcontainer sh -c "mkdir /srv/subdir; echo ${randomcontent[2]} > /srv/subdir/containerfile2"

    # format is: <id> | <source arg to cp> | <destination arg (appended to $srcdir) to cp> | <full dest path (appended to $srcdir)> | <test name>
    tests="
0 | /tmp/containerfile    |          | /containerfile  | copy to srcdir/
0 | /tmp/containerfile    | /        | /containerfile  | copy to srcdir/
0 | /tmp/containerfile    | /.       | /containerfile  | copy to srcdir/.
0 | /tmp/containerfile    | /newfile | /newfile        | copy to srcdir/newfile
1 | containerfile1        | /        | /containerfile1 | copy from workdir (rel path) to srcdir
2 | subdir/containerfile2 | /        | /containerfile2 | copy from workdir/subdir (rel path) to srcdir
"

    # Copy one of the files to the host, cat, confirm the file
    # is there and matches what we expect
    while read id src dest dest_fullname description; do
        # dest may be "''" for empty table cells
        if [[ $dest == "''" ]];then
            unset dest
        fi
        run_podman cp cpcontainer:$src "$srcdir$dest"
        run cat $srcdir$dest_fullname
        is "$output" "${randomcontent[$id]}" "$description (cp ctr:$src to \$srcdir$dest)"
        rm $srcdir/$dest_fullname
    done < <(parse_table "$tests")

    run_podman rm -f cpcontainer
}


@test "podman cp dir from host to container" {
    dirname=dir-test
    srcdir=$PODMAN_TMPDIR/$dirname
    mkdir -p $srcdir
    local -a randomcontent=(
        random-0-$(random_string 10)
        random-1-$(random_string 15)
    )
    echo "${randomcontent[0]}" > $srcdir/hostfile0
    echo "${randomcontent[1]}" > $srcdir/hostfile1

    run_podman run -d --name cpcontainer --workdir=/srv $IMAGE sleep infinity
    run_podman exec cpcontainer mkdir /srv/subdir

    # format is: <source arg to cp (appended to srcdir)> | <destination arg to cp> | <full dest path> | <test name>
    tests="
    | /        | /dir-test             | copy to root
 /  | /tmp     | /tmp/dir-test         | copy to tmp
 /. | /usr/    | /usr/                 | copy contents of dir to usr/
    | .        | /srv/dir-test         | copy to workdir (rel path)
    | subdir/. | /srv/subdir/dir-test | copy to workdir subdir (rel path)
"

    while read src dest dest_fullname description; do
        # src may be "''" for empty table cells
        if [[ $src == "''" ]];then
            unset src
        fi
        run_podman cp $srcdir$src cpcontainer:$dest
        run_podman exec cpcontainer ls $dest_fullname
        run_podman exec cpcontainer cat $dest_fullname/hostfile0
        is "$output" "${randomcontent[0]}" "$description (cp -> ctr:$dest)"
        run_podman exec cpcontainer cat $dest_fullname/hostfile1
        is "$output" "${randomcontent[1]}" "$description (cp -> ctr:$dest)"
    done < <(parse_table "$tests")

    run_podman rm -f cpcontainer
}


@test "podman cp dir from container to host" {
    srcdir=$PODMAN_TMPDIR/dir-test
    mkdir -p $srcdir

    run_podman run -d --name cpcontainer --workdir=/srv $IMAGE sleep infinity
    run_podman exec cpcontainer sh -c 'mkdir /srv/subdir; echo "This first file is on the container" > /srv/subdir/containerfile1'
    run_podman exec cpcontainer sh -c 'echo "This second file is on the container as well" > /srv/subdir/containerfile2'

    run_podman cp cpcontainer:/srv $srcdir
    run cat $srcdir/srv/subdir/containerfile1
    is "$output" "This first file is on the container"
    run cat $srcdir/srv/subdir/containerfile2
    is "$output" "This second file is on the container as well"
    rm -rf $srcdir/srv/subdir

    run_podman cp cpcontainer:/srv/. $srcdir
    run ls $srcdir/subdir
    run cat $srcdir/subdir/containerfile1
    is "$output" "This first file is on the container"
    run cat $srcdir/subdir/containerfile2
    is "$output" "This second file is on the container as well"
    rm -rf $srcdir/subdir

    run_podman cp cpcontainer:/srv/subdir/. $srcdir
    run cat $srcdir/containerfile1
    is "$output" "This first file is on the container"
    run cat $srcdir/containerfile2
    is "$output" "This second file is on the container as well"

    run_podman rm -f cpcontainer
}


@test "podman cp file from host to container volume" {
    srcdir=$PODMAN_TMPDIR/cp-test-volume
    mkdir -p $srcdir
    echo "This file should be in volume2" > $srcdir/hostfile
    volume1=$(random_string 20)
    volume2=$(random_string 20)

    run_podman volume create $volume1
    run_podman volume inspect $volume1 --format "{{.Mountpoint}}"
    volume1_mount="$output"
    run_podman volume create $volume2
    run_podman volume inspect $volume2 --format "{{.Mountpoint}}"
    volume2_mount="$output"

    # Create a container using the volume.  Note that copying on not-running
    # containers is allowed, so Podman has to analyze the container paths and
    # check if they are hitting a volume, and eventually resolve to the path on
    # the *host*.
    # This test is extra tricky, as volume2 is mounted into a sub-directory of
    # volume1.  Podman must copy the file into volume2 and not volume1.
    run_podman create --name cpcontainer -v $volume1:/tmp/volume -v $volume2:/tmp/volume/sub-volume $IMAGE

    run_podman cp $srcdir/hostfile cpcontainer:/tmp/volume/sub-volume

    run cat $volume2_mount/hostfile
    is "$output" "This file should be in volume2"

    # Volume 1 must be empty.
    run ls $volume1_mount
    is "$output" ""

    run_podman rm -f cpcontainer
    run_podman volume rm $volume1 $volume2
}


@test "podman cp file from host to container mount" {
    srcdir=$PODMAN_TMPDIR/cp-test-mount-src
    mountdir=$PODMAN_TMPDIR/cp-test-mount
    mkdir -p $srcdir $mountdir
    echo "This file should be in the mount" > $srcdir/hostfile

    volume=$(random_string 20)
    run_podman volume create $volume

    # Make it a bit more complex and put the mount on a volume.
    run_podman create --name cpcontainer -v $volume:/tmp/volume -v $mountdir:/tmp/volume/mount $IMAGE

    run_podman cp $srcdir/hostfile cpcontainer:/tmp/volume/mount

    run cat $mountdir/hostfile
    is "$output" "This file should be in the mount"

    run_podman rm -f cpcontainer
    run_podman volume rm $volume
}


# Create two random-name random-content files in /tmp in the container
# podman-cp them into the host using '/tmp/*', i.e. asking podman to
# perform wildcard expansion in the container. We should get both
# files copied into the host.
@test "podman cp * - wildcard copy multiple files from container to host" {
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
    is "$output" 'Error: "/tmp/\*" could not be found on container.*'

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
    srcdir=$PODMAN_TMPDIR/cp-test-in
    dstdir=$PODMAN_TMPDIR/cp-test-out
    mkdir -p $srcdir $dstdir
    echo "This file is on the host" > $srcdir/hostfile

    run_podman run --name cpcontainer $IMAGE \
               sh -c "ln -s $srcdir/hostfile file1;ln -s file\* copyme"
    run_podman 125 cp cpcontainer:copyme $dstdir

    is "$output" 'Error: "copyme*" could not be found on container.*'

    # make sure there are no files in dstdir
    is "$(/bin/ls -1 $dstdir)" "" "incorrectly copied symlink from host"

    run_podman rm cpcontainer
}


# Another symlink into host space, this one named '*' (star). cp should fail.
@test "podman cp - will not expand wildcard" {
    srcdir=$PODMAN_TMPDIR/cp-test-in
    dstdir=$PODMAN_TMPDIR/cp-test-out
    mkdir -p $srcdir $dstdir
    echo "This file lives on the host" > $srcdir/hostfile

    run_podman run --name cpcontainer $IMAGE \
               sh -c "ln -s $srcdir/hostfile /tmp/\*"
    run_podman 125 cp 'cpcontainer:/tmp/*' $dstdir

    is "$output" 'Error: "/tmp/\*" could not be found on container.*'

    # dstdir must be empty
    is "$(/bin/ls -1 $dstdir)" "" "incorrectly copied symlink from host"

    run_podman rm cpcontainer
}


# THIS IS EXTREMELY WEIRD. Podman expands symlinks in weird ways.
@test "podman cp into container: weird symlink expansion" {
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
    is "$output" 'Error: "/tmp/d2/x/" could not be found on container cpcontainer: No such file or directory' "cp will not create nonexistent destination directory"

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


@test "podman cp from stdin to container" {
    # Create tempfile with random name and content
    srcdir=$PODMAN_TMPDIR/cp-test-stdin
    mkdir -p $srcdir
    rand_filename=$(random_string 20)
    rand_content=$(random_string 50)
    echo $rand_content > $srcdir/$rand_filename
    chmod 644 $srcdir/$rand_filename

    # Now tar it up!
    tar_file=$PODMAN_TMPDIR/archive.tar.gz
    tar -zvcf $tar_file $srcdir

    run_podman run -d --name cpcontainer $IMAGE sleep infinity

    # NOTE: podman is supposed to auto-detect the gzip compression and
    # decompress automatically.
    #
    # "-" will evaluate to "/dev/stdin" when used a source.
    run_podman cp - cpcontainer:/tmp < $tar_file
    run_podman exec cpcontainer cat /tmp/$srcdir/$rand_filename
    is "$output" "$rand_content"
    run_podman exec cpcontainer rm -rf /tmp/$srcdir

    # Now for "/dev/stdin".
    run_podman cp /dev/stdin cpcontainer:/tmp < $tar_file
    run_podman exec cpcontainer cat /tmp/$srcdir/$rand_filename
    is "$output" "$rand_content"

    # Error checks below ...

    # Input stream must be a (compressed) tar archive.
    run_podman 125 cp - cpcontainer:/tmp < $srcdir/$rand_filename
    is "$output" "Error: source must be a (compressed) tar archive when copying from stdin"

    # Destination must be a directory (on an existing file).
    run_podman exec cpcontainer touch /tmp/file.txt
    run_podman 125 cp /dev/stdin cpcontainer:/tmp/file.txt < $tar_file
    is "$output" 'Error: destination must be a directory when copying from stdin'

    # Destination must be a directory (on an absent path).
    run_podman 125 cp /dev/stdin cpcontainer:/tmp/IdoNotExist < $tar_file
    is "$output" 'Error: destination must be a directory when copying from stdin'

    run_podman rm -f cpcontainer
}


@test "podman cp from container to stdout" {
    srcdir=$PODMAN_TMPDIR/cp-test-stdout
    mkdir -p $srcdir
    rand_content=$(random_string 50)

    run_podman run -d --name cpcontainer $IMAGE sleep infinity

    run_podman exec cpcontainer sh -c "echo '$rand_content' > /tmp/file.txt"
    run_podman exec cpcontainer touch /tmp/empty.txt

    # Copying from stdout will always compress.  So let's copy the previously
    # created file from the container via stdout, untar the archive and make
    # sure the file exists with the expected content.
    #
    # NOTE that we can't use run_podman because that uses the BATS 'run'
    # function which redirects stdout and stderr. Here we need to guarantee
    # that podman's stdout is a pipe, not any other form of redirection.

    # Copy file.
    $PODMAN cp cpcontainer:/tmp/file.txt - > $srcdir/stdout.tar
    if [ $? -ne 0 ]; then
        die "Command failed: podman cp ... - | cat"
    fi

    tar xvf $srcdir/stdout.tar -C $srcdir
    run cat $srcdir/file.txt
    is "$output" "$rand_content"
    run 1 ls $srcdir/empty.txt
    rm -f $srcdir/*

    # Copy directory.
    $PODMAN cp cpcontainer:/tmp - > $srcdir/stdout.tar
    if [ $? -ne 0 ]; then
        die "Command failed: podman cp ... - | cat : $output"
    fi

    tar xvf $srcdir/stdout.tar -C $srcdir
    run cat $srcdir/tmp/file.txt
    is "$output" "$rand_content"
    run cat $srcdir/tmp/empty.txt
    is "$output" ""

    run_podman rm -f cpcontainer
}

function teardown() {
    # In case any test fails, clean up the container we left behind
    run_podman rm -f cpcontainer
    basic_teardown
}

# vim: filetype=sh
