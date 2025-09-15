% podman-ps 1

## NAME
podman\-ps - Print out information about containers

## SYNOPSIS
**podman ps** [*options*]

**podman container ps** [*options*]

**podman container list** [*options*]

**podman container ls** [*options*]

## DESCRIPTION
**podman ps** lists the running containers on the system. Use the **--all** flag to view
all the containers information.  By default it lists:

 * container id
 * the name of the image the container is using
 * the COMMAND the container is executing
 * the time the container was created
 * the status of the container
 * port mappings the container is using
 * alternative names for the container

## OPTIONS

#### **--all**, **-a**

Show all the containers, default is only running containers.

Note: Podman shares containers storage with other tools such as Buildah and CRI-O. In some cases these `external` containers might also exist in the same storage. Use the `--external` option to see these external containers. External containers show the 'storage' status.

#### **--external**

Display external containers that are not controlled by Podman but are stored in containers storage.  These external containers are generally created via other container technology such as Buildah or CRI-O and may depend on the same container images that Podman is also using.  External containers are denoted with either a 'buildah' or 'storage' in the COMMAND and STATUS column of the ps output.

#### **--filter**, **-f**

Filter what containers are shown in the output.
Multiple filters can be given with multiple uses of the --filter flag.
Filters with the same key work inclusive with the only exception being
`label` which is exclusive. Filters with different keys always work exclusive.

Valid filters are listed below:

| **Filter** | **Description**                                                                                 |
|------------|-------------------------------------------------------------------------------------------------|
| id         | [ID] Container's ID (CID prefix match by default; accepts regex)                                |
| name       | [Name] Container's name (accepts regex)                                                         |
| label      | [Key] or [Key=Value] Label assigned to a container                                              |
| label!     | [Key] or [Key=Value] Label NOT assigned to a container                                          |
| exited     | [Int] Container's exit code                                                                     |
| status     | [Status] Container's status: 'created', 'initialized', 'exited', 'paused', 'running', 'unknown' |
| ancestor   | [ImageName] Image or descendant used to create container (accepts regex)                        |
| before     | [ID] or [Name] Containers created before this container                                         |
| since      | [ID] or [Name] Containers created since this container                                          |
| volume     | [VolumeName] or [MountpointDestination] Volume mounted in container                             |
| health     | [Status] healthy or unhealthy                                                                   |
| pod        | [Pod] name or full or partial ID of pod                                                         |
| network    | [Network] name or full ID of network                                                            |
| until      | [DateTime] container created before the given duration or time.                                 |
| command    | [Command] the command the container is executing, only argv[0] is taken  |

#### **--format**=*format*

Pretty-print containers to JSON or using a Go template

Valid placeholders for the Go template are listed below:

| **Placeholder**    | **Description**                              |
|--------------------|----------------------------------------------|
| .AutoRemove        | If true, containers are removed on exit      |
| .CIDFile           | Container ID File                            |
| .Command           | Quoted command used                          |
| .Created ...       | Creation time for container, Y-M-D H:M:S     |
| .CreatedAt         | Creation time for container (same as above)  |
| .CreatedHuman      | Creation time, relative                      |
| .ExitCode          | Container exit code                          |
| .Exited            | "true" if container has exited               |
| .ExitedAt          | Time (epoch seconds) that container exited   |
| .ExposedPorts ...  | Map of exposed ports on this container       |
| .ID                | Container ID                                 |
| .Image             | Image Name/ID                                |
| .ImageID           | Image ID                                     |
| .IsInfra           | "true" if infra container                    |
| .Label *string*    | Specified label of the container             |
| .Labels ...        | All the labels assigned to the container     |
| .Mounts            | Volumes mounted in the container             |
| .Names             | Name of container                            |
| .Networks          | Show all networks connected to the container |
| .Pid               | Process ID on host system                    |
| .Pod               | Pod the container is associated with (SHA)   |
| .PodName           | PodName of the container                     |
| .Ports             | Forwarded and exposed ports                  |
| .Restarts          | Display the container restart count          |
| .RunningFor        | Time elapsed since container was started     |
| .Size              | Size of container                            |
| .StartedAt         | Time (epoch seconds) the container started   |
| .State             | Human-friendly description of ctr state      |
| .Status            | Status of container                          |

