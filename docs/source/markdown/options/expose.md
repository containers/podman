####> This option file is used in:
####>   podman create, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--expose**=*port[/protocol]*

Expose a port or a range of ports (e.g. **--expose=3300-3310**).
The protocol can be `tcp`, `udp` or `sctp` and if not given `tcp` is assumed.
This option matches the EXPOSE instruction for image builds and has no effect on
the actual networking rules unless **-P/--publish-all** is used to forward to all
exposed ports from random host ports. To forward specific ports from the host
into the container use the **-p/--publish** option instead.
