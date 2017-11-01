% kpod(1) kpod-attach - See the output of pid 1 of a container or enter the container
% Dan Walsh
# kpod-attach "1" "September 2017" "kpod"

## NAME
kpod-attach - Attach to a running container

## Description

We chose not to implement the `attach` feature in `kpod` even though the upstream Docker
project has it. The upstream project has had lots of issues with attaching to running
processes that we did not want to replicate. The `kpod exec` and `kpod log` commands
offer you the same functionality far more dependably.

**Reasons to attach to the primary PID of a container:**


1) Executing commands inside of the container

  We recommend that you use `kpod exec` to execute a command within a container

  `kpod exec CONTAINERID /bin/sh`

2) Viewing the output stream of the primary process in the container

  We recommend that you use `kpod logs` to see the output from the container

  `kpod logs CONTAINERID`

## SEE ALSO
kpod(1), kpod-exec(1), kpod-logs(1)
