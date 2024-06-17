# podman-stressor

A tool based on Podman, cgroupv2, and systemd that creates a cgroupv2 namespace with limited resources according to user input and starts 'N' containers to ensure Podman behaves correctly.

Additionally, it is possible to enable a stress feature that uses the stress-ng tool to stress each container in parallel and verify if the applications and services running in the container environment are stable and secure. This tool also ensures that containers do not exceed their resource limits, even in the presence of memory, CPU, or other system interferences.

Furthermore, it is an excellent way to test container scalability and interference between containers using Podman, allowing you to trigger 100, 1k, 5k, or even 1 million containers.

Why bash?

We just call existing programs, no need to add extra layer of python, golang or rust for calling program at this moment.

# ASCII Diagram
```
+-------------------------------------------------------+
|                      User Space                       |
|                                                       |
|  +-----------------------+                            |
|  |    podman-stressor    |                            |
|  |    (Bash Script)      |                            |
|  +-----------+-----------+                            |
|              |                                        |
|              v                                        |
|  +-----------------------+                            |
|  |      systemd-run      |                            |
|  |      (CLI Command)    |                            |
|  +-----------+-----------+                            |
|              |                                        |
|              v                                        |
|  +-----------------------+                            |
|  |        systemd        |                            |
|  |   (Service Manager)   |                            |
|  +-----------+-----------+                            |
|              |                                        |
|              v                                        |
|  +---------------------------------+                  |
|  |         Slice                   |                  |
|  |    (e.g., podman-stressor.slice)|                  |
|  +-----------+---------------------+                  |
|              |                                        |
|              v                                        |
|  +-----------------------+                            |
|  |       cgroupv2        |                            |
|  |    (Control Groups)   |                            |
|  +-----------+-----------+                            |
|              |                                        |
|              v                                        |
|  +-----------------------+                            |
|  |        podman         |                            |
|  |   (Container Engine)  |                            |
|  +-----------+-----------+                            |
|              |                                        |
|              v                                        |
|  +-----------------------+                            |
|  |     Optional:         |                            |
|  |      stress-ng        |                            |
|  | (Stress Testing Tool) |                            |
|  +-----------+-----------+                            |
|              |                                        |
|              v                                        |
|  +-----------------------+                            |
|  |        glibc          |                            |
|  |  (C Library for       |                            |
|  |   System Calls)       |                            |
|  +-----------+-----------+                            |
|              |                                        |
|              v                                        |
+--------------+----------------------------------------+
|                      Kernel Space                     |
|                                                       |
|  +-----------------------+                            |
|  |        Kernel         |                            |
|  |    (Linux Kernel)     |                            |
|  +-----------+-----------+                            |
|              ^                                        |
|              |                                        |
|  +-----------------------+                            |
|  |       cgroupv2        |                            |
|  |    (Kernel Part)      |                            |
|  +-----------------------+                            |
|                                                       |
+-------------------------------------------------------+
```

# How to install and run it?

## git clone
Clone the repo and install via make

```bash
$ git clone https://github.com/dougsland/podman-stressor
$ pushd podman-stressor
    $ sudo make install
    Installation complete.
$ popd

$ sudo podman-stressor
```

# Monitoring resources

Option 1: Using watch + systemd-cgls (cgroup ls) on the host:
```
watch -n 1 \
    "systemd-cgls /sys/fs/cgroup/podman_stressor.slice"
```

Option 2:
Use systemd-cgtop to monitor resource usage of slices and services (on the host):

```
systemd-cgtop
```

# Running

Let's start with an example below:

*"Create a Podman network named my_network, a volume named my_volume, and 200 containers using the automotive-osbuild image with the image command sleep 3600."*

```
sudo CLEANUP=true \
     NETWORK_NAME="my_network" \
     VOLUME_NAME="my_volume" \
     IMAGE_NAME="quay.io/centos-sig-automotive/automotive-osbuild" \
     IMAGE_COMMAND="sleep 3600" \
     NUMBER_OF_CONTAINERS="200" \
     ./podman-stressor
```

# Stressing mode

How to run in the stress mode?

Add the **STRESS_** env variables as showed below.

```
sudo CLEANUP=false \
     VERBOSE=true \
     LIST_CURRENT_STATE=true \
     STRESS_CPU="6" \
     STRESS_DISK="8" \
     STRESS_DISK_SIZE="1G" \
     STRESS_MEMORY="1G" \
     STRESS_VM_STRESSOR_INSTANCES=100 \
     STRESS_TIME="60s" \
     NETWORK_NAME="my_network" \
     VOLUME_NAME="my_volume" \
     IMAGE_NAME="quay.io/centos-sig-automotive/automotive-osbuild" \
     IMAGE_COMMAND="sleep 3600" \
     NUMBER_OF_CONTAINERS="100" \
     ./podman-stressor
```

