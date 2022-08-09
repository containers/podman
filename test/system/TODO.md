![PODMAN logo](https://raw.githubusercontent.com/containers/common/main/logos/podman-logo-full-vert.png)

# Overview

System tests exercise Podman in the context of a complete, composed environment from
distribution packages.  It should match as closely as possible to how an end-user
would experience a fresh-install.  Dependencies on external configuration and resources
must be kept minimal, and the tests must be generic and vendor-neutral.

The system-tests must execute cleanly on all tested platforms.  They may optionally
be executed during continuous-integration testing of code-changes, after all other
testing completes successfully.  For a list of tested platforms, please see [the
CI configuration file.](../../.cirrus.yml)


# Execution

When working from a clone of [the libpod repository](https://github.com/containers/podman),
the main entry-point for humans and automation is `make localsystem`.  When operating
from a packaged version of the system-tests, the entry-point may vary as appropriate.
Running the packaged system-tests assumes the version of Podman matches the test
version, and all standard dependencies are installed.


# Test Design and overview

System-tests should be high-level and user work-flow oriented.  For example, consider
how multiple Podman invocations would be used together by an end-user.  The set of
related commands should be considered a single test.  If one or more intermediate
commands fail, the test could still pass if the end-result is still achieved.


# *TODO*: List of needed System-tests

***Note***: Common operations (like `rm` and `rmi` for cleanup/reset)
have been omitted as they are verified by repeated implied use.

- [ ] pull, build, run, attach, commit, diff, inspect

  - Pull existing image from registry
  - Build new image FROM explicitly pulled image
  - Run built container in detached mode
  - Attach to running container, execute command to modify storage.
  - Commit running container to new image w/ changed ENV VAR
  - Verify attach + commit using diff
  - verify changed ENV VAR with inspect

- [ ] Implied pull, create, start, exec, log, stop, wait, rm

  - Create non-existing local image
  - start stopped container
  - exec simple command in running container
  - verify exec result with log
  - wait on running container
  - stop running container with 2 second timeout
  - verify wait in 4 seconds or less
  - verify stopped by rm **without** --force

- [ ] Implied pull, build, export, modify, import, tag, run, kill

  - Build from Dockerfile FROM non-existing local image
  - Export built container as tarball
  - Modify tarball contents
  - Import tarball
  - Tag imported image
  - Run imported image to confirm tarball modification, block on non-special signal
  - Kill can send non-TERM/KILL signal to container to exit
  - Confirm exit within timeout

- [ ] Container runlabel, exists, checkpoint, exists, restore, stop, prune

  - Using pre-existing remote image, start it with 'podman container runlabel --pull'
  - Run a named container that exits immediately
  - Confirm 'container exists' zero exit (both containers)
  - Checkpoint the running container
  - Confirm 'container exists' non-zero exit (runlabel container)
  - Confirm 'container exists' zero exit (named container)
  - Run 'container restore'
  - Confirm 'container exists' zero exit (both containers)
  - Stop container
  - Run 'container prune'
  - Confirm `podman ps -a` lists no containers


# TODO: List of commands to be combined into additional workflows above.

- podman-remote (workflow TBD)
- history
- image
- load
- mount
- pause
- pod
- port
- login, push, & logout (difficult, save for last)
- restart
- save
- search
- stats
- top
- umount, unmount
- unpause
- volume
- `--namespace`
- `--storage-driver`
