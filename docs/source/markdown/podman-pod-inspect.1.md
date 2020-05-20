% podman-pod-inspect(1)

## NAME
podman\-pod\-inspect - Displays information describing a pod

## SYNOPSIS
**podman pod inspect** [*options*] *pod* ...

## DESCRIPTION
Displays configuration and state information about a given pod.  It also displays information about containers
that belong to the pod.

## OPTIONS
**--latest**, **-l**

Instead of providing the pod name or ID, use the last created pod. If you use methods other than Podman
to run pods such as CRI-O, the last started pod could be from either of those methods.

The latest option is not supported on the remote client.

**-f**, **--format**=*format*

Change the default output format.  This can be of a supported type like 'json'
or a Go template.
Valid placeholders for the Go template are listed below:

| **Placeholder**   | **Description**                                                               |
| ----------------- | ----------------------------------------------------------------------------- |
| .ID               | Pod   ID                                                                      |
| .Name             | Pod   name                                                                    |
| .State            | Pod   state                                                                   |
| .Hostname         | Pod   hostname                                                                |
| .Labels           | Pod   labels                                                                  |
| .Created          | Time when the pod was created                                                 |
| .CreateCgroup     | Whether cgroup was created                                                    |
| .CgroupParent     | Pod   cgroup parent                                                           |
| .CgroupPath       | Pod   cgroup path                                                             |
| .CreateInfra      | Whether infrastructure created                                                |
| .InfraContainerID | Pod   infrastructure ID                                                       |
| .SharedNamespaces | Pod   shared namespaces                                                       |
| .NumContainers    | Number of containers in the pod                                               |
| .Containers       | Pod   containers                                                              |

## EXAMPLE
```
# podman pod inspect foobar
{

     "Id": "3513ca70583dd7ef2bac83331350f6b6c47d7b4e526c908e49d89ebf720e4693",
     "Name": "foobar",
     "Labels": {},
     "CgroupParent": "/libpod_parent",
     "CreateCgroup": true,
     "Created": "2018-08-08T11:15:18.823115347-05:00"
     "State": "created",
     "Hostname": "",
     "SharedNamespaces": [
          "uts",
          "ipc",
          "net"
     ]
     "CreateInfra": false,
     "InfraContainerID": "1020dd70583dd7ff2bac83331350f6b6e007de0d026c908e49d89ebf891d4699"
     "CgroupPath": ""
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
