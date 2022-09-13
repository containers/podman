#### **--tty**, **-t**

Allocate a pseudo-TTY. The default is **false**.

When set to **true**, Podman will allocate a pseudo-tty and attach to the standard
input of the container. This can be used, for example, to run a throwaway
interactive shell.

**NOTE**: The --tty flag prevents redirection of standard output.  It combines STDOUT and STDERR, it can insert control characters, and it can hang pipes. This option should only be used when run interactively in a terminal. When feeding input to Podman, use -i only, not -it.
