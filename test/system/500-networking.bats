#!/usr/bin/env bats   -*- bats -*-
#
# Test podman local networking
#

load helpers

@test "podman network - basic tests" {
    heading="*NETWORK*ID*NAME*VERSION*PLUGINS*"
    run_podman network ls
    if  [[ ${output} != ${heading} ]]; then
       die "network ls expected heading is not available"
    fi

    run_podman network ls --noheading
    if  [[ ${output} = ${heading} ]]; then
       die "network ls --noheading did not remove heading: $output"
    fi
}

# Copied from tsweeney's https://github.com/containers/podman/issues/4827
@test "podman networking: port on localhost" {
    random_1=$(random_string 30)
    random_2=$(random_string 30)

    HOST_PORT=$(random_free_port)
    SERVER=http://127.0.0.1:$HOST_PORT

    # Create a test file with random content
    INDEX1=$PODMAN_TMPDIR/hello.txt
    echo $random_1 > $INDEX1

    # Bind-mount this file with a different name to a container running httpd
    run_podman run -d --name myweb -p "$HOST_PORT:80" \
            -v $INDEX1:/var/www/index.txt:Z \
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
    for cidr in "" "$(random_rfc1918_subnet).0/24"; do
        myport=$(random_free_port 52000-52999)
        if [[ -z $cidr ]]; then
            # regex to match that we are in 10.X subnet
            match="10\..*"
        else
            # Issue #9828 make sure a custom slir4netns cidr also works
            network_arg="--network slirp4netns:cidr=$cidr"
            # slirp4netns interface ip is always .100
            match="${cidr%.*}.100"
        fi

        # Container will exit as soon as 'nc' receives input
        # We use '-n -v' to give us log messages showing an incoming connection
        # and its IP address; the purpose of that is guaranteeing that the
        # remote IP is not 127.0.0.1 (podman PR #9052).
        # We could get more parseable output by using $NCAT_REMOTE_ADDR,
        # but busybox nc doesn't support that.
        run_podman run -d --userns=keep-id $network_arg -p 127.0.0.1:$myport:$myport \
                   $IMAGE nc -l -n -v -p $myport
        cid="$output"

        wait_for_output "listening on .*:$myport .*" $cid

        # emit random string, and check it
        teststring=$(random_string 30)
        echo "$teststring" | nc 127.0.0.1 $myport

        run_podman logs $cid
        # Sigh. We can't check line-by-line, because 'nc' output order is
        # unreliable. We usually get the 'connect to' line before the random
        # string, but sometimes we get it after. So, just do substring checks.
        is "$output" ".*listening on \[::\]:$myport .*" "nc -v shows right port"

        # This is the truly important check: make sure the remote IP is not 127.X.
        is "$output" \
           ".*connect to \[::ffff:$match*\]:$myport from \[::ffff:$match\]:.*" \
           "nc -v shows remote IP address is not 127.0.0.1"
        is "$output" ".*${teststring}.*" "test string received on container"

        # Clean up
        run_podman wait $cid
        run_podman rm $cid
    done
}

@test "podman run with slirp4ns assigns correct addresses to /etc/hosts" {
    CIDR="$(random_rfc1918_subnet)"
    local conname=con-$(random_string 10)
    run_podman run --rm --network slirp4netns:cidr="${CIDR}.0/24" \
                --name $conname --hostname $conname $IMAGE cat /etc/hosts
    is "$output"   ".*${CIDR}.2 host.containers.internal"   "host.containers.internal should be the cidr+2 address"
    is "$output"   ".*${CIDR}.100	$conname $conname"   "$conname should be the cidr+100 address"
}

@test "podman run with slirp4ns adds correct dns address to resolv.conf" {
    CIDR="$(random_rfc1918_subnet)"
    run_podman run --rm --network slirp4netns:cidr="${CIDR}.0/24" \
                $IMAGE grep "${CIDR}" /etc/resolv.conf
    is "$output"   "nameserver ${CIDR}.3"   "resolv.conf should have slirp4netns cidr+3 as a nameserver"
}

@test "podman run with slirp4ns assigns correct ip address container" {
    CIDR="$(random_rfc1918_subnet)"
    run_podman run --rm --network slirp4netns:cidr="${CIDR}.0/24" \
                $IMAGE sh -c "ip address | grep ${CIDR}"
    is "$output"   ".*inet ${CIDR}.100/24 \+"   "container should have slirp4netns cidr+100 assigned to interface"
}

