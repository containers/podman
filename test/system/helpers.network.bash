# -*- bash -*-

_cached_has_pasta=
_cached_has_slirp4netns=

### Feature Checks #############################################################

# has_ipv4() - Check if one default route is available for IPv4
function has_ipv4() {
    [ -n "$(ip -j -4 route show | jq -rM '.[] | select(.dst == "default")')" ]
}

# has_ipv6() - Check if one default route is available for IPv6
function has_ipv6() {
    [ -n "$(ip -j -6 route show | jq -rM '.[] | select(.dst == "default")')" ]
}

# skip_if_no_ipv4() - Skip current test if IPv4 traffic can't be routed
# $1:	Optional message to display
function skip_if_no_ipv4() {
    if ! has_ipv4; then
        local msg=$(_add_label_if_missing "$1" "IPv4")
        skip "${msg:-not applicable with no routable IPv4}"
    fi
}

# skip_if_no_ipv6() - Skip current test if IPv6 traffic can't be routed
# $1:	Optional message to display
function skip_if_no_ipv6() {
    if ! has_ipv6; then
        local msg=$(_add_label_if_missing "$1" "IPv6")
        skip "${msg:-not applicable with no routable IPv6}"
    fi
}

# has_slirp4netns - Check if the slirp4netns(1) command is available
function has_slirp4netns() {
    if [[ -z "$_cached_has_slirp4netns" ]]; then
        _cached_has_slirp4netns=n
        run_podman info --format '{{.Host.Slirp4NetNS.Executable}}'
        if [[ -n "$output" ]]; then
            _cached_has_slirp4netns=y
        fi
    fi
    test "$_cached_has_slirp4netns" = "y"
}

# has_pasta() - Check if the pasta(1) command is available
function has_pasta() {
    if [[ -z "$_cached_has_pasta" ]]; then
        _cached_has_pasta=n
        run_podman info --format '{{.Host.Pasta.Executable}}'
        if [[ -n "$output" ]]; then
            _cached_has_pasta=y
        fi
    fi
    test "$_cached_has_pasta" = "y"
}

# skip_if_no_pasta() - Skip current test if pasta(1) is not available
# $1:	Optional message to display
function skip_if_no_pasta() {
    if ! has_pasta; then
        local msg=$(_add_label_if_missing "$1" "pasta")
        skip "${msg:-not applicable with no pasta binary}"
    fi
}


### procfs access ##############################################################

# ipv6_to_procfs() - RFC 5952 IPv6 address text representation to procfs format
# $1:	Address in any notation described by RFC 5952
function ipv6_to_procfs() {
    local addr="${1}"

    # Add leading zero if missing
    case ${addr} in
        "::"*) addr=0"${addr}" ;;
    esac

    # Double colon can mean any number of all-zero fields. Expand to fill
    # as many colons as are missing. (This will not be a valid IPv6 form,
    # but we don't need it for long). E.g., 0::1 -> 0:::::::1
    case ${addr} in
        *"::"*)
            # All the colons in the address
            local colons
            colons=$(tr -dc : <<<$addr)
            # subtract those from a string of eight colons; this gives us
            # a string of two to six colons...
            local pad
            pad=$(sed -e "s/$colons//" <<<":::::::")
            # ...which we then inject in place of the double colon.
            addr=$(sed -e "s/::/::$pad/" <<<$addr)
            ;;
    esac

    # Print as a contiguous string of zero-filled 16-bit words
    # (The additional ":" below is needed because 'read -d x' actually
    # means "x is a TERMINATOR, not a delimiter")
    local group
    while read -d : group; do
        printf "%04X" "0x${group:-0}"
    done <<<"${addr}:"
}

# __ipv4_to_procfs() - Print bytes in hexadecimal notation reversing arguments
# $@:	IPv4 address as separate bytes
function __ipv4_to_procfs() {
    printf "%02X%02X%02X%02X" ${4} ${3} ${2} ${1}
}

# ipv4_to_procfs() - IPv4 address representation to big-endian procfs format
# $1:	Text representation of IPv4 address
function ipv4_to_procfs() {
    IFS='.' read -r o1 o2 o3 o4 <<< $1
    __ipv4_to_procfs $o1 $o2 $o3 $o4
}


