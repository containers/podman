## `contrib/cni` ##

There are a wide variety of different [CNI][cni] network configurations. This
directory just contains some example configurations that can be used as the
basis for your own configurations (distributions should package these files in
example directories).

To use these configurations, place them in `/etc/cni/net.d` (or the directory
specified by `crio.network.network_dir` in your `crio.conf`).

In addition, you need to install the [CNI plugins][cni] necessary into
`/opt/cni/bin` (or the directory specified by `crio.network.plugin_dir`). The
two plugins necessary for the example CNI configurations are `loopback` and
`bridge`.

[cni]: https://github.com/containernetworking/plugins
