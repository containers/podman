#### **--publish-all**, **-P**

Publish all exposed ports to random ports on the host interfaces. The default is **false**.

When set to **true**, publish all exposed ports to the host interfaces.
If the operator uses **-P** (or **-p**) then Podman will make the
exposed port accessible on the host and the ports will be available to any
client that can reach the host.

When using this option, Podman will bind any exposed port to a random port on the host
within an ephemeral port range defined by */proc/sys/net/ipv4/ip_local_port_range*.
To find the mapping between the host ports and the exposed ports, use **podman port**.