### Addresses, Routes, Links ###################################################

# ipv4_get_addr_global() - Print first global IPv4 address reported by netlink
# $1:	Optional output of 'ip -j -4 address show' from a different context
function ipv4_get_addr_global() {
    local expr='[.[].addr_info[] | select(.scope=="global")] | .[0].local'
    echo "${1:-$(ip -j -4 address show)}" | jq -rM "${expr}"
}

# ipv6_get_addr_global() - Print first global IPv6 address reported by netlink
# $1:	Optional output of 'ip -j -6 address show' from a different context
function ipv6_get_addr_global() {
    local expr='[.[].addr_info[] | select(.scope=="global")] | .[0].local'
    echo "${1:-$(ip -j -6 address show)}" | jq -rM "${expr}"
}

# random_rfc1918_subnet() - Pseudorandom unused subnet in 172.16/12 prefix
#
# Use the class B set, because much of our CI environment (Google, RH)
# already uses up much of the class A, and it's really hard to test
# if a block is in use.
#
# This returns THREE OCTETS! It is up to our caller to append .0/24, .255, &c.
#
function random_rfc1918_subnet() {
    local retries=1024

    while [ "$retries" -gt 0 ];do
        # 172.16.0.0 -> 172.31.255.255
        local n1=172
        local n2=$(( 16 + $RANDOM & 15 ))
        local n3=$(( $RANDOM & 255 ))

        if ! subnet_in_use $n1 $n2 $n3; then
            echo "$n1.$n2.$n3"
            return
        fi

        retries=$(( retries - 1 ))
    done

    die "Could not find a random not-in-use rfc1918 subnet"
}

# subnet_in_use() - true if subnet already routed on host
function subnet_in_use() {
    local subnet_script=${PODMAN_TMPDIR-/var/tmp}/subnet-in-use
    rm -f $subnet_script

    # This would be a nightmare to do in bash. ipcalc, ipcalc-ng, sipcalc
    # would be nice but are unavailable some environments (cough RHEL).
    # Likewise python/perl netmask modules. So, use bare-bones perl.
    cat >$subnet_script <<"EOF"
#!/usr/bin/env perl

use strict;
use warnings;

# 3 octets, in binary: 172.16.x -> 1010 1100 0000 1000 xxxx xxxx ...
my $subnet_to_check = sprintf("%08b%08b%08b", @ARGV);

my $found = 0;

# Input is "ip route list", one or more lines like '10.0.0.0/8 via ...'
while (<STDIN>) {
    # Only interested in x.x.x.x/n lines
    if (m!^([\d.]+)/(\d+)!) {
        my ($ip, $bits) = ($1, $2);

        # Our caller has /24 granularity, so treat /30 on host as /24.
        $bits = 24 if $bits > 24;

        # Temporary: entire subnet as binary string. 4 octets, split,
        # then represented as a 32-bit binary string.
        my $net = sprintf("%08b%08b%08b%08b", split(/\./, $ip));

        # Now truncate those 32 bits down to the route's netmask size.
        # This is the actual subnet range in use on the host.
        my $net_truncated = sprintf("%.*s", $bits, $net);

        # Desired subnet is in use if it matches a host route prefix
#        print STDERR "--- $subnet_to_check in $net_truncated (@ARGV in $ip/$bits)\n";
        $found = 1 if $subnet_to_check =~ /^$net_truncated/;
    }
}

# Convert to shell exit status (0 = success)
exit !$found;
EOF

    chmod 755 $subnet_script

    # This runs 'ip route list', converts x.x.x.x/n to its binary prefix,
    # then checks if our desired subnet matches that prefix (i.e. is in
    # that range). Existing routes with size greater than 24 are
    # normalized to /24 because that's the granularity of our
    # random_rfc1918_subnet code.
    #
    # Contrived examples:
    #    127.0.0.0/1   -> 0
    #    128.0.0.0/1   -> 1
    #    10.0.0.0/8    -> 00001010
    #
    # I'm so sorry for the ugliness.
    ip route list | $subnet_script $*
}

