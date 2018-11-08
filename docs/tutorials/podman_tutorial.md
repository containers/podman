![PODMAN logo](../../logo/podman-logo-source.svg)

# Basic Setup and Use of Podman
Podman is a utility provided as part of the libpod library.  It can be used to create and maintain
containers. The following tutorial will teach you how to set up Podman and perform some basic
commands with Podman.

## Install Podman on Fedora from RPM Repositories
Fedora 27 and later provide Podman via the package manager.
```console
$ sudo dnf install -y podman
```

## Install Podman on Fedora from Source
Many of the basic components to run Podman are readily available from the Fedora RPM repositories.
In this section, we will help you install all the runtime and build dependencies for Podman,
acquire the source, and build it.

### Installing build and runtime dependencies
```console
$ sudo dnf install -y git runc libassuan-devel golang golang-github-cpuguy83-go-md2man glibc-static \
                                    gpgme-devel glib2-devel device-mapper-devel libseccomp-devel \
                                    atomic-registries iptables skopeo-containers containernetworking-cni \
                                    conmon
```
### Building and installing podman

First, configure a `GOPATH` (if you are using go1.8 or later, this defaults to `~/go`), then clone
and make libpod.

```console
$ export GOPATH=~/go
$ mkdir -p $GOPATH
$ git clone https://github.com/containers/libpod/ $GOPATH/src/github.com/containers/libpod
$ cd $GOPATH/src/github.com/containers/libpod
$ make
$ sudo make install PREFIX=/usr
```

