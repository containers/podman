####> This option file is used in:
####>   podman create, run, update
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--health-timeout**=*timeout*

The maximum time allowed to complete the healthcheck before an interval is considered failed. Like start-period, the
value can be expressed in a time format such as **1m22s**. The default value is **30s**.

Note: A timeout marks the healthcheck as failed but does not terminate the running process.
This ensures that a slow but eventually successful healthcheck does not disrupt the container
but is still accounted for in the health status.

Note: This parameter will overwrite related healthcheck configuration from the image.
