% podman-rename(1)

## NAME
podman\-rename - Rename an existing container

## SYNOPSIS
**podman rename** *container* *newname*

**podman container rename** *container* *newname*

## DESCRIPTION
Rename changes the name of an existing container.
The old name will be freed, and will be available for use.
This command can be run on containers in any state.
However, running containers may not fully receive the effects until they are restarted - for example, a running container may still use the old name in its logs.
At present, only containers are supported; pods and volumes cannot be renamed.

## OPTIONS

## EXAMPLES

```
# Rename a container by name
$ podman rename oldContainer aNewName
```

```
# Rename a container by ID
$ podman rename 717716c00a6b testcontainer
```

```
# Use the container rename alias
$ podman container rename 6e7514b47180 databaseCtr
```

## SEE ALSO
**[podman(1)](podman.1.md)**
