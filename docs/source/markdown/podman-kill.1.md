% podman-kill(1)

## NAME
podman\-kill - Kill the main process in one or more containers

## SYNOPSIS
**podman kill** [*options*] [*container* ...]

**podman container kill** [*options*] [*container* ...]

## DESCRIPTION
The main process inside each container specified will be sent SIGKILL, or any signal specified with option --signal.

## OPTIONS
#### **--all**, **-a**

Signal all running containers.  This does not include paused containers.

#### **--cidfile**

Read container ID from the specified file and remove the container.  Can be specified multiple times.

#### **--latest**, **-l**

Instead of providing the container name or ID, use the last created container. If you use methods other than Podman
to run containers such as CRI-O, the last started container could be from either of those methods. (This option is not available with the remote Podman client, including Mac and Windows (excluding WSL2) machines)

#### **--signal**, **-s**

Signal to send to the container. For more information on Linux signals, refer to *man signal(7)*.


## EXAMPLE

podman kill mywebserver

podman kill 860a4b23

podman kill --signal TERM 860a4b23

podman kill --latest

podman kill --signal KILL -a

podman kill --cidfile /home/user/cidfile-1

podman kill --cidfile /home/user/cidfile-1 --cidfile ./cidfile-2

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-stop(1)](podman-stop.1.md)**

## HISTORY
September 2017, Originally compiled by Brent Baude <bbaude@redhat.com>
