% podman-version "1"

## NAME
podman\-version - Display the PODMAN Version Information

## SYNOPSIS
**podman version** [*options*]

## DESCRIPTION
Shows the following information: Version, Go Version, Git Commit, Build Time,
OS, and Architecture.

## OPTIONS

**--help**, **-h**

Print usage statement

**--format**

Change output format to "json" or a Go template.

## Example

A sample output of the `version` command:
```
$ podman version
Version:       0.11.1
Go Version:    go1.11
Git Commit:    "8967a1d691ed44896b81ad48c863033f23c65eb0-dirty"
Built:         Thu Nov  8 22:35:40 2018
OS/Arch:       linux/amd64
```

Filtering out only the version:
```
$ podman version --format '{{.Version}}'
0.11.2
```

## SEE ALSO
podman(1), crio(8)

## HISTORY
November 2018, Added --format flag by Tomas Tomecek <ttomecek@redhat.com>
July 2017, Originally compiled by Urvashi Mohnani <umohnani@redhat.com>
