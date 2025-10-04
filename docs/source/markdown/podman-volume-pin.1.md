% podman-volume-pin 1

## NAME
podman\-volume\-pin - Mark or unmark volumes as pinned

## SYNOPSIS
**podman volume pin** [*options*] *volume* [*volume* ...]

## DESCRIPTION

Mark or unmark one or more volumes as pinned. Pinned volumes are excluded from **podman system prune** and **podman system reset** operations by default, protecting them from accidental deletion.

This is useful for volumes containing important persistent data that should be preserved during cleanup operations.

By default, **podman volume pin** marks volumes as pinned. Use the **--unpin** option to remove the pinned status from volumes.

## OPTIONS

#### **--help**

Print usage statement.

#### **--unpin**

Remove the pinned status from the specified volumes instead of pinning them.

## EXAMPLES

Mark a volume as pinned.
```
$ podman volume pin myvol
Volume myvol is now pinned
```

Mark multiple volumes as pinned.
```
$ podman volume pin vol1 vol2 vol3
Volume vol1 is now pinned
Volume vol2 is now pinned
Volume vol3 is now pinned
```

Remove the pinned status from a volume.
```
$ podman volume pin --unpin myvol
Volume myvol is now unpinned
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-volume(1)](podman-volume.1.md)**, **[podman-volume-create(1)](podman-volume-create.1.md)**, **[podman-volume-prune(1)](podman-volume-prune.1.md)**, **[podman-volume-rm(1)](podman-volume-rm.1.md)**, **[podman-system-prune(1)](podman-system-prune.1.md)**, **[podman-system-reset(1)](podman-system-reset.1.md)**

## HISTORY
October 2025, Originally compiled by TobWen
