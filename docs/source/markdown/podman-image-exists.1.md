% podman-image-exists(1)

## NAME

podman-image-exists - Check if an image exists in local storage

## SYNOPSIS

**podman image exists** [*options*] *name*

## DESCRIPTION

**podman image exists** checks if an image exists in local storage. The *ID* or *name* of the image may be used as input.

Podman will return the following exit codes:

| Exit Code | Description |
|-----------|-------------|
| 0 | Image found. |
| 1 | Image not found. |
| 125 | Could not access local storage. |

## OPTIONS

#### **--help**, **-h**

Print usage statement.

## EXAMPLES

- Check if an image called `webclient` exists in local storage (the image does actually exist).

```
$ podman image exists webclient
$ echo $?
0
```

- Check if an image called `webbackend` exists in local storage (the image does not actually exist).

```
$ podman image exists webbackend
$ echo $?
1
```

## SEE ALSO

**[podman(1)](podman.1.md)**

## HISTORY

November 2018, Originally compiled by Brent Baude <bbaude@redhat.com>
