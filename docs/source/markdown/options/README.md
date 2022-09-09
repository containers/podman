Common Man Page Options
=======================

This subdirectory contains option (flag) names and descriptions
common to multiple podman man pages. Each file is one option. The
filename does not necessarily need to be identical to the option
name: for instance, `hostname.container.md` and `hostname.pod.md`
exist because the **--hostname** option is sufficiently different
between `podman-{create,run}` and `podman-pod-{create,run}` to
warrant living separately.

How
===

The files here are included in `podman-*.md.in` files using the `@@option`
mechanism:

```
@@option foo           ! will include options/foo.md
```

The tool that does this is `hack/markdown-preprocess`. It is a python
script because it needs to run on `readthedocs.io`. From a given `.md.in`
file, this script will create a `.md` file that can then be read by
`go-md2man`, `sphinx`, anything that groks markdown. This runs as
part of `make docs`.

Special Substitutions
=====================

Some options are almost identical except for 'pod' vs 'container'
differences. For those, use `<<text for pods|text for containers>>`.
Order is immaterial: the important thing is the presence of the
string "`pod`" in one half but not the other. The correct string
will be chosen based on the filename: if the file contains `-pod`,
such as `podman-pod-create`, the string with `pod` (case-insensitive)
in it will be chosen.

The string `<<subcommand>>` will be replaced with the podman subcommand
as determined from the filename, e.g., `create` for `podman-create.1.md.in`.
This allows the shared use of examples in the option file:
```
    Example: podman <<subcommand>> --foo --bar
```
As a special case, `podman-pod-X` becomes just `X` (the "pod" is removed).
This makes the `pod-id-file` man page more useful. To get the full
subcommand including 'pod', use `<<fullsubcommand>>`.
