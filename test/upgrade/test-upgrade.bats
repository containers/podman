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
    export HOST_PORT=$(random_free_port)
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

    # cgroup-manager=systemd does not work inside a container
    export _PODMAN_TEST_OPTS="--cgroup-manager=cgroupfs --root=$PODMAN_UPGRADE_WORKDIR/root --runroot=$PODMAN_UPGRADE_WORKDIR/runroot --tmpdir=$PODMAN_UPGRADE_WORKDIR/tmp"
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

# events-backend=journald does not work inside a container
opts="--events-backend=file $_PODMAN_TEST_OPTS"

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

podman \$opts run -d --name myrunningcontainer --label mylabel=$LABEL_RUNNING \
                                               --network bridge \
                                               -p $HOST_PORT:80 \
                                               -p 127.0.0.1:8080-8082:8080-8082 \
                                               -v $pmroot/var/www:/var/www \
                                               -w /var/www \
                                               --mac-address aa:bb:cc:dd:ee:ff \
                                               $IMAGE /bin/busybox-extras httpd -f -p 80

podman \$opts pod create --name mypod

podman \$opts network create --disable-dns mynetwork

echo READY
while :;do
    if [ -e /stop ]; then
        echo STOPPING
        podman \$opts stop -t 0 myrunningcontainer || true
        podman \$opts rm -f     myrunningcontainer || true
        # sigh, network rm fails with exec: "ip": executable file not found in $PATH
        # we cannot change the images afterwards so we remove it manually (#11403)
        # hardcode /etc/cni/net.d dir for now
        podman \$opts network rm -f mynetwork || rm -f /etc/cni/net.d/mynetwork.conflist
        exit 0
    fi
    sleep 0.5
done
EOF
    chmod 555 $pmscript

    # Clean up vestiges of previous run
    $PODMAN rm -f podman_parent || true

    # Not entirely a NOP! This is just so we get the /run/... mount points created on a CI VM
    # Also use --network host to prevent any netavark/cni conflicts
    $PODMAN run --rm --network host $OLD_PODMAN true

    # Podman 4.0 might no longer use cni so /run/cni and /run/containers will no be created in this case
    # Create directories manually to fix this. Also running with netavark can
    # cause connectivity issues since cni and netavark should never be mixed.
    mkdir -p /run/netns /run/cni /run/containers /var/lib/cni /etc/cni/net.d


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
    # mount /var/lib/cni, /run/cni and /etc/cni/net.d for cni networking
    # mount /run/containers for the dnsname plugin
    #
    $PODMAN run -d --name podman_parent --pid=host \
            --privileged \
            --net=host \
            --cgroupns=host \
            --pid=host \
            -v /etc/containers/storage.conf:/etc/containers/storage.conf \
            -v /dev/fuse:/dev/fuse \
            -v /run/crun:/run/crun \
            -v /run/netns:/run/netns:rshared \
            -v /run/containers:/run/containers \
            -v /run/cni:/run/cni \
            -v /var/lib/cni:/var/lib/cni \
            -v /etc/cni/net.d:/etc/cni/net.d \
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

@test "info" {
    # check network backend, since this is a old version we should use CNI
    # when we start testing from 4.0 we should have netavark as backend
    run_podman info --format '{{.Host.NetworkBackend}}'
    is "$output" "cni" "correct network backend"
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
    is "${lines[4]}" "myrunningcontainer--Up .*--0\.0\.0\.0:$HOST_PORT->80\/tcp, 127\.0\.0\.1\:8080-8082->8080-8082\/tcp--$LABEL_RUNNING" "running"

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
created   | created    |  0
done      | exited     |  0
failed    | exited     | 17
"
    while read cname state exitstatus; do
        run_podman inspect --format '{{.State.Status}}--{{.State.ExitCode}}' my${cname}container
        is "$output" "$state--$exitstatus" "status of my${cname}container"
    done < <(parse_table "$tests")
}

@test "network - curl" {
    run curl --max-time 3 -s 127.0.0.1:$HOST_PORT/index.txt
    is "$output" "$RANDOM_STRING_1" "curl on running container"
}

# IMPORTANT: connect should happen before restart, we want to check
# if we can connect on an existing running container
@test "network - connect" {
    skip_if_version_older 2.2.0
    touch $PODMAN_UPGRADE_WORKDIR/ran-network-connect-test

    run_podman network connect mynetwork myrunningcontainer
    run_podman network disconnect podman myrunningcontainer
    run curl --max-time 3 -s 127.0.0.1:$HOST_PORT/index.txt
    is "$output" "$RANDOM_STRING_1" "curl on container with second network connected"
}

@test "network - restart" {
    # restart the container and check if we can still use the port

    # https://github.com/containers/podman/issues/13679
    # The upgrade to podman4 changes the network db format.
    # While it is compatible from 3.X to 4.0 it will fail the other way around.
    # This can be the case when the cleanup process runs before the stop process
    # can do the cleanup.

    # Since there is no easy way to fix this and downgrading is not something
    # we support, just fix this bug in the tests by manually calling
    # network disconnect to teardown the netns.
    if test -e $PODMAN_UPGRADE_WORKDIR/ran-network-connect-test; then
        run_podman network disconnect mynetwork myrunningcontainer
    fi

    run_podman stop -t0 myrunningcontainer

    # now connect again, do this before starting the container
    if test -e $PODMAN_UPGRADE_WORKDIR/ran-network-connect-test; then
        run_podman network connect mynetwork myrunningcontainer
    fi
    run_podman start myrunningcontainer
    run curl --max-time 3 -s 127.0.0.1:$HOST_PORT/index.txt
    is "$output" "$RANDOM_STRING_1" "curl on restarted container"
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

    run_podman pod start mypod
    is "$output" "[0-9a-f]\\{64\\}" "podman pod start"

    run_podman pod ps
    is "$output" ".*mypod.*" "podman pod ps shows name"
    is "$output" ".*Running.*" "podman pod ps shows running state"

    run_podman pod stop mypod
    is "$output" "[0-9a-f]\\{64\\}" "podman pod stop"

    run_podman pod rm mypod
    # FIXME: CI runs show this (non fatal) error:
    # Error updating pod <ID> conmon cgroup PID limit: open /sys/fs/cgroup/libpod_parent/<ID>/conmon/pids.max: no such file or directory
    # Investigate how to fix this (likely a race condition)
    # Let's ignore the logrus messages for now
    is "$output" ".*[0-9a-f]\\{64\\}" "podman pod rm"
}

# FIXME: commit? kill? network? pause? restart? top? volumes? What else?


@test "start" {
    run_podman start -a mydonecontainer
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
