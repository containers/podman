# Podman Golang bindings
The Podman Go bindings are a set of functions to allow developers to execute Podman operations from within their Go based application. The Go bindings
connect to a Podman service which can run locally or on a remote machine. You can perform many operations including pulling and listing images, starting,
stopping or inspecting containers. Currently, the Podman repository has bindings available for operations on images, containers, pods,
networks and manifests among others.

## Quick Start
The bindings require that the Podman system service is running for the specified user.  This can be done with systemd using the `systemctl` command or manually
by calling the service directly.

### Starting the service with system
The command to start the Podman service differs slightly depending on the user that is running the service.  For a rootful service,
start the service like this:
```
# systemctl start podman.socket
```
For a non-privileged, aka rootless, user, start the service like this:

```
$ systemctl start --user podman.socket
```

### Starting the service manually
It can be handy to run the system service manually.  Doing so allows you to enable debug messaging.
```
$ podman --log-level=debug system service -t0
```
If you do not provide a specific path for the socket, a default is provided.  The location of that socket for
rootful connections is `/run/podman/podman.sock` and for rootless it is `/run/USERID#/podman/podman.sock`. For more
information about the Podman system service, see `man podman-system-service`.

### Creating a connection
The first step for using the bindings is to create a connection to the socket.  As mentioned earlier, the destination
of the socket depends on the user who owns it. In this case, a rootful connection is made.

```
import (
	"context"
	"fmt"
	"os"

	"github.com/containers/podman/v3/pkg/bindings"
)

func main() {
	conn, err := bindings.NewConnection(context.Background(), "unix://run/podman/podman.sock")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

}
```
The `conn` variable returned from the `bindings.NewConnection` function can then be used in subsequent function calls
to interact with containers.

### Examples
The following examples build upon the connection example from above.  They are all rootful connections as well.

#### Inspect a container
The following example obtains the inspect information for a container named `foorbar` and then prints
the container's ID. Note the use of optional inspect options for size.
```
import (
	"context"
	"fmt"
	"os"

	"github.com/containers/podman/v3/pkg/bindings"
	"github.com/containers/podman/v3/pkg/bindings/containers"
)

func main() {
	conn, err := bindings.NewConnection(context.Background(), "unix://run/podman/podman.sock")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	inspectData, err := containers.Inspect(conn, "foobar", new(containers.InspectOptions).WithSize(true))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	// Print the container ID
	fmt.Println(inspectData.ID)
}
```

#### Pull an image
The following example pulls the image `quay.ioo/libpod/alpine_nginx` to the local image store.
```
import (
	"context"
	"fmt"
	"os"

	"github.com/containers/podman/v3/pkg/bindings"
	"github.com/containers/podman/v3/pkg/bindings/images"
)

func main() {
	conn, err := bindings.NewConnection(context.Background(), "unix://run/podman/podman.sock")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	_, err = images.Pull(conn, "quay.io/libpod/alpine_nginx", nil)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

```

#### Pull an image, create a container, and start the container
The following example pulls the `quay.io/libpod/alpine_nginx` image and then creates a container named `foobar`
from it.  And finally, it starts the container.
```
import (
	"context"
	"fmt"
	"os"

	"github.com/containers/podman/v3/pkg/bindings"
	"github.com/containers/podman/v3/pkg/bindings/containers"
	"github.com/containers/podman/v3/pkg/bindings/images"
	"github.com/containers/podman/v3/pkg/specgen"
)

func main() {
	conn, err := bindings.NewConnection(context.Background(), "unix://run/podman/podman.sock")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	_, err = images.Pull(conn, "quay.io/libpod/alpine_nginx", nil)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	s := specgen.NewSpecGenerator("quay.io/libpod/alpine_nginx", false)
	s.Name = "foobar"
	createResponse, err := containers.CreateWithSpec(conn, s, nil)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Println("Container created.")
	if err := containers.Start(conn, createResponse.ID, nil); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Println("Container started.")
}
```
