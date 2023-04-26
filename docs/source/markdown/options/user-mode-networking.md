####> This option file is used in:
####>   podman machine init, machine set
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--user-mode-networking**

Whether this machine should relay traffic from the guest through a user-space
process running on the host. In some VPN configurations the VPN may drop
traffic from alternate network interfaces, including VM network devices. By
enabling user-mode networking (a setting of `true`), VPNs will observe all
podman machine traffic as coming from the host, bypassing the problem.

When the qemu backend is used (Linux, Mac), user-mode networking is
mandatory and the only allowed value is `true`. In contrast, The Windows/WSL
backend defaults to `false`, and follows the standard WSL network setup.
Changing this setting to `true` on Windows/WSL will inform Podman to replace
the WSL networking setup on start of this machine instance with a user-mode
networking distribution. Since WSL shares the same kernel across
distributions, all other running distributions will reuse this network.
Likewise, when the last machine instance with a `true` setting stops, the
original networking setup will be restored.
