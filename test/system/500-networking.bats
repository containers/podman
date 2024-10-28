#!/usr/bin/env bats   -*- bats -*-
#
# Test podman local networking
#

load helpers
load helpers.network

# bats test_tags=ci:parallel
@test "podman network - basic tests" {
    heading="NETWORK *ID *NAME *DRIVER"
    run_podman network ls
    assert "${lines[0]}" =~ "^$heading\$" "network ls header missing"
    run_podman network list
    assert "${lines[0]}" =~ "^$heading\$" "network list header missing"

    run_podman network ls --noheading
    assert "$output" !~ "$heading" "network ls --noheading shows header anyway"

    run_podman network ls -n
    assert "$output" !~ "$heading" "network ls -n shows header anyway"

    # check deterministic list order
    local net1=net-a-$(safename)
    local net2=net-b-$(safename)
    local net3=net-c-$(safename)
    run_podman network create $net1
    run_podman network create $net2
    run_podman network create $net3

    run_podman network ls --quiet
    # just check that the order of the created networks is correct
    # we cannot do an exact match since developer and CI systems could contain more networks
    is "$output" ".*$net1.*$net2.*$net3.*podman.*" "networks sorted alphabetically"

    run_podman network rm $net1 $net2 $net3
}

# Copied from tsweeney's https://github.com/containers/podman/issues/4827
# bats test_tags=ci:parallel
@test "podman networking: port on localhost" {
    random_1=$(random_string 30)
    random_2=$(random_string 30)

    HOST_PORT=$(random_free_port)
    SERVER=http://127.0.0.1:$HOST_PORT

    # Create a test file with random content
    INDEX1=$PODMAN_TMPDIR/hello.txt
    echo $random_1 > $INDEX1

    # Bind-mount this file with a different name to a container running httpd
    cname="c-$(safename)"
    run_podman run -d --name $cname -p "$HOST_PORT:80" \
            -v $INDEX1:/var/www/index.txt:Z \
            -w /var/www \
            $IMAGE /bin/busybox-extras httpd -f -p 80
    cid=$output

    # Try to bind the same port again, this must fail.
    # regression test for https://issues.redhat.com/browse/RHEL-50746
    # which caused this command to overwrite the firewall rules as root
    # causing the curl commands below to fail
    run_podman 126 run --rm -p "$HOST_PORT:80" $IMAGE true
    # Note error messages differ between root/rootless, so only check port
    # and the part of the error text that is common.
    assert "$output" =~ "$HOST_PORT.*ddress already in use" "port in use"

    # In that container, create a second file, using exec and redirection
    run_podman exec -i $cname sh -c "cat > index2.txt" <<<"$random_2"
    # ...verify its contents as seen from container.
    run_podman exec -i $cname cat /var/www/index2.txt
    is "$output" "$random_2" "exec cat index2.txt"

    # Verify http contents: curl from localhost
    run curl --max-time 3 -s -S $SERVER/index.txt
    is "$output" "$random_1" "curl 127.0.0.1:/index.txt"
    run curl --max-time 3 -s -S $SERVER/index2.txt
    is "$output" "$random_2" "curl 127.0.0.1:/index2.txt"

    # Verify http contents: wget from a second container
    run_podman run --rm --net=host --http-proxy=false $IMAGE wget -qO - $SERVER/index.txt
    is "$output" "$random_1" "podman wget /index.txt"
    run_podman run --rm --net=host --http-proxy=false $IMAGE wget -qO - $SERVER/index2.txt
    is "$output" "$random_2" "podman wget /index2.txt"

    # Tests #4889 - two-argument form of "podman ports" was broken
    run_podman port $cname
    is "$output" "80/tcp -> 0.0.0.0:$HOST_PORT" "port <cid>"
    run_podman port $cname 80
    is "$output" "0.0.0.0:$HOST_PORT"  "port <cid> 80"
    run_podman port $cname 80/tcp
    is "$output" "0.0.0.0:$HOST_PORT"  "port <cid> 80/tcp"

    run_podman 125 port $cname 99/tcp
    is "$output" 'Error: failed to find published port "99/tcp"'

    # Clean up
    run_podman rm -f -t0 $cname
}

