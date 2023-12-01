#!/usr/bin/env bats
#
# Tests of podman farm commands
#

load helpers.bash


@test "farm - check farm has been created" {
    run_podman farm ls
    assert "$output" =~ $FARMNAME
    assert "$output" =~ "test-node"
}

@test "farm - build on local only" {
    iname="test-image-1"
    empty_farm="empty-farm"
    # create an empty farm
    run_podman farm create $empty_farm
    run_podman farm build --farm $empty_farm -t $iname $PODMAN_TMPDIR
    assert "$output" =~ "Local builder ready"

    # get the system architecture
    run_podman info --format '{{.Host.Arch}}'
    ARCH=$output
    # inspect manifest list built and saved in local containers-storage
    # FIXME: use --format?
    run_podman manifest inspect $iname
    assert "$output" =~ $ARCH

    run_podman images -a

    # FIXME-someday: why do we need the prune?
    run_podman manifest rm $iname
    run_podman image prune -f
}

@test "farm - build on farm node only with --cleanup" {
    iname="test-image-2"
    run_podman farm build --cleanup --local=false -t $iname $PODMAN_TMPDIR
    assert "$output" =~ "Farm \"$FARMNAME\" ready"
    # get the system architecture
    run_podman info --format '{{.Host.Arch}}'
    ARCH=$output
    # inspect manifest list built and saved in dir
    # FIXME FIXME FIXME! #20505: do not write anything under cwd
    ls -l $iname

    # FIXME FIXME FIXME FIXME! NEVER WRITE INTO PWD!
    manifestarch=$(jq -r '.manifests[].platform.architecture' <$iname/manifest.json)
    assert "$manifestarch" = "$ARCH" "arch from $iname/manifest.json"

    # see if we can ssh into node to check the image was cleaned up
    run ssh $ROOTLESS_USER@localhost podman images --filter dangling=true --noheading
    assert "$output" = "" "podman images on remote host"

    # check that no image was built locally
    run_podman images --filter dangling=true --noheading
    assert "$output" = "" "podman images on local host"

    run_podman image prune -f
}

@test "farm - build on farm node and local" {
    iname="test-image-3"
    run_podman farm build -t $iname $PODMAN_TMPDIR
    assert "$output" =~ "Farm \"$FARMNAME\" ready"

    # get the system architecture
    run_podman info --format '{{.Host.Arch}}'
    ARCH=$output
    # inspect manifest list built and saved in dir
    run_podman manifest inspect $iname
    assert "$output" =~ $ARCH

    run_podman manifest rm $iname
    run_podman image prune -f
}

# Test out podman-remote

@test "farm - build on farm node only (podman-remote)" {
    iname="test-image-4"
    run_podman --remote farm build -t $iname $PODMAN_TMPDIR
    assert "$output" =~ "Farm \"$FARMNAME\" ready"

    # get the system architecture
    run_podman --remote info --format '{{.Host.Arch}}'
    ARCH=$output
    # inspect manifest list built and saved in dir
    manifestarch=$(jq -r '.manifests[].platform.architecture' <$iname/manifest.json)
    assert "$manifestarch" = "$ARCH" "arch from $iname/manifest.json"

    run_podman image prune -f
}