You now have a working podman environment.  Jump to [Familiarizing yourself with Podman](#familiarizing-yourself-with-podman)
to begin using Podman.

## Install podman on Ubuntu

The default Ubuntu cloud image size will not allow for the following exercise to be done without increasing its
capacity.  Be sure to add at least 5GB to the image. Instructions to do this are outside the scope of this
tutorial. For this tutorial, the Ubuntu **artful-server-cloudimg** image was used.

### Installing build and runtime dependencies

#### Installing base packages
```console
$ sudo apt-get update
$ sudo apt-get install libdevmapper-dev libglib2.0-dev libgpgme11-dev golang libseccomp-dev \
                        go-md2man libprotobuf-dev libprotobuf-c0-dev libseccomp-dev python3-setuptools
```
#### Building and installing conmon
First, configure a `GOPATH` (if you are using go1.8 or later, this defaults to `~/go`), then clone
and make libpod.

```console
$ export GOPATH=~/go
$ mkdir -p $GOPATH
$ git clone https://github.com/kubernetes-sigs/cri-o $GOPATH/src/github.com/kubernetes-sigs/cri-o
$ cd $GOPATH/src/github.com/kubernetes-sigs/cri-o
$ mkdir bin
$ make bin/conmon
$ sudo install -D -m 755 bin/conmon /usr/libexec/podman/conmon
```
#### Adding required configuration files
```console
$ sudo mkdir -p /etc/containers
$ sudo curl https://raw.githubusercontent.com/projectatomic/registries/master/registries.fedora -o /etc/containers/registries.conf
$ sudo curl https://raw.githubusercontent.com/containers/skopeo/master/default-policy.json -o /etc/containers/policy.json
```
#### Installing CNI plugins
```console
$ git clone https://github.com/containernetworking/plugins.git $GOPATH/src/github.com/containernetworking/plugins
$ cd $GOPATH/src/github.com/containernetworking/plugins
$ ./build_linux.sh
$ sudo mkdir -p /usr/libexec/cni
$ sudo cp bin/* /usr/libexec/cni
```
#### Installing runc
```console
$ git clone https://github.com/opencontainers/runc.git $GOPATH/src/github.com/opencontainers/runc
$ cd $GOPATH/src/github.com/opencontainers/runc
$ make BUILDTAGS="seccomp"
$ sudo cp runc /usr/bin/runc
```

### Building and installing Podman
```console
$ git clone https://github.com/containers/libpod/ $GOPATH/src/github.com/containers/libpod
$ cd $GOPATH/src/github.com/containers/libpod
$ make
$ sudo make install PREFIX=/usr
```

## Familiarizing yourself with Podman

### Running a sample container
This sample container will run a very basic httpd server that serves only its index
page.
```console
$ sudo podman run -dt -e HTTPD_VAR_RUN=/var/run/httpd -e HTTPD_MAIN_CONF_D_PATH=/etc/httpd/conf.d \
                    -e HTTPD_MAIN_CONF_PATH=/etc/httpd/conf \
                    -e HTTPD_CONTAINER_SCRIPTS_PATH=/usr/share/container-scripts/httpd/ \
                    registry.fedoraproject.org/f27/httpd /usr/bin/run-httpd
```
Because the container is being run in detached mode, represented by the *-d* in the podman run command, podman
will print the container ID after it has run.

### Listing running containers
The Podman *ps* command is used to list creating and running containers.
```console
$ sudo podman ps
```

Note: If you add *-a* to the *ps* command, Podman will show all containers.
### Inspecting a running container
You can "inspect" a running container for metadata and details about itself.  We can even use
the inspect subcommand to see what IP address was assigned to the container.
```console
$ sudo podman inspect -l | grep IPAddress\":
        "IPAddress": "10.88.6.140",
```

Note: The -l is convenience arguement for **latest container**.  You can also use the container's ID instead
of -l.

### Testing the httpd server
Now that we have the IP address of the container, we can test the network communication between the host
operating system and the container using curl. The following command should display the index page of our
containerized httpd server.
```console
# curl http://<IP_address>:8080
```

### Viewing the container's logs
You can view the container's logs with Podman as well:
```console
$ sudo podman logs --latest
10.88.0.1 - - [07/Feb/2018:15:22:11 +0000] "GET / HTTP/1.1" 200 612 "-" "curl/7.55.1" "-"
10.88.0.1 - - [07/Feb/2018:15:22:30 +0000] "GET / HTTP/1.1" 200 612 "-" "curl/7.55.1" "-"
10.88.0.1 - - [07/Feb/2018:15:22:30 +0000] "GET / HTTP/1.1" 200 612 "-" "curl/7.55.1" "-"
10.88.0.1 - - [07/Feb/2018:15:22:31 +0000] "GET / HTTP/1.1" 200 612 "-" "curl/7.55.1" "-"
10.88.0.1 - - [07/Feb/2018:15:22:31 +0000] "GET / HTTP/1.1" 200 612 "-" "curl/7.55.1" "-"
```

### Viewing the container's pids
And you can observe the httpd pid in the container with *top*.
```console
$ sudo podman top <container_id>
  UID   PID  PPID  C STIME TTY          TIME CMD
    0 31873 31863  0 09:21 ?        00:00:00 nginx: master process nginx -g daemon off;
  101 31889 31873  0 09:21 ?        00:00:00 nginx: worker process
```

### Checkpointing the container
Checkpointing a container stops the container while writing the state of all processes in the container to disk.
With this a container can later be restored and continue running at exactly the same point in time as the
checkpoint. This capability requires CRIU 3.11 or later installed on the system.
To checkpoint the container use:
```console
$ sudo podman container checkpoint <container_id>
```

### Restoring the container
Restoring a container is only possible for a previously checkpointed container. The restored container will
continue to run at exactly the same point in time it was checkpointed.
To restore the container use:
```console
$ sudo podman container restore <container_id>
```

After being restored, the container will answer requests again as it did before checkpointing.
```console
# curl http://<IP_address>:8080
```

### Stopping the container
To stop the httpd container:
```console
$ sudo podman stop --latest
```
You can also check the status of one or more containers using the *ps* subcommand. In this case, we should
use the *-a* argument to list all containers.
```console
$ sudo podman ps -a
```

### Removing the container
To remove the httpd container:
```console
$ sudo podman rm --latest
```
You can verify the deletion of the container by running *podman ps -a*.

## Integration Tests
For more information on how to setup and run the integration tests in your environment, checkout the Integration Tests [README.md](../../test/README.md)

## More information

For more information on Podman and its subcommands, checkout the asciiart demos on the [README.md](../../README.md#commands)
page.
