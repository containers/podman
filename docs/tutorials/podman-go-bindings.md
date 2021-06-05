![PODMAN logo](../../logo/podman-logo-source.svg)

# Podman Go bindings

## Introduction

In the release of Podman 2.0, we removed the experimental tag
from its recently introduced RESTful service. While it might
be interesting to interact with a RESTFul server using curl,
using a set of Go based bindings is probably a more direct
route to a production ready application.  Let’s take a look
at how easily that can be accomplished.

If you haven't yet, [install Go](https://golang.org/doc/install).

Be careful to double-check that the version of golang is new
enough (i.e. `go version`), version 1.13.x or higher is
supported. If needed, Go sources and binaries can be fetched
from the [official Go website](https://golang.org/dl/).

The Podman Go bindings are a set of functions to allow
developers to execute Podman operations from within their Go
based application. The Go bindings connect to a Podman service
which can run locally or on a remote machine. You can perform
many operations including pulling and listing images, starting,
stopping or inspecting containers. Currently, the Podman
repository has bindings available for operations on images,
containers, pods, networks and manifests among others. The
bindings are available on the [v2.0 branch in the
upstream Podman repository](https://github.com/containers/podman/tree/v2.0).
You can fetch the bindings for your application using Go modules:

```bash
$ cd $HOME
$ mkdir example && cd example
$ go mod init example.com
go: creating new go.mod: module example.com
$ go get github.com/containers/podman/v3
[...]
```

This creates a new `go.mod` file in the current directory that looks as follows:

```bash
module example.com

go 1.16

require github.com/containers/libpod/v3 v3.0.1 // indirect
```

You can also try a demo application with the Go modules created already:

```bash
$ git clone https://github.com/containers/Demos
$ cd Demos/podman_go_bindings
$ ls
README.md  go.mod  go.sum  main.go
```


## How do I use them

In this tutorial, you will learn through basic examples how to:

0. [Start the Podman system service](#start-service)
1. [Connect to the Podman system service](#connect-service)
2. [Pull images](#pull-images)
3. [List images](#list-images)
4. [Create and start a container from an image](#create-start-container)
5. [List containers](#list-containers)
6. [Inspect the container](#inspect-container)
7. [Stop the container](#stop-container)
8. [Debugging tips](#debugging-tips)


### Start the Podman system service <a name="start-service"></a>
The recommended way to start Podman system service in production mode
is via systemd socket-activation:

```bash
$ systemctl --user start podman.socket
```

There’s no timeout specified when starting the system service via socket-activation.

For purposes of this demo, we will start the service using the Podman
command itself. If you prefer the system service to timeout after, say,
5000 seconds, you can run it like so:

```bash
$ podman system service -t 5000
```

Note that the 5000 seconds uptime is refreshed after every command is received.
If you want the service to stay up until the machine is shutdown or the process
is terminated, use `0` (zero) instead of 5000. For this demo, we will use no timeout:

```bash
$ podman system service -t 0
```


Open another terminal window and check if the Podman socket exists:

```bash
$ ls /run/user/${UID}/podman
podman.sock
```

If you’re running the system service as root, podman.sock will be found in /run/podman:
```bash
# ls /run/podman
podman.sock
```


### Connect to the Podman system service <a name="connect-service"></a>
First, you need to create a connection that connects to the system service.
The critical piece of information for setting up a new connection is the endpoint.
The endpoint comes in the form of an URI (method:/path/to/socket). For example,
to connect to the local rootful socket the URI would be `unix:/run/podman/podman.sock`
and for a rootless user it would be `unix:$(XDG_RUNTIME_DIR)/podman/podman.sock`,
typically: `unix:/run/user/${UID}/podman/podman.sock`.


The following Go example snippet shows how to set up a connection for a rootless user.
```Go
package main

import (
        "context"
        "fmt"
        "os"

        "github.com/containers/libpod/v3/libpod/define"
        "github.com/containers/libpod/v3/pkg/bindings"
        "github.com/containers/libpod/v3/pkg/bindings/containers"
        "github.com/containers/libpod/v3/pkg/bindings/images"
        "github.com/containers/libpod/v3/pkg/domain/entities"
        "github.com/containers/libpod/v3/pkg/specgen"
)

func main() {
        fmt.Println("Welcome to the Podman Go bindings tutorial")

        // Get Podman socket location
        sock_dir := os.Getenv("XDG_RUNTIME_DIR")
        socket := "unix:" + sock_dir + "/podman/podman.sock"

        // Connect to Podman socket
        connText, err := bindings.NewConnection(context.Background(), socket)
        if err != nil {
                fmt.Println(err)
                os.Exit(1)
        }
}
```

The `connText` variable received from the NewConnection function is of type
context.Context(). In subsequent uses of the bindings, you will use this context
to direct the bindings to your connection. This can be seen in the examples below.

### Pull an image <a name="pull-images"></a>

Next, we will pull a couple of images using the images.Pull() binding.
This binding takes three arguments:
    - The context variable created by the bindings.NewConnection() call in the first example
    - The image name
    - Options for image pull

**Append the following lines to your function:**

```Go
        // Pull Busybox image (Sample 1)
        fmt.Println("Pulling Busybox image...")
        _, err = images.Pull(connText, "docker.io/busybox", &images.PullOptions{})
        if err != nil {
                fmt.Println(err)
                os.Exit(1)
        }

        // Pull Fedora image (Sample 2)
        rawImage := "registry.fedoraproject.org/fedora:latest"
        fmt.Println("Pulling Fedora image...")
        _, err = images.Pull(connText, rawImage, &images.PullOptions{})
        if err != nil {
                fmt.Println(err)
                os.Exit(1)
        }
```

**Run it:**

```bash
$ go run main.go
Welcome to the Podman Go bindings tutorial
Pulling Busybox image...
Pulling Fedora image...
$
```

The system service side should echo messages like so:

```bash
Trying to pull docker.io/busybox...
Getting image source signatures
Copying blob 61c5ed1cbdf8 [--------------------------------------] 0.0b / 0.0b
Copying config 018c9d7b79 done
Writing manifest to image destination
Storing signatures
Trying to pull registry.fedoraproject.org/fedora:latest...
Getting image source signatures
Copying blob dd9f43919ba0 [--------------------------------------] 0.0b / 0.0b
Copying config 00ff39a8bf done
Writing manifest to image destination
Storing signatures
```


### List images <a name="list-images"></a>
Next, we will pull an image using the images.List() binding.
This binding takes three arguments:
   - The context variable created earlier
   - An optional bool 'all'
   - An optional map of filters

**Append the following lines to your function:**

```Go
        // List images
        imageSummary, err := images.List(connText, &images.ListOptions{})
        if err != nil {
            fmt.Println(err)
            os.Exit(1)
        }
        var names []string
        for _, i := range imageSummary {
            names = append(names, i.RepoTags...)
        }
        fmt.Println("Listing images...")
        fmt.Println(names)
```

**Run it:**

```bash
$ go run main.go
Welcome to the Podman Go bindings tutorial
Pulling Busybox image...
Pulling Fedora image...
Listing images...
[docker.io/library/busybox:latest registry.fedoraproject.org/fedora:latest]
$
```


### Create and Start a Container from an Image <a name="create-start-container"></a>

To create the container spec, we use specgen.NewSpecGenerator() followed by
calling containers.CreateWithSpec() to actually create a new container.
specgen.NewSpecGenerator() takes 2 arguments:
    - name of the image
    - whether it's a rootfs

containers.CreateWithSpec() takes 2 arguments:
    - the context created earlier
    - the spec created by NewSpecGenerator

Next, the container is actually started using the containers.Start() binding.
containers.Start() takes three arguments:
    - the context
    - the name or ID of the container created
    - an optional parameter for detach keys

After the container is started, it's a good idea to ensure the container is
in a running state before you proceed with further operations.
The containers.Wait() takes care of that.
containers.Wait() takes three arguments:
    - the context
    - the name or ID of the container created
    - container state (running/paused/stopped)

**Append the following lines to your function:**

```Go
        // Container create
        s := specgen.NewSpecGenerator(rawImage, false)
        s.Terminal = true
        r, err := containers.CreateWithSpec(connText, s, nil)
        if err != nil {
                fmt.Println(err)
                os.Exit(1)
        }

        // Container start
        fmt.Println("Starting Fedora container...")
        err = containers.Start(connText, r.ID, nil)
        if err != nil {
                fmt.Println(err)
                os.Exit(1)
        }

        running := define.ContainerStateRunning
        _, err = containers.Wait(connText, r.ID, &containers.WaitOptions{Condition: []define.ContainerStatus{running}})
        if err != nil {
                fmt.Println(err)
                os.Exit(1)
        }
```

**Run it:**

```bash
$ go run main.go
Welcome to the Podman Go bindings tutorial
Pulling image...
Starting Fedora container...
$
```

Check if the container is running:

```bash
$ podman ps
CONTAINER ID  IMAGE                                     COMMAND    CREATED                 STATUS                     PORTS   NAMES
665831d31e90  registry.fedoraproject.org/fedora:latest  /bin/bash  Less than a second ago  Up Less than a second ago          dazzling_mclean
$
```


### List Containers <a name="list-containers"></a>

Containers can be listed using the containers.List() binding.
containers.List() takes seven arguments:
	- the context
	- output filters
	- boolean to show all containers, by default only running containers are listed
	- number of latest created containers, all states (running/paused/stopped)
	- boolean to print pod information
	- boolean to print rootfs size
	- boolean to print oci runtime and container state

**Append the following lines to your function:**

```Go
        // Container list
        var latestContainers = 1
        containerLatestList, err := containers.List(connText, &containers.ListOptions{Last: &latestContainers})
        if err != nil {
            fmt.Println(err)
            os.Exit(1)
        }
        fmt.Printf("Latest container is %s\n", containerLatestList[0].Names[0])
```

**Run it:**

```bash
$ go run main.go
Welcome to the Podman Go bindings tutorial
Pulling Busybox image...
Pulling Fedora image...
Listing images...
[docker.io/library/busybox:latest registry.fedoraproject.org/fedora:latest]
Starting Fedora container...
Latest container is dazzling_mclean
$
```


### Inspect Container <a name="inspect-container"></a>
Containers can be inspected using the containers.Inspect() binding.
containers.Inspect() takes 3 arguments:
    - context
    - image name or ID
    - optional boolean to check for container size


**Append the following lines to your function:**

```Go
        // Container inspect
        ctrData, err := containers.Inspect(connText, r.ID, nil)
        if err != nil {
                fmt.Println(err)
                os.Exit(1)
        }
        fmt.Printf("Container uses image %s\n", ctrData.ImageName)
        fmt.Printf("Container running status is %s\n", ctrData.State.Status)
```

**Run it:**

```bash
$ go run main.go
Welcome to the Podman Go bindings tutorial
Pulling Busybox image...
Pulling Fedora image...
Listing images...
[docker.io/library/busybox:latest registry.fedoraproject.org/fedora:latest]
Starting Fedora container...
Latest container is peaceful_noether
Fedora Container uses image registry.fedoraproject.org/fedora:latest
Fedora Container running status is running
$
```


### Stop Container <a name="stop-container"></a>

A container can be stopped by the containers.Stop() binding.
containers.Stop() takes 3 arguments:
    - context
    - image name or ID
    - optional timeout

**Append the following lines to your function:**

```Go
        // Container stop
        fmt.Println("Stopping the container...")
        err = containers.Stop(connText, r.ID, nil)
        if err != nil {
                fmt.Println(err)
                os.Exit(1)
        }
        ctrData, err = containers.Inspect(connText, r.ID, nil)
        if err != nil {
                fmt.Println(err)
                os.Exit(1)
        }
        fmt.Printf("Container running status is now %s\n", ctrData.State.Status)
```

**Run it:**

```bash
$ go run main.go
Welcome to the Podman Go bindings tutorial
Pulling Busybox image...
Pulling Fedora image...
Listing images...
[docker.io/library/busybox:latest registry.fedoraproject.org/fedora:latest]
Starting Fedora container...
Latest container is peaceful_noether
Fedora Container uses image registry.fedoraproject.org/fedora:latest
Fedora Container running status is running
Stopping Fedora container...
Container running status is now exited
```


### Debugging tips <a name="debugging-tips"></a>

To debug in a development setup, you can start the Podman system service
in debug mode like so:

```bash
$ podman --log-level=debug system service -t 0
```

The `--log-level=debug` echoes all the logged requests and is useful to
trace the execution path at a finer granularity. A snippet of a sample run looks like:

```bash
INFO[0000] podman filtering at log level debug
DEBU[0000] Called service.PersistentPreRunE(podman --log-level=debug system service -t0)
DEBU[0000] Ignoring libpod.conf EventsLogger setting "/home/lsm5/.config/containers/containers.conf". Use "journald" if you want to change this setting and remove libpod.conf files.
DEBU[0000] Reading configuration file "/usr/share/containers/containers.conf"
DEBU[0000] Merged system config "/usr/share/containers/containers.conf": {Editors note: the remainder of this line was removed due to Jekyll formatting errors.}
DEBU[0000] Using conmon: "/usr/bin/conmon"
DEBU[0000] Initializing boltdb state at /home/lsm5/.local/share/containers/storage/libpod/bolt_state.db
DEBU[0000] Overriding run root "/run/user/1000/containers" with "/run/user/1000" from database
DEBU[0000] Using graph driver overlay
DEBU[0000] Using graph root /home/lsm5/.local/share/containers/storage
DEBU[0000] Using run root /run/user/1000
DEBU[0000] Using static dir /home/lsm5/.local/share/containers/storage/libpod
DEBU[0000] Using tmp dir /run/user/1000/libpod/tmp
DEBU[0000] Using volume path /home/lsm5/.local/share/containers/storage/volumes
DEBU[0000] Set libpod namespace to ""
DEBU[0000] Not configuring container store
DEBU[0000] Initializing event backend file
DEBU[0000] using runtime "/usr/bin/runc"
DEBU[0000] using runtime "/usr/bin/crun"
WARN[0000] Error initializing configured OCI runtime kata: no valid executable found for OCI runtime kata: invalid argument
DEBU[0000] using runtime "/usr/bin/crun"
INFO[0000] Setting parallel job count to 25
INFO[0000] podman filtering at log level debug
DEBU[0000] Called service.PersistentPreRunE(podman --log-level=debug system service -t0)
DEBU[0000] Ignoring libpod.conf EventsLogger setting "/home/lsm5/.config/containers/containers.conf". Use "journald" if you want to change this setting and remove libpod.conf files.
DEBU[0000] Reading configuration file "/usr/share/containers/containers.conf"
```

If the Podman system service has been started via systemd socket activation,
you can view the logs using journalctl. The logs after a sample run look like so:

```bash
$ journalctl --user --no-pager -u podman.socket
-- Reboot --
Jul 22 13:50:40 nagato.nanadai.me systemd[1048]: Listening on Podman API Socket.
$
```

```bash
$ journalctl --user --no-pager -u podman.service
Jul 22 13:50:53 nagato.nanadai.me systemd[1048]: Starting Podman API Service...
Jul 22 13:50:54 nagato.nanadai.me podman[1527]: time="2020-07-22T13:50:54-04:00" level=error msg="Error refreshing volume 38480630a8bdaa3e1a0ebd34c94038591b0d7ad994b37be5b4f2072bb6ef0879: error acquiring lock 0 for volume 38480630a8bdaa3e1a0ebd34c94038591b0d7ad994b37be5b4f2072bb6ef0879: file exists"
Jul 22 13:50:54 nagato.nanadai.me podman[1527]: time="2020-07-22T13:50:54-04:00" level=error msg="Error refreshing volume 47d410af4d762a0cc456a89e58f759937146fa3be32b5e95a698a1d4069f4024: error acquiring lock 0 for volume 47d410af4d762a0cc456a89e58f759937146fa3be32b5e95a698a1d4069f4024: file exists"
Jul 22 13:50:54 nagato.nanadai.me podman[1527]: time="2020-07-22T13:50:54-04:00" level=error msg="Error refreshing volume 86e73f082e344dad38c8792fb86b2017c4f133f2a8db87f239d1d28a78cf0868: error acquiring lock 0 for volume 86e73f082e344dad38c8792fb86b2017c4f133f2a8db87f239d1d28a78cf0868: file exists"
Jul 22 13:50:54 nagato.nanadai.me podman[1527]: time="2020-07-22T13:50:54-04:00" level=error msg="Error refreshing volume 9a16ea764be490a5563e384d9074ab0495e4d9119be380c664037d6cf1215631: error acquiring lock 0 for volume 9a16ea764be490a5563e384d9074ab0495e4d9119be380c664037d6cf1215631: file exists"
Jul 22 13:50:54 nagato.nanadai.me podman[1527]: time="2020-07-22T13:50:54-04:00" level=error msg="Error refreshing volume bfd6b2a97217f8655add13e0ad3f6b8e1c79bc1519b7a1e15361a107ccf57fc0: error acquiring lock 0 for volume bfd6b2a97217f8655add13e0ad3f6b8e1c79bc1519b7a1e15361a107ccf57fc0: file exists"
Jul 22 13:50:54 nagato.nanadai.me podman[1527]: time="2020-07-22T13:50:54-04:00" level=error msg="Error refreshing volume f9b9f630982452ebcbed24bd229b142fbeecd5d4c85791fca440b21d56fef563: error acquiring lock 0 for volume f9b9f630982452ebcbed24bd229b142fbeecd5d4c85791fca440b21d56fef563: file exists"
Jul 22 13:50:54 nagato.nanadai.me podman[1527]: Trying to pull registry.fedoraproject.org/fedora:latest...
Jul 22 13:50:55 nagato.nanadai.me podman[1527]: Getting image source signatures
Jul 22 13:50:55 nagato.nanadai.me podman[1527]: Copying blob sha256:dd9f43919ba05f05d4f783c31e83e5e776c4f5d29dd72b9ec5056b9576c10053
Jul 22 13:50:55 nagato.nanadai.me podman[1527]: Copying config sha256:00ff39a8bf19f810a7e641f7eb3ddc47635913a19c4996debd91fafb6b379069
Jul 22 13:50:55 nagato.nanadai.me podman[1527]: Writing manifest to image destination
Jul 22 13:50:55 nagato.nanadai.me podman[1527]: Storing signatures
Jul 22 13:50:55 nagato.nanadai.me systemd[1048]: podman.service: unit configures an IP firewall, but not running as root.
Jul 22 13:50:55 nagato.nanadai.me systemd[1048]: (This warning is only shown for the first unit using IP firewalling.)
Jul 22 13:51:15 nagato.nanadai.me systemd[1048]: podman.service: Succeeded.
Jul 22 13:51:15 nagato.nanadai.me systemd[1048]: Finished Podman API Service.
Jul 22 13:51:15 nagato.nanadai.me systemd[1048]: podman.service: Consumed 1.339s CPU time.
$
```


## Wrap Up
Podman provides a set of Go bindings to allow developers to integrate Podman
functionality conveniently in their Go application.  These Go bindings require
the Podman system service to be running in the background and this can easily
be achieved using systemd socket activation. Once set up, you are able to use a
set of Go based bindings to create, maintain and monitor your container images,
containers and pods in a way which fits very nicely in many production environments.


## References
- Podman is available for most major distributions along with macOS and Windows.
Installation details are available on the [Podman official website](https://podman.io/getting-started/).

- Documentation can be found at the [Podman Docs page](https://docs.podman.io).
It also includes a section on the [RESTful API](https://docs.podman.io/en/latest/Reference.html).
