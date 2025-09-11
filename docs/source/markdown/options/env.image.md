####> This option file is used in:
####>   podman build, farm build
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--env**=*env[=value]*

Add a value (e.g. env=*value*) to the built image.  Can be used multiple times.
If neither `=` nor a *value* are specified, but *env* is set in the current
environment, the value from the current environment is added to the image.
To remove an environment variable from the built image, use the `--unsetenv`
option.
