## `cni` ##

**Note**: CNI is being deprecated from Podman and support will be dropped at a future date.  Use of netavark is now
advised and is the default network backend for Podman.

There are a wide variety of different [CNI](https://github.com/containernetworking/cni) network configurations. This
directory just contains an example configuration that can be used as the
basis for your own configuration.

To use this configuration, place it in `/etc/cni/net.d` (or the directory
specified by `cni_config_dir` in your `containers.conf`).

For example a basic network configuration can be achieved with:

```bash
sudo mkdir -p /etc/cni/net.d
curl -qsSL https://raw.githubusercontent.com/containers/podman/main/cni/87-podman-bridge.conflist | sudo tee /etc/cni/net.d/87-podman-bridge.conflist
```

Dependent upon your CNI configuration, you will need to install as a minimum the `port` and `bridge`  [CNI plugins](https://github.com/containernetworking/plugins) into `/opt/cni/bin` (or the directory specified by `cni_plugin_dir` in containers.conf).  Please refer to the [CNI](https://github.com/containernetworking) project page in GitHub for more information.