# ipv4_get_route_default() - Print first default IPv4 route reported by netlink
# $1:	Optional output of 'ip -j -4 route show' from a different context
function ipv4_get_route_default() {
    local jq_gw='[.[] | select(.dst == "default").gateway] | .[0]'
    local jq_nh='[.[] | select(.dst == "default").nexthops[0].gateway] | .[0]'
    local out

    out="$(echo "${1:-$(ip -j -4 route show)}" | jq -rM "${jq_gw}")"
    if [ "${out}" = "null" ]; then
        out="$(echo "${1:-$(ip -j -4 route show)}" | jq -rM "${jq_nh}")"
    fi

    echo "${out}"
}

# ipv6_get_route_default() - Print first default IPv6 route reported by netlink
# $1:	Optional output of 'ip -j -6 route show' from a different context
function ipv6_get_route_default() {
    local jq_gw='[.[] | select(.dst == "default").gateway] | .[0]'
    local jq_nh='[.[] | select(.dst == "default").nexthops[0].gateway] | .[0]'
    local out

    out="$(echo "${1:-$(ip -j -6 route show)}" | jq -rM "${jq_gw}")"
    if [ "${out}" = "null" ]; then
        out="$(echo "${1:-$(ip -j -6 route show)}" | jq -rM "${jq_nh}")"
    fi

    echo "${out}"
}

# ether_get_mtu() - Get MTU of first Ethernet-like link
# $1:	Optional output of 'ip -j link show' from a different context
function ether_get_mtu() {
    local jq_expr='[.[] | select(.link_type == "ether").mtu] | .[0]'
    echo "${1:-$(ip -j link show)}" | jq -rM "${jq_expr}"
}

# ether_get_name() - Get name of first Ethernet-like interface
# $1:	Optional output of 'ip -j link show' from a different context
function ether_get_name() {
    local jq_expr='[.[] | select(.link_type == "ether").ifname] | .[0]'
    echo "${1:-$(ip -j link show)}" | jq -rM "${jq_expr}"
}


### Ports and Ranges ###########################################################

# reserve_port() - create a lock file reserving a port, or return false
function reserve_port() {
    local port=$1

    # NOTE: there's no mechanism to free or unreserve ports.
    #   Should that ever be desired: make $lockdir global,
    #   grep -w $BATS_SUITE_TEST_NUMBER $lockdir/*, and rm those.
    local lockdir=$BATS_SUITE_TMPDIR/reserved-ports
    mkdir -p $lockdir
    local lockfile=$lockdir/$port
    local locktmp=$lockdir/.$port.$$
    echo $BATS_SUITE_TEST_NUMBER >$locktmp

    if ln $locktmp $lockfile; then
        rm -f $locktmp
        return
    fi
    # Port already reserved
    rm -f $locktmp
    false
}

# random_free_port() - Get unbound port with pseudorandom number
# $1:	Optional, dash-separated interval, [5000, 5999] by default
# $2:	Optional binding address, any IPv4 address by default
# $3:	Optional protocol, tcp or udp
function random_free_port() {
    local range=${1:-5000-5999}
    local address=${2:-0.0.0.0}
    local protocol=${3:-tcp}

    local port
    for port in $(shuf -i ${range}); do
        if port_is_free $port $address $protocol; then
            echo $port
            return
        fi
    done

    die "Could not find open port in range $range"
}

# random_free_port_range() - Get range of unbound ports with pseudorandom start
# $1:	Size of range (i.e. number of ports)
# $2:	Optional binding address, any IPv4 address by default
# $3:	Optional protocol, tcp or udp
function random_free_port_range() {
    local size=${1?Usage: random_free_port_range SIZE [ADDRESS [tcp|udp]]}
    local address=${2:-0.0.0.0}
    local protocol=${3:-tcp}

    local maxtries=10
    while [[ $maxtries -gt 0 ]]; do
        local firstport=$(random_free_port)
        local lastport=
        for i in $(seq 1 $((size - 1))); do
            lastport=$((firstport + i))
            if ! port_is_free $lastport $address $protocol; then
                echo "# port $lastport is in use; trying another." >&3
                lastport=
                break
            fi
        done
        if [[ -n "$lastport" ]]; then
            echo "$firstport-$lastport"
            return
        fi

        maxtries=$((maxtries - 1))
    done

    die "Could not find free port range with size $size"
}

