% kpod(1) kpod-exec - Execute a command in a running container
% Brent Baude
# kpod-exec "1" "December 2017" "kpod"

## NAME
kpod-exec - Execute a command in a running container

## SYNOPSIS
**kpod exec**
**CONTAINER**
[COMMAND] [ARG...]
[**--help**|**-h**]

## DESCRIPTION
**kpod exec** executes a command in a running container.

## OPTIONS
**--env, e**
You may specify arbitrary environment variables that are available for the
command to be executed.

**--interactive, -i**
Not supported.  All exec commands are interactive by default.

**--privileged**
Give the process extended Linux capabilities when running the command in container.

**--tty, -t**
Allocate a pseudo-TTY.

**--user, -u**
Sets the username or UID used and optionally the groupname or GID for the specified command.
The following examples are all valid:
--user [user | user:group | uid | uid:gid | user:gid | uid:group ]

## EXAMPLES


## SEE ALSO
kpod(1), kpod-run(1)

## HISTORY
December 2017, Originally compiled by Brent Baude<bbaude@redhat.com>
