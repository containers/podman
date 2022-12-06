####> This option file is used in:
####>   podman create, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--health-startup-cmd**=*"command"* | *'["command", "arg1", ...]'*

Set a startup healthcheck command for a container. This command will be executed inside the container and is used to gate the regular
healthcheck. When the startup command succeeds, the regular healthcheck will begin and the startup healthcheck will cease. Optionally,
if the command fails for a set number of attempts, the container will be restarted. A startup healthcheck can be used to ensure that
containers with an extended startup period are not marked as unhealthy until they are fully started. Startup healthchecks can only be
used when a regular healthcheck (from the container's image or the **--health-cmd** option) is also set.