# "network create" now works rootless, with the help of a special container
@test "podman network create" {
    # Deliberately use a fixed port, not random_open_port, because of #10806
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

    run_podman run -d --network $mynetname -p 127.0.0.1:$myport:$myport \
	       $IMAGE nc -l -n -v -p $myport
    cid="$output"

    # FIXME: debugging for #11871
    run_podman exec $cid cat /etc/resolv.conf
    if is_rootless && ! is_remote; then
        run_podman unshare --rootless-cni cat /etc/resolv.conf
    fi
    ps uxww

    # check that dns is working inside the container
    run_podman exec $cid nslookup google.com

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

    run_podman rm $cid
    run_podman network rm $mynetname
    run_podman 1 network rm $mynetname
}

@test "podman network reload" {
    skip_if_remote "podman network reload does not have remote support"

    random_1=$(random_string 30)
    HOST_PORT=$(random_free_port)
    SERVER=http://127.0.0.1:$HOST_PORT

    # Create a test file with random content
    INDEX1=$PODMAN_TMPDIR/hello.txt
    echo $random_1 > $INDEX1

    # use default network for root
    local netname=podman
    # for rootless we have to create a custom network since there is no default network
    if is_rootless; then
        netname=testnet-$(random_string 10)
        run_podman network create $netname
        is "$output" ".*/cni/net.d/$netname.conflist" "output of 'network create'"
    fi

    # Bind-mount this file with a different name to a container running httpd
    run_podman run -d --name myweb -p "$HOST_PORT:80" \
            --network $netname \
            -v $INDEX1:/var/www/index.txt:Z \
            -w /var/www \
            $IMAGE /bin/busybox-extras httpd -f -p 80
    cid=$output

    run_podman inspect $cid --format "{{(index .NetworkSettings.Networks \"$netname\").IPAddress}}"
    ip="$output"
    run_podman inspect $cid --format "{{(index .NetworkSettings.Networks \"$netname\").MacAddress}}"
    mac="$output"

    # Verify http contents: curl from localhost
    run curl -s $SERVER/index.txt
    is "$output" "$random_1" "curl 127.0.0.1:/index.txt"

    # rootless cannot modify iptables
    if ! is_rootless; then
        # flush the CNI iptables here
        run iptables -t nat -F CNI-HOSTPORT-DNAT

        # check that we cannot curl (timeout after 5 sec)
        run timeout 5 curl -s $SERVER/index.txt
        if [ "$status" -ne 124 ]; then
	        die "curl did not timeout, status code: $status"
        fi
    fi

    # reload the network to recreate the iptables rules
    run_podman network reload $cid
    is "$output" "$cid" "Output does not match container ID"

    # check that we still have the same mac and ip
    run_podman inspect $cid --format "{{(index .NetworkSettings.Networks \"$netname\").IPAddress}}"
    is "$output" "$ip" "IP address changed after podman network reload"
    run_podman inspect $cid --format "{{(index .NetworkSettings.Networks \"$netname\").MacAddress}}"
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

    if is_rootless; then
        run_podman network rm -f $netname
    fi
}

@test "podman rootless cni adds /usr/sbin to PATH" {
    is_rootless || skip "only meaningful for rootless"

    local mynetname=testnet-$(random_string 10)
    run_podman network create $mynetname

    # Test that rootless cni adds /usr/sbin to $PATH
    # iptables is located under /usr/sbin and is needed for the CNI plugins.
    # Debian doesn't add /usr/sbin to $PATH for rootless users so we have to add it.
    PATH=/usr/local/bin:/usr/bin run_podman run --rm --network $mynetname $IMAGE ip addr
    is "$output" ".*eth0.*" "Interface eth0 not found in ip addr output"

    run_podman network rm -f $mynetname
}

