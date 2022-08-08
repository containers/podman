#### **--network-alias**=*alias*

Add a network-scoped alias for the <<container|pod>>, setting the alias for all networks that the container joins. To set a
name only for a specific network, use the alias option as described under the **--network** option.
If the network has DNS enabled (`podman network inspect -f {{.DNSEnabled}} <name>`),
these aliases can be used for name resolution on the given network. This option can be specified multiple times.
NOTE: When using CNI a <<container|pod>> will only have access to aliases on the first network that it joins. This limitation does
not exist with netavark/aardvark-dns.
