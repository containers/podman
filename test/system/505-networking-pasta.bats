#!/usr/bin/env bats   -*- bats -*-
#
# Dummy change to get system tests to run
#
# SPDX-License-Identifier: Apache-2.0
#
# Networking with pasta(1)
#
# Copyright (c) 2022 Red Hat GmbH
# Author: Stefano Brivio <sbrivio@redhat.com>

load helpers
load helpers.network

function setup() {
    basic_setup
    skip_if_not_rootless "pasta networking only available in rootless mode"
    skip_if_no_pasta "pasta not found: install pasta(1) to run these tests"

    XFER_FILE="${PODMAN_TMPDIR}/pasta.bin"
}

# _set_opt() - meta-helper for pasta_test_do.
#
# Sets an option, but panics if option is already set (e.g. UDP+TCP, IPv4/v6)
function _set_opt() {
    local opt_name=$1
    local -n opt_ref=$1
    local newval=$2

    if [[ -n "$opt_ref" ]]; then
        # $kw sneakily inherited from caller
        die "'$kw' in test name sets $opt_name='$newval', but $opt_name has already been set to '$opt_ref'"
    fi
    opt_ref=$newval
}

# pasta_test_do() - Run tests involving clients and servers
#
# This helper function is invoked without arguments; it determines what to do
# based on the @test name.
function pasta_test_do() {
    local ip_ver iftype proto range delta bind_type bytes

    # Normalize test name back to human-readable form. BATS gives us a
    # sanitized string with non-alnum converted to '-XX' (dash-hexbyte)
    # and spaces converted to underscores. Convert all of those to spaces.
    # This then gives us only the important (mutable) part of the test:
    #
    #    test_TCP_translated_..._forwarding-2c_IPv4-2c_loopback
    # ->      TCP translated ... forwarding    IPv4    loopback
    # ->      TCP translated     forwarding    IPv4    loopback
    local test_name=$(printf "$(sed \
                      -e 's/^test_//'                 \
                      -e 's/-\([0-9a-f]\{2\}\)/ /gI' \
                      -e 's/_/ /g'                   \
                      <<<"${BATS_TEST_NAME}")")

    # We now have the @test name as specified in the script, minus punctuation.
    # From each of the name components, determine an action.
    #
    #    TCP translated port range forwarding  IPv4  loopback
    #    |   |          |    |     |           |     \__ iftype=loopback
    #    |   |          |    |     |           \________ ip_ver=4
    #    |   |          |    |     \____________________ bytes=1
    #    |   |          |    \__________________________ range=3
    #    |   |          \_______________________________ (ignored)
    #    |   \__________________________________________ delta=1
    #    \______________________________________________ proto=tcp
    #
    # Each keyword maps to one option. Conflicts ("TCP ... UDP") are fatal
    # errors, as are unknown keywords.
    for kw in $test_name; do
        case $kw in
            TCP|UDP)           _set_opt proto ${kw,,} ;;
            IPv*)              _set_opt ip_ver $(expr "$kw" : "IPv\(.\)") ;;
            Single)            _set_opt range 1 ;;
            range)             _set_opt range 3 ;;
            Address|Interface) _set_opt bind_type ${kw,,} ;;
            bound)             assert "$bind_type" != "" "WHAT-bound???" ;;
            [Tt]ranslated)     _set_opt delta    1 ;;
            loopback|tap)      _set_opt iftype $kw ;;
            port)              ;;   # always occurs with 'forwarding'; ignore
            forwarding)        _set_opt bytes   1 ;;
            large|small)       _set_opt bytes $kw ;;
            transfer)          assert "$bytes" != "" "'transfer' must be preceded by 'large' or 'small'" ;;
            *)                 die "cannot grok '$kw' in test name" ;;
        esac
    done

    # Sanity checks: all test names must include IPv4/6 and TCP/UDP
    test -n "$ip_ver" || die "Test name must include IPv4 or IPv6"
    test -n "$proto"  || die "Test name must include TCP or UDP"
    test -n "$bytes"  || die "Test name must include 'forwarding' or 'large/small transfer'"

    # Major decision point: simple forwarding test, or multi-byte transfer?
    if [[ $bytes -eq 1 ]]; then
        # Simple forwarding check
        # We can't always determine these from the test name. Use sane defaults.
        range=${range:-1}
        delta=${delta:-0}
        bind_type=${bind_type:-port}
    else
        # Data transfer. Translate small/large to dd-recognizable sizes
        case "$bytes" in
            small)  bytes="2k" ;;
            large)  case "$proto" in
                        tcp) bytes="10M" ;;
                        udp) bytes=$(($(cat /proc/sys/net/core/wmem_default) / 4)) ;;
                        *)   die "Internal error: unknown proto '$proto'" ;;
                    esac
                    ;;
            *)      die "Internal error: unknown transfer size '$bytes'" ;;
        esac

        # On data transfers, no other input args can be set in test name.
        # Confirm that they are not defined, and set to a suitable default.
        kw="something"
        _set_opt range     1
        _set_opt delta     0
        _set_opt bind_type port
    fi

    # Dup check: make sure we haven't already run this combination of settings.
    # This serves two purposes:
    #  1) prevent developer from accidentally copy/pasting the same test
    #  2) make sure our test-name-parsing code isn't missing anything important
    local tests_run=${BATS_FILE_TMPDIR}/tests_run
    touch ${tests_run}
    local testid="IPv${ip_ver} $proto $iftype $bind_type range=$range delta=$delta bytes=$bytes"
    if grep -q -F -- "$testid" ${tests_run}; then
        die "Duplicate test! Have already run $testid"
    fi
    echo "$testid" >>${tests_run}

    # Done figuring out test params. Now do the real work.
    # Calculate and set addresses,
    if [ ${ip_ver} -eq 4 ]; then
        skip_if_no_ipv4 "IPv4 not routable on the host"
    elif [ ${ip_ver} -eq 6 ]; then
        skip_if_no_ipv6 "IPv6 not routable on the host"
    else
        skip "Unsupported IP version"
    fi

    if [ ${iftype} = "loopback" ]; then
        local ifname="lo"
    else
        local ifname="$(default_ifname "${ip_ver}")"
    fi

    local addr="$(default_addr "${ip_ver}" "${ifname}")"

    # ports,
    if [ ${range} -gt 1 ]; then
        local port="$(random_free_port_range ${range} ${addr} ${proto})"
        local xport="$((${port%-*} + delta))-$((${port#*-} + delta))"
        local seq="$(echo ${port} | tr '-' ' ')"
        local xseq="$(echo ${xport} | tr '-' ' ')"
    else
        local port=$(random_free_port "" ${address} ${proto})
        local xport="$((port + delta))"
        local seq="${port} ${port}"
        local xseq="${xport} ${xport}"
    fi

    local proto_upper="$(echo "${proto}" | tr [:lower:] [:upper:])"

    # socat options for first <address> in server ("LISTEN" address types),
    local bind="${proto_upper}${ip_ver}-LISTEN:\${port}"
    # For IPv6 via tap, we can pick either link-local or global unicast
    if [ ${ip_ver} -eq 4 ] || [ ${iftype} = "loopback" ]; then
        bind="${bind},bind=[${addr}]"
    fi
    if [ "${proto}" = "udp" ]; then
        bind="${bind},null-eof"
    fi

    # socat options for second <address> in server ("STDOUT" or "EXEC"),
    if [ "${bytes}" = "1" ]; then
        recv="STDOUT"
    else
        recv="EXEC:md5sum"
    fi

    # and port forwarding configuration for Podman and pasta.
    #
    # TODO: Use Podman options once/if
    # https://github.com/containers/podman/issues/14425 is solved
    case ${bind_type} in
    "interface")
        local pasta_spec=":--${proto}-ports,${addr}%${ifname}/${port}:${xport}"
        local podman_spec=
        ;;
    "address")
        local pasta_spec=
        local podman_spec="[${addr}]:${port}:${xport}/${proto}"
        ;;
    *)
        local pasta_spec=
        local podman_spec="[${addr}]:${port}:${xport}/${proto}"
        ;;
    esac

    # Fill in file for data transfer tests, and expected output strings
    if [ "${bytes}" != "1" ]; then
        dd if=/dev/urandom bs=${bytes} count=1 of="${XFER_FILE}"
        local expect="$(cat "${XFER_FILE}" | md5sum)"
    else
        printf "x" > "${XFER_FILE}"
        local expect="$(for i in $(seq ${seq}); do printf "x"; done)"
    fi

    # Set retry/initial delay for client to connect
    local delay=1
    if [ ${ip_ver} -eq 6 ] && [ "${addr}" != "::1" ]; then
        # Duplicate Address Detection on link-local
        delay=3
    elif [ "${proto}" = "udp" ]; then
        # With Podman up, and socat still starting, UDP clients send to nowhere
        delay=2
    fi

    # Now actually run the test: client,
    for one_port in $(seq ${seq}); do
        local connect="${proto_upper}${ip_ver}:[${addr}]:${one_port}"
        [ "${proto}" = "udp" ] && connect="${connect},shut-null"

        local retries=10
        (while sleep ${delay} && test $((retries--)) -gt 0 && ! timeout --foreground -v --kill=5 90 socat -u "OPEN:${XFER_FILE}" "${connect}"; do :
         done) &
    done

    # and server,
    run_podman run --net=pasta"${pasta_spec}" -p "${podman_spec}" "${IMAGE}" \
                   sh -c 'for port in $(seq '"${xseq}"'); do '\
