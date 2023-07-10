####> This option file is used in:
####>   podman auto update, build, container runlabel, create, kube play, login, manifest add, manifest create, manifest inspect, manifest push, pull, push, run, search
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--tls-verify**

Require HTTPS and verify certificates when contacting registries (default: **true**).
If explicitly set to **true**, TLS verification is used.
If set to **false**, TLS verification is not used.
If not specified, TLS verification is used unless the target registry
is listed as an insecure registry in **[containers-registries.conf(5)](https://github.com/containers/image/blob/main/docs/containers-registries.conf.5.md)**
