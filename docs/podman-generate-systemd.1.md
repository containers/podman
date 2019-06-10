% podman-generate-systemd(1)

## NAME
podman-generate-systemd- Generate Systemd Unit file

## SYNOPSIS
**podman generate systemd** [*options*] *container*

## DESCRIPTION
**podman generate systemd** will create a Systemd unit file that can be used to control a container.  The
command will dynamically create the unit file and output it to stdout where it can be piped by the user
to a file.  The options can be used to influence the results of the output as well.


## OPTIONS:

**--name**, **-n**

Use the name of the container for the start, stop, and description in the unit file

**--timeout**, **-t**=*value*

Override the default stop timeout for the container with the given value.

**--restart-policy**=*policy*
Set the SystemD restart policy.  The restart-policy must be one of: "no", "on-success", "on-failure", "on-abnormal",
"on-watchdog", "on-abort", or "always".  The default policy is *on-failure*.

## Examples
Create a systemd unit file for a container running nginx:

```
$ sudo podman generate systemd nginx
[Unit]
Description=c21da63c4783be2ac2cd3487ef8d2ec15ee2a28f63dd8f145e3b05607f31cffc Podman Container
[Service]
Restart=on-failure
ExecStart=/usr/bin/podman start c21da63c4783be2ac2cd3487ef8d2ec15ee2a28f63dd8f145e3b05607f31cffc
ExecStop=/usr/bin/podman stop -t 10 c21da63c4783be2ac2cd3487ef8d2ec15ee2a28f63dd8f145e3b05607f31cffc
KillMode=none
Type=forking
PIDFile=/var/lib/containers/storage/overlay-containers/c21da63c4783be2ac2cd3487ef8d2ec15ee2a28f63dd8f145e3b05607f31cffc/userdata/c21da63c4783be2ac2cd3487ef8d2ec15ee2a28f63dd8f145e3b05607f31cffc.pid
[Install]
WantedBy=multi-user.target
```

Create a systemd unit file for a container running nginx with an *always* restart policy and 1-second timeout.
```
$ sudo podman generate systemd --restart-policy=always -t 1 nginx
[Unit]
Description=c21da63c4783be2ac2cd3487ef8d2ec15ee2a28f63dd8f145e3b05607f31cffc Podman Container
[Service]
Restart=always
ExecStart=/usr/bin/podman start c21da63c4783be2ac2cd3487ef8d2ec15ee2a28f63dd8f145e3b05607f31cffc
ExecStop=/usr/bin/podman stop -t 1 c21da63c4783be2ac2cd3487ef8d2ec15ee2a28f63dd8f145e3b05607f31cffc
KillMode=none
Type=forking
PIDFile=/var/lib/containers/storage/overlay-containers/c21da63c4783be2ac2cd3487ef8d2ec15ee2a28f63dd8f145e3b05607f31cffc/userdata/c21da63c4783be2ac2cd3487ef8d2ec15ee2a28f63dd8f145e3b05607f31cffc.pid
[Install]
WantedBy=multi-user.target
```

## SEE ALSO
podman(1), podman-container(1)

## HISTORY
April 2019, Originally compiled by Brent Baude (bbaude at redhat dot com)
