% containers-mounts.conf(5)

# NAME
containers-mounts.conf - configuration file for default mounts in containers

# DESCRIPTION
The mounts.conf file specifies volume mount directories that are automatically mounted inside containers. Container processes can then use this content. Usually these directories are used for passing secrets or credentials required by the package software to access remote package repositories. Note that for security reasons, tools adhering to the mounts.conf are expected to copy the contents instead of bind mounting the paths from the host.

# FORMAT
The format of the mounts.conf is the volume format `/SRC:/DEST`, one mount per line. For example, a mounts.conf with the line `/usr/share/secrets:/run/secrets` would cause the contents of the `/usr/share/secrets` directory on the host to be mounted on the `/run/secrets` directory inside the container. Setting mountpoints allows containers to use the files of the host, for instance, to use the host's subscription to some enterprise Linux distribution.

# FILES
Some distributions may provide a `/usr/share/containers/mounts.conf` file to provide default mounts, but users can create a `/etc/containers/mounts.conf`, to specify their own special volumes to mount in the container.

## HISTORY
Aug 2018, Originally compiled by Valentin Rothberg <vrothberg@suse.com>
