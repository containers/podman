####> This option file is used in:
####>   podman create, kube play, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--log-opt**=*name=value*

Logging driver specific options.

Set custom logging configuration. The following *name*s are supported:

**path**: specify a path to the log file
    (e.g. **--log-opt path=/var/log/container/mycontainer.json**);

**max-size**: specify a max size of the log file
    (e.g. **--log-opt max-size=10mb**);

**tag**: specify a custom log tag for the container
    (e.g. **--log-opt tag="{{.ImageName}}"**.
It supports the same keys as **podman inspect --format**.
This option is currently supported only by the **journald** log driver.
