#!/usr/bin/env bats   -*- bats -*-
# shellcheck disable=SC2096
#
# Tests for podman build
#

load helpers

function _require_crun() {
    runtime=$(podman_runtime)
    if [[ $runtime != "crun" ]]; then
        skip "runtime is $runtime; keep-groups requires crun"
    fi
}

@test "podman --group-add keep-groups while in a userns" {
    skip_if_rootless "chroot is not allowed in rootless mode"
    skip_if_remote "--group-add keep-groups not supported in remote mode"
    _require_crun
    run chroot --groups 1234 / ${PODMAN} run --rm --uidmap 0:200000:5000 --group-add keep-groups $IMAGE id
    is "$output" ".*65534(nobody)" "Check group leaked into user namespace"
}

@test "podman --group-add keep-groups while not in a userns" {
    skip_if_rootless "chroot is not allowed in rootless mode"
    skip_if_remote "--group-add keep-groups not supported in remote mode"
    _require_crun
    run chroot --groups 1234,5678 / ${PODMAN} run --rm --group-add keep-groups $IMAGE id
    is "$output" ".*1234" "Check group leaked into container"
}

@test "podman --group-add without keep-groups while in a userns" {
    skip_if_cgroupsv1 "FIXME: #15025: run --uidmap fails on cgroups v1"
    skip_if_rootless "chroot is not allowed in rootless mode"
    skip_if_remote "--group-add keep-groups not supported in remote mode"
    run chroot --groups 1234,5678 / ${PODMAN} run --rm --uidmap 0:200000:5000 --group-add 457 $IMAGE id
    is "$output" ".*457" "Check group leaked into container"
}

@test "rootful pod with custom ID mapping" {
    skip_if_cgroupsv1 "FIXME: #15025: run --uidmap fails on cgroups v1"
    skip_if_rootless "does not work rootless - rootful feature"
    random_pod_name=$(random_string 30)
    run_podman pod create --uidmap 0:200000:5000 --name=$random_pod_name
    run_podman pod start $random_pod_name
    run_podman pod inspect --format '{{.InfraContainerID}}' $random_pod_name
    run_podman inspect --format '{{.HostConfig.IDMappings.UIDMap}}' $output
    is "$output" ".*0:200000:5000" "UID Map Successful"

    # Remove the pod and the pause image
    run_podman pod rm $random_pod_name
    run_podman version --format "{{.Server.Version}}-{{.Server.Built}}"
    run_podman rmi -f localhost/podman-pause:$output
}

@test "podman --remote --group-add keep-groups " {
    if is_remote; then
        run_podman 125 run --rm --group-add keep-groups $IMAGE id
        is "$output" ".*not supported in remote mode" "Remote check --group-add keep-groups"
    fi
}

@test "podman --group-add without keep-groups " {
    run_podman run --rm --group-add 457 $IMAGE id
    is "$output" ".*457" "Check group leaked into container"
}

@test "podman --group-add keep-groups plus added groups " {
    run_podman 125 run --rm --group-add keep-groups --group-add 457 $IMAGE id
    is "$output" ".*the '--group-add keep-groups' option is not allowed with any other --group-add options" "Check group leaked into container"
}

@test "podman userns=auto in config file" {
    skip_if_remote "userns=auto is set on the server"

    if is_rootless; then
        egrep -q "^$(id -un):" /etc/subuid || skip "no IDs allocated for current user"
    else
        egrep -q "^containers:" /etc/subuid || skip "no IDs allocated for user 'containers'"
    fi

    cat > $PODMAN_TMPDIR/userns_auto.conf <<EOF
[containers]
userns="auto"
EOF
    # First make sure a user namespace is created
    CONTAINERS_CONF=$PODMAN_TMPDIR/userns_auto.conf run_podman run -d $IMAGE sleep infinity
    cid=$output

    run_podman inspect --format '{{.HostConfig.UsernsMode}}' $cid
    is "$output" "private" "Check that a user namespace was created for the container"

    run_podman rm -t 0 -f $cid

    # Then check that the main user is not mapped into the user namespace
    CONTAINERS_CONF=$PODMAN_TMPDIR/userns_auto.conf run_podman 0 run --rm $IMAGE awk '{if($2 == "0"){exit 1}}' /proc/self/uid_map /proc/self/gid_map
}

@test "podman userns=auto and secrets" {
    ns_user="containers"
    if is_rootless; then
        ns_user=$(id -un)
    fi
    egrep -q "${ns_user}:" /etc/subuid || skip "no IDs allocated for user ${ns_user}"
    test_name="test_$(random_string 12)"
    secret_file=$PODMAN_TMPDIR/secret$(random_string 12)
    secret_content=$(random_string)
    echo ${secret_content} > ${secret_file}
    run_podman secret create ${test_name} ${secret_file}
    run_podman run --rm --secret=${test_name} --userns=auto:size=1000 $IMAGE cat /run/secrets/${test_name}
    is ${output} ${secret_content} "Secrets should work with user namespace"
    run_podman secret rm ${test_name}
}

@test "podman userns=nomap" {
    if is_rootless; then
        ns_user=$(id -un)
        baseuid=$(egrep "${ns_user}:" /etc/subuid | cut -f2 -d:)
        test ! -z ${baseuid} ||  skip "no IDs allocated for user ${ns_user}"

        test_name="test_$(random_string 12)"
        run_podman run -d --userns=nomap $IMAGE sleep 100
        cid=${output}
        run_podman top ${cid} huser
        is "${output}" "HUSER.*${baseuid}" "Container should start with baseuid from /etc/subuid not user UID"
        run_podman rm -t 0 --force ${cid}
    else
        run_podman 125 run -d --userns=nomap $IMAGE sleep 100
        is "${output}" "Error: nomap is only supported in rootless mode" "Container should fail to start since nomap is not supported in rootful mode"
    fi
}

@test "podman userns=keep-id" {
    if is_rootless; then
        user=$(id -u)
        run_podman run --rm --userns=keep-id $IMAGE id -u
        is "${output}" "$user" "Container should run as the current user"
    else
        run_podman 125 run --rm --userns=keep-id $IMAGE id -u
        is "${output}" "Error: keep-id is only supported in rootless mode" "Container should fail to start since keep-id is not supported in rootful mode"
    fi
}

@test "podman userns=keep-id in a pod" {
    if is_rootless; then
        user=$(id -u)
	run_podman pod create --userns keep-id
	pid=$output
        run_podman run --rm --pod $pid $IMAGE id -u
        is "${output}" "$user" "Container should run as the current user"
    else
	run_podman 125 pod create --userns keep-id
        is "${output}" 'Error:.*keep-id is only supported in rootless mode' "pod should fail to be created since keep-id is not supported in rootful mode"
    fi
}