'                             socat -u '"${bind}"' '"${recv}"' & '\
'                         done; wait'

    # which should give us the expected output back.
    assert "${output}" = "${expect}" "Mismatch between data sent and received"
}

### Addresses ##################################################################

@test "IPv4 default address assignment" {
    skip_if_no_ipv4 "IPv4 not routable on the host"

    run_podman run --net=pasta $IMAGE ip -j -4 address show

    local container_address="$(ipv4_get_addr_global "${output}")"
    local host_address="$(default_addr 4)"

    assert "${container_address}" = "${host_address}" \
           "Container address not matching host"
}

@test "IPv4 address assignment" {
    skip_if_no_ipv4 "IPv4 not routable on the host"

    run_podman run --net=pasta:-a,192.0.2.1 $IMAGE ip -j -4 address show

    local container_address="$(ipv4_get_addr_global "${output}")"

    assert "${container_address}" = "192.0.2.1" \
           "Container address not matching configured value"
}

@test "No IPv4" {
    skip_if_no_ipv4 "IPv4 not routable on the host"
    skip_if_no_ipv6 "IPv6 not routable on the host"

    run_podman run --net=pasta:-6 $IMAGE ip -j -4 address show

    local container_address="$(ipv4_get_addr_global "${output}")"

    assert "${container_address}" = "null" \
           "Container has IPv4 global address with IPv4 disabled"
}

