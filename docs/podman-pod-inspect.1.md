% podman-pod-inspect "1"

## NAME
podman\-pod\-inspect - Displays information describing a pod

## SYNOPSIS
**podman pod inspect** [*options*] *pod* ...

## DESCRIPTION
Displays configuration and state information about a given pod.  It also displays information about containers
that belong to the pod.

## OPTIONS
**--latest, -l**

Instead of providing the pod name or ID, use the last created pod. If you use methods other than Podman
to run pods such as CRI-O, the last started pod could be from either of those methods.


## EXAMPLE
```
# podman pod inspect foobar
{
     "Config": {
          "id": "3513ca70583dd7ef2bac83331350f6b6c47d7b4e526c908e49d89ebf720e4693",
          "name": "foobar",
          "labels": {},
          "cgroupParent": "/libpod_parent",
          "UsePodCgroup": true,
          "created": "2018-08-08T11:15:18.823115347-05:00"
     },
     "State": {
          "CgroupPath": ""
     },
     "Containers": [
          {
               "id": "d53f8bf1e9730281264aac6e6586e327429f62c704abea4b6afb5d8a2b2c9f2c",
               "state": "configured"
          }
     ]
}
```

## SEE ALSO
podman-pod(1), podman-pod-ps(1)

## HISTORY
August 2018, Originally compiled by Brent Baude <bbaude@redhat.com>