@test "podman ipv6 in /etc/resolv.conf" {
    ipv6_regex='([0-9A-Fa-f]{0,4}:){2,7}([0-9A-Fa-f]{0,4})(%\w+)?'

    # Make sure to read the correct /etc/resolv.conf file in case of systemd-resolved.
    resolve_file=$(readlink -f /etc/resolv.conf)
    if [[ "$resolve_file" == "/run/systemd/resolve/stub-resolv.conf" ]]; then
        resolve_file="/run/systemd/resolve/resolv.conf"
    fi

    # If the host doesn't have an ipv6 in resolv.conf skip this test.
    # We should never modify resolv.conf on the host.
    if ! grep -E "$ipv6_regex" "$resolve_file"; then
        skip "This test needs an ipv6 nameserver in $resolve_file"
    fi

    # ipv4 slirp
    run_podman run --rm --network slirp4netns:enable_ipv6=false $IMAGE cat /etc/resolv.conf
    if grep -E "$ipv6_regex" <<< $output; then
        die "resolv.conf contains a ipv6 nameserver"
    fi

    # ipv6 slirp
    run_podman run --rm --network slirp4netns:enable_ipv6=true $IMAGE cat /etc/resolv.conf
    # "is" does not like the ipv6 regex
    if ! grep -E "$ipv6_regex" <<< $output; then
        die "resolv.conf does not contain a ipv6 nameserver"
    fi

    # ipv4 cni
    local mysubnet=$(random_rfc1918_subnet)
    local netname=testnet-$(random_string 10)

    run_podman network create --subnet $mysubnet.0/24 $netname
    is "$output" ".*/cni/net.d/$netname.conflist" "output of 'network create'"

    run_podman run --rm --network $netname $IMAGE cat /etc/resolv.conf
    if grep -E "$ipv6_regex" <<< $output; then
        die "resolv.conf contains a ipv6 nameserver"
    fi

    run_podman network rm -f $netname

    # ipv6 cni
    mysubnet=fd00:4:4:4:4::/64
    netname=testnet-$(random_string 10)

    run_podman network create --subnet $mysubnet $netname
    is "$output" ".*/cni/net.d/$netname.conflist" "output of 'network create'"

    run_podman run --rm --network $netname $IMAGE cat /etc/resolv.conf
    # "is" does not like the ipv6 regex
    if ! grep -E "$ipv6_regex" <<< $output; then
        die "resolv.conf does not contain a ipv6 nameserver"
    fi

    run_podman network rm -f $netname
}

# Test for https://github.com/containers/podman/issues/10052
@test "podman network connect/disconnect with port forwarding" {
    random_1=$(random_string 30)
    HOST_PORT=$(random_free_port)
    SERVER=http://127.0.0.1:$HOST_PORT

    # Create a test file with random content
    INDEX1=$PODMAN_TMPDIR/hello.txt
    echo $random_1 > $INDEX1

    local netname=testnet-$(random_string 10)
    run_podman network create $netname
    is "$output" ".*/cni/net.d/$netname.conflist" "output of 'network create'"

    local netname2=testnet2-$(random_string 10)
    run_podman network create $netname2
    is "$output" ".*/cni/net.d/$netname2.conflist" "output of 'network create'"

    # First, run a container in background to ensure that the rootless cni ns
    # is not destroyed after network disconnect.
    run_podman run -d --network $netname $IMAGE top
    background_cid=$output

    # Run a httpd container on first network with exposed port
    run_podman run -d -p "$HOST_PORT:80" \
            --network $netname \
            -v $INDEX1:/var/www/index.txt:Z \
            -w /var/www \
            $IMAGE /bin/busybox-extras httpd -f -p 80
    cid=$output

    # Verify http contents: curl from localhost
    run curl --max-time 3 -s $SERVER/index.txt
    is "$output" "$random_1" "curl 127.0.0.1:/index.txt"

    run_podman inspect $cid --format "{{(index .NetworkSettings.Networks \"$netname\").IPAddress}}"
    ip="$output"
    run_podman inspect $cid --format "{{(index .NetworkSettings.Networks \"$netname\").MacAddress}}"
    mac="$output"

    run_podman network disconnect $netname $cid
    is "$output" "" "Output should be empty (no errors)"

    # check that we cannot curl (timeout after 3 sec)
    run curl --max-time 3 -s $SERVER/index.txt
    if [ "$status" -eq 0 ]; then
	    die "curl did not fail, it should have timed out or failed with non zero exit code"
    fi

    run_podman network connect $netname $cid
    is "$output" "" "Output should be empty (no errors)"

    # curl should work again
    run curl --max-time 3 -s $SERVER/index.txt
    is "$output" "$random_1" "curl 127.0.0.1:/index.txt should work again"

    # check that we have a new ip and mac
    # if the ip is still the same this whole test turns into a nop
    run_podman inspect $cid --format "{{(index .NetworkSettings.Networks \"$netname\").IPAddress}}"
    if [[ "$output" == "$ip" ]]; then
        die "IP address did not change after podman network disconnect/connect"
    fi
    run_podman inspect $cid --format "{{(index .NetworkSettings.Networks \"$netname\").MacAddress}}"
    if [[ "$output" == "$mac" ]]; then
        die "MAC address did not change after podman network disconnect/connect"
    fi

    # Disconnect/reconnect of a container *with no ports* should succeed quietly
    run_podman network disconnect $netname $background_cid
    is "$output" "" "disconnect of container with no open ports"
    run_podman network connect $netname $background_cid
    is "$output" "" "(re)connect of container with no open ports"

    # connect a second network
    run_podman network connect $netname2 $cid
    is "$output" "" "Output should be empty (no errors)"

    # curl should work
    run curl --max-time 3 -s $SERVER/index.txt
    is "$output" "$random_1" "curl 127.0.0.1:/index.txt should work"

    # disconnect the first network
    run_podman network disconnect $netname $cid

    # curl should still work
    run curl --max-time 3 -s $SERVER/index.txt
    is "$output" "$random_1" "curl 127.0.0.1:/index.txt should still work"

    # cleanup
    run_podman stop -t 0 $cid $background_cid
    run_podman rm -f $cid $background_cid
    run_podman network rm -f $netname $netname2
}

