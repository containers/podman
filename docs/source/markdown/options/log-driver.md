#### **--log-driver**=*driver*

Logging driver for the container. Currently available options are **k8s-file**, **journald**, **none** and **passthrough**, with **json-file** aliased to **k8s-file** for scripting compatibility. (Default **journald**).

The podman info command below will display the default log-driver for the system.
```
$ podman info --format '{{ .Host.LogDriver }}'
journald
```
The **passthrough** driver passes down the standard streams (stdin, stdout, stderr) to the
container.  It is not allowed with the remote Podman client, including Mac and Windows (excluding WSL2) machines, and on a tty, since it is
vulnerable to attacks via TIOCSTI.