@test "IPv6 default address assignment" {
    skip_if_no_ipv6 "IPv6 not routable on the host"

    run_podman run --net=pasta $IMAGE ip -j -6 address show

    local container_address="$(ipv6_get_addr_global "${output}")"
    local host_address="$(default_addr 6)"

    assert "${container_address}" = "${host_address}" \
           "Container address not matching host"
}

@test "IPv6 address assignment" {
    skip_if_no_ipv6 "IPv6 not routable on the host"

    run_podman run --net=pasta:-a,2001:db8::1 $IMAGE ip -j -6 address show

    local container_address="$(ipv6_get_addr_global "${output}")"

    assert "${container_address}" = "2001:db8::1" \
           "Container address not matching configured value"
}

@test "No IPv6" {
    skip_if_no_ipv6 "IPv6 not routable on the host"
    skip_if_no_ipv4 "IPv4 not routable on the host"

    run_podman run --net=pasta:-4 $IMAGE ip -j -6 address show

    local container_address="$(ipv6_get_addr_global "${output}")"

    assert "${container_address}" = "null" \
           "Container has IPv6 global address with IPv6 disabled"
}

@test "podman puts pasta IP in /etc/hosts" {
    skip_if_no_ipv4 "IPv4 not routable on the host"

    pname="p$(random_string 30)"
    ip="$(default_addr 4)"

    run_podman pod create --net=pasta --name "${pname}"
    run_podman run --pod="${pname}" "${IMAGE}" getent hosts "${pname}"

    assert "$(echo ${output} | cut -f1 -d' ')" = "${ip}" "Correct /etc/hosts entry missing"

    run_podman pod rm "${pname}"
    run_podman rmi $(pause_image)
}

