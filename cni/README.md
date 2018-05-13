## `cni` ##

There are a wide variety of different [CNI][cni] network configurations. This
directory just contains an example configuration that can be used as the
basis for your own configuration.

To use this configuration, place it in `/etc/cni/net.d` (or the directory
specified by `cni_config_dir` in your `libpod.conf`).

In addition, you need to install the [CNI plugins][cni] necessary into
`/opt/cni/bin` (or the directory specified by `cni_plugin_dir`). The
two plugins necessary for the example CNI configurations are `portmap` and
`bridge`.

[cni]: https://github.com/containernetworking/plugins
