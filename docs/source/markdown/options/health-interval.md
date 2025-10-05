####> This option file is used in:
####>   podman create, run, update
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--health-interval**=*interval*

Set an interval for the healthchecks. An _interval_ of **disable** results in no automatic timer setup. The default is **30s**.

Note: This parameter will overwrite related healthcheck configuration from the image.