# port_is_bound() - Check if TCP or UDP port is bound for a given address
# $1:	Port number
# $2:	Optional protocol, or optional IPv4 or IPv6 address, default: tcp
# $3:	Optional IPv4 or IPv6 address, or optional protocol, default: any
function port_is_bound() {
    local port=${1?Usage: port_is_bound PORT [tcp|udp] [ADDRESS]}

    # First make sure no other tests are using it
    reserve_port $port || return 0

    if   [ "${2}" = "tcp" ] || [ "${2}" = "udp" ]; then
        local address="${3}"
        local proto="${2}"
    elif [ "${3}" = "tcp" ] || [ "${3}" = "udp" ]; then
        local address="${2}"
        local proto="${3}"
    else
        local address="${2}"	# Might be empty
        local proto="tcp"
    fi

    # /proc/net/tcp is insufficient: it does not show some rootless ports.
    # ss does, so check it first.
    run ss -${proto:0:1}nlH sport = $port
    if [[ -n "$output" ]]; then
        return
    fi

    port=$(printf %04X ${port})
    case "${address}" in
    *":"*)
        grep -e "^[^:]*: $(ipv6_to_procfs "${address}"):${port} .*" \
             -e "^[^:]*: $(ipv6_to_procfs "::0"):${port} .*"        \
             -q "/proc/net/${proto}6"
        ;;
    *"."*)
        grep -e "^[^:]*: $(ipv4_to_procfs "${address}"):${port}"    \
             -e "^[^:]*: $(ipv4_to_procfs "0.0.0.0"):${port}"       \
             -e "^[^:]*: $(ipv4_to_procfs "127.0.0.1"):${port}"     \
             -q "/proc/net/${proto}"
        ;;
    *)
        # No address: check both IPv4 and IPv6, for any bound address
        grep "^[^:]*: [^:]*:${port} .*" -q "/proc/net/${proto}6" || \
        grep "^[^:]*: [^:]*:${port} .*" -q "/proc/net/${proto}"
        ;;
    esac
}

# port_is_free() - Check if TCP or UDP port is free to bind for a given address
# $1:	Port number
# $2:	Optional protocol, or optional IPv4 or IPv6 address, default: tcp
# $3:	Optional IPv4 or IPv6 address, or optional protocol, default: any
function port_is_free() {
    ! port_is_bound ${@}
}

# wait_for_port() - Return once port is bound (available for use by clients)
# $1:	Host or address to check for possible binding
# $2:	Port number
# $3:	Optional timeout, 5 seconds if not given
function wait_for_port() {
    local host=$1
    local port=$2
    local _timeout=${3:-5}

    # Wait
    while [ $_timeout -gt 0 ]; do
        port_is_bound ${port} "${host}" && return
        sleep 1
        _timeout=$(( $_timeout - 1 ))
    done

    die "Timed out waiting for $host:$port"
}

# tcp_port_probe() - Check if a TCP port has an active listener
# $1:	Port number
# $2:	Optional address, 0.0.0.0 by default
function tcp_port_probe() {
    local address="${2:-0.0.0.0}"

    : | nc "${address}" "${1}"
}

### Pasta Helpers ##############################################################

function default_ifname() {
    local jq_expr='[.[] | select(.dst == "default").dev] | .[0]'
    local jq_expr_nh='[.[] | select(.dst == "default").nexthops[0].dev] | .[0]'
    local ip_ver="${1}"
    local out

    out="$(ip -j -"${ip_ver}" route show | jq -rM "${jq_expr}")"
    if [ "${out}" = "null" ]; then
        out="$(ip -j -"${ip_ver}" route show | jq -rM "${jq_expr_nh}")"
    fi

    echo "${out}"
}

function default_addr() {
    local ip_ver="${1}"
    local ifname="${2:-$(default_ifname "${ip_ver}")}"

    local expr='[.[0].addr_info[] | select(.deprecated != true)][0].local'
    ip -j -"${ip_ver}" addr show "${ifname}" | jq -rM "${expr}"
}
