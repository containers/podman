####> This option file is used in:
####>   podman attach, exec, run, start
####> If you edit this file, make sure your changes
####> are applicable to all of those files.
#### **--detach-keys**=*sequence*

Specify the key sequence for detaching a container. Format is a single character `[a-Z]` or one or more `ctrl-<value>` characters where `<value>` is one of: `a-z`, `@`, `^`, `[`, `,` or `_`. Specifying "" will disable this feature. The default is *ctrl-p,ctrl-q*.

This option can also be set in **containers.conf**(5) file.
