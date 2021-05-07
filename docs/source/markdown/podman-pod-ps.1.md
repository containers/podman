% podman-pod-ps(1)

## NAME
podman\-pod\-ps - Prints out information about pods

## SYNOPSIS
**podman pod ps** [*options*]

## DESCRIPTION
**podman pod ps** lists the pods on the system.
By default it lists:

 * pod id
 * pod name
 * number of containers attached to pod
 * status of pod as defined by the following table

|  **Status**  | **Description**                                 |
| ------------ | ------------------------------------------------|
| Created      | No containers running nor stopped               |
| Running      | at least one container is running               |
| Stopped      | At least one container stopped and none running |
| Exited       | All containers stopped in pod                   |
| Dead         | Error retrieving state                          |


## OPTIONS

#### **--ctr-names**

Includes the container names in the container info field

#### **--ctr-ids**

Includes the container IDs in the container info field

#### **--ctr-status**

Includes the container statuses in the container info field

#### **--latest**, **-l**

Show the latest pod created (all states) (This option is not available with the remote Podman client)

#### **--noheading**

Omit the table headings from the listing of pods.

#### **--no-trunc**

Display the extended information

#### **--ns**

Display namespace information of the pod

#### **--quiet**, **-q**

Print the numeric IDs of the pods only

#### **--format**=*format*

Pretty-print containers to JSON or using a Go template

Valid placeholders for the Go template are listed below:

|   **Placeholder**   | **Description**                                                                                 |
| ------------------- | ----------------------------------------------------------------------------------------------- |
| .ID                 | Container ID                                                                                    |
| .Name               | Name of pod                                                                                     |
| .Status             | Status of pod                                                                                   |
| .Labels             | All the labels assigned to the pod                                                              |
| .NumberOfContainers | Show the number of containers attached to pod                                                   |
| .Cgroup             | Cgroup path of pod                                                                              |
| .Created            | Creation time of pod                                                                            |
| .InfraID            | Pod infra container ID                                                                          |
| .Networks           | Show all networks connected to the infra container                                              |

#### **--sort**

Sort by created, ID, name, status, or number of containers

Default: created

#### **--filter**, **-f**=*filter*

Filter output based on conditions given.
Multiple filters can be given with multiple uses of the --filter flag.
Filters with the same key work inclusive with the only exception being
`label` which is exclusive. Filters with different keys always work exclusive.

Valid filters are listed below:

| **Filter** | **Description**                                                                       |
| ---------- | ------------------------------------------------------------------------------------- |
| id         | [ID] Pod's ID (accepts regex)                                                         |
| name       | [Name] Pod's name (accepts regex)                                                     |
| label      | [Key] or [Key=Value] Label assigned to a container                                    |
| status     | Pod's status: `stopped`, `running`, `paused`, `exited`, `dead`, `created`, `degraded` |
| network    | [Network] name or full ID of network                                                  |
| ctr-names  | Container name within the pod (accepts regex)                                         |
| ctr-ids    | Container ID within the pod (accepts regex)                                           |
| ctr-status | Container status within the pod                                                       |
| ctr-number | Number of containers in the pod                                                       |

#### **--help**, **-h**

Print usage statement

## EXAMPLES

```
$ podman pod ps
POD ID         NAME              STATUS    NUMBER OF CONTAINERS
00dfd6fa02c0   jolly_goldstine   Running   1
f4df8692e116   nifty_torvalds    Created   2
```

```
$ podman pod ps --ctr-names
POD ID         NAME              STATUS    CONTAINER INFO
00dfd6fa02c0   jolly_goldstine   Running   [ loving_archimedes ]
f4df8692e116   nifty_torvalds    Created   [ thirsty_hawking ] [ wizardly_golick ]
```

```
$ podman pod ps --ctr-status --ctr-names --ctr-ids
POD ID         NAME              STATUS    CONTAINER INFO
00dfd6fa02c0   jolly_goldstine   Running   [ ba465ab0a3a4 loving_archimedes Running ]
f4df8692e116   nifty_torvalds    Created   [ 331693bff40a thirsty_hawking Created ] [ 8e428daeb89e wizardly_golick Created ]
```

```
$ podman pod ps --format "{{.ID}}  {{.ContainerInfo}}  {{.Cgroup}}" --ctr-names
00dfd6fa02c0      [ loving_archimedes ]                         /libpod_parent
f4df8692e116      [ thirsty_hawking ] [ wizardly_golick ]       /libpod_parent
```

```
$ podman pod ps --cgroup
POD ID         NAME              STATUS    NUMBER OF CONTAINERS   CGROUP           USE POD CGROUP
00dfd6fa02c0   jolly_goldstine   Running   1                      /libpod_parent   true
f4df8692e116   nifty_torvalds    Created   2                      /libpod_parent   true
```

```
$ podman pod ps --sort id --filter ctr-number=2
POD ID         NAME             STATUS    NUMBER OF CONTAINERS
f4df8692e116   nifty_torvalds   Created   2
```

```
$ podman pod ps  --ctr-ids
POD ID         NAME              STATUS    CONTAINER INFO
00dfd6fa02c0   jolly_goldstine   Running   [ ba465ab0a3a4 ]
f4df8692e116   nifty_torvalds    Created   [ 331693bff40a ] [ 8e428daeb89e ]
```

```
$ podman pod ps --no-trunc --ctr-ids
POD ID                                                             NAME              STATUS    CONTAINER INFO
00dfd6fa02c0a2daaedfdf8fcecd06f22ad114d46d167d71777224735f701866   jolly_goldstine   Running   [ ba465ab0a3a4e15e3539a1e79c32d1213a02b0989371e274f98e0f1ae9de7050 ]
f4df8692e116a3e6d1d62572644ed36ca475d933808cc3c93435c45aa139314b   nifty_torvalds    Created   [ 331693bff40a0ef2f05a3aba73ce49e3243108911927fff04d1f7fc44dda8022 ] [ 8e428daeb89e69b71e7916a13accfb87d122889442b5c05c2d99cf94a3230e9d ]
```

```
$ podman pod ps --ctr-names
POD ID         NAME   STATUS    CONTAINER INFO
314f4da82d74   hi     Created   [ jovial_jackson ] [ hopeful_archimedes ] [ vibrant_ptolemy ] [ heuristic_jennings ] [ keen_raman ] [ hopeful_newton ] [ mystifying_bose ] [ silly_lalande ] [ serene_lichterman ] ...
```

## pod ps
Print a list of pods

## SEE ALSO
podman-pod(1)

## HISTORY
July 2018, Originally compiled by Peter Hunt <pehunt@redhat.com>
