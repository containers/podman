% podman-pod-ps "1"

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

## OPTIONS

**--ctr-names**

Includes the container names in the container info field

**--ctr-ids**

Includes the container IDs in the container info field

**--ctr-status**

Includes the container statuses in the container info field

**--no-trunc**

Display the extended information

**--quiet, -q**

Print the numeric IDs of the pods only

**--format**

Pretty-print containers to JSON or using a Go template

Valid placeholders for the Go template are listed below:

|   **Placeholder**   | **Description**                                     |
| ------------------- | --------------------------------------------------- |
| .ID                 | Container ID                                        |
| .Name               | Name of pod                                         |
| .Labels             | All the labels assigned to the pod                  |
| .ContainerInfo      | Show the names, ids and/or statuses of containers   |
| .NumberOfContainers | Show the number of containers attached to pod       |
| .Cgroup             | Cgroup path of pod                                  |
| .UsePodCgroup       | Whether containers use the Cgroup of the pod        |

**--sort**

Sort by ID, name, or number
Default: name

**--filter, -f**

Filter output based on conditions given

Valid filters are listed below:

| **Filter**      | **Description**                                                     |
| --------------- | ------------------------------------------------------------------- |
| id              | [ID] Pod's ID                                                       |
| name            | [Name] Pod's name                                                   |
| label           | [Key] or [Key=Value] Label assigned to a container                  |
| ctr-names       | Container name within the pod                                       |
| ctr-ids         | Container ID within the pod                                         |
| ctr-status      | Container status within the pod                                     |
| ctr-number      | Number of containers in the pod                                     |

**--help**, **-h**

Print usage statement

## EXAMPLES

```
sudo podman pod ps
POD ID         NAME               NUMBER OF CONTAINERS
817195e3eaf2   gallant_swartz     1
b9a2481261b3   heuristic_kilby    0
3e617011dfaa   peaceful_johnson   0
```

```
sudo podman pod ps --ctr-names
POD ID         NAME               NUMBER OF CONTAINERS   CONTAINER INFO
817195e3eaf2   gallant_swartz     1                      [ silly_jennings ]
b9a2481261b3   heuristic_kilby    0
3e617011dfaa   peaceful_johnson   0
```

```
podman pod ps --ctr-status --ctr-names --ctr-ids
POD ID         NAME                 CONTAINER INFO
817195e3eaf2   gallant_swartz       [ 12e4200bc803 eloquent_elion Running ] [ 5859146f7829 silly_jennings Exited ] [ fc27970f696c suspicious_mayer Exited ]
b9a2481261b3   heuristic_kilby
```

```
sudo podman pod ps --format "{{.ID}}  {{.ContainerInfo}}  {{.Cgroup}}" --ctr-names
817195e3eaf2     [ silly_jennings ]     /libpod_parent
b9a2481261b3                          /libpod_parent
3e617011dfaa                          /libpod_parent
```

```
sudo podman pod ps --cgroup
POD ID         NAME               NUMBER OF CONTAINERS   CGROUP           USE POD CGROUP
817195e3eaf2   gallant_swartz     1                      /libpod_parent   true
b9a2481261b3   heuristic_kilby    0                      /libpod_parent   true
3e617011dfaa   peaceful_johnson   0                      /libpod_parent   true
```

```
sudo podman pod ps --sort id --filter ctr-number=0
POD ID         NAME               NUMBER OF CONTAINERS
3e617011dfaa   peaceful_johnson   0
b9a2481261b3   heuristic_kilby    0
```

```
sudo podman pod ps  --ctr-ids
POD ID         NAME               NUMBER OF CONTAINERS   CONTAINER INFO
817195e3eaf2   gallant_swartz     1                      [ 5859146f7829 ]
b9a2481261b3   heuristic_kilby    0
3e617011dfaa   peaceful_johnson   0
```

```
sudo podman pod ps --no-trunc --ctr-ids
POD ID                                                             NAME               NUMBER OF CONTAINERS   CONTAINER INFO
817195e3eaf285f01744566da5b2d11216ff798c61a104d070827362ff08985d   gallant_swartz     1                      [ 5859146f78290e8b468ac1fa7ee7e521b8fdaccf66eac622950d176b9e2b76d6 ]
b9a2481261b3f1b871314df727d34c424251cb0fe8749f29ecb8b0c24e055385   heuristic_kilby    0
3e617011dfaa8b4044daf5ce6a64d27aeacc595c72a26a354601a21de46153da   peaceful_johnson   0
```

## pod ps
Print a list of pods

## SEE ALSO
podman-pod(1)

## HISTORY
August 2017, Originally compiled by Urvashi Mohnani <umohnani@redhat.com>
July 2018, Adapted from podman-ps-1 by Peter Hunt <pehunt@redhat.com>