Output:
```
<snip>
[ PASS ] All containers requested are running successfully.
[ INFO ] Running stress-ng in container test_container_100...
stress-ng: info:  [48] setting to a 1 min, 0 secs run per stressor
stress-ng: info:  [48] dispatching hogs: 100 vm, 6 cpu, 8 hdd
stress-ng: info:  [52] setting to a 1 min, 0 secs run per stressor
stress-ng: info:  [52] dispatching hogs: 100 vm, 6 cpu, 8 hdd
stress-ng: info:  [45] setting to a 1 min, 0 secs run per stressor
stress-ng: info:  [45] dispatching hogs: 100 vm, 6 cpu, 8 hdd
stress-ng: info:  [48] skipped: 0
stress-ng: info:  [48] passed: 114: vm (100) cpu (6) hdd (8)
stress-ng: info:  [48] failed: 0
stress-ng: info:  [48] metrics untrustworthy: 0
stress-ng: info:  [48] successful run completed in 1 min, 22.14 secs
stress-ng: info:  [52] skipped: 0
stress-ng: info:  [52] passed: 114: vm (100) cpu (6) hdd (8)
stress-ng: info:  [52] failed: 0
stress-ng: info:  [52] metrics untrustworthy: 0
stress-ng: info:  [52] successful run completed in 1 min, 22.27 secs
stress-ng: info:  [45] skipped: 0
stress-ng: info:  [45] passed: 114: vm (100) cpu (6) hdd (8)
stress-ng: info:  [45] failed: 0
stress-ng: info:  [45] metrics untrustworthy: 0
stress-ng: info:  [45] successful run completed in 1 min, 22.49 secs
<snip>
```

# Other examples

*"Create a Podman network named my_network, a volume named my_volume, and 100 containers using the alpine image with the image command sleep 3600. List current network, storage and verbose mode. Once created, remove everything, as this setup is just for testing the environment!"*

```
sudo CLEANUP=true \
     VERBOSE=true \
     LIST_CURRENT_STATE=true \
     NETWORK_NAME="my_network" \
     VOLUME_NAME="my_volume" \
     IMAGE_NAME="alpine" \
     IMAGE_COMMAND="sleep 3600" \
     NUMBER_OF_CONTAINERS="100" \
     ./podman-stressor
```

**Still interested to continue reading about?**

See the output for a PASS test (no VERBOSE mode or LIST_CURRENT_STATE):
```
sudo CLEANUP=false \
     NETWORK_NAME="my_network" \
     VOLUME_NAME="my_volume" \
     IMAGE_NAME="alpine" \
     IMAGE_COMMAND="sleep 3600" \
     NUMBER_OF_CONTAINERS="100" \
     ./podman-stressor
```

Output
```
[ PASS ] volume my_volume created.
[ PASS ] network my_network created.
[ PASS ] All containers requested are running successfully.
[ PASS ] Total number of containers created in parallel: 100
[ PASS ] Time taken: 1 seconds.

[ PASS ] All tests passed.
```

Checking if really worked:
```
$ podman ps
CONTAINER ID  IMAGE                            COMMAND     CREATED        STATUS        PORTS       NAMES
e7c04505c83d  docker.io/library/alpine:latest  sleep 3600  2 seconds ago  Up 2 seconds              test_container_1
abb513b64cd5  docker.io/library/alpine:latest  sleep 3600  2 seconds ago  Up 2 seconds              test_container_2
....
```

Checking volume and network created:
```
$ podman volume ls | grep my_volume
local       my_volume

$ podman network ls | grep my_network
1a33f12e7eee  my_network  bridge
```

Output for a FAIL test:
```
sudo CLEANUP=false \
     NETWORK_NAME="my_network" \
     VOLUME_NAME="my_volume" \
     IMAGE_NAME="alpine" \
     IMAGE_COMMAND="sleep 3600" \
     NUMBER_OF_CONTAINERS="100" \
     ./podman-stressor

Error: volume with name my_volume already exists: volume already exists
[ FAIL ] unable to create volume my_volume.
```

Let's get an output from a more verbose mode (VERBOSE=true plus LIST_CURRENT_STATE=true):

