####> This option file is used in:
####>   podman create, run, update
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--health-startup-success**=*retries*

The number of successful runs required before the startup healthcheck succeeds and the regular healthcheck begins. A value
of **0** means that any success begins the regular healthcheck. The default is **0**.
