####> This option file is used in:
####>   podman artifact pull, artifact push, auto update, build, container runlabel, create, farm build, image sign, kube play, login, logout, manifest add, manifest inspect, manifest push, pull, push, run, search
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--authfile**=*path*

Path of the authentication file. Default is `${XDG_RUNTIME_DIR}/containers/auth.json` on Linux, and `$HOME/.config/containers/auth.json` on Windows/macOS.
The file is created by **[podman login](podman-login.1.md)**. If the authorization state is not found there, `$HOME/.docker/config.json` is checked, which is set using **docker login**.

Note: There is also the option to override the default path of the authentication file by setting the `REGISTRY_AUTH_FILE` environment variable. This can be done with **export REGISTRY_AUTH_FILE=_path_**.
