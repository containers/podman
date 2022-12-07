####> This option file is used in:
####>   podman create, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--health-cmd**=*"command"* | *'["command", "arg1", ...]'*

Set or alter a healthcheck command for a container. The command is a command to be executed inside the
container that determines the container health. The command is required for other healthcheck options
to be applied. A value of **none** disables existing healthchecks.

Multiple options can be passed in the form of a JSON array; otherwise, the command will be interpreted
as an argument to **/bin/sh -c**.