#### **--help**, **-h**

Print usage statement

#### **--last**, **-n**

Print the n last created containers (all states)

#### **--latest**, **-l**

Show the latest container created (all states) (This option is not available with the remote Podman client, including Mac and Windows (excluding WSL2) machines)

#### **--namespace**, **--ns**

Display namespace information

#### **--no-trunc**

Do not truncate the output (default *false*).

#### **--noheading**

Omit the table headings from the listing of containers.

#### **--pod**, **-p**

Display the pods the containers are associated with

#### **--quiet**, **-q**

Print the numeric IDs of the containers only

#### **--size**, **-s**

Display the total file size

#### **--sort**=*created*

Sort by command, created, id, image, names, runningfor, size, or status",
Note: Choosing size sorts by size of rootFs, not alphabetically like the rest of the options

#### **--sync**

Force a sync of container state with the OCI runtime.
In some cases, a container's state in the runtime can become out of sync with Podman's state.
This updates Podman's state based on what the OCI runtime reports.
Forcibly syncing is much slower, but can resolve inconsistent state issues.

#### **--watch**, **-w**

Refresh the output with current containers on an interval in seconds.

## EXAMPLES

List running containers.
```
$ podman ps
CONTAINER ID  IMAGE                            COMMAND    CREATED        STATUS        PORTS                                                   NAMES
4089df24d4f3  docker.io/library/centos:latest  /bin/bash  2 minutes ago  Up 2 minutes  0.0.0.0:80->8080/tcp, 0.0.0.0:2000-2006->2000-2006/tcp  manyports
92f58933c28c  docker.io/library/centos:latest  /bin/bash  3 minutes ago  Up 3 minutes  192.168.99.100:1000-1006->1000-1006/tcp                 zen_sanderson
```

List all containers.
```
$ podman ps -a
CONTAINER ID   IMAGE         COMMAND         CREATED       STATUS                    PORTS     NAMES
02f65160e14ca  redis:alpine  "redis-server"  19 hours ago  Exited (-1) 19 hours ago  6379/tcp  k8s_podsandbox1-redis_podsandbox1_redhat.test.crio_redhat-test-crio_0
69ed779d8ef9f  redis:alpine  "redis-server"  25 hours ago  Created                   6379/tcp  k8s_container1_podsandbox1_redhat.test.crio_redhat-test-crio_1
```

List all containers including their size. Note: this can take longer since Podman needs to calculate the size from the file system.
```
$ podman ps -a -s
CONTAINER ID   IMAGE         COMMAND         CREATED       STATUS                    PORTS     NAMES                                                                  SIZE
02f65160e14ca  redis:alpine  "redis-server"  20 hours ago  Exited (-1) 20 hours ago  6379/tcp  k8s_podsandbox1-redis_podsandbox1_redhat.test.crio_redhat-test-crio_0  27.49 MB
69ed779d8ef9f  redis:alpine  "redis-server"  25 hours ago  Created                   6379/tcp  k8s_container1_podsandbox1_redhat.test.crio_redhat-test-crio_1         27.49 MB
```

List all containers, running or not, using a custom Go format.
```
$ podman ps -a --format "{{.ID}}  {{.Image}}  {{.Labels}}  {{.Mounts}}"
02f65160e14ca  redis:alpine  tier=backend  proc,tmpfs,devpts,shm,mqueue,sysfs,cgroup,/var/run/,/var/run/
69ed779d8ef9f  redis:alpine  batch=no,type=small  proc,tmpfs,devpts,shm,mqueue,sysfs,cgroup,/var/run/,/var/run/
```

