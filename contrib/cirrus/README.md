![PODMAN logo](../../logo/podman-logo-source.svg)

# Cirrus-CI

Similar to other integrated github CI/CD services, Cirrus utilizes a simple
YAML-based configuration/description file: ``.cirrus.yml``.  Ref: https://cirrus-ci.org/

## Workflow

All tasks execute in parallel, unless there are conditions or dependencies
which alter this behavior.  Within each task, each script executes in sequence,
so long as any previous script exited successfully.  The overall state of each
task (pass or fail) is set based on the exit status of the last script to execute.

### ``full_vm_testing`` Task

1. Unconditionally, spin up one VM per ``matrix: image_name`` item defined
   in ``.cirrus.yml``.  Once accessible, ``ssh`` into each VM and run the following
   scripts.

2. ``setup_environment.sh``: Configure root's ``.bash_profile``
   for all subsequent scripts (each run in a new shell).  Any
   distribution-specific environment variables are also defined
   here.  For example, setting tags/flags to use compiling.

3. ``verify_source.sh``: Perform per-distribution source
   verification, lint-checking, etc.  This acts as a minimal
   gate, blocking extended use of VMs when a PR's code or commits
   would otherwise not be accepted.  Should run for less than a minute.

4. ``unit_test.sh``: Execute unit-testing, as defined by the ``Makefile``.
   This should execute within 10-minutes, but often much faster.

5. ``integration_test.sh``: Execute integration-testing.  This is
   much more involved, and relies on access to external
   resources like container images and code from other repositories.
   Total execution time is capped at 2-hours (includes all the above)
   but this script normally completes in less than an hour.

### ``build_vm_images`` Task

1. When a PR is merged (``$CIRRUS_BRANCH`` == ``master``), run another
   round of the ``full_vm_testing`` task (above).

2. After confirming the tests all pass post-merge, spin up a special VM
   capable of communicating with the GCE API.  Once accessible, ``ssh`` into
   the special VM and run the following scripts.

3. ``setup_environment.sh``: Configure root's ``.bash_profile``
   for all subsequent scripts (each run in a new shell).  Any
   distribution-specific environment variables are also defined
   here.  For example, setting tags/flags to use compiling.

4. ``build_vm_images.sh``: Examine the merged PR's description on github.
   If it contains the magic string ``***CIRRUS: REBUILD IMAGES***``, then
   continue.  Otherwise display a message, take no further action, and
   exit successfully.  This prevents production of new VM images unless
   they are called for, thereby saving the cost of needlessly storing them.

5. If the magic string was found, utilize [the packer tool](http://packer.io/docs/)
   to produce new VM images.  Create a new VM from each base-image, connect
   to them with ``ssh``, and perform these steps as defined by the
   ``libpod_images.json`` file.

    1. Copy the current state of the repository into ``/tmp/libpod``.
    2. Execute distribution-specific scripts to prepare the image for
       use by the ``full_vm_testing`` task (above).
    3. If successful, shut down each VM and create a new GCE Image
       named after the base image and the commit sha of the merge.

***Note:*** The ``.cirrus.yml`` file must be manually updated with the new
images names, then the change sent in via a secondary pull-request.  This
ensures that all the ``full_vm_testing`` tasks can pass with the new images,
before subjecting all future PRs to them.  A workflow to automate this
process is described in comments at the end of the ``.cirrus.yml`` file.
