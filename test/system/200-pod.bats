#!/usr/bin/env bats

load helpers

LOOPDEVICE=

# This is a long ugly way to clean up pods and remove the pause image
function teardown() {
    run_podman pod rm -f -t 0 -a
    run_podman rm -f -t 0 -a
    run_podman rmi --ignore $(pause_image)
    basic_teardown
    if [[ -n "$LOOPDEVICE" ]]; then
        losetup -d $LOOPDEVICE
    fi
}


@test "podman pod - basic tests" {
    run_podman pod list --noheading
    is "$output" "" "baseline: empty results from list --noheading"

    run_podman pod ls --noheading
    is "$output" "" "baseline: empty results from ls --noheading"

    run_podman pod ps --noheading
    is "$output" "" "baseline: empty results from ps --noheading"
}

@test "podman pod top - containers in different PID namespaces" {
    # With infra=false, we don't get a /pause container
    no_infra='--infra=false'
    run_podman pod create $no_infra
    podid="$output"

    # Start two containers...
    run_podman run -d --pod $podid $IMAGE top -d 2
    cid1="$output"
    run_podman run -d --pod $podid $IMAGE top -d 2
    cid2="$output"

    # ...and wait for them to actually start.
    wait_for_output "PID \+PPID \+USER " $cid1
    wait_for_output "PID \+PPID \+USER " $cid2

    # Both containers have emitted at least one top-like line.
    # Now run 'pod top', and expect two 'top -d 2' processes running.
    run_podman pod top $podid
    is "$output" ".*root.*top -d 2.*root.*top -d 2" "two 'top' containers"

    # By default (podman pod create w/ default --infra) there should be
    # a /pause container.
    if [ -z "$no_infra" ]; then
        is "$output" ".*0 \+1 \+0 \+[0-9. ?s]\+/pause" "there is a /pause container"
    fi

    # Clean up
    run_podman --noout pod rm -f -t 0 $podid
    is "$output" "" "output should be empty"
}


@test "podman pod create - custom infra image" {
    skip_if_remote "CONTAINERS_CONF only affects server side"
    image="i.do/not/exist:image"
    tmpdir=$PODMAN_TMPDIR/pod-test
    mkdir -p $tmpdir
    containersconf=$tmpdir/containers.conf
    cat >$containersconf <<EOF
[engine]
infra_image="$image"
EOF

    run_podman 125 pod create --infra-image $image
    is "$output" ".*initializing source docker://$image:.*"

    CONTAINERS_CONF=$containersconf run_podman 125 pod create
    is "$output" ".*initializing source docker://$image:.*"

    CONTAINERS_CONF=$containersconf run_podman 125 create --pod new:test $IMAGE
    is "$output" ".*initializing source docker://$image:.*"
}

@test "podman pod - communicating between pods" {
    podname=pod$(random_string)
    run_podman 1 pod exists $podname
    run_podman pod create --infra=true --name=$podname
    podid="$output"
    run_podman pod exists $podname
    run_podman pod exists $podid

    # (Assert that output is formatted, not a one-line blob: #8021)
    run_podman pod inspect $podname
    assert "${#lines[*]}" -ge 10 "Output from 'pod inspect'; see #8011"

    # Randomly-assigned port in the 5xxx range
    port=$(random_free_port)

    # Listener. This will exit as soon as it receives a message.
    run_podman run -d --pod $podname $IMAGE nc -l -p $port
    cid1="$output"

    # (While we're here, test the 'Pod' field of 'podman ps'. Expect two ctrs)
    run_podman ps --format '{{.Pod}}'
    newline="
"
    is "$output" "${podid:0:12}${newline}${podid:0:12}" "ps shows 2 pod IDs"

    # Talker: send the message via common port on localhost
    message=$(random_string 15)
    run_podman run --rm --pod $podname $IMAGE \
               sh -c "echo $message | nc 127.0.0.1 $port"

    # Back to the first (listener) container. Make sure message was received.
    run_podman logs $cid1
    is "$output" "$message" "message sent from one container to another"

    # Clean up. First the nc -l container...
    run_podman rm $cid1

    # ...then rm the pod, then rmi the pause image so we don't leave strays.
    run_podman pod rm $podname

    # Pod no longer exists
    run_podman 1 pod exists $podid
    run_podman 1 pod exists $podname
}