List all containers and display their namespaces.
```
$ podman ps --ns -a
CONTAINER ID    NAMES                                                                   PID     CGROUP       IPC          MNT          NET          PIDNS        USER         UTS
3557d882a82e3   k8s_container2_podsandbox1_redhat.test.crio_redhat-test-crio_1          29910   4026531835   4026532585   4026532593   4026532508   4026532595   4026531837   4026532594
09564cdae0bec   k8s_container1_podsandbox1_redhat.test.crio_redhat-test-crio_1          29851   4026531835   4026532585   4026532590   4026532508   4026532592   4026531837   4026532591
a31ebbee9cee7   k8s_podsandbox1-redis_podsandbox1_redhat.test.crio_redhat-test-crio_0   29717   4026531835   4026532585   4026532587   4026532508   4026532589   4026531837   4026532588
```

List all containers including size sorted by names.
```
$ podman ps -a --size --sort names
CONTAINER ID   IMAGE         COMMAND         CREATED       STATUS                    PORTS     NAMES
69ed779d8ef9f  redis:alpine  "redis-server"  25 hours ago  Created                   6379/tcp  k8s_container1_podsandbox1_redhat.test.crio_redhat-test-crio_1
02f65160e14ca  redis:alpine  "redis-server"  19 hours ago  Exited (-1) 19 hours ago  6379/tcp  k8s_podsandbox1-redis_podsandbox1_redhat.test.crio_redhat-test-crio_0
```

List all external containers created by tools other than Podman.
```
$ podman ps --external -a
CONTAINER ID  IMAGE                             COMMAND  CREATED      STATUS  PORTS  NAMES
69ed779d8ef9f  redis:alpine  "redis-server"  25 hours ago  Created                   6379/tcp  k8s_container1_podsandbox1_redhat.test.crio_redhat-test-crio_1
38a8a78596f9  docker.io/library/busybox:latest  buildah  2 hours ago  storage        busybox-working-container
fd7b786b5c32  docker.io/library/alpine:latest   buildah  2 hours ago  storage        alpine-working-container
f78620804e00  scratch                           buildah  2 hours ago  storage        working-container
```

List containers with their associated pods.
```
$ podman ps --pod
CONTAINER ID  IMAGE                            COMMAND    CREATED        STATUS        PORTS     NAMES               POD ID        PODNAME
4089df24d4f3  docker.io/library/nginx:latest  nginx      2 minutes ago  Up 2 minutes  80/tcp    webserver           1234567890ab  web-pod
92f58933c28c  docker.io/library/redis:latest  redis      3 minutes ago  Up 3 minutes  6379/tcp  cache               1234567890ab  web-pod
a1b2c3d4e5f6  docker.io/library/centos:latest /bin/bash  1 minute ago   Up 1 minute             standalone-container
```

List all containers with pod information, including those not in pods.
```
$ podman ps -a --pod
CONTAINER ID  IMAGE                            COMMAND    CREATED        STATUS                    PORTS     NAMES                POD ID        PODNAME
4089df24d4f3  docker.io/library/nginx:latest  nginx      2 minutes ago  Up 2 minutes              80/tcp    webserver            1234567890ab  web-pod
92f58933c28c  docker.io/library/redis:latest  redis      3 minutes ago  Up 3 minutes              6379/tcp  cache                1234567890ab  web-pod
69ed779d8ef9f redis:alpine                     redis      25 hours ago   Exited (0) 25 hours ago   6379/tcp  old-cache            5678901234cd  old-pod
a1b2c3d4e5f6  docker.io/library/centos:latest /bin/bash  1 minute ago   Up 1 minute                         standalone-container
```

