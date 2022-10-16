# -*- bash -*-


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
    IFS='.' __ipv4_to_procfs ${1}
}


### Addresses ##################################################################

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
        local cidr=172.$(( 16 + $RANDOM % 16 )).$(( $RANDOM & 255 ))

        in_use=$(ip route list | fgrep $cidr)
        if [ -z "$in_use" ]; then
            echo "$cidr"
            return
        fi

        retries=$(( retries - 1 ))
    done

    die "Could not find a random not-in-use rfc1918 subnet"
}


### Ports and Ranges ###########################################################

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

# wait_for_port() - Return once port is available on the host
# $1:	Host or address to check for possible binding
# $2:	Port number
# $3:	Optional timeout, 5 seconds if not given
function wait_for_port() {
    local host=$1
    local port=$2
    local _timeout=${3:-5}

    # Wait
    while [ $_timeout -gt 0 ]; do
        port_is_free ${port} "${host}" && return
        sleep 1
        _timeout=$(( $_timeout - 1 ))
    done

    die "Timed out waiting for $host:$port"
}
