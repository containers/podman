####> This option file is used in:
####>   podman create, run, update
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--health-timeout**=*timeout*

The maximum time allowed to complete the healthcheck before an interval is considered failed. Like start-period, the
value can be expressed in a time format such as **1m22s**. The default value is **30s**.

Note: A timeout marks the healthcheck as failed. If the healthcheck command itself runs longer than the specified *timeout*,
it will be sent a `SIGKILL` signal.

Note: This parameter will overwrite related healthcheck configuration from the image.
