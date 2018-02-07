![PODMAN logo](https://cdn.rawgit.com/kubernetes-incubator/cri-o/master/logo/crio-logo.svg)

# Basic Setup and Use of Podman
Podman is a utility provided as part of the libpod library.  It can be used to create and maintain
containers. The following tutorial will teach you how to set up Podman and perform some basic
commands with Podman.

## Install Podman on Fedora
Many of the basic components to run Podman are readily available from the Fedora RPM repositories; the only
exception is Podman itself.  In this section, we will help you install all the runtime and build dependencies
for Podman until an RPM becomes available.

### Installing build and runtime dependencies
```
$ sudo dnf install -y git runc libassuan-devel golang golang-github-cpuguy83-go-md2man glibc-static \
                                    gpgme-devel glib2-devel device-mapper-devel libseccomp-devel \
                                    atomic-registries iptables skopeo-containers containernetworking-cni \
                                    conmon
```
### Building and installing podman
```
# git clone https://github.com/projectatomic/libpod/ ~/src/github.com/projectatomic/libpod
# cd !$
# make
$ sudo make install PREFIX=/usr
```

You now have a working podman environment.  Jump to [Familiarizing yourself with Podman](Familiarizing yourself with Podman)
to begin using Podman.

## Install podman on Ubuntu

The default Ubuntu cloud image size will not allow for the following exercise to be done without increasing its
capacity.  Be sure to add at least 5GB to the image. Instructions to do this are outside the scope of this
tutorial. For this tutorial, the Ubuntu **artful-server-cloudimg** image was used.

### Installing build and runtime dependencies

#### Installing base packages
```
$ sudo apt-get update
$ sudo apt-get install libdevmapper-dev libglib2.0-dev libgpgme11-dev golang libseccomp-dev \
                        go-md2man libprotobuf-dev libprotobuf-c0-dev libseccomp-dev
```
#### Building and installing conmon
```
# git clone https://github.com/kubernetes-incubator/cri-o ~/src/github.com/kubernetes-incubator/cri-o
# cd ~/src/github.com/kubernetes-incubator/cri-o
# mkdir bin
# make conmon
$ sudo install -D -m 755 bin/conmon /usr/libexec/crio/conmon
```
#### Adding required configuration files
```
$ sudo mkdir -p /etc/containers
$ sudo curl https://raw.githubusercontent.com/projectatomic/registries/master/registries.fedora -o /etc/containers/registries.conf
$ sudo curl https://raw.githubusercontent.com/projectatomic/skopeo/master/default-policy.json -o /etc/containers/policy.json
```
#### Installing CNI plugins
```
# git clone https://github.com/containernetworking/plugins.git ~/src/github.com/containernetworking/plugins
# cd ~/src/github.com/containernetworking/plugins
# ./build.sh
$ sudo mkdir -p /usr/libexec/cni
$ sudo cp bin/* /usr/libexec/cni
```
#### Installing runc
```
# git clone https://github.com/opencontainers/runc.git ~/src/github.com/opencontainers/runc
# cd ~/src/github.com/opencontainers/runc
# GOPATH=~/ make static BUILDTAGS="seccomp selinux"
$ sudo cp runc /usr/bin/runc
```

### Building and installing Podman
```
# git clone https://github.com/projectatomic/libpod/ ~/src/github.com/projectatomic/libpod
# cd ~/src/github.com/projectatomic/libpod
# make
$ sudo make install PREFIX=/usr
```

## Familiarizing yourself with Podman

### Running a sample container
This sample container will run a very basic httpd server that serves only its index
page.
```
$ sudo podman run -dt -e HTTPD_VAR_RUN=/var/run/httpd -e HTTPD_MAIN_CONF_D_PATH=/etc/httpd/conf.d \
                    -e HTTPD_MAIN_CONF_PATH=/etc/httpd/conf \
                    -e HTTPD_CONTAINER_SCRIPTS_PATH=/usr/share/container-scripts/httpd/ \
                    registry.fedoraproject.org/f26/httpd /usr/bin/run-httpd
```
Because the container is being run in detached mode, represented by the *-d* in the podman run command, podman
will print the container ID after it has run.

### Listing running containers
The Podman *ps* command is used to list creating and running containers.
```
$ sudo podman ps
```

Note: If you add *-a* to the *ps* command, Podman will show all containers.

### Executing a command in a running container
You can use the *exec* subcommand to execute a command in a running container.  Eventually you will be able to
obtain the IP address of the container through inspection, but that is not enabled yet.  Therefore, we will
install *iproute* in the container.  Notice here that we use the switch **--latest** as a shortcut for the latest
created container.  You could also use the container's ID listed during *podman ps* in the previous step or
when you ran the container.
```
$ sudo podman exec --latest -t dnf -y install iproute
$ sudo podman exec --latest -t ip a
```

Note the IP address of the *ethernet* device.

### Testing the httpd server
Now that we have the IP address of the container, we can test the network communication between the host
operating system and the container using curl. The following command should display the index page of our
containerized httpd server.
```
# curl http://<IP_address>:8080
```

### Viewing the container's logs
You can view the container's logs with Podman as well:
```
$ sudo podman logs --latest
```

<!-- (
### Viewing the container's pids
And you can observe the httpd pid in the container with *top*.
```
$ sudo podman top <container_id>
``` ) -->
### Stopping the container
To stop the httpd container:
```
$ sudo podman stop --latest
```
You can also check the status of one or more containers using the *ps* subcommand. In this case, we should
use the *-a* argument to list all containers.
```
$ sudo podman ps -a
```

### Removing the container
To remove the httpd container:
```
$ sudo podman rm --latest
```
You can verify the deletion of the container by running *podman ps -a*.
## More information

For more information on Podman and its subcommands, checkout the asciiart demos on the [README](https://github.com/projectatomic/libpod#commands)
page.
