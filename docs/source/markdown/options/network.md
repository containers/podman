#### **--network**=*mode*, **--net**

Set the network mode for the <<container|pod>>.

Valid _mode_ values are:

- **bridge[:OPTIONS,...]**: Create a network stack on the default bridge. This is the default for rootful containers. It is possible to specify these additional options:
  - **alias=name**: Add network-scoped alias for the container.
  - **ip=IPv4**: Specify a static ipv4 address for this container.
  - **ip=IPv6**: Specify a static ipv6 address for this container.
  - **mac=MAC**: Specify a static mac address for this container.
  - **interface_name**: Specify a name for the created network interface inside the container.

  For example to set a static ipv4 address and a static mac address, use `--network bridge:ip=10.88.0.10,mac=44:33:22:11:00:99`.
- \<network name or ID\>[:OPTIONS,...]: Connect to a user-defined network; this is the network name or ID from a network created by **[podman network create](podman-network-create.1.md)**. Using the network name implies the bridge network mode. It is possible to specify the same options described under the bridge mode above. You can use the **--network** option multiple times to specify additional networks.
- **none**: Create a network namespace for the container but do not configure network interfaces for it, thus the container has no network connectivity.
- **container:**_id_: Reuse another container's network stack.
- **host**: Do not create a network namespace, the container will use the host's network. Note: The host mode gives the container full access to local system services such as D-bus and is therefore considered insecure.
- **ns:**_path_: Path to a network namespace to join.
- **private**: Create a new namespace for the container. This will use the **bridge** mode for rootful containers and **slirp4netns** for rootless ones.
- **slirp4netns[:OPTIONS,...]**: use **slirp4netns**(1) to create a user network stack. This is the default for rootless containers. It is possible to specify these additional options, they can also be set with `network_cmd_options` in containers.conf:
  - **allow_host_loopback=true|false**: Allow slirp4netns to reach the host loopback IP (default is 10.0.2.2 or the second IP from slirp4netns cidr subnet when changed, see the cidr option below). The default is false.
  - **mtu=MTU**: Specify the MTU to use for this network. (Default is `65520`).
  - **cidr=CIDR**: Specify ip range to use for this network. (Default is `10.0.2.0/24`).
  - **enable_ipv6=true|false**: Enable IPv6. Default is true. (Required for `outbound_addr6`).
  - **outbound_addr=INTERFACE**: Specify the outbound interface slirp should bind to (ipv4 traffic only).
  - **outbound_addr=IPv4**: Specify the outbound ipv4 address slirp should bind to.
  - **outbound_addr6=INTERFACE**: Specify the outbound interface slirp should bind to (ipv6 traffic only).
  - **outbound_addr6=IPv6**: Specify the outbound ipv6 address slirp should bind to.
  - **port_handler=rootlesskit**: Use rootlesskit for port forwarding. Default.
  Note: Rootlesskit changes the source IP address of incoming packets to an IP address in the container network namespace, usually `10.0.2.100`. If your application requires the real source IP address, e.g. web server logs, use the slirp4netns port handler. The rootlesskit port handler is also used for rootless containers when connected to user-defined networks.
  - **port_handler=slirp4netns**: Use the slirp4netns port forwarding, it is slower than rootlesskit but preserves the correct source IP address. This port handler cannot be used for user-defined networks.
