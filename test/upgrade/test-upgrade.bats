# -*- bats -*-

load helpers

# Create a var-lib-containers dir for this podman. We need to bind-mount
# this into the container, and use --root and --runroot and --tmpdir
# options both in the container podman and out here: that's the only
# way to share image and container storage.
if [ -z "${PODMAN_UPGRADE_WORKDIR}" ]; then
    # Much as I'd love a descriptive name like "podman-upgrade-tests.XXXXX",
    # keep it short ("pu") because of the 100-character path length limit
    # for UNIX sockets (needed by conmon)
    export PODMAN_UPGRADE_WORKDIR=$(mktemp -d --tmpdir=${BATS_TMPDIR:-${TMPDIR:-/tmp}} pu.XXXXXX)

    touch $PODMAN_UPGRADE_WORKDIR/status
fi

# Generate a set of random strings used for content verification
if [ -z "${RANDOM_STRING_1}" ]; then
    export RANDOM_STRING_1=$(random_string 15)
    export LABEL_CREATED=$(random_string 16)
    export LABEL_FAILED=$(random_string 17)
    export LABEL_RUNNING=$(random_string 18)

    # FIXME: randomize this
    HOST_PORT=34567
fi

# Version string of the podman we're actually testing, e.g. '3.0.0-dev-d1a26013'
PODMAN_VERSION=$($PODMAN version  |awk '/^Version:/ { V=$2 } /^Git Commit:/ { G=$3 } END { print V "-" substr(G,0,8) }')

setup() {
    skip_if_rootless

    # The podman-in-podman image (old podman)
    if [[ -z "$PODMAN_UPGRADE_FROM" ]]; then
        echo "# \$PODMAN_UPGRADE_FROM is undefined (should be e.g. v1.9.0)" >&3
        false
    fi

    if [ "$(< $PODMAN_UPGRADE_WORKDIR/status)" = "failed" ]; then
        # FIXME: exit instead?
        echo "*** setup failed - no point in running tests"
        false
    fi

    export _PODMAN_TEST_OPTS="--root=$PODMAN_UPGRADE_WORKDIR/root --runroot=$PODMAN_UPGRADE_WORKDIR/runroot --tmpdir=$PODMAN_UPGRADE_WORKDIR/tmp"
}

###############################################################################
# BEGIN setup

@test "initial setup: start $PODMAN_UPGRADE_FROM containers" {
    echo failed >| $PODMAN_UPGRADE_WORKDIR/status

    OLD_PODMAN=quay.io/podman/stable:$PODMAN_UPGRADE_FROM
    $PODMAN pull $OLD_PODMAN

    # Shortcut name, because we're referencing it a lot
    pmroot=$PODMAN_UPGRADE_WORKDIR

    # WWW content to share
    mkdir -p $pmroot/var/www
    echo $RANDOM_STRING_1 >$pmroot/var/www/index.txt

    # podman tmpdir
    mkdir -p $pmroot/tmp

    #
    # Script to run >>OLD<< podman commands.
    #
    # These commands will be run inside a podman container. The "podman"
    # command in this script will be the desired old-podman version.
    #
    pmscript=$pmroot/setup
    cat >| $pmscript <<EOF
#!/bin/bash

# cgroup-manager=systemd does not work inside a container
opts="--cgroup-manager=cgroupfs --events-backend=file $_PODMAN_TEST_OPTS"

set -ex

# Try try again, because network flakiness makes this a point of failure
podman \$opts pull $IMAGE \
  || (sleep 10; podman \$opts pull $IMAGE) \
  || (sleep 30; podman \$opts pull $IMAGE)


podman \$opts create --name mycreatedcontainer --label mylabel=$LABEL_CREATED \
                                               $IMAGE false

podman \$opts run    --name mydonecontainer    $IMAGE echo ++$RANDOM_STRING_1++

podman \$opts run    --name myfailedcontainer  --label mylabel=$LABEL_FAILED \
                                               $IMAGE sh -c 'exit 17' || true

# FIXME: add "-p $HOST_PORT:80"
#    ...I tried and tried, and could not get this to work. I could never
#    connect to the port from the host, nor even from the podman_parent
#    container; I could never see the port listed in 'ps' nor 'inspect'.
#    And, finally, I ended up in a state where the container wouldn't
#    even start, and via complicated 'podman logs' found out:
#        httpd: bind: Address in use
#    So I just give up for now.
#
podman \$opts run -d --name myrunningcontainer --label mylabel=$LABEL_RUNNING \
                                               -v $pmroot/var/www:/var/www \
                                               -w /var/www \
                                               $IMAGE /bin/busybox-extras httpd -f -p 80

podman \$opts pod create --name mypod

echo READY
while :;do
    if [ -e /stop ]; then
        echo STOPPING
        podman \$opts stop -t 0 myrunningcontainer || true
        podman \$opts rm -f     myrunningcontainer || true
        exit 0
    fi
    sleep 0.5
done
EOF
    chmod 555 $pmscript

    # Clean up vestiges of previous run
    $PODMAN rm -f podman_parent || true

    # Not entirely a NOP! This is just so we get /run/crun created on a CI VM
    $PODMAN run --rm $OLD_PODMAN true

    #
    # Use new-podman to run the above script under old-podman.
    #
    # DO NOT USE run_podman HERE! That would use $_PODMAN_TEST_OPTS
    # and would write into our shared test dir, which would then
    # pollute it for use by old-podman. We must keep that pristine
    # so old-podman is the first to write to it.
    #
    # mount /etc/containers/storage.conf to use the same storage settings as on the host
    # mount /dev/shm because the container locks are stored there
    #
    $PODMAN run -d --name podman_parent --pid=host \
            --privileged \
            --net=host \
            --cgroupns=host \
            --pid=host \
            -v /etc/containers/storage.conf:/etc/containers/storage.conf \
            -v /dev/fuse:/dev/fuse \
            -v /run/crun:/run/crun \
            -v /dev/shm:/dev/shm \
            -v $pmroot:$pmroot \
            $OLD_PODMAN $pmroot/setup

    _PODMAN_TEST_OPTS= wait_for_ready podman_parent

    echo OK >| $PODMAN_UPGRADE_WORKDIR/status
}

