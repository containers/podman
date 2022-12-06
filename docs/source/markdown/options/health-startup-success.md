####> This option file is used in:
####>   podman create, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--health-startup-success**=*retries*

The number of successful runs required before the startup healthcheck will succeed and the regular healthcheck will begin. A value
of **0** means that any success will begin the regular healthcheck. The default is **0**.