### Routes #####################################################################

@test "IPv4 default route" {
    skip_if_no_ipv4 "IPv4 not routable on the host"

    run_podman run --net=pasta $IMAGE ip -j -4 route show

    local container_route="$(ipv4_get_route_default "${output}")"
    local host_route="$(ipv4_get_route_default)"

    assert "${container_route}" = "${host_route}" \
           "Container route not matching host"
}

@test "IPv4 default route assignment" {
    skip_if_no_ipv4 "IPv4 not routable on the host"

    run_podman run --net=pasta:-a,192.0.2.2,-g,192.0.2.1 $IMAGE \
        ip -j -4 route show

    local container_route="$(ipv4_get_route_default "${output}")"

    assert "${container_route}" = "192.0.2.1" \
           "Container route not matching configured value"
}

@test "IPv6 default route" {
    skip_if_no_ipv6 "IPv6 not routable on the host"

    run_podman run --net=pasta $IMAGE ip -j -6 route show

    local container_route="$(ipv6_get_route_default "${output}")"
    local host_route="$(ipv6_get_route_default)"

    assert "${container_route}" = "${host_route}" \
           "Container route not matching host"
}

@test "IPv6 default route assignment" {
    skip_if_no_ipv6 "IPv6 not routable on the host"

    run_podman run --net=pasta:-a,2001:db8::2,-g,2001:db8::1 $IMAGE \
        ip -j -6 route show

    local container_route="$(ipv6_get_route_default "${output}")"

    assert "${container_route}" = "2001:db8::1" \
           "Container route not matching configured value"
}

### Interfaces #################################################################

@test "Default MTU" {
    run_podman run --net=pasta $IMAGE ip -j link show

    container_tap_mtu="$(ether_get_mtu "${output}")"

    assert "${container_tap_mtu}" = "65520" \
           "Container's default MTU not 65220 bytes by default"
}

@test "MTU assignment" {
    run_podman run --net=pasta:-m,1280 $IMAGE ip -j link show

    container_tap_mtu="$(ether_get_mtu "${output}")"

    assert "${container_tap_mtu}" = "1280" \
           "Container's default MTU not matching configured 1280 bytes"
}

@test "Loopback interface state" {
    run_podman run --net=pasta $IMAGE ip -j link show

    local jq_expr='.[] | select(.link_type == "loopback").flags | '\
'              contains(["UP"])'

    container_loopback_up="$(printf '%s' "${output}" | jq -rM "${jq_expr}")"

    assert "${container_loopback_up}" = "true" \
           "Container's loopback interface not up"
}

### DNS ########################################################################

@test "External resolver, IPv4" {
    skip_if_no_ipv4 "IPv4 not routable on the host"

    run_podman '?' run --net=pasta $IMAGE nslookup 127.0.0.1

    assert "$output" =~ "1.0.0.127.in-addr.arpa" \
           "127.0.0.1 not resolved"
}

@test "External resolver, IPv6" {
    skip_if_no_ipv6 "IPv6 not routable on the host"

    run_podman run --net=pasta $IMAGE nslookup ::1 || :

    assert "$output" =~ "1.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.ip6.arpa" \
           "::1 not resolved"
}

