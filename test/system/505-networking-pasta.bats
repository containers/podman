#!/usr/bin/env bats   -*- bats -*-
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

# pasta_test_do() - Run tests involving clients and servers
# $1:    IP version: 4 or 6
# $2:    Interface type: "tap" or "loopback"
# $3:    Protocol: "tcp" or "udp"
# $4:    Size of port range, 1 for tests over a single port
# $5:    Delta for remapped destination ports in guest
# $6:    How specifically pasta will bind: "port", "address", or "interface"
# $7:    Bytes to transfer, with multiplicative suffixes as supported by dd(1)
function pasta_test_do() {
    local ip_ver="${1}"
    local iftype="${2}"
    local proto="${3}"
    local range="${4}"
    local delta="${5}"
    local bind_type="${6}"
    local bytes="${7}"

    # Calculate and set addresses,
    if [ ${ip_ver} -eq 4 ]; then
        skip_if_no_ipv4 "IPv4 not routable on the host"
        if [ ${iftype} = "loopback" ]; then
            local addr="127.0.0.1"
        else
            local addr="$(ipv4_get_addr_global)"
        fi
    elif [ ${ip_ver} -eq 6 ]; then
        skip_if_no_ipv6 "IPv6 not routable on the host"
        if [ ${iftype} = "loopback" ]; then
            local addr="::1"
        else
            local addr="$(ipv6_get_addr_global)"
        fi
    else
        skip "Unsupported IP version"
    fi

    # interface names,
    if [ ${iftype} = "loopback" ]; then
        local ifname="lo"
    else
        local ifname="$(ether_get_name)"
    fi

    # ports,
    if [ ${range} -gt 1 ]; then
        local port="$(random_free_port_range ${range} ${addr} ${proto})"
        local xport="$((${port%-*} + delta))-$((${port#*-} + delta))"
        local seq="$(echo ${port} | tr '-' ' ')"
        local xseq="$(echo ${xport} | tr '-' ' ')"
    else
        local port=$(random_free_port "" ${address} ${proto})
        local xport="$((port + port_delta))"
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

    # socat options for first <address> in client ("OPEN" or "EXEC"),
    if [ "${bytes}" = "1" ]; then
        send="EXEC:printf x"
    else
        send="OPEN:${XFER_FILE}"
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

        (while sleep ${delay} && ! socat -u "${send}" "${connect}"; do :
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

function teardown() {
    rm -f "${XFER_FILE}"
}

### Addresses ##################################################################

@test "podman networking with pasta(1) - IPv4 default address assignment" {
    skip_if_no_ipv4 "IPv4 not routable on the host"

    run_podman run --net=pasta $IMAGE ip -j -4 address show

    local container_address="$(ipv4_get_addr_global "${output}")"
    local host_address="$(ipv4_get_addr_global)"

    assert "${container_address}" = "${host_address}" \
           "Container address not matching host"
}

@test "podman networking with pasta(1) - IPv4 address assignment" {
    skip_if_no_ipv4 "IPv4 not routable on the host"

    run_podman run --net=pasta:-a,192.0.2.1 $IMAGE ip -j -4 address show

    local container_address="$(ipv4_get_addr_global "${output}")"

    assert "${container_address}" = "192.0.2.1" \
           "Container address not matching configured value"
}

@test "podman networking with pasta(1) - No IPv4" {
    skip_if_no_ipv4 "IPv4 not routable on the host"
    skip_if_no_ipv6 "IPv6 not routable on the host"

    run_podman run --net=pasta:-6 $IMAGE ip -j -4 address show

    local container_address="$(ipv4_get_addr_global "${output}")"

    assert "${container_address}" = "null" \
           "Container has IPv4 global address with IPv4 disabled"
}

@test "podman networking with pasta(1) - IPv6 default address assignment" {
    skip_if_no_ipv6 "IPv6 not routable on the host"

    run_podman run --net=pasta $IMAGE ip -j -6 address show

    local container_address="$(ipv6_get_addr_global "${output}")"
    local host_address="$(ipv6_get_addr_global)"

    assert "${container_address}" = "${host_address}" \
           "Container address not matching host"
}

@test "podman networking with pasta(1) - IPv6 address assignment" {
    skip_if_no_ipv6 "IPv6 not routable on the host"

    run_podman run --net=pasta:-a,2001:db8::1 $IMAGE ip -j -6 address show

    local container_address="$(ipv6_get_addr_global "${output}")"

    assert "${container_address}" = "2001:db8::1" \
           "Container address not matching configured value"
}

@test "podman networking with pasta(1) - No IPv6" {
    skip_if_no_ipv6 "IPv6 not routable on the host"
    skip_if_no_ipv4 "IPv4 not routable on the host"

    run_podman run --net=pasta:-4 $IMAGE ip -j -6 address show

    local container_address="$(ipv6_get_addr_global "${output}")"

    assert "${container_address}" = "null" \
           "Container has IPv6 global address with IPv6 disabled"
}

### Routes #####################################################################

@test "podman networking with pasta(1) - IPv4 default route" {
    skip_if_no_ipv4 "IPv4 not routable on the host"

    run_podman run --net=pasta $IMAGE ip -j -4 route show

    local container_route="$(ipv4_get_route_default "${output}")"
    local host_route="$(ipv4_get_route_default)"

    assert "${container_route}" = "${host_route}" \
           "Container route not matching host"
}

@test "podman networking with pasta(1) - IPv4 default route assignment" {
    skip_if_no_ipv4 "IPv4 not routable on the host"

    run_podman run --net=pasta:-a,192.0.2.2,-g,192.0.2.1 $IMAGE \
        ip -j -4 route show

    local container_route="$(ipv4_get_route_default "${output}")"

    assert "${container_route}" = "192.0.2.1" \
           "Container route not matching configured value"
}

@test "podman networking with pasta(1) - IPv6 default route" {
    skip_if_no_ipv6 "IPv6 not routable on the host"

    run_podman run --net=pasta $IMAGE ip -j -6 route show

    local container_route="$(ipv6_get_route_default "${output}")"
    local host_route="$(ipv6_get_route_default)"

    assert "${container_route}" = "${host_route}" \
           "Container route not matching host"
}

@test "podman networking with pasta(1) - IPv6 default route assignment" {
    skip_if_no_ipv6 "IPv6 not routable on the host"

    run_podman run --net=pasta:-a,2001:db8::2,-g,2001:db8::1 $IMAGE \
        ip -j -6 route show

    local container_route="$(ipv6_get_route_default "${output}")"

    assert "${container_route}" = "2001:db8::1" \
           "Container route not matching configured value"
}

### Interfaces #################################################################

@test "podman networking with pasta(1) - Default MTU" {
    run_podman run --net=pasta $IMAGE ip -j link show

    container_tap_mtu="$(ether_get_mtu "${output}")"

    assert "${container_tap_mtu}" = "65520" \
           "Container's default MTU not 65220 bytes by default"
}

@test "podman networking with pasta(1) - MTU assignment" {
    run_podman run --net=pasta:-m,1280 $IMAGE ip -j link show

    container_tap_mtu="$(ether_get_mtu "${output}")"

    assert "${container_tap_mtu}" = "1280" \
           "Container's default MTU not matching configured 1280 bytes"
}

@test "podman networking with pasta(1) - Loopback interface state" {
    run_podman run --net=pasta $IMAGE ip -j link show

    local jq_expr='.[] | select(.link_type == "loopback").flags | '\
'              contains(["UP"])'

    container_loopback_up="$(printf '%s' "${output}" | jq -rM "${jq_expr}")"

    assert "${container_loopback_up}" = "true" \
           "Container's loopback interface not up"
}

### DNS ########################################################################

@test "podman networking with pasta(1) - External resolver, IPv4" {
    skip_if_no_ipv4 "IPv4 not routable on the host"

    run_podman run --net=pasta $IMAGE nslookup 127.0.0.1 || :

    assert "$output" =~ "1.0.0.127.in-addr.arpa" \
           "127.0.0.1 not resolved"
}

@test "podman networking with pasta(1) - External resolver, IPv6" {
    skip_if_no_ipv6 "IPv6 not routable on the host"

    run_podman run --net=pasta $IMAGE nslookup ::1 || :

    assert "$output" =~ "1.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.ip6.arpa" \
           "::1 not resolved"
}

@test "podman networking with pasta(1) - Local forwarder, IPv4" {
    skip "FIXME: #17074: some pasta dns problem"
    skip_if_no_ipv4 "IPv4 not routable on the host"

    run_podman run --dns 198.51.100.1 \
        --net=pasta:--dns-forward,198.51.100.1 $IMAGE nslookup 127.0.0.1

    assert "$output" =~ "1.0.0.127.in-addr.arpa" \
           "127.0.0.1 not resolved to 1.0.0.127.in-addr.arpa"
}

@test "podman networking with pasta(1) - Local forwarder, IPv6" {
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

@test "podman networking with pasta(1) - Single TCP port forwarding, IPv4, tap" {
    pasta_test_do 4 tap      tcp 1 0 "port"      1
}

@test "podman networking with pasta(1) - Single TCP port forwarding, IPv4, loopback" {
    pasta_test_do 4 loopback tcp 1 0 "port"      1
}

@test "podman networking with pasta(1) - TCP port range forwarding, IPv4, tap" {
    pasta_test_do 4 tap      tcp 3 0 "port"      1
}

@test "podman networking with pasta(1) - TCP port range forwarding, IPv4, loopback" {
    pasta_test_do 4 loopback tcp 3 0 "port"      1
}

@test "podman networking with pasta(1) - Translated TCP port forwarding, IPv4, tap" {
    pasta_test_do 4 tap      tcp 1 1 "port"      1
}

@test "podman networking with pasta(1) - Translated TCP port forwarding, IPv4, loopback" {
    pasta_test_do 4 loopback tcp 1 1 "port"      1
}

@test "podman networking with pasta(1) - TCP translated port range forwarding, IPv4, tap" {
    pasta_test_do 4 tap      tcp 3 1 "port"      1
}

@test "podman networking with pasta(1) - TCP translated port range forwarding, IPv4, loopback" {
    pasta_test_do 4 loopback tcp 3 1 "port"      1
}

@test "podman networking with pasta(1) - Address-bound TCP port forwarding, IPv4, tap" {
    pasta_test_do 4 tap      tcp 1 0 "address"   1
}

@test "podman networking with pasta(1) - Address-bound TCP port forwarding, IPv4, loopback" {
    pasta_test_do 4 loopback tcp 1 0 "address"   1
}

@test "podman networking with pasta(1) - Interface-bound TCP port forwarding, IPv4, tap" {
    pasta_test_do 4 tap      tcp 1 0 "interface" 1
}

@test "podman networking with pasta(1) - Interface-bound TCP port forwarding, IPv4, loopback" {
    pasta_test_do 4 loopback tcp 1 0 "interface" 1
}

### TCP/IPv6 Port Forwarding ###################################################

@test "podman networking with pasta(1) - Single TCP port forwarding, IPv6, tap" {
    pasta_test_do 6 tap      tcp 1 0 "port"      1
}

@test "podman networking with pasta(1) - Single TCP port forwarding, IPv6, loopback" {
    pasta_test_do 6 loopback tcp 1 0 "port"      1
}

@test "podman networking with pasta(1) - TCP port range forwarding, IPv6, tap" {
    pasta_test_do 6 tap      tcp 3 0 "port"      1
}

@test "podman networking with pasta(1) - TCP port range forwarding, IPv6, loopback" {
    pasta_test_do 6 loopback tcp 3 0 "port"      1
}

@test "podman networking with pasta(1) - Translated TCP port forwarding, IPv6, tap" {
    pasta_test_do 6 tap      tcp 1 1 "port"      1
}

@test "podman networking with pasta(1) - Translated TCP port forwarding, IPv6, loopback" {
    pasta_test_do 6 loopback tcp 1 1 "port"      1
}

@test "podman networking with pasta(1) - TCP translated port range forwarding, IPv6, tap" {
    pasta_test_do 6 tap      tcp 3 1 "port"      1
}

@test "podman networking with pasta(1) - TCP translated port range forwarding, IPv6, loopback" {
    pasta_test_do 6 loopback tcp 3 1 "port"      1
}

@test "podman networking with pasta(1) - Address-bound TCP port forwarding, IPv6, tap" {
    pasta_test_do 6 tap      tcp 1 0 "address"   1
}

@test "podman networking with pasta(1) - Address-bound TCP port forwarding, IPv6, loopback" {
    pasta_test_do 6 loopback tcp 1 0 "address"   1
}

@test "podman networking with pasta(1) - Interface-bound TCP port forwarding, IPv6, tap" {
    pasta_test_do 6 tap      tcp 1 0 "interface" 1
}

@test "podman networking with pasta(1) - Interface-bound TCP port forwarding, IPv6, loopback" {
    pasta_test_do 6 loopback tcp 1 0 "interface" 1
}

### UDP/IPv4 Port Forwarding ###################################################

@test "podman networking with pasta(1) - Single UDP port forwarding, IPv4, tap" {
    pasta_test_do 4 tap      udp 1 0 "port"      1
}

@test "podman networking with pasta(1) - Single UDP port forwarding, IPv4, loopback" {
    pasta_test_do 4 loopback udp 1 0 "port"      1
}

@test "podman networking with pasta(1) - UDP port range forwarding, IPv4, tap" {
    pasta_test_do 4 tap      udp 3 0 "port"      1
}

@test "podman networking with pasta(1) - UDP port range forwarding, IPv4, loopback" {
    pasta_test_do 4 loopback udp 3 0 "port"      1
}

@test "podman networking with pasta(1) - Translated UDP port forwarding, IPv4, tap" {
    pasta_test_do 4 tap      udp 1 1 "port"      1
}

@test "podman networking with pasta(1) - Translated UDP port forwarding, IPv4, loopback" {
    pasta_test_do 4 loopback udp 1 1 "port"      1
}

@test "podman networking with pasta(1) - UDP translated port range forwarding, IPv4, tap" {
    pasta_test_do 4 tap      udp 3 1 "port"      1
}

@test "podman networking with pasta(1) - UDP translated port range forwarding, IPv4, loopback" {
    pasta_test_do 4 loopback udp 3 1 "port"      1
}

@test "podman networking with pasta(1) - Address-bound UDP port forwarding, IPv4, tap" {
    pasta_test_do 4 tap      udp 1 0 "address"   1
}

@test "podman networking with pasta(1) - Address-bound UDP port forwarding, IPv4, loopback" {
    pasta_test_do 4 loopback udp 1 0 "address"   1
}

@test "podman networking with pasta(1) - Interface-bound UDP port forwarding, IPv4, tap" {
    pasta_test_do 4 tap      udp 1 0 "interface" 1
}

@test "podman networking with pasta(1) - Interface-bound UDP port forwarding, IPv4, loopback" {
    pasta_test_do 4 loopback udp 1 0 "interface" 1
}

### UDP/IPv6 Port Forwarding ###################################################

@test "podman networking with pasta(1) - Single UDP port forwarding, IPv6, tap" {
    pasta_test_do 6 tap      udp 1 0 "port"      1
}

@test "podman networking with pasta(1) - Single UDP port forwarding, IPv6, loopback" {
    pasta_test_do 6 loopback udp 1 0 "port"      1
}

@test "podman networking with pasta(1) - UDP port range forwarding, IPv6, tap" {
    pasta_test_do 6 tap      udp 3 0 "port"      1
}

@test "podman networking with pasta(1) - UDP port range forwarding, IPv6, loopback" {
    pasta_test_do 6 loopback udp 3 0 "port"      1
}

@test "podman networking with pasta(1) - Translated UDP port forwarding, IPv6, tap" {
    pasta_test_do 6 tap      udp 1 1 "port"      1
}

@test "podman networking with pasta(1) - Translated UDP port forwarding, IPv6, loopback" {
    pasta_test_do 6 loopback udp 1 1 "port"      1
}

@test "podman networking with pasta(1) - UDP translated port range forwarding, IPv6, tap" {
    pasta_test_do 6 tap      udp 3 1 "port"      1
}

@test "podman networking with pasta(1) - UDP translated port range forwarding, IPv6, loopback" {
    pasta_test_do 6 loopback udp 3 1 "port"      1
}

@test "podman networking with pasta(1) - Address-bound UDP port forwarding, IPv6, tap" {
    pasta_test_do 6 tap      udp 1 0 "address"   1
}

@test "podman networking with pasta(1) - Address-bound UDP port forwarding, IPv6, loopback" {
    pasta_test_do 6 loopback udp 1 0 "address"   1
}

@test "podman networking with pasta(1) - Interface-bound UDP port forwarding, IPv6, tap" {
    pasta_test_do 6 tap      udp 1 0 "interface" 1
}

@test "podman networking with pasta(1) - Interface-bound UDP port forwarding, IPv6, loopback" {
    pasta_test_do 6 loopback udp 1 0 "interface" 1
}

### TCP/IPv4 transfer ##########################################################

@test "podman networking with pasta(1) - TCP/IPv4 small transfer, tap" {
    pasta_test_do 4 tap      tcp 1 0 "port"      2k
}

@test "podman networking with pasta(1) - TCP/IPv4 small transfer, loopback" {
    pasta_test_do 4 loopback tcp 1 0 "port"      2k
}

@test "podman networking with pasta(1) - TCP/IPv4 large transfer, tap" {
    pasta_test_do 4 tap      tcp 1 0 "port"      10M
}

@test "podman networking with pasta(1) - TCP/IPv4 large transfer, loopback" {
    pasta_test_do 4 loopback tcp 1 0 "port"      10M
}

### TCP/IPv6 transfer ##########################################################

@test "podman networking with pasta(1) - TCP/IPv6 small transfer, tap" {
    pasta_test_do 6 tap      tcp 1 0 "port"      2k
}

@test "podman networking with pasta(1) - TCP/IPv6 small transfer, loopback" {
    pasta_test_do 6 loopback tcp 1 0 "port"      2k
}

@test "podman networking with pasta(1) - TCP/IPv6 large transfer, tap" {
    pasta_test_do 6 tap      tcp 1 0 "port"      10M
}

@test "podman networking with pasta(1) - TCP/IPv6 large transfer, loopback" {
    pasta_test_do 6 loopback tcp 1 0 "port"      10M
}

### UDP/IPv4 transfer ##########################################################

@test "podman networking with pasta(1) - UDP/IPv4 small transfer, tap" {
    pasta_test_do 4 tap      udp 1 0 "port"      2k
}

@test "podman networking with pasta(1) - UDP/IPv4 small transfer, loopback" {
    pasta_test_do 4 loopback udp 1 0 "port"      2k
}

@test "podman networking with pasta(1) - UDP/IPv4 large transfer, tap" {
    pasta_test_do 4 tap      udp 1 0 "port"       $(($(cat /proc/sys/net/core/wmem_default) / 4))
}

@test "podman networking with pasta(1) - UDP/IPv4 large transfer, loopback" {
    pasta_test_do 4 loopback udp 1 0 "port"       $(($(cat /proc/sys/net/core/wmem_default) / 4))
}

### UDP/IPv6 transfer ##########################################################

@test "podman networking with pasta(1) - UDP/IPv6 small transfer, tap" {
    pasta_test_do 6 tap      udp 1 0 "port"      2k
}

@test "podman networking with pasta(1) - UDP/IPv6 small transfer, loopback" {
    pasta_test_do 6 loopback udp 1 0 "port"      2k
}

@test "podman networking with pasta(1) - UDP/IPv6 large transfer, tap" {
    pasta_test_do 6 tap      udp 1 0 "port"       $(($(cat /proc/sys/net/core/wmem_default) / 4))
}

@test "podman networking with pasta(1) - UDP/IPv6 large transfer, loopback" {
    pasta_test_do 6 loopback udp 1 0 "port"       $(($(cat /proc/sys/net/core/wmem_default) / 4))
}

### ICMP, ICMPv6 ###############################################################

@test "podman networking with pasta(1) - ICMP echo request" {
    skip_if_no_ipv4 "IPv6 not routable on the host"

    local minuid=$(cut -f1 /proc/sys/net/ipv4/ping_group_range)
    local maxuid=$(cut -f2 /proc/sys/net/ipv4/ping_group_range)

    if [ $(id -u) -lt ${minuid} ] || [ $(id -u) -gt ${maxuid} ]; then
        skip "ICMP echo sockets not available for this UID"
    fi

    run_podman run --net=pasta $IMAGE \
        sh -c 'ping -c3 -W1 sed -nr "s/^nameserver[ ]{1,}([^.]*).(.*)/\1.\2/p" /etc/resolv.conf | head -1'
}

@test "podman networking with pasta(1) - ICMPv6 echo request" {
    skip_if_no_ipv6 "IPv6 not routable on the host"

    local minuid=$(cut -f1 /proc/sys/net/ipv4/ping_group_range)
    local maxuid=$(cut -f2 /proc/sys/net/ipv4/ping_group_range)

    if [ $(id -u) -lt ${minuid} ] || [ $(id -u) -gt ${maxuid} ]; then
        skip "ICMPv6 echo sockets not available for this UID"
    fi

    run_podman run --net=pasta $IMAGE ping -6 -c3 -W1 \
        sh -c 'ping -c3 -W1 sed -nr "s/^nameserver[ ]{1,}([^:]*):(.*)/\1:\2/p" /etc/resolv.conf | head -1'
}

### Lifecycle ##################################################################

@test "podman networking with pasta(1) - pasta(1) quits when the namespace is gone" {
    local pidfile="${PODMAN_TMPDIR}/pasta.pid"

    run_podman run "--net=pasta:--pid,${pidfile}" $IMAGE true
    sleep 1
    ! ps -p $(cat "${pidfile}") && rm "${pidfile}"
}