@test "podman pod - communicating via /dev/shm " {
    podname=pod$(random_string)
    run_podman 1 pod exists $podname
    run_podman pod create --infra=true --name=$podname
    podid="$output"
    run_podman pod exists $podname
    run_podman pod exists $podid

    run_podman run --rm --pod $podname $IMAGE touch /dev/shm/test1
    run_podman run --rm --pod $podname $IMAGE ls /dev/shm/test1
    is "$output" "/dev/shm/test1"

    # ...then rm the pod, then rmi the pause image so we don't leave strays.
    run_podman pod rm $podname

    # Pod no longer exists
    run_podman 1 pod exists $podid
    run_podman 1 pod exists $podname
}

# Random byte
function octet() {
    echo $(( $RANDOM & 255 ))
}

# random MAC address: convention seems to be that 2nd lsb=1, lsb=0
# (i.e. 0bxxxxxx10) in the first octet guarantees a private space.
# FIXME: I can't find a definitive reference for this though
# Generate the address IN CAPS (A-F), but we will test it in lowercase.
function random_mac() {
    local mac=$(printf "%02X" $(( $(octet) & 242 | 2 )) )
    for i in $(seq 2 6); do
        mac+=$(printf ":%02X" $(octet))
    done

    echo $mac
}

# Random RFC1918 IP address
function random_ip() {
    local ip="172.20"
    for i in 1 2;do
        ip+=$(printf ".%d" $(octet))
    done
    echo $ip
}