@test "Local forwarder, IPv4" {
    skip_if_no_ipv4 "IPv4 not routable on the host"

    # pasta is the default now so no need to set it
    run_podman run --rm $IMAGE grep nameserver /etc/resolv.conf
    assert "${lines[0]}" == "nameserver 169.254.0.1" "default dns forward server"

    run_podman run --rm --net=pasta:--dns-forward,198.51.100.1 \
        $IMAGE nslookup 127.0.0.1 || :
    assert "$output" =~ "1.0.0.127.in-addr.arpa" "No answer from resolver"
}

@test "Local forwarder, IPv6" {
    skip_if_no_ipv6 "IPv6 not routable on the host"

    # TODO: Two issues here:
    skip "Currently unsupported"
    # run_podman run --dns 2001:db8::1 \
    #   --net=pasta:--dns-forward,2001:db8::1 $IMAGE nslookup ::1
    #
    # 1. With this, Podman writes "nameserver 2001:db8::1" to
    #    /etc/resolv.conf, without zone, and the query originates from ::1.
    #    Passing:
    #   --dns "2001:db8::2%eth0"
    #    results in:
    #   Error: 2001:db8::2%eth0 is not an ip address
    #    Fix the issue in Podman once we figure out 2. below.
    #
    #
    # run_podman run --dns 2001:db8::1 \
    #   --net=pasta:--dns-forward,2001:db8::1 $IMAGE \
    #   sh -c 'echo 2001:db8::1%eth0 >/etc/resolv.conf; nslookup ::1'
    #
    # 2. This fixes the source address of the query, but the answer is
    #    discarded. Figure out if it's an issue in Busybox, in musl, if we
    #    should just include a full-fledged resolver in the test image, etc.
}

### TCP/IPv4 Port Forwarding ###################################################

@test "Single TCP port forwarding, IPv4, tap" {
    pasta_test_do
}

@test "Single TCP port forwarding, IPv4, loopback" {
    pasta_test_do
}

@test "TCP port range forwarding, IPv4, tap" {
    pasta_test_do
}

@test "TCP port range forwarding, IPv4, loopback" {
    pasta_test_do
}

@test "Translated TCP port forwarding, IPv4, tap" {
    pasta_test_do
}

@test "Translated TCP port forwarding, IPv4, loopback" {
    pasta_test_do
}

@test "TCP translated port range forwarding, IPv4, tap" {
    pasta_test_do
}

@test "TCP translated port range forwarding, IPv4, loopback" {
    pasta_test_do
}

@test "Address-bound TCP port forwarding, IPv4, tap" {
    pasta_test_do
}

@test "Address-bound TCP port forwarding, IPv4, loopback" {
    pasta_test_do
}

@test "Interface-bound TCP port forwarding, IPv4, tap" {
    pasta_test_do
}

@test "Interface-bound TCP port forwarding, IPv4, loopback" {
    pasta_test_do
}

### TCP/IPv6 Port Forwarding ###################################################

@test "Single TCP port forwarding, IPv6, tap" {
    pasta_test_do
}

@test "Single TCP port forwarding, IPv6, loopback" {
    pasta_test_do
}

@test "TCP port range forwarding, IPv6, tap" {
    pasta_test_do
}

@test "TCP port range forwarding, IPv6, loopback" {
    pasta_test_do
}

@test "Translated TCP port forwarding, IPv6, tap" {
    pasta_test_do
}

@test "Translated TCP port forwarding, IPv6, loopback" {
    pasta_test_do
}

@test "TCP translated port range forwarding, IPv6, tap" {
    pasta_test_do
}

@test "TCP translated port range forwarding, IPv6, loopback" {
    pasta_test_do
}

@test "Address-bound TCP port forwarding, IPv6, tap" {
    pasta_test_do
}

@test "Address-bound TCP port forwarding, IPv6, loopback" {
    pasta_test_do
}

@test "Interface-bound TCP port forwarding, IPv6, tap" {
    pasta_test_do
}

@test "Interface-bound TCP port forwarding, IPv6, loopback" {
    pasta_test_do
}

