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
    # We use '-n -v' to give us log messages showing an incoming connection
    # and its IP address; the purpose of that is guaranteeing that the
    # remote IP is not 127.0.0.1 (podman PR #9052).
    # We could get more parseable output by using $NCAT_REMOTE_ADDR,
    # but busybox nc doesn't support that.
    run_podman run -d --userns=keep-id -p 127.0.0.1:$myport:$myport \
               $IMAGE nc -l -n -v -p $myport
    cid="$output"

    # emit random string, and check it
    teststring=$(random_string 30)
    echo "$teststring" | nc 127.0.0.1 $myport

    run_podman logs $cid
    # Sigh. We can't check line-by-line, because 'nc' output order is
    # unreliable. We usually get the 'connect to' line before the random
    # string, but sometimes we get it after. So, just do substring checks.
    is "$output" ".*listening on \[::\]:$myport .*" "nc -v shows right port"

    # This is the truly important check: make sure the remote IP is
    # in the 10.X range, not 127.X.
    is "$output" \
       ".*connect to \[::ffff:10\..*\]:$myport from \[::ffff:10\..*\]:.*" \
       "nc -v shows remote IP address in 10.X space (not 127.0.0.1)"
    is "$output" ".*${teststring}.*" "test string received on container"

    # Clean up
    run_podman rm $cid
}

# "network create" now works rootless, with the help of a special container
@test "podman network create" {
    myport=54322

    local mynetname=testnet-$(random_string 10)
    local mysubnet=$(random_rfc1918_subnet)

    run_podman network create --subnet "${mysubnet}.0/24" $mynetname
    is "$output" ".*/cni/net.d/$mynetname.conflist" "output of 'network create'"

    # (Assert that output is formatted, not a one-line blob: #8011)
    run_podman network inspect $mynetname
    if [[ "${#lines[*]}" -lt 5 ]]; then
        die "Output from 'pod inspect' is only ${#lines[*]} lines; see #8011"
    fi

    run_podman run --rm --network $mynetname $IMAGE ip a
    is "$output" ".* inet ${mysubnet}\.2/24 brd ${mysubnet}\.255 " \
       "sdfsdf"

    run_podman run --rm -d --network $mynetname -p 127.0.0.1:$myport:$myport \
               $IMAGE nc -l -n -v -p $myport
    cid="$output"

    # emit random string, and check it
    teststring=$(random_string 30)
    echo "$teststring" | nc 127.0.0.1 $myport

    run_podman logs $cid
    # Sigh. We can't check line-by-line, because 'nc' output order is
    # unreliable. We usually get the 'connect to' line before the random
    # string, but sometimes we get it after. So, just do substring checks.
    is "$output" ".*listening on \[::\]:$myport .*" "nc -v shows right port"

    # This is the truly important check: make sure the remote IP is
    # in the 172.X range, not 127.X.
    is "$output" \
       ".*connect to \[::ffff:172\..*\]:$myport from \[::ffff:172\..*\]:.*" \
       "nc -v shows remote IP address in 172.X space (not 127.0.0.1)"
    is "$output" ".*${teststring}.*" "test string received on container"

    # Cannot create network with the same name
    run_podman 125 network create $mynetname
    is "$output" "Error: the network name $mynetname is already used" \
       "Trying to create an already-existing network"

    run_podman network rm $mynetname
    run_podman 1 network rm $mynetname

    # rootless CNI leaves behind an image pulled by SHA, hence with no tag.
    # Remove it if present; we can only remove it by ID.
    run_podman images --format '{{.Id}}' rootless-cni-infra
    if [ -n "$output" ]; then
        run_podman rmi $output
    fi
}

@test "podman network reload" {
    skip_if_remote "podman network reload does not have remote support"
    skip_if_rootless "podman network reload does not work rootless"

    random_1=$(random_string 30)
    HOST_PORT=12345
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

    run_podman inspect $cid --format "{{.NetworkSettings.IPAddress}}"
    ip="$output"
    run_podman inspect $cid --format "{{.NetworkSettings.MacAddress}}"
    mac="$output"

    # Verify http contents: curl from localhost
    run curl -s $SERVER/index.txt
    is "$output" "$random_1" "curl 127.0.0.1:/index.txt"

    # flush the CNI iptables here
    run iptables -t nat -F CNI-HOSTPORT-DNAT

    # check that we cannot curl (timeout after 5 sec)
    run timeout 5 curl -s $SERVER/index.txt
    if [ "$status" -ne 124 ]; then
        die "curl did not timeout, status code: $status"
    fi

    # reload the network to recreate the iptables rules
    run_podman network reload $cid
    is "$output" "$cid" "Output does not match container ID"

    # check that we still have the same mac and ip
    run_podman inspect $cid --format "{{.NetworkSettings.IPAddress}}"
    is "$output" "$ip" "IP address changed after podman network reload"
    run_podman inspect $cid --format "{{.NetworkSettings.MacAddress}}"
    is "$output" "$mac" "MAC address changed after podman network reload"

    # check that we can still curl
    run curl -s $SERVER/index.txt
    is "$output" "$random_1" "curl 127.0.0.1:/index.txt"

    # make sure --all is working and that this
    # cmd also works if the iptables still exists
    run_podman network reload --all
    is "$output" "$cid" "Output does not match container ID"

    # check that we can still curl
    run curl -s $SERVER/index.txt
    is "$output" "$random_1" "curl 127.0.0.1:/index.txt"

    # cleanup the container
    run_podman rm -f $cid
}

# vim: filetype=sh