@test "podman pod create - hashtag AllTheOptions" {
    mac=$(random_mac)
    add_host_ip=$(random_ip)
    add_host_n=$(random_string | tr A-Z a-z).$(random_string | tr A-Z a-z).xyz

    dns_server=$(random_ip)
    dns_opt="ndots:$(octet)"
    dns_search=$(random_string 15 | tr A-Z a-z).abc

    hostname=$(random_string | tr A-Z a-z).$(random_string | tr A-Z a-z).net

    labelname=$(random_string 11)
    labelvalue=$(random_string 22)

    pod_id_file=${PODMAN_TMPDIR}/pod-id-file

    # Randomly-assigned ports in the 5xxx and 6xxx range
    port_in=$(random_free_port 5000-5999)
    port_out=$(random_free_port 6000-6999)

    # Create a pod with all the desired options
    # FIXME: --ip=$ip fails:
    #      Error adding network: failed to allocate all requested IPs
    local mac_option="--mac-address=$mac"

    # Create a custom image so we can test --infra-image and -command.
    # It will have a randomly generated infra command, using the
    # existing 'pause' script in our testimage. We assign a bogus
    # entrypoint to confirm that --infra-command will override.
    local infra_image="infra_$(random_string 10 | tr A-Z a-z)"
    local infra_command="/pause_$(random_string 10)"
    local infra_name="infra_container_$(random_string 10 | tr A-Z a-z)"
    run_podman build -t $infra_image - << EOF
FROM $IMAGE
RUN ln /home/podman/pause $infra_command
ENTRYPOINT ["/original-entrypoint-should-be-overridden"]
EOF

    if is_rootless; then
        mac_option=
    fi
    run_podman pod create --name=mypod                   \
               --pod-id-file=$pod_id_file                \
               $mac_option                               \
               --hostname=$hostname                      \
               --add-host   "$add_host_n:$add_host_ip"   \
               --dns        "$dns_server"                \
               --dns-search "$dns_search"                \
               --dns-option "$dns_opt"                   \
               --publish    "$port_out:$port_in"         \
               --label      "${labelname}=${labelvalue}" \
               --infra-image   "$infra_image"            \
               --infra-command "$infra_command"          \
               --infra-name "$infra_name"
    pod_id="$output"

    # Check --pod-id-file
    is "$(<$pod_id_file)" "$pod_id" "contents of pod-id-file"

    # Get ID of infra container
    run_podman pod inspect --format '{{(index .Containers 0).ID}}' mypod
    local infra_cid="$output"
    # confirm that entrypoint is what we set
    run_podman container inspect --format '{{.Config.Entrypoint}}' $infra_cid
    is "$output" "$infra_command" "infra-command took effect"
    # confirm that infra container name is set
    run_podman container inspect --format '{{.Name}}' $infra_cid
    is "$output" "$infra_name" "infra-name took effect"

    # Check each of the options
    if [ -n "$mac_option" ]; then
        run_podman run --rm --pod mypod $IMAGE ip link show
        # 'ip' outputs hex in lower-case, ${expr,,} converts UC to lc
        is "$output" ".* link/ether ${mac,,} " "requested MAC address was set"
    fi

    run_podman run --rm --pod mypod $IMAGE hostname
    is "$output" "$hostname" "--hostname set the hostname"
    run_podman 125 run --rm --pod mypod --hostname foobar $IMAGE hostname
    is "$output" ".*invalid config provided: cannot set hostname when joining the pod UTS namespace: invalid configuration" "--hostname should not be allowed in share UTS pod"

    run_podman run --rm --pod $pod_id $IMAGE cat /etc/hosts
    is "$output" ".*$add_host_ip[[:blank:]]$add_host_n" "--add-host was added"
    is "$output" ".*	$hostname"            "--hostname is in /etc/hosts"
    #               ^^^^ this must be a tab, not a space

    run_podman run --rm --pod mypod $IMAGE cat /etc/resolv.conf
    is "$output" ".*nameserver $dns_server"  "--dns [server] was added"
    is "$output" ".*search $dns_search"      "--dns-search was added"
    is "$output" ".*options $dns_opt"        "--dns-option was added"

    # pod inspect
    run_podman pod inspect --format '{{.Name}}: {{.ID}} : {{.NumContainers}} : {{.Labels}}' mypod
    is "$output" "mypod: $pod_id : 1 : map\[${labelname}:${labelvalue}]" \
       "pod inspect --format ..."

    # pod ps
    run_podman pod ps --format '{{.ID}} {{.Name}} {{.Status}} {{.Labels}}'
    is "$output" "${pod_id:0:12} mypod Running map\[${labelname}:${labelvalue}]"  "pod ps"

    run_podman pod ps --no-trunc --filter "label=${labelname}=${labelvalue}" --format '{{.ID}}'
    is "$output" "$pod_id" "pod ps --filter label=..."

    # Test local port forwarding, as well as 'ps' output showing ports
    # Run 'nc' in a container, waiting for input on the published port.
    c_name=$(random_string 15)
    run_podman run -d --pod mypod --name $c_name $IMAGE nc -l -p $port_in
    cid="$output"

    # Try running another container also listening on the same port.
    run_podman 1 run --pod mypod --name dsfsdfsdf $IMAGE nc -l -p $port_in
    is "$output" "nc: bind: Address in use" \
       "two containers cannot bind to same port"

    # make sure we can ping; failure here might mean that capabilities are wrong
    run_podman run --rm --pod mypod $IMAGE ping -c1 127.0.0.1
    run_podman run --rm --pod mypod $IMAGE ping -c1 $hostname

    # While the container is still running, run 'podman ps' (no --format)
    # and confirm that the output includes the published port
    run_podman ps --filter id=$cid
    is "${lines[1]}" "${cid:0:12}  $IMAGE  nc -l -p $port_in .* 0.0.0.0:$port_out->$port_in/tcp  $c_name" \
       "output of 'podman ps'"

    # send a random string to the container. This will cause the container
    # to output the string to its logs, then exit.
    teststring=$(random_string 30)
    echo "$teststring" | nc 127.0.0.1 $port_out

    # Confirm that the container log output is the string we sent it.
    run_podman logs $cid
    is "$output" "$teststring" "test string received on container"

    # Finally, confirm the infra-container and -command. We run this late,
    # not at pod creation, to give the infra container time to start & log.
    run_podman logs $infra_cid
    is "$output" "Confirmed: testimage pause invoked as $infra_command" \
       "pod ran with our desired infra container + command"

    # Clean up
    run_podman rm $cid
    run_podman pod rm -t 0 -f mypod
    run_podman rmi $infra_image
}

@test "podman pod create should fail when infra-name is already in use" {
    local infra_name="infra_container_$(random_string 10 | tr A-Z a-z)"
    local infra_image="k8s.gcr.io/pause:3.5"
    local pod_name="$(random_string 10 | tr A-Z a-z)"

    run_podman --noout pod create --name $pod_name --infra-name "$infra_name" --infra-image "$infra_image"
    is "$output" "" "output from pod create should be empty"

    run_podman 125 pod create --infra-name "$infra_name"
    assert "$output" =~ "^Error: .*: the container name \"$infra_name\" is already in use by .* You have to remove that container to be able to reuse that name: that name is already in use" \
           "Trying to create two pods with same infra-name"

    run_podman pod rm -f $pod_name
    run_podman rmi $infra_image
}

