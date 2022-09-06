#!/usr/bin/env bats   -*- bats -*-
#
# Test podman local networking
#

load helpers

@test "podman network - basic tests" {
    heading="NETWORK *ID *NAME *DRIVER"
    run_podman network ls
    assert "${lines[0]}" =~ "^$heading\$" "network ls header missing"

    run_podman network ls --noheading
    assert "$output" !~ "$heading" "network ls --noheading shows header anyway"

    # check deterministic list order
    local net1=a-$(random_string 10)
    local net2=b-$(random_string 10)
    local net3=c-$(random_string 10)
    run_podman network create $net1
    run_podman network create $net2
    run_podman network create $net3

    run_podman network ls --quiet
    # just check the the order of the created networks is correct
    # we cannot do an exact match since developer and CI systems could contain more networks
    is "$output" ".*$net1.*$net2.*$net3.*podman.*" "networks sorted alphabetically"

    run_podman network rm $net1 $net2 $net3
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
    run_podman run --rm --net=host --http-proxy=false $IMAGE wget -qO - $SERVER/index.txt
    is "$output" "$random_1" "podman wget /index.txt"
    run_podman run --rm --net=host --http-proxy=false $IMAGE wget -qO - $SERVER/index2.txt
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
@test "podman networking: port with --userns=keep-id for rootless or --uidmap=* for rootful" {
    skip_if_cgroupsv1 "FIXME: #15025: run --uidmap fails on cgroups v1"
    for cidr in "" "$(random_rfc1918_subnet).0/24"; do
        myport=$(random_free_port 52000-52999)
        if [[ -z $cidr ]]; then
            # regex to match that we are in 10.X subnet
            match="10\..*"
            # force bridge networking also for rootless
            # this ensures that rootless + bridge + userns + ports works
            network_arg="--network bridge"
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
        userns="--userns=keep-id"
        is_rootless || userns="--uidmap=0:1111111:65536 --gidmap=0:1111111:65536"
        run_podman run -d ${userns} $network_arg -p 127.0.0.1:$myport:$myport \
                   $IMAGE nc -l -n -v -p $myport
        cid="$output"

        # check that podman stores the network info correctly when a userns is used (#14465)
        run_podman container inspect --format "{{.NetworkSettings.SandboxKey}}" $cid
        assert "$output" =~ ".*/netns/netns-.*" "Netns path should be set"

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

@test "podman pod manages /etc/hosts correctly" {
    local pod_name=pod-$(random_string 10)
    local infra_name=infra-$(random_string 10)
    local con1_name=con1-$(random_string 10)
    local con2_name=con2-$(random_string 10)
    run_podman pod create --name $pod_name  --infra-name $infra_name
    pid="$output"
    run_podman run --pod $pod_name --name $con1_name $IMAGE cat /etc/hosts
    is "$output" ".*\s$pod_name $infra_name.*" "Pod hostname in /etc/hosts"
    is "$output" ".*127.0.0.1\s$con1_name.*" "Container1 name in /etc/hosts"
    # get the length of the hosts file
    old_lines=${#lines[@]}

    # since the first container should be cleaned up now we should only see the
    # new host entry and the old one should be removed (lines check)
    run_podman run --pod $pod_name --name $con2_name $IMAGE cat /etc/hosts
    is "$output" ".*\s$pod_name $infra_name.*" "Pod hostname in /etc/hosts"
    is "$output" ".*127.0.0.1\s$con2_name.*" "Container2 name in /etc/hosts"
    is "${#lines[@]}" "$old_lines" "Number of hosts lines is equal"

    run_podman run --pod $pod_name  $IMAGE sh -c  "hostname && cat /etc/hostname"
    is "${lines[0]}" "$pod_name" "hostname is the pod hostname"
    is "${lines[1]}" "$pod_name" "/etc/hostname contains correct pod hostname"

    run_podman pod rm $pod_name
    is "$output" "$pid" "Only ID in output (no extra errors)"

    # Clean up
    run_podman rmi $(pause_image)
}

@test "podman run with slirp4ns assigns correct addresses to /etc/hosts" {
    CIDR="$(random_rfc1918_subnet)"
    IP=$(hostname -I | cut -f 1 -d " ")
    local conname=con-$(random_string 10)
    run_podman run --rm --network slirp4netns:cidr="${CIDR}.0/24" \
                --name $conname --hostname $conname $IMAGE cat /etc/hosts
    is "$output"   ".*${IP}	host.containers.internal"   "host.containers.internal should be host address"
    is "$output"   ".*${CIDR}.100	$conname $conname"   "$conname should be the cidr+100 address"

    if is_rootless; then
    # check the slirp ip also works correct with userns
        run_podman run --rm --userns keep-id --network slirp4netns:cidr="${CIDR}.0/24" \
                --name $conname --hostname $conname $IMAGE cat /etc/hosts
        is "$output"   ".*${IP}	host.containers.internal"   "host.containers.internal should be host address"
        is "$output"   ".*${CIDR}.100	$conname $conname"   "$conname should be the cidr+100 address"
    fi
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
    is "$output" "$mynetname" "output of 'network create'"

    # (Assert that output is formatted, not a one-line blob: #8011)
    run_podman network inspect $mynetname
    assert "${#lines[*]}" -ge 5 "Output from 'pod inspect'; see #8011"

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
    is "$output" "Error: network name $mynetname already used: network already exists" \
       "Trying to create an already-existing network"

    run_podman rm -t 0 -f $cid
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

    # use default network
    local netname=podman

    # Bind-mount this file with a different name to a container running httpd
    run_podman run -d --name myweb -p "$HOST_PORT:80" \
            --network $netname \
            -v $INDEX1:/var/www/index.txt:Z \
            -w /var/www \
            $IMAGE /bin/busybox-extras httpd -f -p 80
    cid=$output

    run_podman inspect $cid --format "{{(index .NetworkSettings.Networks \"$netname\").IPAddress}}"
    ip1="$output"
    run_podman inspect $cid --format "{{(index .NetworkSettings.Networks \"$netname\").MacAddress}}"
    mac1="$output"

    # Verify http contents: curl from localhost
    run curl -s $SERVER/index.txt
    is "$output" "$random_1" "curl 127.0.0.1:/index.txt"

    # rootless cannot modify iptables
    if ! is_rootless; then
        # flush the port forwarding iptable rule here
        chain="CNI-HOSTPORT-DNAT"
        if is_netavark; then
            chain="NETAVARK-HOSTPORT-DNAT"
        fi
        run iptables -t nat -F "$chain"

        # check that we cannot curl (timeout after 5 sec)
        run timeout 5 curl -s $SERVER/index.txt
        assert $status -eq 124 "curl did not time out"
    fi

    # reload the network to recreate the iptables rules
    run_podman network reload $cid
    is "$output" "$cid" "Output does match container ID"

    # check that we still have the same mac and ip
    run_podman inspect $cid --format "{{(index .NetworkSettings.Networks \"$netname\").IPAddress}}"
    is "$output" "$ip1" "IP address changed after podman network reload"
    run_podman inspect $cid --format "{{(index .NetworkSettings.Networks \"$netname\").MacAddress}}"
    is "$output" "$mac1" "MAC address changed after podman network reload"

    # check that we can still curl
    run curl -s $SERVER/index.txt
    is "$output" "$random_1" "curl 127.0.0.1:/index.txt"

    # create second network
    netname2=testnet-$(random_string 10)
    # TODO add --ipv6 and uncomment the ipv6 checks below once cni plugins 1.0 is available on ubuntu CI VMs.
    run_podman network create $netname2
    is "$output" "$netname2" "output of 'network create'"

    # connect the container to the second network
    run_podman network connect $netname2 $cid

    run_podman inspect $cid --format "{{(index .NetworkSettings.Networks \"$netname2\").IPAddress}}"
    ip2="$output"
    #run_podman inspect $cid --format "{{(index .NetworkSettings.Networks \"$netname2\").GlobalIPv6Address}}"
    #is "$output" "fd.*:.*" "IPv6 address should start with fd..."
    #ipv6="$output"
    run_podman inspect $cid --format "{{(index .NetworkSettings.Networks \"$netname2\").MacAddress}}"
    mac2="$output"

    # make sure --all is working and that this
    # cmd also works if the iptables still exists
    run_podman network reload --all
    is "$output" "$cid" "Output does match container ID"

    # check that both network keep there ip and mac
    run_podman inspect $cid --format "{{(index .NetworkSettings.Networks \"$netname\").IPAddress}}"
    is "$output" "$ip1" "IP address changed after podman network reload ($netname)"
    run_podman inspect $cid --format "{{(index .NetworkSettings.Networks \"$netname\").MacAddress}}"
    is "$output" "$mac1" "MAC address changed after podman network reload ($netname)"
    run_podman inspect $cid --format "{{(index .NetworkSettings.Networks \"$netname2\").IPAddress}}"
    is "$output" "$ip2" "IP address changed after podman network reload ($netname2)"
    #run_podman inspect $cid --format "{{(index .NetworkSettings.Networks \"$netname2\").GlobalIPv6Address}}"
    #is "$output" "$ipv6" "IPv6 address changed after podman network reload ($netname2)"
    run_podman inspect $cid --format "{{(index .NetworkSettings.Networks \"$netname2\").MacAddress}}"
    is "$output" "$mac2" "MAC address changed after podman network reload ($netname2)"

    # check that we can still curl
    run curl -s $SERVER/index.txt
    is "$output" "$random_1" "curl 127.0.0.1:/index.txt"

    # clean up the container
    run_podman rm -t 0 -f $cid

    # test that we cannot remove the default network
    run_podman 125 network rm -t 0 -f $netname
    is "$output" "Error: default network $netname cannot be removed" "Remove default network"

    run_podman network rm -t 0 -f $netname2
}

@test "podman rootless cni adds /usr/sbin to PATH" {
    is_rootless || skip "only meaningful for rootless"

    local mynetname=testnet-$(random_string 10)
    run_podman --noout network create $mynetname
    is "$output" "" "output should be empty"

    # Test that rootless cni adds /usr/sbin to $PATH
    # iptables is located under /usr/sbin and is needed for the CNI plugins.
    # Debian doesn't add /usr/sbin to $PATH for rootless users so we have to add it.
    PATH=/usr/local/bin:/usr/bin run_podman run --rm --network $mynetname $IMAGE ip addr
    is "$output" ".*eth0.*" "Interface eth0 not found in ip addr output"

    run_podman --noout network rm -t 0 -f $mynetname
    is "$output" "" "output should be empty"
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
    assert "$output" !~ "$ipv6_regex" "resolv.conf should not contain ipv6 nameserver"

    # ipv6 slirp
    run_podman run --rm --network slirp4netns:enable_ipv6=true $IMAGE cat /etc/resolv.conf
    assert "$output" =~ "$ipv6_regex" "resolv.conf should contain ipv6 nameserver"

    # ipv4 cni
    local mysubnet=$(random_rfc1918_subnet)
    local netname=testnet-$(random_string 10)

    run_podman network create --subnet $mysubnet.0/24 $netname
    is "$output" "$netname" "output of 'network create'"

    run_podman run --rm --network $netname $IMAGE cat /etc/resolv.conf
    assert "$output" !~ "$ipv6_regex" "resolv.conf should not contain ipv6 nameserver"

    run_podman network rm -t 0 -f $netname

    # ipv6 cni
    mysubnet=fd00:4:4:4:4::/64
    netname=testnet-$(random_string 10)

    run_podman network create --subnet $mysubnet $netname
    is "$output" "$netname" "output of 'network create'"

    run_podman run --rm --network $netname $IMAGE cat /etc/resolv.conf
    assert "$output" =~ "$ipv6_regex" "resolv.conf should contain ipv6 nameserver"

    run_podman network rm -t 0 -f $netname
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
    is "$output" "$netname" "output of 'network create'"

    local netname2=testnet2-$(random_string 10)
    run_podman network create $netname2
    is "$output" "$netname2" "output of 'network create'"

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

    # check network alias for container short id
    run_podman inspect $cid --format "{{(index .NetworkSettings.Networks \"$netname\").Aliases}}"
    is "$output" "[${cid:0:12}]" "short container id in network aliases"

    # check /etc/hosts for our entry
    run_podman exec $cid cat /etc/hosts
    is "$output" ".*$ip.*" "hosts contain expected ip"

    run_podman network disconnect $netname $cid
    is "$output" "" "Output should be empty (no errors)"

    # check /etc/hosts again, the entry should be gone now
    run_podman exec $cid cat /etc/hosts
    assert "$output" !~ "$ip" "IP ($ip) should no longer be in /etc/hosts"

    # check that we cannot curl (timeout after 3 sec)
    run curl --max-time 3 -s $SERVER/index.txt
    assert $status -ne 0 \
           "curl did not fail, it should have timed out or failed with non zero exit code"

    run_podman network connect $netname $cid
    is "$output" "" "Output should be empty (no errors)"

    # curl should work again
    run curl --max-time 3 -s $SERVER/index.txt
    is "$output" "$random_1" "curl 127.0.0.1:/index.txt should work again"

    # check that we have a new ip and mac
    # if the ip is still the same this whole test turns into a nop
    run_podman inspect $cid --format "{{(index .NetworkSettings.Networks \"$netname\").IPAddress}}"
    new_ip="$output"
    assert "$new_ip" != "$ip" \
           "IP address did not change after podman network disconnect/connect"

    run_podman inspect $cid --format "{{(index .NetworkSettings.Networks \"$netname\").MacAddress}}"
    assert "$output" != "$mac" \
           "MAC address did not change after podman network disconnect/connect"

    # check /etc/hosts for the new entry
    run_podman exec $cid cat /etc/hosts
    is "$output" ".*$new_ip.*" "hosts contain expected new ip"

    # Disconnect/reconnect of a container *with no ports* should succeed quietly
    run_podman network disconnect $netname $background_cid
    is "$output" "" "disconnect of container with no open ports"
    run_podman network connect $netname $background_cid
    is "$output" "" "(re)connect of container with no open ports"

    # FIXME FIXME FIXME: #11825: bodhi tests are failing, remote+rootless only,
    # with "dnsmasq: failed to create inotify". This error has never occurred
    # in CI, and Ed has been unable to reproduce it on 1minutetip. This next
    # line is a suggestion from Paul Holzinger for trying to shed light on
    # the system context before the failure. This output will be invisible
    # if the test passes.
    for foo in /proc/\*/fd/*; do readlink -f $foo; done |grep '^/proc/.*inotify' |cut -d/ -f3 | xargs -I '{}' -- ps --no-headers -o '%p %U %a' -p '{}' |uniq -c |sort -n

    # connect a second network
    run_podman network connect $netname2 $cid
    is "$output" "" "Output should be empty (no errors)"

    # check network2 alias for container short id
    run_podman inspect $cid --format "{{(index .NetworkSettings.Networks \"$netname2\").Aliases}}"
    is "$output" "[${cid:0:12}]" "short container id in network aliases"

    # curl should work
    run curl --max-time 3 -s $SERVER/index.txt
    is "$output" "$random_1" "curl 127.0.0.1:/index.txt should work"

    # disconnect the first network
    run_podman network disconnect $netname $cid

    # curl should still work
    run curl --max-time 3 -s $SERVER/index.txt
    is "$output" "$random_1" "curl 127.0.0.1:/index.txt should still work"

    # clean up
    run_podman rm -t 0 -f $cid $background_cid
    run_podman network rm -t 0 -f $netname $netname2
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
    is "$output" "$netname" "output of 'network create'"

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
            assert $retries -gt 0 "Process $pid (container $cid) refused to die"
        done

        # Wait for container to restart
        retries=20
        while :;do
            run_podman container inspect --format "{{.State.Pid}}" $cid
            # pid is 0 as long as the container is not running
            if [[ $output -ne 0 ]]; then
                assert "$output" != "$pid" \
                       "This should never happen! Restarted container has same PID as killed one!"
                break
            fi
            sleep 0.5
            retries=$((retries - 1))
            assert $retries -gt 0 "Timed out waiting for container to restart"
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

        run_podman rm -t 0 -f $cid
    done

    # Clean up network
    run_podman network rm -t 0 -f $netname
}

@test "podman run CONTAINERS_CONF dns options" {
    skip_if_remote "CONTAINERS_CONF redirect does not work on remote"
    # Test on the CLI and via containers.conf
    containersconf=$PODMAN_TMPDIR/containers.conf

    searchIP="100.100.100.100"
    cat >$containersconf <<EOF
[containers]
  dns_searches  = [ "example.com"]
  dns_servers = [
    "1.1.1.1",
    "$searchIP",
    "1.0.0.1",
    "8.8.8.8",
]
EOF

    local nl="
"

    CONTAINERS_CONF=$containersconf run_podman run --rm $IMAGE cat /etc/resolv.conf
    is "$output" "search example.com$nl.*" "correct search domain"
    is "$output" ".*nameserver 1.1.1.1${nl}nameserver $searchIP${nl}nameserver 1.0.0.1${nl}nameserver 8.8.8.8" "nameserver order is correct"

    # create network with dns
    local netname=testnet-$(random_string 10)
    local subnet=$(random_rfc1918_subnet)
    run_podman network create --subnet "$subnet.0/24"  $netname
    # custom server overwrites the network dns server
    CONTAINERS_CONF=$containersconf run_podman run --network $netname --rm $IMAGE cat /etc/resolv.conf
    is "$output" "search example.com$nl.*" "correct search domain"
    is "$output" ".*nameserver 1.1.1.1${nl}nameserver $searchIP${nl}nameserver 1.0.0.1${nl}nameserver 8.8.8.8" "nameserver order is correct"

    # we should use the integrated dns server
    run_podman run --network $netname --rm $IMAGE cat /etc/resolv.conf
    is "$output" "search dns.podman.*" "correct search domain"
    is "$output" ".*nameserver $subnet.1.*" "integrated dns nameserver is set"

    # host network should keep localhost nameservers
    if grep 127.0.0. /etc/resolv.conf >/dev/null; then
        run_podman run --network host --rm $IMAGE cat /etc/resolv.conf
        is "$output" ".*nameserver 127\.0\.0.*" "resolv.conf contains localhost nameserver"
    fi
    # host net + dns still works
    run_podman run --network host --dns 1.1.1.1 --rm $IMAGE cat /etc/resolv.conf
    is "$output" ".*nameserver 1\.1\.1\.1.*" "resolv.conf contains 1.1.1.1 nameserver"
}

@test "podman run port forward range" {
    for netmode in bridge slirp4netns:port_handler=slirp4netns slirp4netns:port_handler=rootlesskit; do
        local range=$(random_free_port_range 3)
        # die() inside $(...) does not actually stop us.
        assert "$range" != "" "Could not find free port range"

        local port="${range%-*}"
        local end_port="${range#*-}"
        local random=$(random_string)

        run_podman run --network $netmode -p "$range:$range" -d $IMAGE sleep inf
        cid="$output"
        for port in $(seq $port $end_port); do
            run_podman exec -d $cid nc -l -p $port -e /bin/cat
            # -w 1 adds a 1 second timeout. For some reason, ubuntu's ncat
            # doesn't close the connection on EOF, and other options to
            # change this are not portable across distros. -w seems to work.
            run nc -w 1 127.0.0.1 $port <<<$random
            is "$output" "$random" "ncat got data back (netmode=$netmode port=$port)"
        done

        run_podman rm -f -t0 $cid
    done
}

@test "podman run CONTAINERS_CONF /etc/hosts options" {
    skip_if_remote "CONTAINERS_CONF redirect does not work on remote"

    containersconf=$PODMAN_TMPDIR/containers.conf
    basehost=$PODMAN_TMPDIR/host

    ip1="$(random_rfc1918_subnet).$((RANDOM % 256))"
    name1=host1$(random_string)
    ip2="$(random_rfc1918_subnet).$((RANDOM % 256))"
    name2=host2$(random_string)

    cat >$basehost <<EOF
$ip1 $name1
$ip2 $name2 #some comment
EOF

    containersinternal_ip="$(random_rfc1918_subnet).$((RANDOM % 256))"
    cat >$containersconf <<EOF
[containers]
  base_hosts_file = "$basehost"
  host_containers_internal_ip = "$containersinternal_ip"
EOF

    ip3="$(random_rfc1918_subnet).$((RANDOM % 256))"
    name3=host3$(random_string)

    CONTAINERS_CONF=$containersconf run_podman run --rm --add-host $name3:$ip3 $IMAGE cat /etc/hosts
    is "$output" ".*$ip3[[:blank:]]$name3.*" "--add-host entry in /etc/host"
    is "$output" ".*$ip1[[:blank:]]$name1.*" "first base entry in /etc/host"
    is "$output" ".*$ip2[[:blank:]]$name2.*" "second base entry in /etc/host"
    is "$output" ".*127.0.0.1[[:blank:]]localhost.*" "ipv4 localhost entry added"
    is "$output" ".*::1[[:blank:]]localhost.*" "ipv6 localhost entry added"
    is "$output" ".*$containersinternal_ip[[:blank:]]host\.containers\.internal.*" "host.containers.internal ip from config in /etc/host"
    is "${#lines[@]}" "7" "expect 7 host entries in /etc/hosts"

    # now try again with container name and hostname == host entry name
    # in this case podman should not add its own entry thus we only have 5 entries (-1 for the removed --add-host)
    CONTAINERS_CONF=$containersconf run_podman run --rm --name $name1 --hostname $name1 $IMAGE cat /etc/hosts
    is "$output" ".*$ip1[[:blank:]]$name1.*" "first base entry in /etc/host"
    is "$output" ".*$ip2[[:blank:]]$name2.*" "second base entry in /etc/host"
    is "$output" ".*$containersinternal_ip[[:blank:]]host\.containers\.internal.*" "host.containers.internal ip from config in /etc/host"
    is "${#lines[@]}" "5" "expect 5 host entries in /etc/hosts"
}

@test "podman run /etc/* permissions" {
    skip_if_cgroupsv1 "FIXME: #15025: run --uidmap fails on cgroups v1"
    userns="--userns=keep-id"
    if ! is_rootless; then
        userns="--uidmap=0:1111111:65536 --gidmap=0:1111111:65536"
    fi
    # check with and without userns
    for userns in "" "$userns"; do
        # check the /etc/hosts /etc/hostname /etc/resolv.conf are owned by root
        run_podman run $userns --rm $IMAGE stat -c %u:%g /etc/hosts /etc/resolv.conf /etc/hostname
        is "${lines[0]}" "0\:0" "/etc/hosts owned by root"
        is "${lines[1]}" "0\:0" "/etc/resolv.conf owned by root"
        is "${lines[2]}" "0\:0" "/etc/hosts owned by root"
    done
}

@test "podman network rm --force bogus" {
    run_podman 1 network rm bogus
    is "$output" "Error: unable to find network with name or ID bogus: network not found" "Should print error"
    run_podman network rm --force bogus
    is "$output" "" "Should print no output"
}

@test "podman network rm --dns-option " {
    dns_opt=dns$(random_string)
    run_podman run --rm --dns-opt=${dns_opt} $IMAGE cat /etc/resolv.conf
    is "$output" ".*options ${dns_opt}" "--dns-opt was added"

    dns_opt=dns$(random_string)
    run_podman run --rm --dns-option=${dns_opt} $IMAGE cat /etc/resolv.conf
    is "$output" ".*options ${dns_opt}" "--dns-option was added"
}

# vim: filetype=sh