Filter containers by pod name.
```
$ podman ps --filter pod=web-pod
CONTAINER ID  IMAGE                            COMMAND    CREATED        STATUS        PORTS     NAMES
4089df24d4f3  docker.io/library/nginx:latest  nginx      2 minutes ago  Up 2 minutes  80/tcp    webserver
92f58933c28c  docker.io/library/redis:latest  redis      3 minutes ago  Up 3 minutes  6379/tcp  cache
```
Filter containers by Status.
```
$ podman ps --filter status=running
CONTAINER ID  IMAGE                             COMMAND               CREATED         STATUS                  PORTS                           NAMES
ff660efda598  docker.io/library/nginx:latest    nginx -g daemon o...  3 minutes ago   Up 3 minutes            0.0.0.0:8080->80/tcp            webserver
5693e934f4c6  docker.io/library/redis:latest    redis-server          3 minutes ago   Up 3 minutes            6379/tcp                        cache
2b271d67dbb6                                                          3 minutes ago   Up 3 minutes            0.0.0.0:9090->80/tcp            463241862e7e-infra
23f99674da1c  docker.io/library/nginx:latest    nginx -g daemon o...  3 minutes ago   Up 3 minutes            0.0.0.0:9090->80/tcp            pod-nginx
62180adfbd42  docker.io/library/redis:latest    redis-server          3 minutes ago   Up 3 minutes            0.0.0.0:9090->80/tcp, 6379/tcp  pod-redis
5e3694604817  quay.io/centos/centos:latest      sleep 300             3 minutes ago   Up 3 minutes                                            centos-test
af3d8b3f5471  docker.io/library/busybox:latest  sleep 1000            3 minutes ago   Up 3 minutes                                            test-dev
b6ee47492b64  docker.io/library/nginx:latest    nginx -g daemon o...  3 minutes ago   Up 3 minutes (healthy)  80/tcp                          healthy-nginx
db75e6c397db  docker.io/library/busybox:latest  sleep 300             23 seconds ago  Up 23 seconds                                         test-volume-container
```
```
$ podman ps -a --filter status=exited
CONTAINER ID  IMAGE                            COMMAND     CREATED        STATUS                    PORTS                NAMES
94f211cc6e36  docker.io/library/alpine:latest  sleep 1     4 minutes ago  Exited (0) 4 minutes ago                       old-alpine
75a800cb848a  docker.io/library/mysql:latest   mysqld      3 minutes ago  Exited (1) 3 minutes ago  3306/tcp, 33060/tcp  db-container
2d9b3a94e31e  docker.io/library/nginx:latest   nginx       3 minutes ago  Exited (0) 3 minutes ago  80/tcp               nginx-cmd
```

Filter containers by name.
```
$ podman ps --filter name=webserver
CONTAINER ID  IMAGE                           COMMAND               CREATED        STATUS        PORTS                 NAMES
ff660efda598  docker.io/library/nginx:latest  nginx -g daemon o...  3 minutes ago  Up 3 minutes  0.0.0.0:8080->80/tcp  webserver
```

Filter containers by label.
```
$ podman ps --filter label=app=frontend
CONTAINER ID  IMAGE                           COMMAND               CREATED        STATUS        PORTS                 NAMES
ff660efda598  docker.io/library/nginx:latest  nginx -g daemon o...  3 minutes ago  Up 3 minutes  0.0.0.0:8080->80/tcp  webserver
```
Filter containers by volume.
```
$ podman ps --filter volume=mydata
CONTAINER ID  IMAGE                             COMMAND     CREATED         STATUS         PORTS       NAMES
db75e6c397db  docker.io/library/busybox:latest  sleep 300   42 seconds ago  Up 42 seconds              test-volume-container
```

Filter containers by network.
```
$ podman ps --filter network=web-net
CONTAINER ID  IMAGE                         COMMAND     CREATED        STATUS        PORTS       NAMES
5e3694604817  quay.io/centos/centos:latest  sleep 300   3 minutes ago  Up 3 minutes              centos-test
```

Use custom format to show container and pod information.
```
$ podman ps --format "{{.Names}} is in pod {{.PodName}} ({{.Pod}})"
webserver is in pod web-pod (1234567890ab)
cache is in pod web-pod (1234567890ab)
standalone-container is in pod  ()
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[buildah(1)](https://github.com/containers/buildah/blob/main/docs/buildah.1.md)**, **[crio(8)](https://github.com/cri-o/cri-o/blob/main/docs/crio.8.md)**

## HISTORY
August 2017, Originally compiled by Urvashi Mohnani <umohnani@redhat.com>