@test "podman pod create --share" {
    local pod_name="$(random_string 10 | tr A-Z a-z)"
    run_podman 125 pod create --share bogus --name $pod_name
    is "$output" ".*invalid kernel namespace to share: bogus. Options are: cgroup, ipc, net, pid, uts or none" \
       "pod test for bogus --share option"
    run_podman pod create --share ipc --name $pod_name
    run_podman pod inspect $pod_name --format "{{.SharedNamespaces}}"
    is "$output" "[ipc]"
    run_podman run --rm --pod $pod_name --hostname foobar $IMAGE hostname
    is "$output" "foobar" "--hostname should work with non share UTS namespace"
    run_podman pod create --share +pid --replace --name $pod_name
    run_podman pod inspect $pod_name --format "{{.SharedNamespaces}}"
    for ns in uts pid ipc net; do
        is "$output" ".*$ns"
    done
}

@test "podman pod create --pod new:$POD --hostname" {
    local pod_name="$(random_string 10 | tr A-Z a-z)"
    run_podman run --rm --pod "new:$pod_name" --hostname foobar $IMAGE hostname
    is "$output" "foobar" "--hostname should work when creating a new:pod"
    run_podman pod rm $pod_name
    run_podman run --rm --pod "new:$pod_name" $IMAGE hostname
    is "$output" "$pod_name" "new:POD should have hostname name set to podname"
}

@test "podman rm --force to remove infra container" {
    local pod_name="$(random_string 10 | tr A-Z a-z)"
    run_podman create --pod "new:$pod_name" $IMAGE
    container_ID="$output"
    run_podman pod inspect --format "{{.InfraContainerID}}" $pod_name
    infra_ID="$output"

    run_podman 125 container rm $infra_ID
    is "$output" ".* and cannot be removed without removing the pod"
    run_podman 125 container rm --force $infra_ID
    is "$output" ".* and cannot be removed without removing the pod"

    run_podman container rm --depend $infra_ID
    is "$output" ".*$infra_ID.*"
    is "$output" ".*$container_ID.*"

    # Now make sure that --force --all works as well
    run_podman create --pod "new:$pod_name" $IMAGE
    container_1_ID="$output"
    run_podman create --pod "$pod_name" $IMAGE
    container_2_ID="$output"
    run_podman create $IMAGE
    container_3_ID="$output"
    run_podman pod inspect --format "{{.InfraContainerID}}" $pod_name
    infra_ID="$output"

    run_podman container rm --force --all $infraID
    is "$output" ".*$infra_ID.*"
    is "$output" ".*$container_1_ID.*"
    is "$output" ".*$container_2_ID.*"
    is "$output" ".*$container_3_ID.*"
}

@test "podman pod create share net" {
    run_podman pod create --name test
    run_podman pod inspect test --format {{.InfraConfig.HostNetwork}}
    is "$output" "false" "Default network sharing should be false"
    run_podman pod rm test

    run_podman pod create --share ipc  --network private test
    run_podman pod inspect test --format {{.InfraConfig.HostNetwork}}
    is "$output" "false" "Private network sharing with only ipc should be false"
    run_podman pod rm test

    local name="$(random_string 10 | tr A-Z a-z)"
    run_podman pod create --name $name --share net  --network private
    run_podman pod inspect $name --format {{.InfraConfig.HostNetwork}}
    is "$output" "false" "Private network sharing with only net should be false"

    run_podman pod create --share net --network host --replace $name
    run_podman pod inspect $name --format {{.InfraConfig.HostNetwork}}
    is "$output" "true" "Host network sharing with only net should be true"
    run_podman pod rm $name

    run_podman pod create --name test --share ipc --network host
    run_podman pod inspect test --format {{.InfraConfig.HostNetwork}}
    is "$output" "true" "Host network sharing with only ipc should be true"
    run_podman pod rm test
}

