% podman-rename 1

## NAME
podman\-rename - Rename an existing container or volume

## SYNOPSIS
**podman rename** *container|volume* *newname*

**podman container rename** *container* *newname*

**podman volume rename** *volume* *newname*

## DESCRIPTION
Rename changes the name of an existing container or volume.
The old name is freed, and is available for use.
For containers, this command can be run in any state.
However, running containers may not fully receive the effects until they are restarted - for example, a running container may still use the old name in its logs.
Use **podman container rename** to rename containers only, and **podman volume rename** to rename volumes only.
At present, only containers and volumes are supported; pods cannot be renamed.

## OPTIONS

## EXAMPLES

Rename container with a given name.
```
$ podman rename oldContainer aNewName
```

Rename container with a given ID.
```
$ podman rename 717716c00a6b testcontainer
```

Create an alias for container with a given ID.
```
$ podman container rename 6e7514b47180 databaseCtr
```

Rename volume with a given name.
```
$ podman rename oldVolume aNewName
```

Rename volume with the volume-specific command.
```
$ podman volume rename oldVolume aNewName
```

## SEE ALSO
**[podman(1)](podman.1.md)**