@test "podman network after restart" {
    random_1=$(random_string 30)

    HOST_PORT=$(random_free_port)
    SERVER=http://127.0.0.1:$HOST_PORT

    # Create a test file with random content
    INDEX1=$PODMAN_TMPDIR/hello.txt
    echo $random_1 > $INDEX1

    local netname=testnet-$(random_string 10)
    run_podman network create $netname
    is "$output" ".*/cni/net.d/$netname.conflist" "output of 'network create'"

    for network in "slirp4netns" "$netname"; do
        # Start container with the restart always policy
        run_podman run -d --name myweb -p "$HOST_PORT:80" \
                --restart always \
                --network $network \
                -v $INDEX1:/var/www/index.txt:Z \
                -w /var/www \
                $IMAGE /bin/busybox-extras httpd -f -p 80
        cid=$output

        # Tests #10310: podman will restart slirp4netns on container restart
        run_podman container inspect --format "{{.State.Pid}}" $cid
        pid=$output

        # Kill the process; podman restart policy will bring up a new container.
        # -9 is crucial: busybox httpd ignores all other signals.
        kill -9 $pid
        # Wait for process to exit
        retries=30
        while kill -0 $pid; do
            sleep 0.5
            retries=$((retries - 1))
            if [[ $retries -eq 0 ]]; then
                die "Process $pid (container $cid) refused to die"
            fi
        done

        # Wait for container to restart
        retries=20
        while :;do
            run_podman container inspect --format "{{.State.Pid}}" $cid
            # pid is 0 as long as the container is not running
            if [[ $output -ne 0 ]]; then
                if [[ $output == $pid ]]; then
                    die "This should never happen! Restarted container has same PID ($output) as killed one!"
                fi
                break
            fi
            sleep 0.5
            retries=$((retries - 1))
            if [[ $retries -eq 0 ]]; then
                die "Timed out waiting for container to restart"
            fi
        done

        # Verify http contents again: curl from localhost
        # Use retry since it can take a moment until the new container is ready
        run curl --retry 2 -s $SERVER/index.txt
        is "$output" "$random_1" "curl 127.0.0.1:/index.txt after auto restart"

        run_podman restart $cid
        # Verify http contents again: curl from localhost
        # Use retry since it can take a moment until the new container is ready
        run curl --retry 2 -s $SERVER/index.txt
        is "$output" "$random_1" "curl 127.0.0.1:/index.txt after podman restart"

        run_podman stop -t 0 $cid
        run_podman rm -f $cid
    done

    # Cleanup network
    run_podman network rm $netname
}

# vim: filetype=sh