# END   setup
###############################################################################
# BEGIN actual tests

# This is a NOP; used only so the version string will show up in logs
@test "upgrade: $PODMAN_UPGRADE_FROM -> $PODMAN_VERSION" {
    :
}

@test "images" {
    run_podman images -a --format '{{.Names}}'
    is "$output" "\[$IMAGE\]" "podman images"
}

@test "ps : one container running" {
    run_podman ps --format '{{.Image}}--{{.Names}}'
    is "$output" "$IMAGE--myrunningcontainer" "ps: one container running"
}

@test "ps -a : shows all containers" {
    # IMPORTANT: we can't use --sort=created, because that requires #8427
    # on the *creating* podman end.
    run_podman ps -a \
               --format '{{.Names}}--{{.Status}}--{{.Ports}}--{{.Labels.mylabel}}' \
               --sort=names
    is "${lines[0]}" ".*-infra--Created----<no value>" "infra container"
    is "${lines[1]}" "mycreatedcontainer--Created----$LABEL_CREATED" "created"
    is "${lines[2]}" "mydonecontainer--Exited (0).*----<no value>" "done"
    is "${lines[3]}" "myfailedcontainer--Exited (17) .*----$LABEL_FAILED" "fail"
    is "${lines[4]}" "myrunningcontainer--Up .*----$LABEL_RUNNING" "running"

    # For debugging: dump containers and IDs
    if [[ -n "$PODMAN_UPGRADE_TEST_DEBUG" ]]; then
        run_podman ps -a
        for l in "${lines[@]}"; do
            echo "# $l" >&3
        done
    fi
}


@test "inspect - all container status" {
    tests="
running   | running    |  0
created   | configured |  0
done      | exited     |  0
failed    | exited     | 17
"
    while read cname state exitstatus; do
        run_podman inspect --format '{{.State.Status}}--{{.State.ExitCode}}' my${cname}container
        is "$output" "$state--$exitstatus" "status of my${cname}container"
    done < <(parse_table "$tests")
}

@test "logs" {
    run_podman logs mydonecontainer
    is "$output" "++$RANDOM_STRING_1++" "podman logs on stopped container"
}

@test "exec" {
    run_podman exec myrunningcontainer cat /var/www/index.txt
    is "$output" "$RANDOM_STRING_1" "exec into myrunningcontainer"
}

@test "load" {
    # FIXME, is this really necessary?
    skip "TBI. Not sure if there's any point to this."
}

@test "mount" {
    skip "TBI"
}

@test "pods" {
    run_podman pod inspect mypod
    is "$output" ".*mypod.*"

    run_podman --cgroup-manager=cgroupfs pod start mypod
    is "$output" "[0-9a-f]\\{64\\}" "podman pod start"

    run_podman pod ps
    is "$output" ".*mypod.*" "podman pod ps shows name"
    is "$output" ".*Running.*" "podman pod ps shows running state"

    run_podman pod stop mypod
    is "$output" "[0-9a-f]\\{64\\}" "podman pod stop"

    run_podman --cgroup-manager=cgroupfs pod rm mypod
    # FIXME: CI runs show this (non fatal) error:
    # Error updating pod <ID> conmon cgroup PID limit: open /sys/fs/cgroup/libpod_parent/<ID>/conmon/pids.max: no such file or directory
    # Investigate how to fix this (likely a race condition)
    # Let's ignore the logrus messages for now
    is "$output" ".*[0-9a-f]\\{64\\}" "podman pod rm"
}

# FIXME: commit? kill? network? pause? restart? top? volumes? What else?


@test "start" {
    run_podman --cgroup-manager=cgroupfs start -a mydonecontainer
    is "$output" "++$RANDOM_STRING_1++" "start on already-run container"
}

@test "rm a stopped container" {
    run_podman rm myfailedcontainer
    is "$output" "[0-9a-f]\\{64\\}" "podman rm myfailedcontainer"

    run_podman rm mydonecontainer
    is "$output" "[0-9a-f]\\{64\\}" "podman rm mydonecontainer"
}


@test "stop and rm" {
    run_podman stop myrunningcontainer
    run_podman rm   myrunningcontainer
}

@test "clean up parent" {
    if [[ -n "$PODMAN_UPGRADE_TEST_DEBUG" ]]; then
        skip "workdir is $PODMAN_UPGRADE_WORKDIR"
    fi

    # We're done with shared environment. By clearing this, we can now
    # use run_podman for actions on the podman_parent container
    unset _PODMAN_TEST_OPTS

    # (Useful for debugging the 'rm -f' step below, which, when it fails, only
    # gives a container ID. This 'ps' confirms that the CID is podman_parent)
    run_podman ps -a

    # Stop the container gracefully
    run_podman exec podman_parent touch /stop
    run_podman wait podman_parent

    run_podman logs podman_parent
    run_podman rm -f podman_parent

    umount $PODMAN_UPGRADE_WORKDIR/root/overlay || true

    rm -rf $PODMAN_UPGRADE_WORKDIR
}

# FIXME: now clean up
