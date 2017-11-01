% kpod(1) kpod - Simple management tool for pods and images
% Dan Walsh
# kpod "1" "September 2016" "kpod"
## NAME
kpod - Simple management tool for containers and images

## SYNOPSIS
**kpod** [*options*] COMMAND

# DESCRIPTION
kpod is a simple client only tool to help with debugging issues when daemons
such as CRI runtime and the kubelet are not responding or failing. A shared API
layer could be created to share code between the daemon and kpod. kpod does not
require any daemon running. kpod utilizes the same underlying components that
crio uses i.e. containers/image, container/storage, oci-runtime-tool/generate,
runc or any other OCI compatible runtime. kpod shares state with crio and so
has the capability to debug pods/images created by crio.

**kpod [GLOBAL OPTIONS]**

## GLOBAL OPTIONS

**--help, -h**
  Print usage statement

**--config value, -c**=**"config.file"**
   Path of a config file detailing container server configuration options

**--log-level**
   log messages above specified level: debug, info, warn, error (default), fatal or panic

**--root**=**value**
   Path to the root directory in which data, including images, is stored

**--runroot**=**value**
   Path to the 'run directory' where all state information is stored

**--runtime**=**value**
    Path to the OCI compatible binary used to run containers

**--storage-driver, -s**=**value**
   Select which storage driver is used to manage storage of images and containers (default is overlay)

**--storage-opt**=**value**
   Used to pass an option to the storage driver

**--version, -v**
  Print the version

## COMMANDS

### diff
Inspect changes on a container or image's filesystem

### export
Export container's filesystem contents as a tar archive

### history
Shows the history of an image

### images
List images in local storage

### info
Displays system information

### inspect
Display a container or image's configuration

### kill
Kill the main process in one or more containers

### load
Load an image from docker archive

### login
Login to a container registry

### logout
Logout of a container registry

### logs
Display the logs of a container

### mount
Mount a working container's root filesystem

### pause
Pause one or more containers

### ps
Prints out information about containers

### pull
Pull an image from a registry

### push
Push an image from local storage to elsewhere

### rename
Rename a container

### rm
Remove one or more containers

### rmi
Removes one or more locally stored images

### save
Save an image to docker-archive or oci

### stats
Display a live stream of one or more containers' resource usage statistics

### stop
Stops one or more running containers.

### tag
Add an additional name to a local image

### umount
Unmount a working container's root file system

### unpause
Unpause one or more containers

### version
Display the version information

### wait
Wait on one or more containers to stop and print their exit codes

## SEE ALSO
crio(8), crio.conf(5)

## HISTORY
Dec 2016, Originally compiled by Dan Walsh <dwalsh@redhat.com>