```
sudo CLEANUP=true \
     VERBOSE=true \
     LIST_CURRENT_STATE=true \
     NETWORK_NAME="my_network" \
     VOLUME_NAME="my_volume" \
     IMAGE_NAME="alpine" \
     IMAGE_COMMAND="sleep 3600" \
     NUMBER_OF_CONTAINERS="3" \
     ./podman-stressor

[ INFO ] =======================================================
[ INFO ] VERBOSE MODE IS ON
[ INFO ] =======================================================
[ INFO ] NETWORK_NAME is my_network
[ INFO ] VOLUME_NAME is my_volume
[ INFO ] NUMBER_OF_CONTAINERS is 100
[ INFO ] IMAGE_NAME is alpine
[ INFO ] IMAGE_COMMAND is sleep 3600
[ INFO ] LIST_CURRENT_STATE is set

[ INFO ] ===============================================
[ INFO ]              Listing current podman processes
[ INFO ] ===============================================
[ INFO ] CONTAINER ID  IMAGE       COMMAND     CREATED     STATUS      PORTS       NAMES
[ INFO ] ===============================================

[ INFO ] ===============================================
[ INFO ]              Listing current podman volume
[ INFO ] ===============================================
[ INFO ] DRIVER      VOLUME NAME
[ INFO ] local       test
[ INFO ] local       super
[ INFO ] local       dogz
[ INFO ] local       medogz
[ INFO ] ===============================================

[ INFO ] ===============================================
[ INFO ]              Listing current podman network
[ INFO ] ===============================================
[ INFO ] NETWORK ID    NAME        DRIVER
[ INFO ] 41af11b0f3d5  netcow      bridge
[ INFO ] 2f259bab93aa  podman      bridge
[ INFO ] ===============================================
[ INFO ] creating volume my_volume
[ PASS ] volume my_volume created.
[ INFO ] creating network my_network
[ PASS ] network my_network created.
[ INFO ] creating container test_container_1
[ INFO ] creating container test_container_2
...
[ PASS ] All containers requested are running successfully.
[ PASS ] Total number of containers created in parallel: 100
[ PASS ] Time taken: 1 seconds.

[ INFO ] ===============================================
[ INFO ]              Listing current podman processes
[ INFO ] ===============================================
[ INFO ] CONTAINER ID  IMAGE                            COMMAND     CREATED                 STATUS                 PORTS       NAMES
[ INFO ] 1012e9a1e865  docker.io/library/alpine:latest  sleep 3600  Less than a second ago  Up Less than a second              test_container_2
[ INFO ] 1ee043d0a2ed  docker.io/library/alpine:latest  sleep 3600  Less than a second ago  Up Less than a second              test_container_1
....
[ INFO ] ===============================================

[ INFO ] ===============================================
[ INFO ]              Listing current podman volume
[ INFO ] ===============================================
[ INFO ] DRIVER      VOLUME NAME
[ INFO ] local       my_volume
[ INFO ] local       test
[ INFO ] local       super
[ INFO ] local       dogz
[ INFO ] local       medogz
[ INFO ] ===============================================

[ INFO ] ===============================================
[ INFO ]              Listing current podman network
[ INFO ] ===============================================
[ INFO ] NETWORK ID    NAME        DRIVER
[ INFO ] 2a2543c7b7d9  my_network  bridge
[ INFO ] 41af11b0f3d5  netcow      bridge
[ INFO ] 2f259bab93aa  podman      bridge
[ INFO ] ===============================================

[ PASS ] All tests passed.
```

# Building rpm

Use rpmbuild tool to create an rpm package.

```bash
$ git clone https://github.com/dougsland/podman-stressor
$ mv podman-stress podman-stress-0.1.0
$ tar cvz -f v0.1.0.tar.gz podman-stress-0.1.0
$ cp v0.1.0.tar.gz ~/rpmbuild/SOURCES/
$ cd podman-stressor
$ rpmbuild -ba podman-stressor.spec
```

# Additional help
## How to enable cgroup v2?
```
sudo grubby --update-kernel=ALL --args="systemd.unified_cgroup_hierarchy=1"
```

Reboot the system for the changes to take effect.

## Configuring sudo without asking password
To configure sudo to not ask for a password, you need to modify the /etc/sudoers file.
This can be done using the visudo command, which safely edits the sudoers file.

```bash
sudo visudo
```

Add the following line to the file:
```bash
username_to_run_script_here ALL=(ALL) NOPASSWD: ALL
```

# Behind the scenes

As podman-stressor is based on cgroupv2 and systemd lets share some
common knowledge.

In systemd and cgroup v2 terminology, a "slice" is a unit of resource
management that groups together related services or scopes. When you
create a slice using systemd, it also creates the corresponding cgroup
hierarchy under the cgroup v2 filesystem.

List Processes in the Podman Stressor Slice
```
systemd-cgls /sys/fs/cgroup/podman_stressor.slice
Directory /sys/fs/cgroup/podman_stressor.slice:
├─run-r809b4252c4834592933b123056e6254d.scope …
│ └─1944002 /usr/bin/conmon --api-version 1 -c be7879833ce654481fe90c1abd3ccd1b3b2ed93f6974494cf43cff92a0>
├─run-r6851c76c5ca74580ba2353b7f6b7df0d.scope …
│ └─1944165 /usr/bin/conmon --api-version 1 -c 514c304c5c66481a4a05ab0c8366635dc98400a9d8cfa5c58ace33e5dc>
└─run-r25ae5d9b81fd43a79fbc203bbea2ad59.scope …
  └─1944087 /usr/bin/conmon --api-version 1 -c 76b2df670d9536ff7d145c98607627f941e1dd59a910d5516350ef2e70>
```
