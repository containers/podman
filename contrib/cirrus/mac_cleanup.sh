#!/bin/bash

# This script is intended to be called by Cirrus-CI on a Mac M1 persistent worker.
# It performs a best-effort attempt at cleaning up from one task execution to the next.
# Since it run both before and after tasks, it must exit cleanly if there was a cleanup
# failure (i.e. file or directory not found).

# Help anybody debugging side-effects, since failures are ignored (by necessity).
set +e -x

# These are the main processes which could leak out of testing.
killall podman vfkit gvproxy make go ginkgo

# Golang will leave behind lots of read-only bits, ref:
# https://go.dev/ref/mod#module-cache
# However other tools/scripts could also set things read-only.
# At this point in CI, we really want all this stuff gone-gone,
# so there's actually zero-chance it can interfere.
chmod -R u+w /private/tmp/ci/* /private/tmp/ci/.??*

# This is defined as $TMPDIR during setup.  Name must be kept
# "short" as sockets may reside here.  Darwin suffers from
# the same limited socket-pathname character-length restriction
# as Linux.
rm -rf /private/tmp/ci/* /private/tmp/ci/.??*

# Don't change or clobber anything under $CIRRUS_WORKING_DIR for
# the currently running task.  But make sure we have write permission
# (go get sets dependencies ro) for everything else, before removing it.
# First make everything writeable - see the "Golang will..." comment above.
# shellcheck disable=SC2154
find "${ORIGINAL_HOME:-$HOME}/ci" -mindepth 1 -maxdepth 1 \
    -not -name "*task-${CIRRUS_TASK_ID}*" -prune -exec chmod -R u+w '{}' +
find "${ORIGINAL_HOME:-$HOME}/ci" -mindepth 1 -maxdepth 1 \
    -not -name "*task-${CIRRUS_TASK_ID}*" -prune -exec rm -rf '{}' +

# Bash scripts exit with the status of the last command.
true
