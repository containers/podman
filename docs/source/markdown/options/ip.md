####> This option file is used in:
####>   podman podman-container.unit.5.md.in, create, pod create, podman-pod.unit.5.md.in, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `IP=ipv4`
<< else >>
#### **--ip**=*ipv4*
<< endif >>

Specify a static IPv4 address for the <<container|pod>>, for example **10.88.64.128**.
This option can only be used if the <<container|pod>> is joined to only a single network - i.e.,
<< '**Network=network-name**' if is_quadlet else '**--network=network-name**' >> is used at most once -
and if the <<container|pod>> is not joining another container's network namespace via
<< '**Network=container:_id_**' if is_quadlet else '**--network=container:_id_**' >>.
The address must be within the network's IP address pool (default **10.88.0.0/16**).

To specify multiple static IP addresses per <<container|pod>>, set multiple networks using
the << '**Network=**' if is_quadlet else '**--network' >> option with a static IP address
specified for each using the `ip` mode for that option.