### UDP/IPv4 Port Forwarding ###################################################

@test "Single UDP port forwarding, IPv4, tap" {
    pasta_test_do
}

@test "Single UDP port forwarding, IPv4, loopback" {
    pasta_test_do
}

@test "UDP port range forwarding, IPv4, tap" {
    pasta_test_do
}

@test "UDP port range forwarding, IPv4, loopback" {
    pasta_test_do
}

@test "Translated UDP port forwarding, IPv4, tap" {
    pasta_test_do
}

@test "Translated UDP port forwarding, IPv4, loopback" {
    pasta_test_do
}

@test "UDP translated port range forwarding, IPv4, tap" {
    pasta_test_do
}

@test "UDP translated port range forwarding, IPv4, loopback" {
    pasta_test_do
}

@test "Address-bound UDP port forwarding, IPv4, tap" {
    pasta_test_do
}

@test "Address-bound UDP port forwarding, IPv4, loopback" {
    pasta_test_do
}

@test "Interface-bound UDP port forwarding, IPv4, tap" {
    pasta_test_do
}

@test "Interface-bound UDP port forwarding, IPv4, loopback" {
    pasta_test_do
}

### UDP/IPv6 Port Forwarding ###################################################

@test "Single UDP port forwarding, IPv6, tap" {
    pasta_test_do
}

@test "Single UDP port forwarding, IPv6, loopback" {
    pasta_test_do
}

@test "UDP port range forwarding, IPv6, tap" {
    pasta_test_do
}

@test "UDP port range forwarding, IPv6, loopback" {
    pasta_test_do
}

@test "Translated UDP port forwarding, IPv6, tap" {
    pasta_test_do
}

@test "Translated UDP port forwarding, IPv6, loopback" {
    pasta_test_do
}

@test "UDP translated port range forwarding, IPv6, tap" {
    pasta_test_do
}

@test "UDP translated port range forwarding, IPv6, loopback" {
    pasta_test_do
}

@test "Address-bound UDP port forwarding, IPv6, tap" {
    pasta_test_do
}

@test "Address-bound UDP port forwarding, IPv6, loopback" {
    pasta_test_do
}

@test "Interface-bound UDP port forwarding, IPv6, tap" {
    pasta_test_do
}

@test "Interface-bound UDP port forwarding, IPv6, loopback" {
    pasta_test_do
}

### TCP/IPv4 transfer ##########################################################

@test "TCP/IPv4 small transfer, tap" {
    pasta_test_do
}

@test "TCP/IPv4 small transfer, loopback" {
    pasta_test_do
}

@test "TCP/IPv4 large transfer, tap" {
    pasta_test_do
}

@test "TCP/IPv4 large transfer, loopback" {
    pasta_test_do
}

### TCP/IPv6 transfer ##########################################################

@test "TCP/IPv6 small transfer, tap" {
    pasta_test_do
}

@test "TCP/IPv6 small transfer, loopback" {
    pasta_test_do
}

@test "TCP/IPv6 large transfer, tap" {
    pasta_test_do
}

@test "TCP/IPv6 large transfer, loopback" {
    pasta_test_do
}

### UDP/IPv4 transfer ##########################################################

@test "UDP/IPv4 small transfer, tap" {
    pasta_test_do
}

@test "UDP/IPv4 small transfer, loopback" {
    pasta_test_do
}

@test "UDP/IPv4 large transfer, tap" {
    pasta_test_do
}

@test "UDP/IPv4 large transfer, loopback" {
    pasta_test_do
}

### UDP/IPv6 transfer ##########################################################

@test "UDP/IPv6 small transfer, tap" {
    pasta_test_do
}

@test "UDP/IPv6 small transfer, loopback" {
    pasta_test_do
}

@test "UDP/IPv6 large transfer, tap" {
    pasta_test_do
}

@test "UDP/IPv6 large transfer, loopback" {
    pasta_test_do
}

### Lifecycle ##################################################################

