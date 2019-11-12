% podman-remote(1)

## NAME
podman-remote - A remote CLI for Podman: A Simple management tool for pods, containers and images.

## SYNOPSIS
**podman-remote** [*options*] *command*

## DESCRIPTION
Podman (Pod Manager) is a fully featured container engine that is a simple daemonless tool.
Podman provides a Docker-CLI comparable command line that eases the transition from other
container engines and allows the management of pods, containers and images.  Simply put: `alias docker=podman`.
Most Podman commands can be run as a regular user, without requiring additional
privileges.

Podman uses Buildah(1) internally to create container images. Both tools share image
(not container) storage, hence each can use or manipulate images (but not containers)
created by the other.

Podman-remote provides a local client interacting with a Podman backend node through a varlink ssh connection. In this context, a Podman node is a Linux system with Podman installed on it and the varlink service activated. Credentials for this session can be passed in using flags, environment variables, or in `podman-remote.conf`

**podman [GLOBAL OPTIONS]**

## GLOBAL OPTIONS

**--connection**=*name*

Remote connection name

**--help**, **-h**

Print usage statement

**--log-level**=*level*

Log messages above specified level: debug, info, warn, error (default), fatal or panic

**--port**=*integer*

Use an alternative port for the ssh connections.  The default port is 22

**--remote-config-path**=*path*

Alternate path for configuration file

**--remote-host**=*ip*

Remote host IP

**--syslog**

Output logging information to syslog as well as the console

**--username**=*string*

Username on the remote host (defaults to current username)

**--version**

Print the version

## Exit Status

The exit code from `podman` gives information about why the container
failed to run or why it exited.  When `podman` commands exit with a non-zero code,
the exit codes follow the `chroot` standard, see below:

**_125_** if the error is with podman **_itself_**

    $ podman run --foo busybox; echo $?
    Error: unknown flag: --foo
      125

**_126_** if executing a **_contained command_** and the **_command_** cannot be invoked

    $ podman run busybox /etc; echo $?
    Error: container_linux.go:346: starting container process caused "exec: \"/etc\": permission denied": OCI runtime error
      126

**_127_** if executing a **_contained command_** and the **_command_** cannot be found
    $ podman run busybox foo; echo $?
    Error: container_linux.go:346: starting container process caused "exec: \"foo\": executable file not found in $PATH": OCI runtime error
      127

**_Exit code_** of **_contained command_** otherwise

    $ podman run busybox /bin/sh -c 'exit 3'
    # 3


## COMMANDS

| Command                                          | Description                                                                 |
| ------------------------------------------------ | --------------------------------------------------------------------------- |
| [podman-attach(1)](podman-attach.1.md)           | Attach to a running container.                                              |
| [podman-build(1)](podman-build.1.md)             | Build a container image using a Dockerfile.                                 |
| [podman-commit(1)](podman-commit.1.md)           | Create new image based on the changed container.                            |
| [podman-container(1)](podman-container.1.md)     | Manage containers.                                                          |
| [podman-cp(1)](podman-cp.1.md)                   | Copy files/folders between a container and the local filesystem.            |
| [podman-create(1)](podman-create.1.md)           | Create a new container.                                                     |
| [podman-diff(1)](podman-diff.1.md)               | Inspect changes on a container or image's filesystem.                       |
| [podman-events(1)](podman-events.1.md)           | Monitor Podman events                                                       |
| [podman-export(1)](podman-export.1.md)           | Export a container's filesystem contents as a tar archive.                  |
| [podman-generate(1)](podman-generate.1.md)       | Generate structured data based for a containers and pods.                   |
| [podman-healthcheck(1)](podman-healthcheck.1.md) | Manage healthchecks for containers                                          |
| [podman-history(1)](podman-history.1.md)         | Show the history of an image.                                               |
| [podman-image(1)](podman-image.1.md)             | Manage images.                                                              |
| [podman-images(1)](podman-images.1.md)           | List images in local storage.                                               |
| [podman-import(1)](podman-import.1.md)           | Import a tarball and save it as a filesystem image.                         |
| [podman-info(1)](podman-info.1.md)               | Displays Podman related system information.                                 |
| [podman-init(1)](podman-init.1.md)               | Initialize a container                                                      |
| [podman-inspect(1)](podman-inspect.1.md)         | Display a container or image's configuration.                               |
| [podman-kill(1)](podman-kill.1.md)               | Kill the main process in one or more containers.                            |
| [podman-load(1)](podman-load.1.md)               | Load an image from a container image archive into container storage.        |
| [podman-logs(1)](podman-logs.1.md)               | Display the logs of a container.                                            |
| [podman-pause(1)](podman-pause.1.md)             | Pause one or more containers.                                               |
| [podman-pod(1)](podman-pod.1.md)                 | Management tool for groups of containers, called pods.                      |
| [podman-port(1)](podman-port.1.md)               | List port mappings for a container.                                         |
| [podman-ps(1)](podman-ps.1.md)                   | Prints out information about containers.                                    |
| [podman-pull(1)](podman-pull.1.md)               | Pull an image from a registry.                                              |
| [podman-push(1)](podman-push.1.md)               | Push an image from local storage to elsewhere.                              |
| [podman-restart(1)](podman-restart.1.md)         | Restart one or more containers.                                             |
| [podman-rm(1)](podman-rm.1.md)                   | Remove one or more containers.                                              |
| [podman-rmi(1)](podman-rmi.1.md)                 | Removes one or more locally stored images.                                  |
| [podman-run(1)](podman-run.1.md)                 | Run a command in a new container.                                           |
| [podman-save(1)](podman-save.1.md)               | Save an image to a container archive.                                       |
| [podman-start(1)](podman-start.1.md)             | Start one or more containers.                                               |
| [podman-stop(1)](podman-stop.1.md)               | Stop one or more running containers.                                        |
| [podman-system(1)](podman-system.1.md)           | Manage podman.                                                              |
| [podman-tag(1)](podman-tag.1.md)                 | Add an additional name to a local image.                                    |
| [podman-top(1)](podman-top.1.md)                 | Display the running processes of a container.                               |
| [podman-unpause(1)](podman-unpause.1.md)         | Unpause one or more containers.                                             |
| [podman-version(1)](podman-version.1.md)         | Display the Podman version information.                                     |
| [podman-volume(1)](podman-volume.1.md)           | Manage Volumes.                                                             |

## FILES

**podman-remote.conf** (`~/.config/containers/podman-remote.conf`)

    The podman-remote.conf file is the default configuration file for the podman
    remote client.  It is in the TOML format.  It is primarily used to keep track
    of the user's remote connections.

## SEE ALSO
`podman-remote.conf(5)`
