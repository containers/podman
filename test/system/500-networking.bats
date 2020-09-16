#!/usr/bin/env bats   -*- bats -*-
#
# Test podman local networking
#

load helpers

# Copied from tsweeney's https://github.com/containers/podman/issues/4827
@test "podman networking: port on localhost" {
    skip_if_remote "FIXME: reevaluate this one after #7360 is fixed"
    random_1=$(random_string 30)
    random_2=$(random_string 30)

    HOST_PORT=8080
    SERVER=http://127.0.0.1:$HOST_PORT

    # Create a test file with random content
    INDEX1=$PODMAN_TMPDIR/hello.txt
    echo $random_1 > $INDEX1

    # Bind-mount this file with a different name to a container running httpd
    run_podman run -d --name myweb -p "$HOST_PORT:80" \
               -v $INDEX1:/var/www/index.txt \
               -w /var/www \
               $IMAGE /bin/busybox-extras httpd -f -p 80
    cid=$output

    # In that container, create a second file, using exec and redirection
    run_podman exec -i myweb sh -c "cat > index2.txt" <<<"$random_2"
    # ...verify its contents as seen from container.
    run_podman exec -i myweb cat /var/www/index2.txt
    is "$output" "$random_2" "exec cat index2.txt"

    # Verify http contents: curl from localhost
    run curl -s $SERVER/index.txt
    is "$output" "$random_1" "curl 127.0.0.1:/index.txt"
    run curl -s $SERVER/index2.txt
    is "$output" "$random_2" "curl 127.0.0.1:/index2.txt"

    # Verify http contents: wget from a second container
    run_podman run --rm --net=host $IMAGE wget -qO - $SERVER/index.txt
    is "$output" "$random_1" "podman wget /index.txt"
    run_podman run --rm --net=host $IMAGE wget -qO - $SERVER/index2.txt
    is "$output" "$random_2" "podman wget /index2.txt"

    # Tests #4889 - two-argument form of "podman ports" was broken
    run_podman port myweb
    is "$output" "80/tcp -> 0.0.0.0:$HOST_PORT" "port <cid>"
    run_podman port myweb 80
    is "$output" "0.0.0.0:$HOST_PORT"  "port <cid> 80"
    run_podman port myweb 80/tcp
    is "$output" "0.0.0.0:$HOST_PORT"  "port <cid> 80/tcp"

    run_podman 125 port myweb 99/tcp
    is "$output" 'Error: failed to find published port "99/tcp"'

    # Clean up
    run_podman stop -t 1 myweb
    run_podman rm myweb
}

# Issue #5466 - port-forwarding doesn't work with this option and -d
@test "podman networking: port with --userns=keep-id" {
    # FIXME: randomize port, and create second random host port
    myport=54321

    # Container will exit as soon as 'nc' receives input
    run_podman run -d --userns=keep-id -p 127.0.0.1:$myport:$myport \
               $IMAGE nc -l -p $myport
    cid="$output"

    # emit random string, and check it
    teststring=$(random_string 30)
    echo "$teststring" | nc 127.0.0.1 $myport

    run_podman logs $cid
    is "$output" "$teststring" "test string received on container"

    # Clean up
    run_podman rm $cid
}

# "network create" now works rootless, with the help of a special container
@test "podman network create" {
    local mynetname=testnet-$(random_string 10)
    local mysubnet=$(random_rfc1918_subnet)

    run_podman network create --subnet "${mysubnet}.0/24" $mynetname
    is "$output" ".*/cni/net.d/$mynetname.conflist" "output of 'network create'"

    # WARNING: this pulls a ~100MB image from quay.io, hence is slow/flaky
    run_podman run --rm --network $mynetname $IMAGE ip a
    is "$output" ".* inet ${mysubnet}\.2/24 brd ${mysubnet}\.255 " \
       "sdfsdf"

    # Cannot create network with the same name
    run_podman 125 network create $mynetname
    is "$output" "Error: the network name $mynetname is already used" \
       "Trying to create an already-existing network"

    run_podman network rm $mynetname
    run_podman 125 network rm $mynetname

    # rootless CNI leaves behind an image pulled by SHA, hence with no tag.
    # Remove it if present; we can only remove it by ID.
    run_podman images --format '{{.Id}}' rootless-cni-infra
    if [ -n "$output" ]; then
        run_podman rmi $output
    fi
}

# vim: filetype=sh
