####> This option file is used in:
####>   podman create, pod create, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--ip6**=*ipv6*

Specify a static IPv6 address for the <<container|pod>>, for example **fd46:db93:aa76:ac37::10**.
This option can only be used if the <<container|pod>> is joined to only a single network - i.e., **--network=network-name** is used at most once -
and if the <<container|pod>> is not joining another container's network namespace via **--network=container:_id_**.
The address must be within the network's IPv6 address pool.

To specify multiple static IPv6 addresses per <<container|pod>>, set multiple networks using the **--network** option with a static IPv6 address specified for each using the `ip6` mode for that option.
