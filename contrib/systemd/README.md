# Setting up Podman service for systemd socket activation

## system-wide (podman service run as root)

The following unit file examples assume:
 1. copied the `service` executable into `/usr/local/bin`
 1. `chcon system_u:object_r:container_runtime_exec_t:s0 /usr/local/bin/service`

then:
 1. copy the `podman.service` and `podman.socket` files into `/etc/systemd/system`
 1. `systemctl daemon-reload`
 1. `systemctl enable podman.socket`
 1. `systemctl start podman.socket`
 1. `systemctl status podman.socket podman.service`

Assuming the status messages show no errors, the libpod service is ready to respond to the APIv2 on the unix domain socket `/run/podman/podman.sock`

### podman.service
```toml
[Unit]
Description=Podman API Service
Requires=podman.socket
After=podman.socket
Documentation=man:podman-api(1)
StartLimitIntervalSec=0

[Service]
Type=oneshot
Environment=REGISTRIES_CONFIG_PATH=/etc/containers/registries.conf
ExecStart=/usr/local/bin/service
TimeoutStopSec=30
KillMode=process

[Install]
WantedBy=multi-user.target
Also=podman.socket
```
### podman.socket

```toml
[Unit]
Description=Podman API Socket
Documentation=man:podman-api(1)

[Socket]
ListenStream=%t/podman/podman.sock
SocketMode=0660

[Install]
WantedBy=sockets.target
```
## user (podman service run as given user aka "rootless")

The following unit file examples assume:
 1. you have a created a directory `~/bin`
 1. copied the `service` executable into `~/bin`
 1. `chcon system_u:object_r:container_runtime_exec_t:s0 ~/bin/service`

then:
 1. `mkdir -p ~/.config/systemd/user`
 1. copy the `podman.service` and `podman.socket` files into `~/.config/systemd/user`
 1. `systemctl --user enable podman.socket`
 1. `systemctl --user start podman.socket`
 1. `systemctl --user status podman.socket podman.service`

Assuming the status messages show no errors, the libpod service is ready to respond to the APIv2 on the unix domain socket `/run/user/$(id -u)/podman/podman.sock`

### podman.service

```toml
[Unit]
Description=Podman API Service
Requires=podman.socket
After=podman.socket
Documentation=man:podman-api(1)
StartLimitIntervalSec=0

[Service]
Type=oneshot
Environment=REGISTRIES_CONFIG_PATH=/etc/containers/registries.conf
ExecStart=%h/bin/service
TimeoutStopSec=30
KillMode=process

[Install]
WantedBy=multi-user.target
Also=podman.socket
```
### podman.socket

```toml
[Unit]
Description=Podman API Socket
Documentation=man:podman-api(1)

[Socket]
ListenStream=%t/podman/podman.sock
SocketMode=0660

[Install]
WantedBy=sockets.target
```
