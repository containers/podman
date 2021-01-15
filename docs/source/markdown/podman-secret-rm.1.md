% podman-secret-rm(1)

## NAME
podman\-secret\-rm - Remove one or more secrets

## SYNOPSIS
**podman secret rm** [*options*] *secret* [...]

## DESCRIPTION

Removes one or more secrets.

## OPTIONS

#### **--all**, **-a**

Remove all existing secrets.

#### **--help**

Print usage statement.

## EXAMPLES

```
$ podman secret rm mysecret1 mysecret2
```

## SEE ALSO
podman-secret(1)

## HISTORY
January 2021, Originally compiled by Ashley Cui <acui@redhat.com>
