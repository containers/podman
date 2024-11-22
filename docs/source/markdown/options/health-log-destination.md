####> This option file is used in:
####>   podman create, run, update
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--health-log-destination**=*directory_path*

Set the destination of the HealthCheck log. Directory path, local or events_logger (local use container state file) (Default: local)

* `local`: (default) HealthCheck logs are stored in overlay containers. (For example: `$runroot/healthcheck.log`)
* `directory`: creates a log file named `<container-ID>-healthcheck.log` with HealthCheck logs in the specified directory.
* `events_logger`: The log will be written with logging mechanism set by events_logger. It also saves the log to a default directory, for performance on a system with a large number of logs.
