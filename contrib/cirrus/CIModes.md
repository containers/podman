The following is a list (incomplete) of the primary contexts and runtime
"modes" supported by podman CI.  Note that there may be additional checks
done regarding "skipping work" in the `runner.sh` script.  This document
only details the controls at the `.cirrus.yml` level.

## Visualization

The relationship between tasks can be incredibly hard to understand by
staring at the YAML.
[A tool exists](https://github.com/containers/automation/tree/main/cirrus-task-map)
for producing a graph (flow-chart) of the `.cirrus.yml` file.  A (possibly
outdated) example of it's output can be seen below:

![cirrus-task-map output](https://github.com/containers/podman/wiki/cirrus-map.svg)

## Implementation notes

+ The `skip` conditional should never be used for tasks.
  While it's arguably easier to read that `only_if`, it leads to a cluttered
  status output that's harder to page through when reviewing PRs.  As opposed
  to `only_if` which will bypass creation of the task (at runtime) completely.
  Also, by sticking to one conditional style, it's easer to re-use the YAML
  statements across multiple tasks.

+ The only variables which can be used as part of conditions are defined by
  Cirrus-CI.
  [The list is documented](https://cirrus-ci.org/guide/writing-tasks/#environment-variables).  Reference to any variables defined in YAML will **not** behave how
  you expect, don't use them!

* Some Cirrus-CI defined variables contain non-empty values outside their
  obvious context. For example, when running for a PR a task will have
  `$CIRRUS_BRANCH` set to `pull/<number>`.

* Conditions which use positive or negative regular-expressions have several
  "flags" set: "Multi-line" and "Case-insensitive".

## Testing

Executing most of the modes can be mocked by forcing values for (otherwise)
Cirrus-CI defined variables.  For example `$CIRRUS_TAG`.  As of the publishing
of this document, it's not possible to override the behavior of `$CIRRUS_PR`.

## Cirrus Task contexts and runtime modes

### Intended general PR Tasks (*italic*: matrix)
+ *build*
+ validate
+ bindings
+ swagger
+ *alt_build*
+ osx_alt_build
+ docker-py_test
+ *unit_test*
+ apiv2_test
+ *compose_test*
+ *local_integration_test*
+ *remote_integration_test*
+ *container_integration_test*
+ *rootless_integration_test*
+ *local_system_test*
+ *remote_system_test*
+ *rootless_remote_system_test*
+ *buildah_bud_test*
+ *rootless_system_test*
+ rootless_gitlab_test
+ *upgrade_test*
+ meta
+ success
+ artifacts

### Intended for PR w/ "release" or "bump" in title:
+ (All the general PR tasks above)
+ release_test

### Intended `[CI:DOCS]` PR Tasks:
+ *build*
+ validate
+ swagger
+ meta
+ success

### Intended `[CI:COPR]` PR Tasks:
+ *build*
+ validate
+ swagger
+ meta
+ success

### Intend `[CI:BUILD]` PR Tasks:
+ *build*
+ validate
+ *alt_build*
+ osx_alt_build
+ test_image_build
+ meta
+ success
+ artifacts

### Intended Branch tasks (and Cirrus-cron jobs, except "multiarch"):
+ *build*
+ swagger
+ *alt_build*
+ osx_alt_build
+ *local_system_test*
+ *remote_system_test*
+ *rootless_remote_system_test*
+ *rootless_system_test*
+ meta
+ success
+ artifacts

### Intended for "multiarch" Cirrus-Cron (always a branch):
+ image_build
+ meta
+ success

### Intended for new Tag tasks:
+ *build*
+ swagger
+ *alt_build*
+ osx_alt_build
+ meta
+ success
+ artifacts
+ release
