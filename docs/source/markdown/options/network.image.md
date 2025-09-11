####> This option file is used in:
####>   podman build, farm build
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--network**=*mode*, **--net**

Sets the configuration for network namespaces when handling `RUN` instructions.

Valid _mode_ values are:

- **none**: no networking.
- **host**: use the Podman host network stack. Note: the host mode gives the
container full access to local system services such as D-bus and is therefore
considered insecure.
- **ns:**_path_: path to a network namespace to join.
- **private**: create a new namespace for the container (default)
- **\<network name|ID\>**: Join the network with the given name or ID, e.g. use `--network mynet` to join the network with the name mynet. Only supported for rootful users.
- **slirp4netns[:OPTIONS,...]**: use **slirp4netns**(1) to create a user network stack. It is possible to specify these additional options, they can also be set with `network_cmd_options` in containers.conf:
  - **allow_host_loopback=true|false**: Allow slirp4netns to reach the host loopback IP (default is 10.0.2.2 or the second IP from slirp4netns cidr subnet when changed, see the cidr option below). The default is false.
  - **mtu=MTU**: Specify the MTU to use for this network. (Default is `65520`).
  - **cidr=CIDR**: Specify ip range to use for this network. (Default is `10.0.2.0/24`).
  - **enable_ipv6=true|false**: Enable IPv6. Default is true. (Required for `outbound_addr6`).
  - **outbound_addr=INTERFACE**: Specify the outbound interface slirp binds to (ipv4 traffic only).
  - **outbound_addr=IPv4**: Specify the outbound ipv4 address slirp binds to.
  - **outbound_addr6=INTERFACE**: Specify the outbound interface slirp binds to (ipv6 traffic only).
  - **outbound_addr6=IPv6**: Specify the outbound ipv6 address slirp binds to.
- **pasta[:OPTIONS,...]**: use **pasta**(1) to create a user-mode networking
    stack. \
    This is the default for rootless containers and only supported in rootless mode. \
    By default, IPv4 and IPv6 addresses and routes, as well as the pod interface
    name, are copied from the host. If port forwarding isn't configured, ports
    are forwarded dynamically as services are bound on either side (init
    namespace or container namespace). Port forwarding preserves the original
    source IP address. Options described in pasta(1) can be specified as
    comma-separated arguments. \
    In terms of pasta(1) options, **--config-net** is given by default, in
    order to configure networking when the container is started, and
    **--no-map-gw** is also assumed by default, to avoid direct access from
    container to host using the gateway address. The latter can be overridden
    by passing **--map-gw** in the pasta-specific options (despite not being an
    actual pasta(1) option). \
    Also, **-t none** and **-u none** are passed to disable
    automatic port forwarding based on bound ports. Similarly, **-T none** and
    **-U none** are given to disable the same functionality from container to
    host. \
    Some examples:
    - **pasta:--map-gw**: Allow the container to directly reach the host using the
        gateway address.
    - **pasta:--mtu,1500**: Specify a 1500 bytes MTU for the _tap_ interface in
        the container.
    - **pasta:--ipv4-only,-a,10.0.2.0,-n,24,-g,10.0.2.2,--dns-forward,10.0.2.3,-m,1500,--no-ndp,--no-dhcpv6,--no-dhcp**,
        equivalent to default slirp4netns(1) options: disable IPv6, assign
        `10.0.2.0/24` to the `tap0` interface in the container, with gateway
        `10.0.2.3`, enable DNS forwarder reachable at `10.0.2.3`, set MTU to 1500
        bytes, disable NDP, DHCPv6 and DHCP support.
    - **pasta:-I,tap0,--ipv4-only,-a,10.0.2.0,-n,24,-g,10.0.2.2,--dns-forward,10.0.2.3,--no-ndp,--no-dhcpv6,--no-dhcp**,
        equivalent to default slirp4netns(1) options with Podman overrides: same as
        above, but leave the MTU to 65520 bytes
    - **pasta:-t,auto,-u,auto,-T,auto,-U,auto**: enable automatic port forwarding
        based on observed bound ports from both host and container sides
    - **pasta:-T,5201**: enable forwarding of TCP port 5201 from container to
        host, using the loopback interface instead of the tap interface for improved
        performance
