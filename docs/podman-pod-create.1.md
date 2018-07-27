% podman-pod-create "1"

## NAME
podman\-pod\-create - Create a new pod

## SYNOPSIS
**podman pod create** [*options*]

## DESCRIPTION

Creates an empty pod, or unit of multiple containers, and prepares it to have
containers added to it. The pod id is printed to STDOUT. You can then use
**podman create --pod <pod_id|pod_name> ...** to add containers to the pod, and
**podman pod start <pod_id|pod_name>** to start the pod.

## OPTIONS

**--cgroup-parent**=*true*|*false*

Path to cgroups under which the cgroup for the pod will be created. If the path is not absolute, the path is considered to be relative to the cgroups path of the init process. Cgroups will be created if they do not already exist.

**--podidfile**=""

Write the pod ID to the file

**--help**

Print usage statement

**-l**, **--label**=[]

Add metadata to a pod (e.g., --label com.example.key=value)

**--label-file**=[]

Read in a line delimited file of labels

**-n**, **--name**=""

Assign a name to the pod

**--pause**

Create a pause container and associate it with the pod. A pause container is a lightweight container used to coordinate the shared kernel namespace of a pod. Default: true

**--pause-command**=""

The command that will be run to start the pause container. Default: "/pause"

**--pause-image**=""

The image that will be created for the pause container. Default: "k8s.gcr.io/pause:3.1"

**--share**=""

A comma deliminated list of kernel namespaces to share. If none or "" is specified, no namespaces will be shared. The namespaces to choose from are ipc, net, pid, user, uts.

The operator can identify a pod in three ways:
UUID long identifier (“f78375b1c487e03c9438c729345e54db9d20cfa2ac1fc3494b6eb60872e74778”)
UUID short identifier (“f78375b1c487”)
Name (“jonah”)

podman generates a UUID for each pod, and if a name is not assigned
to the container with **--name** then a random string name will be generated
for it. The name is useful any place you need to identify a pod.

## EXAMPLES


# podman pod create --name test

# podman pod create --pause=false

# podman pod create --pause-command /top

## SEE ALSO
podman-pod(1)

## HISTORY
July 2018, Originally compiled by Peter Hunt <pehunt@redhat.com>
