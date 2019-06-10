# Podman remote-client tutorial

## What is the remote-client

First and foremost, the remote-client is under heavy development.  We are adding new
commands and functions frequently.  We also are working on a rootless implementation that
does not require privileged users.

The purpose of the Podman remote-client is to allow users to interact with a Podman "backend"
while on a separate client.  The command line interface of the remote client is exactly the
same as the regular Podman commands with the exception of some flags being removed as they
do not apply to the remote-client.

## What you need
To use the remote-client, you will need a binary for your client and a Podman "backend"; hereafter
referred to as the Podman node.  In this context, a Podman node is a Linux system with Podman
installed on it and the varlink service activated.  You will also need to be able to ssh into this
system as a user with privileges to the varlink socket (more on this later).

## Building the remote client
At this time, the remote-client is not being packaged for any distribution.  It must be built from
source.  To set up your build environment, see [Installation notes](install.md) and follow the
section [Building from scratch](install.md#building-from-scratch).  Once you can successfully
build the regular Podman binary, you can now build the remote-client.
```
$ make podman-remote
```
Like building the regular Podman, the resulting binary will be in the *bin* directory.  This is the binary
you will run on the remote node later in the instructions.

## Setting up the remote and Podman nodes

To use the remote-client, you must perform some setup on both the remote and Podman nodes. In this case,
the remote node refers to where the remote-client is being run; and the Podman node refers to where
Podman and its storage reside.

### Podman node setup
The Podman node must have Podman (not the remote-client) installed as normal. If your system uses systemd,
then simply start the Podman varlink socket.
```
$ sudo systemctl start io.podman.socket
```

If your system cannot use systemd, then you can manually establish the varlink socket with the Podman
command:
```
$ sudo podman --log-level debug varlink --timeout 0  unix://run/podman/io.podman
```

### Required permissions
For now, the remote-client requires that you be able to run a privileged Podman and have privileged ssh
access to the remote system.  This limitation is being worked on.

### Remote node setup

#### Initiate an ssh session to the Podman node
To use the remote client, we must establish an ssh connection to the Podman server.  We will also use
that session to bind the remote varlink socket locally.

```
$ ssh -L 127.0.0.1:1234:/run/podman/io.podman root@remotehost
```
Note here we are binding the Podman socket to a local TCP socket on port 1234.

#### Running the remote client
With the ssh session established, we can now run the remote client in a different terminal window. You
must inform Podman where to look for the bound socket you created in the previous step using an
environment variable.

```
$ PODMAN_VARLINK_ADDRESS="tcp:127.0.0.1:1234" bin/podman-remote images
REPOSITORY                     TAG      IMAGE ID       CREATED         SIZE
docker.io/library/ubuntu       latest   47b19964fb50   2 weeks ago     90.7 MB
docker.io/library/alpine       latest   caf27325b298   3 weeks ago     5.8 MB
quay.io/cevich/gcloud_centos   latest   641dad61989a   5 weeks ago     489 MB
k8s.gcr.io/pause               3.1      da86e6ba6ca1   14 months ago   747 kB
```
