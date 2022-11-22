####> This option file is used in:
####>   podman create, exec, run
####> If you edit this file, make sure your changes
####> are applicable to all of those files.
#### **--workdir**, **-w**=*dir*

Working directory inside the container.

The default working directory for running binaries within a container is the root directory (**/**).
The image developer can set a different default with the WORKDIR instruction. The operator
can override the working directory by using the **-w** option.
