#!/bin/bash

# This script is intended to be called by Cirrus-CI on a Mac M1 persistent worker.
# It performs a best-effort attempt at cleaning up from one task execution to the next.
# Since it run both before and after tasks, it must exit cleanly if there was a cleanup
# failure (i.e. file or directory not found).

# Help anybody debugging side-effects, since failures are ignored (by necessity).
set +e -x

# These are the main processes which could leak out of testing.
killall podman vfkit gvproxy make go ginkgo

# This is defined as $TMPDIR during setup.  Name must be kept
# "short" as sockets may reside here.  Darwin suffers from
# the same limited socket-pathname character-length restriction
# as Linux.
rm -rf /private/tmp/ci/* /private/tmp/ci/.??*

# Don't clobber the $CIRRUS_WORKING_DIR for this (running) task.
# shellcheck disable=SC2154
find "${ORIGINAL_HOME:-$HOME}/ci" -mindepth 1 -maxdepth 1 \
    -not -name "*task-${CIRRUS_TASK_ID}*" -prune -exec rm -rf '{}' +

# Bash scripts exit with the status of the last command.
true