# Issue #5466 - port-forwarding doesn't work with this option and -d
# FIXME: random_rfc1918_subnet is not parallel-safe
@test "podman networking: port with --userns=keep-id for rootless or --uidmap=* for rootful" {
    skip_if_cgroupsv1 "run --uidmap fails on cgroups v1 (issue 15025, wontfix)"
    for cidr in "" "$(random_rfc1918_subnet).0/24"; do
        myport=$(random_free_port 52000-52999)
        if [[ -z $cidr ]]; then
            # regex to match that we are in 10.X subnet
            match="10\..*"
            # force bridge networking also for rootless
            # this ensures that rootless + bridge + userns + ports works
            network_arg="--network bridge"
        elif has_slirp4netns; then
            # Issue #9828 make sure a custom slirp4netns cidr also works
            network_arg="--network slirp4netns:cidr=$cidr"
            # slirp4netns interface ip is always .100
            match="${cidr%.*}.100"
        else
            echo "# [skipping subtest of $cidr - slirp4netns unavailable]" >&3
            continue
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

# bats test_tags=ci:parallel
@test "podman pod manages /etc/hosts correctly" {
    local pod_name=pod-$(safename)
    local infra_name=infra-$(safename)
    local con1_name=con1-$(safename)
    local con2_name=con2-$(safename)
    run_podman pod create --name $pod_name  --infra-name $infra_name
    pid="$output"
    run_podman run --rm --pod $pod_name --name $con1_name $IMAGE cat /etc/hosts
    assert "$output" =~ ".*\s$pod_name $infra_name.*" \
           "Pod hostname in /etc/hosts"
    assert "$output" =~ ".*127.0.0.1\s$con1_name.*" \
           "Container1 name in /etc/hosts"
    # get the length of the hosts file
    old_lines=${#lines[@]}

    # since the first container should be cleaned up now we should only see the
    # new host entry and the old one should be removed (lines check)
    run_podman run --pod $pod_name --name $con2_name $IMAGE cat /etc/hosts
    assert "$output" =~ ".*\s$pod_name $infra_name.*" \
           "Pod hostname in /etc/hosts"
    assert "$output" =~ ".*127.0.0.1\s$con2_name.*" \
           "Container2 name in /etc/hosts"
    assert "$output" !~ "$con1_name" \
           "Container1 name should not be in /etc/hosts"
    is "${#lines[@]}" "$old_lines" \
       "Number of hosts lines is equal"

    run_podman run --pod $pod_name  $IMAGE sh -c  "hostname && cat /etc/hostname"
    is "${lines[0]}" "$pod_name" "hostname is the pod hostname"
    is "${lines[1]}" "$pod_name" "/etc/hostname contains correct pod hostname"

    run_podman pod rm -f -t0 $pod_name
    is "$output" "$pid" "Only ID in output (no extra errors)"
}

# FIXME: random_rfc1918_subnet is not parallel-safe
@test "podman run with slirp4ns assigns correct addresses to /etc/hosts" {
    has_slirp4netns || skip "slirp4netns unavailable"

    CIDR="$(random_rfc1918_subnet)"
    IP=$(hostname -I | cut -f 1 -d " ")
    local conname=con-$(safename)
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

# FIXME: random_rfc1918_subnet is not parallel-safe
@test "podman run with slirp4ns adds correct dns address to resolv.conf" {
    has_slirp4netns || skip "slirp4netns unavailable"

    CIDR="$(random_rfc1918_subnet)"
    run_podman run --rm --network slirp4netns:cidr="${CIDR}.0/24" \
                $IMAGE cat /etc/resolv.conf
    assert "$output" =~ "nameserver ${CIDR}.3" "resolv.conf should have slirp4netns cidr+3 as first nameserver"
    no_userns_out="$output"

    if is_rootless; then
    # check the slirp ip also works correct with userns
        run_podman run --rm --userns keep-id --network slirp4netns:cidr="${CIDR}.0/24" \
                $IMAGE cat /etc/resolv.conf
        assert "$output" =~ "nameserver ${CIDR}.3" "resolv.conf should have slirp4netns cidr+3 as first nameserver with userns"
        assert "$output" == "$no_userns_out" "resolv.conf should look the same for userns"
    fi

}

# FIXME: random_rfc1918_subnet is not parallel-safe
@test "podman run with slirp4ns assigns correct ip address container" {
    has_slirp4netns || skip "slirp4netns unavailable"

    CIDR="$(random_rfc1918_subnet)"
    run_podman run --rm --network slirp4netns:cidr="${CIDR}.0/24" \
                $IMAGE sh -c "ip address | grep ${CIDR}"
    is "$output"   ".*inet ${CIDR}.100/24 \+"   "container should have slirp4netns cidr+100 assigned to interface"
}

# "network create" now works rootless, with the help of a special container
# FIXME: random_rfc1918_subnet is not parallel-safe
@test "podman network create" {
    myport=$(random_free_port)

    local mynetname=testnet-$(safename)
    local mysubnet=$(random_rfc1918_subnet)

    run_podman network create --subnet "${mysubnet}.0/24" $mynetname
    is "$output" "$mynetname" "output of 'network create'"

    # (Assert that output is formatted, not a one-line blob: #8011)
    run_podman network inspect $mynetname
    assert "${#lines[*]}" -ge 5 "Output from 'pod inspect'; see #8011"

    run_podman run --rm --network $mynetname $IMAGE ip a
    is "$output" ".* inet ${mysubnet}\.2/24 brd ${mysubnet}\.255 " \
       "sdfsdf"

    local cname="c-$(safename)"
    run_podman run -d --network $mynetname --name $cname -p 127.0.0.1:$myport:$myport \
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

# CANNOT BE PARALLELIZED due to iptables/nft commands
# bats test_tags=distro-integration
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
    local cname=c-$(safename)
    run_podman run -d --name $cname -p "$HOST_PORT:80" \
            --network $netname \
            -v $INDEX1:/var/www/index.txt:Z \
            -w /var/www \
            $IMAGE /bin/busybox-extras httpd -f -p 80
    cid=$output

    run_podman inspect $cid --format "{{(index .NetworkSettings.Networks \"$netname\").IPAddress}}
{{(index .NetworkSettings.Networks \"$netname\").MacAddress}}"
    ip1="${lines[0]}"
    mac1="${lines[1]}"

    # Verify http contents: curl from localhost
    run curl -s -S $SERVER/index.txt
    is "$output" "$random_1" "curl 127.0.0.1:/index.txt"

    # rootless cannot modify the host firewall
    if ! is_rootless; then
        # for debugging only
        iptables -t nat -nvL || true
        nft list ruleset     || true

        # flush the firewall rule here to break port forwarding
        # netavark can use either iptables or nftables, so try flushing both
        iptables -t nat -F "NETAVARK-HOSTPORT-DNAT" || true
        nft delete table inet netavark              || true

        # check that we cannot curl (timeout after 1 sec)
        run curl --max-time 1 -s $SERVER/index.txt
        assert $status -eq 28 "curl did not time out"
    fi

    # reload the network to recreate the iptables rules
    run_podman network reload $cid
    is "$output" "$cid" "Output does match container ID"

    # check that we still have the same mac and ip
    run_podman inspect $cid --format "{{(index .NetworkSettings.Networks \"$netname\").IPAddress}}
{{(index .NetworkSettings.Networks \"$netname\").MacAddress}}"
    is "${lines[0]}" "$ip1" "IP address changed after podman network reload"
    is "${lines[1]}" "$mac1" "MAC address changed after podman network reload"

    # check that we can still curl
    run curl -s $SERVER/index.txt
    is "$output" "$random_1" "curl 127.0.0.1:/index.txt"

    # create second network
    netname2=testnet-$(safename)
    run_podman network create --ipv6 $netname2
    is "$output" "$netname2" "output of 'network create'"

    # connect the container to the second network
    run_podman network connect $netname2 $cid

    run_podman inspect $cid --format "{{(index .NetworkSettings.Networks \"$netname2\").IPAddress}}
{{(index .NetworkSettings.Networks \"$netname2\").GlobalIPv6Address}}
{{(index .NetworkSettings.Networks \"$netname2\").MacAddress}}"
    ip2="${lines[0]}"
    is "${lines[1]}" "fd.*:.*" "IPv6 address should start with fd..."
    ipv6="${lines[1]}"
    mac2="${lines[2]}"

    # make sure --all is working and that this
    # cmd also works if the iptables still exists
    run_podman network reload --all
    is "$output" "$cid" "Output does match container ID"

    # check that both network keep there ip and mac
    run_podman inspect $cid --format "{{(index .NetworkSettings.Networks \"$netname\").IPAddress}}
{{(index .NetworkSettings.Networks \"$netname\").MacAddress}}
{{(index .NetworkSettings.Networks \"$netname2\").IPAddress}}
{{(index .NetworkSettings.Networks \"$netname2\").GlobalIPv6Address}}
{{(index .NetworkSettings.Networks \"$netname2\").MacAddress}}
"

    is "${lines[0]}" "$ip1" "IP address changed after podman network reload ($netname)"
    is "${lines[1]}" "$mac1" "MAC address changed after podman network reload ($netname)"
    is "${lines[2]}" "$ip2" "IP address changed after podman network reload ($netname2)"
    is "${lines[3]}" "$ipv6" "IPv6 address changed after podman network reload ($netname2)"
    is "${lines[4]}" "$mac2" "MAC address changed after podman network reload ($netname2)"

    # check that we can still curl
    run curl -s -S $SERVER/index.txt
    is "$output" "$random_1" "curl 127.0.0.1:/index.txt"

    # clean up the container
    run_podman rm -t 0 -f $cid

    # test that we cannot remove the default network
    run_podman 125 network rm -t 0 -f $netname
    is "$output" "Error: default network $netname cannot be removed" "Remove default network"

    run_podman network rm -t 0 -f $netname2
}

# bats test_tags=ci:parallel
@test "podman rootless cni adds /usr/sbin to PATH" {
    is_rootless || skip "only meaningful for rootless"

    local mynetname=testnet-$(safename)
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

# FIXME: random_rfc1918_subnet is not parallel-safe
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

    if has_slirp4netns; then
        # ipv4 slirp
        run_podman run --rm --network slirp4netns:enable_ipv6=false $IMAGE cat /etc/resolv.conf
        assert "$output" !~ "$ipv6_regex" "resolv.conf should not contain ipv6 nameserver"

        # ipv6 slirp
        run_podman run --rm --network slirp4netns:enable_ipv6=true $IMAGE cat /etc/resolv.conf
        assert "$output" =~ "$ipv6_regex" "resolv.conf should contain ipv6 nameserver"
    fi

    # ipv4 cni
    local mysubnet=$(random_rfc1918_subnet)
    local netname=testnet1-$(safename)

    run_podman network create --subnet $mysubnet.0/24 $netname
    is "$output" "$netname" "output of 'network create'"

    run_podman run --rm --network $netname $IMAGE cat /etc/resolv.conf
    assert "$output" !~ "$ipv6_regex" "resolv.conf should not contain ipv6 nameserver"

    run_podman network rm -t 0 -f $netname

    # ipv6 cni
    mysubnet=fd00:4:4:4:4::/64
    netname=testnet2-$(safename)

    run_podman network create --subnet $mysubnet $netname
    is "$output" "$netname" "output of 'network create'"

    run_podman run --rm --network $netname $IMAGE cat /etc/resolv.conf
    assert "$output" =~ "$ipv6_regex" "resolv.conf should contain ipv6 nameserver"

    run_podman network rm -t 0 -f $netname
}

# Test for https://github.com/containers/podman/issues/10052
# bats test_tags=distro-integration, ci:parallel
@test "podman network connect/disconnect with port forwarding" {
    random_1=$(random_string 30)
    HOST_PORT=$(random_free_port)
    SERVER=http://127.0.0.1:$HOST_PORT

    # Create a test file with random content
    INDEX1=$PODMAN_TMPDIR/hello.txt
    echo $random_1 > $INDEX1

    local netname=testnet1-$(safename)
    run_podman network create $netname
    is "$output" "$netname" "output of 'network create'"

    local netname2=testnet2-$(safename)
    run_podman network create $netname2
    is "$output" "$netname2" "output of 'network create'"

    # First, run a container in background to ensure that the rootless netns
    # is not destroyed after network disconnect.
    run_podman run -d --network $netname $IMAGE top
    background_cid=$output

    local hostname=host-$(safename)
    # Run a httpd container on first network with exposed port
    run_podman run -d -p "$HOST_PORT:80" \
            --hostname $hostname \
            --network $netname \
            -v $INDEX1:/var/www/index.txt:Z \
            -w /var/www \
            $IMAGE /bin/busybox-extras httpd -f -p 80
    cid=$output

    # Verify http contents: curl from localhost. This is the first time
    # connecting, so, allow retries until httpd starts.
    run curl --retry 2 --retry-connrefused -s $SERVER/index.txt
    is "$output" "$random_1" "curl $SERVER/index.txt"

    run_podman inspect $cid --format "{{(index .NetworkSettings.Networks \"$netname\").IPAddress}}
{{(index .NetworkSettings.Networks \"$netname\").MacAddress}}
{{(index .NetworkSettings.Networks \"$netname\").Aliases}}"
    ip="${lines[0]}"
    mac="${lines[1]}"

    # check network alias for container short id
    is "${lines[2]}" "[${cid:0:12} $hostname]" "short container id and hostname in network aliases"

    # check /etc/hosts for our entry
    run_podman exec $cid cat /etc/hosts
    is "$output" ".*$ip.*" "hosts contain expected ip"

    run_podman network disconnect $netname $cid
    is "$output" "" "Output should be empty (no errors)"

    # check /etc/hosts again, the entry should be gone now
    run_podman exec $cid cat /etc/hosts
    assert "$output" !~ "$ip" "IP ($ip) should no longer be in /etc/hosts"

    # check that we cannot curl (timeout after 3 sec). Fails with inconsistent
    # curl exit codes, so, just check for nonzero.
    run curl --max-time 3 -s -S $SERVER/index.txt
    assert $status -ne 0 \
           "curl did not fail, it should have timed out or failed with non zero exit code"

    run_podman network connect $netname $cid
    is "$output" "" "Output should be empty (no errors)"

    # curl should work again
    run curl --max-time 3 -s -S $SERVER/index.txt
    is "$output" "$random_1" "curl 127.0.0.1:/index.txt should work again"

    # check that we have a new ip and mac
    # if the ip is still the same this whole test turns into a nop
    run_podman inspect $cid --format "{{(index .NetworkSettings.Networks \"$netname\").IPAddress}}
{{(index .NetworkSettings.Networks \"$netname\").MacAddress}}"
    new_ip="${lines[0]}"
    assert "$new_ip" != "$ip" \
           "IP address did not change after podman network disconnect/connect"
    assert "${lines[1]}" != "$mac" \
           "MAC address did not change after podman network disconnect/connect"

    # check /etc/hosts for the new entry
    run_podman exec $cid cat /etc/hosts
    is "$output" ".*$new_ip.*" "hosts contain expected new ip"

    # Disconnect/reconnect of a container *with no ports* should succeed quietly
    run_podman network disconnect $netname $background_cid
    is "$output" "" "disconnect of container with no open ports"
    run_podman network connect $netname $background_cid
    is "$output" "" "(re)connect of container with no open ports"

    # connect a second network
    run_podman network connect $netname2 $cid
    is "$output" "" "Output should be empty (no errors)"

    # check network2 alias for container short id
    run_podman inspect $cid --format "{{(index .NetworkSettings.Networks \"$netname2\").Aliases}}"
    is "$output" "[${cid:0:12} $hostname]" "short container id and hostname in network2 aliases"

    # curl should work
    run curl --max-time 3 -s -S $SERVER/index.txt
    is "$output" "$random_1" "curl 127.0.0.1:/index.txt should work"

    # disconnect the first network
    run_podman network disconnect $netname $cid

    # curl should still work
    run curl --max-time 3 -s -S $SERVER/index.txt
    is "$output" "$random_1" "curl 127.0.0.1:/index.txt should still work"

    # clean up
    run_podman rm -t 0 -f $cid $background_cid
    run_podman network rm -t 0 -f $netname $netname2
}

# bats test_tags=ci:parallel
@test "podman network after restart" {
    random_1=$(random_string 30)

    HOST_PORT=$(random_free_port)
    SERVER=http://127.0.0.1:$HOST_PORT

    # Create a test file with random content
    INDEX1=$PODMAN_TMPDIR/hello.txt
    echo $random_1 > $INDEX1

    local netname=testnet-$(safename)
    run_podman network create $netname
    is "$output" "$netname" "output of 'network create'"

    local -a networks=("$netname")
    if has_slirp4netns; then
        networks+=("slirp4netns")
    fi
    for network in "${networks[@]}"; do
        # Start container with the restart always policy
        local cname=c-$(safename)
        run_podman run -d --name $cname -p "$HOST_PORT:80" \
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
        local curlcmd="curl --retry 2 --retry-connrefused -s $SERVER/index.txt"
        echo "$_LOG_PROMPT $curlcmd"
        run $curlcmd
        echo "$output"
        assert "$status" == 0 "curl exit status"
        assert "$output" = "$random_1" "curl $SERVER/index.txt after auto restart"

        run_podman 0+w restart -t1 $cid
        if ! is_remote; then
            require_warning "StopSignal SIGTERM failed to stop container .* in 1 seconds, resorting to SIGKILL" \
                            "podman restart issues warning"
        fi

        # Verify http contents again: curl from localhost
        # Use retry since it can take a moment until the new container is ready
        echo "$_LOG_PROMPT $curlcmd"
        run $curlcmd
        echo "$output"
        assert "$status" == 0 "curl exit status"
        assert "$output" = "$random_1" "curl $SERVER/index.txt after podman restart"

        run_podman rm -t 0 -f $cid
    done

    # Clean up network
    run_podman network rm -t 0 -f $netname
}

# FIXME: random_rfc1918_subnet is not parallel-safe
@test "podman run CONTAINERS_CONF_OVERRIDE dns options" {
    skip_if_remote "CONTAINERS_CONF_OVERRIDE redirect does not work on remote"
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

    CONTAINERS_CONF_OVERRIDE=$containersconf run_podman run --rm $IMAGE cat /etc/resolv.conf
    is "$output" "search example.com.*" "correct search domain"
    is "$output" ".*nameserver 1.1.1.1${nl}nameserver $searchIP${nl}nameserver 1.0.0.1${nl}nameserver 8.8.8.8" "nameserver order is correct"

    # create network with dns
    local netname=testnet-$(safename)
    local subnet=$(random_rfc1918_subnet)
    run_podman network create --subnet "$subnet.0/24"  $netname
    # custom server overwrites the network dns server
    CONTAINERS_CONF_OVERRIDE=$containersconf run_podman run --network $netname --rm $IMAGE cat /etc/resolv.conf
    is "$output" "search example.com.*" "correct search domain"
    local store=$output
    if is_netavark; then
        assert "$store" == "search example.com${nl}nameserver $subnet.1" "only integrated dns nameserver is set"
    else
        assert "$store" == "search example.com
nameserver 1.1.1.1
nameserver $searchIP
nameserver 1.0.0.1
nameserver 8.8.8.8" "nameserver order is correct"
    fi
    # we should use the integrated dns server
    run_podman run --network $netname --rm $IMAGE cat /etc/resolv.conf
    assert "$output" =~ "search dns.podman.*" "correct search domain"
    assert "$output" =~ ".*nameserver $subnet.1.*" \
           "integrated dns nameserver is set"

    # host network should keep localhost nameservers
    if grep 127.0.0. /etc/resolv.conf >/dev/null; then
        run_podman run --network host --rm $IMAGE cat /etc/resolv.conf
        assert "$output" =~ ".*nameserver 127\.0\.0.*" \
               "resolv.conf contains localhost nameserver"
    fi
    # host net + dns still works
    run_podman run --network host --dns 1.1.1.1 --rm $IMAGE cat /etc/resolv.conf
    assert "$output" =~ ".*nameserver 1\.1\.1\.1.*" \
           "resolv.conf contains 1.1.1.1 nameserver"

    run_podman network rm -f $netname
}

# bats test_tags=distro-integration, ci:parallel
@test "podman run port forward range" {
    # we run a long loop of tests lets run all combinations before bailing out
    defer-assertion-failures

    local -a netmodes=("bridge")
    # As of podman 5.0, slirp4netns is optional
    if has_slirp4netns; then
        netmodes+=("slirp4netns:port_handler=slirp4netns" "slirp4netns:port_handler=rootlesskit")
    fi
    # pasta only works rootless
    if is_rootless; then
        if has_pasta; then
            netmodes+=("pasta")
        else
            echo "# WARNING: pasta unavailable!" >&3
        fi
    fi

    for netmode in "${netmodes[@]}"; do
        local range=$(random_free_port_range 3)
        # die() inside $(...) does not actually stop us.
        assert "$range" != "" "Could not find free port range"

        local port="${range%-*}"
        local end_port="${range#*-}"
        local random=$(random_string)

        run_podman run --network $netmode -p "$range:$range" -d $IMAGE sleep inf
        cid="$output"

        # make sure binding the same port fails
        run timeout 5 nc -l 127.0.0.1 $port
        assert "$status" -eq 2 "ncat unexpected exit code"
        assert "$output" =~ "127.0.0.1:$port: Address already in use" "ncat error message"

        for port in $(seq $port $end_port); do
            run_podman exec -d $cid nc -l -p $port -e /bin/cat

            # we have to rety ncat as it can flake as we exec in the background so nc -l
            # might not have bound the port yet, retry seems simpler than checking if the
            # port is bound in the container, https://github.com/containers/podman/issues/21561.
            retries=5
            while [[ $retries -gt 0 ]]; do
                # -w 1 adds a 1 second timeout. For some reason, ubuntu's ncat
                # doesn't close the connection on EOF, and other options to
                # change this are not portable across distros. -w seems to work.
                run nc -w 1 127.0.0.1 $port <<<$random
                if [[ $status -eq 0 ]]; then
                    break
                fi
                sleep 0.5
                retries=$((retries -1))
            done
            is "$output" "$random" "ncat got data back (netmode=$netmode port=$port)"
        done

        run_podman rm -f -t0 $cid
    done
}

# FIXME: random_rfc1918_subnet is not parallel-safe
@test "podman run CONTAINERS_CONF_OVERRIDE /etc/hosts options" {
    skip_if_remote "CONTAINERS_CONF_OVERRIDE redirect does not work on remote"

    containersconf=$PODMAN_TMPDIR/containers.conf
    basehost=$PODMAN_TMPDIR/host

    ip1="$(random_rfc1918_subnet).$((RANDOM % 256))"
    name1=host1-$(safename)
    ip2="$(random_rfc1918_subnet).$((RANDOM % 256))"
    name2=host2-$(safename)

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
    name3=host3-$(safename)

    CONTAINERS_CONF_OVERRIDE=$containersconf run_podman run --rm --add-host $name3:$ip3 $IMAGE cat /etc/hosts
    is "$output" ".*$ip3[[:blank:]]$name3.*" "--add-host entry in /etc/host"
    is "$output" ".*$ip1[[:blank:]]$name1.*" "first base entry in /etc/host"
    is "$output" ".*$ip2[[:blank:]]$name2.*" "second base entry in /etc/host"
    is "$output" ".*127.0.0.1[[:blank:]]localhost.*" "ipv4 localhost entry added"
    is "$output" ".*::1[[:blank:]]localhost.*" "ipv6 localhost entry added"
    is "$output" ".*$containersinternal_ip[[:blank:]]host\.containers\.internal.*" "host.containers.internal ip from config in /etc/host"
    is "${#lines[@]}" "7" "expect 7 host entries in /etc/hosts"

    # now try again with container name and hostname == host entry name
    # in this case podman should not add its own entry thus we only have 5 entries (-1 for the removed --add-host)
    CONTAINERS_CONF_OVERRIDE=$containersconf run_podman run --rm --name $name1 --hostname $name1 $IMAGE cat /etc/hosts
    is "$output" ".*$ip1[[:blank:]]$name1.*" "first base entry in /etc/host"
    is "$output" ".*$ip2[[:blank:]]$name2.*" "second base entry in /etc/host"
    is "$output" ".*$containersinternal_ip[[:blank:]]host\.containers\.internal.*" "host.containers.internal ip from config in /etc/host"
    is "${#lines[@]}" "5" "expect 5 host entries in /etc/hosts"
}

# bats test_tags=ci:parallel
@test "podman run /etc/* permissions" {
    skip_if_cgroupsv1 "run --uidmap fails on cgroups v1 (issue 15025, wontfix)"
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

# bats test_tags=ci:parallel
@test "podman network rm --force bogus" {
    bogusnet=bogusnet-$(safename)
    run_podman 1 network rm $bogusnet
    is "$output" "Error: unable to find network with name or ID $bogusnet: network not found" "Should print error"
    run_podman network rm -t -1 --force $bogusnet
    is "$output" "" "Should print no output"

    netname=testnet-$(safename)
    run_podman network create $netname
    run_podman network rm --force $bogusnet $netname
    assert "$output" = "$netname" "rm network"
    run_podman network ls -q
    assert "$output" !~ "$(safename)" "all networks from this test should be gone"
}

# bats test_tags=ci:parallel
@test "podman network rm --dns-option " {
    dns_opt=dns$(random_string)
    run_podman run --rm --dns-opt=${dns_opt} $IMAGE cat /etc/resolv.conf
    is "$output" ".*options ${dns_opt}" "--dns-opt was added"

    dns_opt=dns$(random_string)
    run_podman run --rm --dns-option=${dns_opt} $IMAGE cat /etc/resolv.conf
    is "$output" ".*options ${dns_opt}" "--dns-option was added"
}

# bats test_tags=ci:parallel
@test "podman rootless netns works when XDG_RUNTIME_DIR includes symlinks" {
    # regression test for https://github.com/containers/podman/issues/14606
    is_rootless || skip "only meaningful for rootless"

    # Create a tmpdir symlink pointing to /run, and use it briefly
    ln -s /run $PODMAN_TMPDIR/run
    local tmp_run=$PODMAN_TMPDIR/run/user/$(id -u)
    test -d $tmp_run || skip "/run/user/MYUID unavailable"

    # This 'run' would previously fail with:
    #    IPAM error: failed to open database ....
    XDG_RUNTIME_DIR=$tmp_run run_podman run --network bridge --rm $IMAGE ip a
    assert "$output" =~ "eth0"
}

# bats test_tags=ci:parallel
@test "podman inspect list networks " {
    run_podman create $IMAGE
    cid=${output}
    run_podman inspect --format '{{ .NetworkSettings.Networks }}' $cid
    if is_rootless; then
        is "$output" "map\[pasta:.*" "NeworkSettings should contain one network named pasta"
    else
        is "$output" "map\[podman:.*" "NeworkSettings should contain one network named podman"
    fi
    run_podman rm $cid

    for network in "host" "none"; do
        run_podman create --network=$network $IMAGE
        cid=${output}
        run_podman inspect --format '{{ .NetworkSettings.Networks }}' $cid
        is "$output" "map\[$network:.*" "NeworkSettings should contain one network named $network"
        run_podman inspect --format '{{ .NetworkSettings.SandboxKey }}' $cid
        assert "$output" == "" "SandboxKey for network=$network should be empty when not running"
        run_podman rm $cid
    done

    run_podman run -d --network=none $IMAGE top
    cid=${output}
    run_podman inspect --format '{{ .NetworkSettings.SandboxKey }}' $cid
    assert "$output" =~ "^/proc/[0-9]+/ns/net\$" "SandboxKey for network=none when running"
    run_podman rm -f -t0 $cid

    # Check with ns:/PATH
    if ! is_rootless; then
        netns=netns-$(safename)
        echo "$_LOG_PROMPT ip netns add $netns"
        ip netns add $netns
        run_podman create --network=ns:/var/run/netns/$netns $IMAGE
        cid=${output}
        run_podman inspect --format '{{ .NetworkSettings.Networks }}' $cid
        is "$output" 'map[]' "NeworkSettings should be empty"
        run_podman rm $cid
        echo "$_LOG_PROMPT ip netns delete $netns"
        ip netns delete $netns
     fi
}

function wait_for_restart_count() {
    local cname="$1"
    local count="$2"
    local tname="$3"

    local timeout=10
    while :; do
        # Previously this would fail as the container would run out of ips after 5 restarts.
        run_podman inspect --format "{{.RestartCount}}" $cname
        if [[ "$output" == "$2" ]]; then
            break
        fi

        timeout=$((timeout - 1))
        if [[ $timeout -eq 0 ]]; then
            die "Timed out waiting for RestartCount with $tname"
        fi
        sleep 0.5
    done
}

# Test for https://github.com/containers/podman/issues/18615
# CANNOT BE PARALLELIZED due to strict checking of /run/netns
@test "podman network cleanup --userns + --restart" {
    skip_if_cgroupsv1 "run --uidmap fails on cgroups v1 (issue 15025, wontfix)"

    local net1=net-a-$(safename)
    # use /29 subnet to limit available ip space, a 29 gives 5 usable addresses (6 - 1 for the gw)
    local subnet="$(random_rfc1918_subnet).0/29"
    run_podman network create --subnet $subnet $net1
    local cname=con1-$(safename)
    local cname2=
    local cname3=

    local netns_count=
    if ! is_rootless; then
        netns_count=$(ls /run/netns | wc -l)
    fi

    # This will cause 7 containers runs with the restart policy (one more than the on failure limit)
    # Since they run sequentially they should run fine without allocating all ips.
    run_podman 1 run --name $cname --network $net1 --restart on-failure:6 --userns keep-id $IMAGE false
    wait_for_restart_count $cname 6 "custom network"
    run_podman wait $cname

    # Now make sure we can still run a container with free ips.
    run_podman run --rm --network $net1 $IMAGE true

    # And now because of all the fun we have to check the same with slirp4netns and pasta because
    # that uses slightly different code paths. Note this would deadlock before the fix.
    # https://github.com/containers/podman/issues/21477
    if has_slirp4netns; then
        cname2=con2-$(safename)
        run_podman 1 run --name $cname2 --network slirp4netns --restart on-failure:2 --userns keep-id $IMAGE false
        wait_for_restart_count $cname2 2 "slirp4netns"
        run_podman wait $cname2
    fi

    if is_rootless; then
        # pasta can only run rootless
        cname3=con3-$(safename)
        run_podman 1 run --name $cname3 --network pasta --restart on-failure:2 --userns keep-id $IMAGE false
        wait_for_restart_count $cname3 2 "pasta"
        run_podman wait $cname3
    else
        # This is racy if other programs modify /run/netns while the test is running.
        # However I think the risk is minimal and I think checking for this is important.
        assert "$(ls /run/netns | wc -l)" == "$netns_count" "/run/netns has no leaked netns files"
    fi

    run_podman rm $cname $cname2 $cname3
    run_podman network rm $net1
}

# Issue #20448 - /etc/hostname with --uts=host must show "uname -n"
# bats test_tags=ci:parallel
@test "podman --uts=host must use 'uname -n' for /etc/hostname" {
    run_podman info --format '{{.Host.Hostname}}'
    hostname="$output"
    run_podman run --rm --uts=host $IMAGE cat /etc/hostname
    assert "$output" = $hostname "/etc/hostname with --uts=host must be equal to 'uname -n'"

    run_podman run --rm --net=host --uts=host $IMAGE cat /etc/hostname
    assert "$output" = $hostname "/etc/hostname with --uts=host --net=host must be equal to 'uname -n'"
}

# FIXME: random_rfc1918_subnet is not parallel-safe
@test "podman network inspect running containers" {
    local cname1=c1-$(safename)
    local cname2=c2-$(safename)
    local cname3=c3-$(safename)

    local netname=net-$(safename)
    local subnet=$(random_rfc1918_subnet)

    run_podman network create --subnet "${subnet}.0/24" $netname

    run_podman network inspect --format "{{json .Containers}}" $netname
    assert "$output" == "{}" "no containers on the network"

    run_podman create --name $cname1 --network $netname $IMAGE top
    cid1="$output"
    run_podman create --name $cname2 --network $netname $IMAGE top
    cid2="$output"

    # containers should only be part of the output when they are running
    run_podman network inspect --format "{{json .Containers}}" $netname
    assert "$output" == "{}" "no running containers on the network"

    # start the containers to setup the network info
    run_podman start $cname1 $cname2

    # also run a third container on different network (should not be part of inspect then)
    run_podman run -d --name $cname3 --network podman $IMAGE top
    cid3="$output"

    # Map ordering is not deterministic so we check each container one by one
    local expect="\{\"name\":\"$cname1\",\"interfaces\":\{\"eth0\":\{\"subnets\":\[\{\"ipnet\":\"${subnet}.2/24\"\,\"gateway\":\"${subnet}.1\"\}\],\"mac_address\":\"[0-9a-f]{2}:.*\"\}\}\}"
    run_podman network inspect --format "{{json (index .Containers \"$cid1\")}}" $netname
    assert "$output" =~ "$expect" "container 1 on the network"

    local expect="\{\"name\":\"$cname2\",\"interfaces\":\{\"eth0\":\{\"subnets\":\[\{\"ipnet\":\"${subnet}.3/24\"\,\"gateway\":\"${subnet}.1\"\}\],\"mac_address\":\"[0-9a-f]{2}:.*\"\}\}\}"
    run_podman network inspect --format "{{json (index .Containers \"$cid2\")}}" $netname
    assert "$output" =~ "$expect" "container 2 on the network"

    # container 3 should not be part of the inspect, index does not error if the key does not
    # exists so just make sure the cid3 and cname3 are not in the json.
    run_podman network inspect --format "{{json .Containers}}" $netname
    assert "$output" !~ "$cid3" "container 3 on the network (cid)"
    assert "$output" !~ "$cname3" "container 3 on the network (name)"

    run_podman rm -f -t0 $cname1 $cname2 $cname3
    run_podman network rm $netname
}

# 2024-07-23 moved from 505 pasta tests because it can't run in parallel
# CANNOT BE PARALLELIZED
@test "Podman unshare --rootless-netns with Pasta" {
    skip_if_remote "unshare is local-only"
    skip_if_not_rootless "pasta networking only available in rootless mode"
    skip_if_no_pasta "pasta not found; this test requires pasta"

    pasta_iface=$(default_ifname 4)
    assert "$pasta_iface" != "" "pasta_iface is set"

    # First let's force a setup error by making pasta be "false".
    ln -s /usr/bin/false $PODMAN_TMPDIR/pasta
    CONTAINERS_HELPER_BINARY_DIR="$PODMAN_TMPDIR" run_podman 125 unshare --rootless-netns ip addr
    assert "$output" =~ "pasta failed with exit code 1"

    # Now this should recover from the previous error and setup the netns correctly.
    run_podman unshare --rootless-netns ip addr
    is "$output" ".*${pasta_iface}.*"
}

# vim: filetype=sh
