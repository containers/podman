% podman-network-exists(1)

## NAME
podman\-network\-exists - Check if the given network exists

## SYNOPSIS
**podman network exists** *network*

## DESCRIPTION
**podman network exists** checks if a network exists. The **Name** or **ID**
of the network may be used as input.  Podman will return an exit code
of `0` when the network is found.  A `1` will be returned otherwise. An exit code of
`125` indicates there was another issue.


## OPTIONS

#### **--help**, **-h**

Print usage statement

## EXAMPLE

Check if a network called `net1` exists (the network does actually exist).
```
$ podman network exists net1
$ echo $?
0
$
```

Check if an network called `webbackend` exists (the network does not actually exist).
```
$ podman network exists webbackend
$ echo $?
1
$
```

## SEE ALSO
podman(1), podman-network-create(1), podman-network-rm(1)

## HISTORY
January 2021, Originally compiled by Paul Holzinger `<paul.holzinger@web.de>`