@test "pod exit policies" {
    # Test setting exit policies
    run_podman pod create
    podID="$output"
    run_podman pod inspect $podID --format "{{.ExitPolicy}}"
    is "$output" "continue" "default exit policy"
    run_podman pod rm $podID

    run_podman pod create --exit-policy stop
    podID="$output"
    run_podman pod inspect $podID --format "{{.ExitPolicy}}"
    is "$output" "stop" "custom exit policy"
    run_podman pod rm $podID

    run_podman 125 pod create --exit-policy invalid
    is "$output" "Error: .*running pod create option: invalid pod exit policy: \"invalid\"" "invalid exit policy"

    # Test exit-policy behaviour
    run_podman pod create --exit-policy continue
    podID="$output"
    run_podman run --pod $podID $IMAGE true
    run_podman pod inspect $podID --format "{{.State}}"
    _ensure_pod_state $podID Degraded
    run_podman pod rm $podID

    run_podman pod create --exit-policy stop
    podID="$output"
    run_podman run --pod $podID $IMAGE true
    run_podman pod inspect $podID --format "{{.State}}"
    _ensure_pod_state $podID Exited
    run_podman pod rm $podID
}

@test "pod exit policies - play kube" {
    # play-kube sets the exit policy to "stop"
    local name="$(random_string 10 | tr A-Z a-z)"

    kubeFile="apiVersion: v1
kind: Pod
metadata:
  name: $name-pod
spec:
  containers:
  - command:
    - \"true\"
    image: $IMAGE
    name: ctr
  restartPolicy: OnFailure"

    echo "$kubeFile" > $PODMAN_TMPDIR/test.yaml
    run_podman play kube $PODMAN_TMPDIR/test.yaml
    run_podman pod inspect $name-pod --format "{{.ExitPolicy}}"
    is "$output" "stop" "custom exit policy"
    _ensure_pod_state $name-pod Exited
    run_podman pod rm $name-pod
}

@test "pod resource limits" {
    skip_if_remote "resource limits only implemented on non-remote"
    skip_if_rootless "resource limits only work with root"
    skip_if_cgroupsv1 "resource limits only meaningful on cgroups V2"
    skip_if_aarch64 "FIXME: #15074 - flakes often on aarch64"

    # create loopback device
    lofile=${PODMAN_TMPDIR}/disk.img
    fallocate -l 1k  ${lofile}
    LOOPDEVICE=$(losetup --show -f $lofile)

    # tr needed because losetup seems to use %2d
    lomajmin=$(losetup -l --noheadings --output MAJ:MIN $LOOPDEVICE | tr -d ' ')
    run grep -w bfq /sys/block/$(basename ${LOOPDEVICE})/queue/scheduler
    if [ $status -ne 0 ]; then
        losetup -d $LOOPDEVICE
        LOOPDEVICE=
        skip "BFQ scheduler is not supported on the system"
    fi
    echo bfq > /sys/block/$(basename ${LOOPDEVICE})/queue/scheduler

    # FIXME: #15464: blkio-weight-device not working
    expected_limits="
cpu.max         | 500000 100000
memory.max      | 5242880
memory.swap.max | 1068498944
io.bfq.weight   | default 50
io.max          | $lomajmin rbps=1048576 wbps=1048576 riops=max wiops=max
"

    for cgm in systemd cgroupfs; do
        local name=resources-$cgm
        run_podman --cgroup-manager=$cgm pod create --name=$name --cpus=5 --memory=5m --memory-swap=1g --cpu-shares=1000 --cpuset-cpus=0 --cpuset-mems=0 --device-read-bps=${LOOPDEVICE}:1mb --device-write-bps=${LOOPDEVICE}:1mb --blkio-weight=50
        run_podman --cgroup-manager=$cgm pod start $name
        run_podman pod inspect --format '{{.CgroupPath}}' $name
        local cgroup_path="$output"

        while read unit expect; do
            local actual=$(< /sys/fs/cgroup/$cgroup_path/$unit)
            is "$actual" "$expect" "resource limit under $cgm: $unit"
        done < <(parse_table "$expected_limits")
        run_podman --cgroup-manager=$cgm pod rm -f $name
    done

    # Clean up, and prevent duplicate cleanup in teardown
    losetup -d $LOOPDEVICE
    LOOPDEVICE=
}

@test "podman pod ps doesn't race with pod rm" {
    # create a few pods
    for i in {0..10}; do
        run_podman pod create
    done

    # and delete them
    $PODMAN pod rm -a &

    # pod ps should not fail while pods are deleted
    run_podman pod ps -q

    # wait for pod rm -a
    wait
}

@test "podman pod rm --force bogus" {
    run_podman 1 pod rm bogus
    is "$output" "Error: .*bogus.*: no such pod" "Should print error"
    run_podman pod rm --force bogus
    is "$output" "" "Should print no output"
}

# vim: filetype=sh
