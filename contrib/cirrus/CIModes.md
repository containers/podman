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
  Also, by sticking to one conditional style, it's easier to reuse the YAML
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

By default cirrus will trigger task depending on the source changes.

It is implemented using the `only_if` field for the cirrus tasks, this logic
uses the following main rules:
 - Never skip on cron runs: `$CIRRUS_PR == ''`
 - Never skip when using the special `CI:ALL` title: `$CIRRUS_CHANGE_TITLE =~ '.*CI:ALL.*'`, see below.
 - Never skip when a danger file is changed, these files contain things that can
   affect any tasks so the code cannot skip it. It includes
   - `.cirrus.yml` (cirrus changes)
   - `Makefile` (make targets are used to trigger tests)
   - `contrib/cirrus/**` (cirrus scripts to run the tests)
   - `vendor/**` (dependency updates)
   - `test/tools/**` (test dependency code, i.e. ginkgo)
   - `hack/**` (contains scripts used by several tests)
   - `version/rawversion/*` (podman version changes, intended to ensure all release PRs test everything to not release known broken code)

After that, task-specific rules are added, check [.cirrus.yml](../../.cirrus.yml) for them.
Another common rule used there is `(changesInclude('**/*.go', '**/*.c', '**/*.h') && !changesIncludeOnly('test/**', 'pkg/machine/e2e/**'))`.
This rule defines the set of source code. Podman uses both go and c source code (including header files),
however as some tests are also using go code we manually exclude the test
directories from this list.

### Intended `[CI:ALL]` behavior:

As of June 2024, the default Cirrus CI setup skips tasks that it deems
unnecessary, such as running e2e or system tests on a doc-only PR (see
#23174). This string in a PR title forces all CI jobs to run.

### Intended `[CI:NEXT]` behavior:

If and only if the PR is in **draft-mode**, update Fedora CI VMs at runtime
to the latest packages available in the podman-next COPR repo.  These packages
represent primary podman dependencies, and are regularly built from their
upstream repos.  These are **runtime changes** only, and will not persist
or impact other PRs in any way.

The intent is to temporarily support testing of updates with the latest podman
code & tests.  To help prevent accidents, when the PR is not in draft-mode, the
presence of the magic string will cause VM-setup script to fail, until the magic
is removed.

**Note:** When changing the draft-status of PR, you will need to re-push a
commit-change before Cirrus-CI will notice the draft-status update (i.e.
pressing the re-run button **is not** good enough).
