#!/usr/bin/env bats

load helpers

@test "podman info - basic test" {
    run_podman info

    expected_keys="
buildahVersion: *[0-9.]\\\+
conmon:\\\s\\\+package:
distribution:
logDriver:
ociRuntime:\\\s\\\+name:
os:
rootless:
registries:
store:
graphDriverName:
graphRoot:
graphStatus:
imageStore:\\\s\\\+number: 1
runRoot:
cgroupManager: \\\(systemd\\\|cgroupfs\\\)
cgroupVersion: v[12]
"
    while read expect; do
        is "$output" ".*$expect" "output includes '$expect'"
    done < <(parse_table "$expected_keys")
}

@test "podman info - json" {
    run_podman info --format=json

    expr_nvr="[a-z0-9-]\\\+-[a-z0-9.]\\\+-[a-z0-9]\\\+\."
    expr_path="/[a-z0-9\\\/.-]\\\+\\\$"

    # FIXME: if we're ever able to get package versions on Debian,
    #        add '-[0-9]' to all '*.package' queries below.
    tests="
host.buildahVersion       | [1-9][0-9]*\.[0-9.]\\\+.*
host.conmon.path          | $expr_path
host.conmon.package       | .*conmon.*
host.cgroupManager        | \\\(systemd\\\|cgroupfs\\\)
host.cgroupVersion        | v[12]
host.ociRuntime.path      | $expr_path
store.configFile          | $expr_path
store.graphDriverName     | [a-z0-9]\\\+\\\$
store.graphRoot           | $expr_path
store.imageStore.number   | 1
host.slirp4netns.executable | $expr_path
"

    parse_table "$tests" | while read field expect; do
        actual=$(echo "$output" | jq -r ".$field")
        dprint "# actual=<$actual> expect=<$expect>"
        is "$actual" "$expect" "jq .$field"
    done
}

@test "podman info - confirm desired runtime" {
    if [[ -z "$CI_DESIRED_RUNTIME" ]]; then
        # When running in Cirrus, CI_DESIRED_RUNTIME *must* be defined
        # in .cirrus.yml so we can double-check that all CI VMs are
        # using crun/runc as desired.
        if [[ -n "$CIRRUS_CI" ]]; then
            die "CIRRUS_CI is set, but CI_DESIRED_RUNTIME is not! See #14912"
        fi

        # Not running under Cirrus (e.g., gating tests, or dev laptop).
        # Totally OK to skip this test.
        skip "CI_DESIRED_RUNTIME is unset--OK, because we're not in Cirrus"
    fi

    run_podman info --format '{{.Host.OCIRuntime.Name}}'
    is "$output" "$CI_DESIRED_RUNTIME" "CI_DESIRED_RUNTIME (from .cirrus.yml)"
}

# 2021-04-06 discussed in watercooler: RHEL must never use crun, even if
# using cgroups v2.
@test "podman info - RHEL8 must use runc" {
    local osrelease=/etc/os-release
    test -e $osrelease || skip "Not a RHEL system (no $osrelease)"

    local osname=$(source $osrelease; echo $NAME)
    if [[ $osname =~ Red.Hat || $osname =~ CentOS ]]; then
        # Version can include minor; strip off first dot an all beyond it
        local osver=$(source $osrelease; echo $VERSION_ID)
        test ${osver%%.*} -le 8 || skip "$osname $osver > RHEL8"

        # RHEL or CentOS 8.
        # FIXME: what does 'CentOS 8' even mean? What is $VERSION_ID in CentOS?
        is "$(podman_runtime)" "runc" "$osname only supports OCI Runtime = runc"
    else
        skip "only applicable on RHEL, this is $osname"
    fi
}

@test "podman info --storage-opt='' " {
    skip_if_remote "--storage-opt flag is not supported for remote"
    skip_if_rootless "storage opts are required for rootless running"
    run_podman --storage-opt='' info
    # Note this will not work in rootless mode, unless you specify
    # storage-driver=vfs, until we have kernels that support rootless overlay
    # mounts.
    is "$output" ".*graphOptions: {}" "output includes graphOptions: {}"
}

@test "podman info netavark " {
    # Confirm netavark in use when explicitly required by execution environment.
    if [[ "$NETWORK_BACKEND" == "netavark" ]]; then
        if ! is_netavark; then
            # Assume is_netavark() will provide debugging feedback.
            die "Netavark driver testing required, but not in use by podman."
        fi
    else
        skip "Netavark testing not requested (\$NETWORK_BACKEND='$NETWORK_BACKEND')"
    fi
}

@test "podman --root PATH info - basic output" {
    if ! is_remote; then
        run_podman --storage-driver=vfs --root ${PODMAN_TMPDIR}/nothing-here-move-along info --format '{{ .Store.GraphOptions }}'
        is "$output" "map\[\]" "'podman --root should reset Graphoptions to []"
    fi
}

@test "podman --root PATH --volumepath info - basic output" {
    volumePath=${PODMAN_TMPDIR}/volumesGoHere
    if ! is_remote; then
        run_podman --storage-driver=vfs --root ${PODMAN_TMPDIR}/nothing-here-move-along --volumepath ${volumePath} info --format '{{ .Store.VolumePath }}'
        is "$output" "${volumePath}" "'podman --volumepath should reset VolumePath"
    fi
}

# vim: filetype=sh
