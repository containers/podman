####> This option file is used in:
####>   podman artifact pull, artifact push, build, container runlabel, create, farm build, image sign, kube play, login, manifest add, manifest push, pull, push, run, search
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--cert-dir**=*path*

Use certificates at *path* (\*.crt, \*.cert, \*.key) to connect to the registry. (Default: /etc/containers/certs.d)
For details, see **[containers-certs.d(5)](https://github.com/containers/image/blob/main/docs/containers-certs.d.5.md)**.
(This option is not available with the remote Podman client, including Mac and Windows (excluding WSL2) machines)
