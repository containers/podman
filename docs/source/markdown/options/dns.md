####> This option file is used in:
####>   podman build, create, farm build, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--dns**=*ipaddr*

Set custom DNS servers.

This option can be used to override the DNS
configuration passed to the container. Typically this is necessary when the
host DNS configuration is invalid for the container (e.g., **127.0.0.1**). When this
is the case the **--dns** flag is necessary for every run.

The special value **none** can be specified to disable creation of _/etc/resolv.conf_ in the container by Podman.
The _/etc/resolv.conf_ file in the image is then used without changes.

Note that **ipaddr** may be added directly to the container's _/etc/resolv.conf_.
This is not guaranteed though, and for example passing a network name to **--network** will result in _/etc/resolv.conf_ not being altered but rather netavark/aardvark-dns redirecting to the supplied **ipaddr**.