@test "pasta(1) quits when the namespace is gone" {
    local pidfile="${PODMAN_TMPDIR}/pasta.pid"

    run_podman run "--net=pasta:--pid,${pidfile}" $IMAGE true
    sleep 1
    ! ps -p $(cat "${pidfile}") && rm "${pidfile}"
}

### Options ####################################################################
@test "Unsupported protocol in port forwarding" {
    local port=$(random_free_port "" "" tcp)

    run_podman 126 run --net=pasta -p "${port}:${port}/sctp" $IMAGE true
    is "$output" "Error: .*can't forward protocol: sctp"
}

@test "Use options from containers.conf" {
    skip_if_remote "containers.conf must be set for the server"

    containersconf=$PODMAN_TMPDIR/containers.conf
    mac="9a:dd:31:ea:92:98"
    cat >$containersconf <<EOF
[network]
default_rootless_network_cmd = "pasta"
pasta_options = ["-I", "myname", "--ns-mac-addr", "$mac"]
EOF

    # 2023-06-29 DO NOT INCLUDE "--net=pasta" on this line!
    # This tests containers.conf:default_rootless_network_cmd (pr #19032)
    CONTAINERS_CONF_OVERRIDE=$containersconf run_podman run $IMAGE ip link show myname
    assert "$output" =~ "$mac" "mac address is set on custom interface"

    # now, again but this time overwrite a option on the cli.
    mac2="aa:bb:cc:dd:ee:ff"
    CONTAINERS_CONF_OVERRIDE=$containersconf run_podman run --net=pasta:--ns-mac-addr,"$mac2" $IMAGE ip link show myname
    assert "$output" =~ "$mac2" "mac address from cli is set on custom interface"
}

### Rootless unshare testing

@test "Podman unshare --rootless-netns with Pasta" {
    skip_if_remote "unshare is local-only"

    pasta_iface=$(default_ifname)

    # First let's force a setup error by making pasta be "false".
    ln -s /usr/bin/false $PODMAN_TMPDIR/pasta
    CONTAINERS_HELPER_BINARY_DIR="$PODMAN_TMPDIR" run_podman 125 unshare --rootless-netns ip addr
    assert "$output" =~ "pasta failed with exit code 1"

    # Now this should recover from the previous error and setup the netns correctly.
    run_podman unshare --rootless-netns ip addr
    is "$output" ".*${pasta_iface}.*"
}

# https://github.com/containers/podman/issues/22653
@test "pasta/bridge and host.containers.internal" {
    skip_if_no_ipv4 "IPv4 not routable on the host"
    pasta_ip="$(default_addr 4)"

    # FIXME FIXME FIXME FOR DEBUGGING
    echo
    echo "$_LOG_PROMPT ip -j -4 addr show"
    ip -j -4 addr show
    echo

    for network in "pasta" "bridge"; do
        # special exit code logic needed here, it is possible that there is no host.containers.internal
        # when there is only one ip one the host and that one is used by pasta.
        # As such we have to deal with both cases.
        run_podman '?' run --rm --network=$network $IMAGE grep host.containers.internal /etc/hosts
        if [ "$status" -eq 0 ]; then
            assert "$output" !~ "$pasta_ip" "pasta host ip must not be assigned ($network)"
            assert "$(hostname -I)" =~ "$(cut -f1 <<<$output)" "ip is one of the host ips ($network)"
        elif [ "$status" -eq 1 ]; then
            # if only pasta ip then we cannot have a host.containers.internal entry
            # make sure this fact is actually the case
            assert "$pasta_ip" == "$(hostname -I | tr -d '[:space:]')" "pasta ip must the only one one the host ($network)"
        else
            die "unexpected exit code '$status' from grep or podman ($network)"
        fi
    done

    host_ip=$(hostname -I | cut -f 1 -d " ")
    run_podman run --rm --network=pasta:-a,169.254.0.2,-g,169.254.0.1,-n,24 $IMAGE grep host.containers.internal /etc/hosts
    assert "$output" =~ "^$host_ip" "uses host first ip"
}
