#!/usr/bin/env bats
#
# Tests of podman farm commands
#

load helpers.bash

###############################################################################
# BEGIN tests

fname="test-farm"
containerfile="test/farm/Containerfile"

@test "farm - check farm has been created" {
    run_podman farm ls
    assert "$output" =~ $fname
    assert "$output" =~ "test-node"
}

@test "farm - build on local only" {
    iname="test-image-1"
    empty_farm="empty-farm"
    # create an empty farm
    run_podman farm create $empty_farm
    run_podman farm --farm $empty_farm build -f $containerfile -t $iname .
    assert "$output" =~ "Local builder ready"
    # get the system architecture
    run_podman info --format '{{.Host.Arch}}'
    ARCH=$output
    # inspect manifest list built and saved in local containers-storage
    run_podman manifest inspect $iname
    assert "$output" =~ $ARCH
}

@test "farm - build on farm node only with --cleanup" {
    iname="test-image-2"
    run_podman farm build -f $containerfile --cleanup --local=false -t $iname .
    assert "$output" =~ "Farm \"$fname\" ready"
    # get the system architecture
    run_podman info --format '{{.Host.Arch}}'
    ARCH=$output
    # inspect manifest list built and saved in dir
    manifest=$(cat $iname/manifest.json)
    assert "$manifest" =~ $ARCH
    # see if we can ssh into node to check the image was cleaned up
    nodeimg=$(ssh $ROOTLESS_USER@localhost podman images --filter dangling=true --noheading 2>&1)
    assert "$nodeimg" = ""
    # check that no image was built locally
    run_podman images --filter dangling=true --noheading
    assert "$output" = ""
}

@test "farm - build on farm node and local" {
    iname="test-image-3"
    run_podman farm build -f $containerfile -t $iname .
    assert "$output" =~ "Farm \"$fname\" ready"
    # get the system architecture
    run_podman info --format '{{.Host.Arch}}'
    ARCH=$output
    # inspect manifest list built and saved in dir
    run_podman manifest inspect $iname
    assert "$output" =~ $ARCH
}

# Test out podman-remote

@test "farm - build on farm node only (podman-remote)" {
    iname="test-image-4"
    run_podman --remote farm build -f $containerfile -t $iname .
    assert "$output" =~ "Farm \"$fname\" ready"
    # get the system architecture
    run_podman --remote info --format '{{.Host.Arch}}'
    ARCH=$output
    # inspect manifest list built and saved in dir
    manifest=$(cat $iname/manifest.json)
    assert "$manifest" =~ $ARCH
}
