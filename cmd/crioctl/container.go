package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/kubernetes-incubator/cri-o/client"
	"github.com/urfave/cli"
	"golang.org/x/net/context"
	remocommandconsts "k8s.io/apimachinery/pkg/util/remotecommand"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	pb "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
)

var containerCommand = cli.Command{
	Name:    "container",
	Aliases: []string{"ctr"},
	Subcommands: []cli.Command{
		createContainerCommand,
		inspectContainerCommand,
		startContainerCommand,
		stopContainerCommand,
		removeContainerCommand,
		containerStatusCommand,
		listContainersCommand,
		execSyncCommand,
		execCommand,
	},
}

type createOptions struct {
	// configPath is path to the config for container
	configPath string
	// name sets the container name
	name string
	// podID of the container
	podID string
	// labels for the container
	labels map[string]string
}

var createContainerCommand = cli.Command{
	Name:  "create",
	Usage: "create a container",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "pod",
			Usage: "the id of the pod sandbox to which the container belongs",
		},
		cli.StringFlag{
			Name:  "config",
			Value: "config.json",
			Usage: "the path of a container config file",
		},
		cli.StringFlag{
			Name:  "name",
			Value: "",
			Usage: "the name of the container",
		},
		cli.StringSliceFlag{
			Name:  "label",
			Usage: "add key=value labels to the container",
		},
	},
	Action: func(context *cli.Context) error {
		// Set up a connection to the server.
		conn, err := getClientConnection(context)
		if err != nil {
			return fmt.Errorf("failed to connect: %v", err)
		}
		defer conn.Close()
		client := pb.NewRuntimeServiceClient(conn)

		if !context.IsSet("pod") {
			return fmt.Errorf("Please specify the id of the pod sandbox to which the container belongs via the --pod option")
		}

		opts := createOptions{
			configPath: context.String("config"),
			name:       context.String("name"),
			podID:      context.String("pod"),
			labels:     make(map[string]string),
		}

		for _, l := range context.StringSlice("label") {
			pair := strings.Split(l, "=")
			if len(pair) != 2 {
				return fmt.Errorf("incorrectly specified label: %v", l)
			}
			opts.labels[pair[0]] = pair[1]
		}

		// Test RuntimeServiceClient.CreateContainer
		err = CreateContainer(client, opts)
		if err != nil {
			return fmt.Errorf("Creating container failed: %v", err)
		}
		return nil
	},
}

var startContainerCommand = cli.Command{
	Name:  "start",
	Usage: "start a container",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "id",
			Value: "",
			Usage: "id of the container",
		},
	},
	Action: func(context *cli.Context) error {
		// Set up a connection to the server.
		conn, err := getClientConnection(context)
		if err != nil {
			return fmt.Errorf("failed to connect: %v", err)
		}
		defer conn.Close()
		client := pb.NewRuntimeServiceClient(conn)

		err = StartContainer(client, context.String("id"))
		if err != nil {
			return fmt.Errorf("Starting the container failed: %v", err)
		}
		return nil
	},
}

var stopContainerCommand = cli.Command{
	Name:  "stop",
	Usage: "stop a container",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "id",
			Value: "",
			Usage: "id of the container",
		},
		cli.Int64Flag{
			Name:  "timeout",
			Value: 10,
			Usage: "seconds to wait to kill the container after a graceful stop is requested",
		},
	},
	Action: func(context *cli.Context) error {
		// Set up a connection to the server.
		conn, err := getClientConnection(context)
		if err != nil {
			return fmt.Errorf("failed to connect: %v", err)
		}
		defer conn.Close()
		client := pb.NewRuntimeServiceClient(conn)

		err = StopContainer(client, context.String("id"), context.Int64("timeout"))
		if err != nil {
			return fmt.Errorf("Stopping the container failed: %v", err)
		}
		return nil
	},
}

var removeContainerCommand = cli.Command{
	Name:  "remove",
	Usage: "remove a container",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "id",
			Value: "",
			Usage: "id of the container",
		},
	},
	Action: func(context *cli.Context) error {
		// Set up a connection to the server.
		conn, err := getClientConnection(context)
		if err != nil {
			return fmt.Errorf("failed to connect: %v", err)
		}
		defer conn.Close()
		client := pb.NewRuntimeServiceClient(conn)

		err = RemoveContainer(client, context.String("id"))
		if err != nil {
			return fmt.Errorf("Removing the container failed: %v", err)
		}
		return nil
	},
}

var containerStatusCommand = cli.Command{
	Name:  "status",
	Usage: "get the status of a container",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "id",
			Value: "",
			Usage: "id of the container",
		},
	},
	Action: func(context *cli.Context) error {
		// Set up a connection to the server.
		conn, err := getClientConnection(context)
		if err != nil {
			return fmt.Errorf("failed to connect: %v", err)
		}
		defer conn.Close()
		client := pb.NewRuntimeServiceClient(conn)

		err = ContainerStatus(client, context.String("id"))
		if err != nil {
			return fmt.Errorf("Getting the status of the container failed: %v", err)
		}
		return nil
	},
}

var execSyncCommand = cli.Command{
	Name:  "execsync",
	Usage: "exec a command synchronously in a container",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "id",
			Value: "",
			Usage: "id of the container",
		},
		cli.Int64Flag{
			Name:  "timeout",
			Value: 0,
			Usage: "timeout for the command",
		},
	},
	Action: func(context *cli.Context) error {
		// Set up a connection to the server.
		conn, err := getClientConnection(context)
		if err != nil {
			return fmt.Errorf("failed to connect: %v", err)
		}
		defer conn.Close()
		client := pb.NewRuntimeServiceClient(conn)

		err = ExecSync(client, context.String("id"), context.Args(), context.Int64("timeout"))
		if err != nil {
			return fmt.Errorf("execing command in container failed: %v", err)
		}
		return nil
	},
}

var execCommand = cli.Command{
	Name:  "exec",
	Usage: "prepare a streaming endpoint to execute a command in the container",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "id",
			Value: "",
			Usage: "id of the container",
		},
		cli.BoolFlag{
			Name:  "tty",
			Usage: "whether to use tty",
		},
		cli.BoolFlag{
			Name:  "stdin",
			Usage: "whether to stream to stdin",
		},
		cli.BoolFlag{
			Name:  "url",
			Usage: "do not exec command, just prepare streaming endpoint",
		},
	},
	Action: func(context *cli.Context) error {
		// Set up a connection to the server.
		conn, err := getClientConnection(context)
		if err != nil {
			return fmt.Errorf("failed to connect: %v", err)
		}
		defer conn.Close()
		client := pb.NewRuntimeServiceClient(conn)

		err = Exec(client, context.String("id"), context.Bool("tty"), context.Bool("stdin"), context.Bool("url"), context.Args())
		if err != nil {
			return fmt.Errorf("execing command in container failed: %v", err)
		}
		return nil
	},
}

type listOptions struct {
	// id of the container
	id string
	// podID of the container
	podID string
	// state of the container
	state string
	// quiet is for listing just container IDs
	quiet bool
	// labels are selectors for the container
	labels map[string]string
}

var listContainersCommand = cli.Command{
	Name:  "list",
	Usage: "list containers",
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "quiet",
			Usage: "list only container IDs",
		},
		cli.StringFlag{
			Name:  "id",
			Value: "",
			Usage: "filter by container id",
		},
		cli.StringFlag{
			Name:  "pod",
			Value: "",
			Usage: "filter by container pod id",
		},
		cli.StringFlag{
			Name:  "state",
			Value: "",
			Usage: "filter by container state",
		},
		cli.StringSliceFlag{
			Name:  "label",
			Usage: "filter by key=value label",
		},
	},
	Action: func(context *cli.Context) error {
		// Set up a connection to the server.
		conn, err := getClientConnection(context)
		if err != nil {
			return fmt.Errorf("failed to connect: %v", err)
		}
		defer conn.Close()
		client := pb.NewRuntimeServiceClient(conn)
		opts := listOptions{
			id:     context.String("id"),
			podID:  context.String("pod"),
			state:  context.String("state"),
			quiet:  context.Bool("quiet"),
			labels: make(map[string]string),
		}

		for _, l := range context.StringSlice("label") {
			pair := strings.Split(l, "=")
			if len(pair) != 2 {
				return fmt.Errorf("incorrectly specified label: %v", l)
			}
			opts.labels[pair[0]] = pair[1]
		}

		err = ListContainers(client, opts)
		if err != nil {
			return fmt.Errorf("listing containers failed: %v", err)
		}
		return nil
	},
}

// CreateContainer sends a CreateContainerRequest to the server, and parses
// the returned CreateContainerResponse.
func CreateContainer(client pb.RuntimeServiceClient, opts createOptions) error {
	config, err := loadContainerConfig(opts.configPath)
	if err != nil {
		return err
	}

	// Override the name by the one specified through CLI
	if opts.name != "" {
		config.Metadata.Name = opts.name
	}

	for k, v := range opts.labels {
		config.Labels[k] = v
	}

	r, err := client.CreateContainer(context.Background(), &pb.CreateContainerRequest{
		PodSandboxId: opts.podID,
		Config:       config,
		// TODO(runcom): this is missing PodSandboxConfig!!!
		// we should/could find a way to retrieve it from the fs and set it here
	})
	if err != nil {
		return err
	}
	fmt.Println(r.ContainerId)
	return nil
}

// StartContainer sends a StartContainerRequest to the server, and parses
// the returned StartContainerResponse.
func StartContainer(client pb.RuntimeServiceClient, ID string) error {
	if ID == "" {
		return fmt.Errorf("ID cannot be empty")
	}
	_, err := client.StartContainer(context.Background(), &pb.StartContainerRequest{
		ContainerId: ID,
	})
	if err != nil {
		return err
	}
	fmt.Println(ID)
	return nil
}

// StopContainer sends a StopContainerRequest to the server, and parses
// the returned StopContainerResponse.
func StopContainer(client pb.RuntimeServiceClient, ID string, timeout int64) error {
	if ID == "" {
		return fmt.Errorf("ID cannot be empty")
	}
	_, err := client.StopContainer(context.Background(), &pb.StopContainerRequest{
		ContainerId: ID,
		Timeout:     timeout,
	})
	if err != nil {
		return err
	}
	fmt.Println(ID)
	return nil
}

// RemoveContainer sends a RemoveContainerRequest to the server, and parses
// the returned RemoveContainerResponse.
func RemoveContainer(client pb.RuntimeServiceClient, ID string) error {
	if ID == "" {
		return fmt.Errorf("ID cannot be empty")
	}
	_, err := client.RemoveContainer(context.Background(), &pb.RemoveContainerRequest{
		ContainerId: ID,
	})
	if err != nil {
		return err
	}
	fmt.Println(ID)
	return nil
}

// ContainerStatus sends a ContainerStatusRequest to the server, and parses
// the returned ContainerStatusResponse.
func ContainerStatus(client pb.RuntimeServiceClient, ID string) error {
	if ID == "" {
		return fmt.Errorf("ID cannot be empty")
	}
	r, err := client.ContainerStatus(context.Background(), &pb.ContainerStatusRequest{
		ContainerId: ID})
	if err != nil {
		return err
	}
	fmt.Printf("ID: %s\n", r.Status.Id)
	if r.Status.Metadata != nil {
		if r.Status.Metadata.Name != "" {
			fmt.Printf("Name: %s\n", r.Status.Metadata.Name)
		}
		fmt.Printf("Attempt: %v\n", r.Status.Metadata.Attempt)
	}
	// TODO(mzylowski): print it prettier
	fmt.Printf("Status: %s\n", r.Status.State)
	ctm := time.Unix(0, r.Status.CreatedAt)
	fmt.Printf("Created: %v\n", ctm)
	stm := time.Unix(0, r.Status.StartedAt)
	fmt.Printf("Started: %v\n", stm)
	ftm := time.Unix(0, r.Status.FinishedAt)
	fmt.Printf("Finished: %v\n", ftm)
	fmt.Printf("Exit Code: %v\n", r.Status.ExitCode)
	fmt.Printf("Reason: %v\n", r.Status.Reason)
	if r.Status.Image != nil {
		fmt.Printf("Image: %v\n", r.Status.Image.Image)
	}
	fmt.Printf("ImageRef: %v\n", r.Status.ImageRef)

	return nil
}

// ExecSync sends an ExecSyncRequest to the server, and parses
// the returned ExecSyncResponse.
func ExecSync(client pb.RuntimeServiceClient, ID string, cmd []string, timeout int64) error {
	if ID == "" {
		return fmt.Errorf("ID cannot be empty")
	}
	r, err := client.ExecSync(context.Background(), &pb.ExecSyncRequest{
		ContainerId: ID,
		Cmd:         cmd,
		Timeout:     timeout,
	})
	if err != nil {
		return err
	}
	fmt.Println("Stdout:")
	fmt.Println(string(r.Stdout))
	fmt.Println("Stderr:")
	fmt.Println(string(r.Stderr))
	fmt.Printf("Exit code: %v\n", r.ExitCode)

	return nil
}

// Exec sends an ExecRequest to the server, and parses
// the returned ExecResponse.
func Exec(client pb.RuntimeServiceClient, ID string, tty bool, stdin bool, urlOnly bool, cmd []string) error {
	if ID == "" {
		return fmt.Errorf("ID cannot be empty")
	}
	r, err := client.Exec(context.Background(), &pb.ExecRequest{
		ContainerId: ID,
		Cmd:         cmd,
		Tty:         tty,
		Stdin:       stdin,
	})
	if err != nil {
		return err
	}

	if urlOnly {
		fmt.Println("URL:")
		fmt.Println(r.Url)
		return nil
	}

	execURL, err := url.Parse(r.Url)
	if err != nil {
		return err
	}

	streamExec, err := remotecommand.NewExecutor(&restclient.Config{}, "GET", execURL)
	if err != nil {
		return err
	}

	options := remotecommand.StreamOptions{
		SupportedProtocols: remocommandconsts.SupportedStreamingProtocols,
		Stdout:             os.Stdout,
		Stderr:             os.Stderr,
		Tty:                tty,
	}

	if stdin {
		options.Stdin = os.Stdin
	}

	return streamExec.Stream(options)
}

// ListContainers sends a ListContainerRequest to the server, and parses
// the returned ListContainerResponse.
func ListContainers(client pb.RuntimeServiceClient, opts listOptions) error {
	filter := &pb.ContainerFilter{}
	if opts.id != "" {
		filter.Id = opts.id
	}
	if opts.podID != "" {
		filter.PodSandboxId = opts.podID
	}
	if opts.state != "" {
		st := &pb.ContainerStateValue{}
		st.State = pb.ContainerState_CONTAINER_UNKNOWN
		switch opts.state {
		case "created":
			st.State = pb.ContainerState_CONTAINER_CREATED
			filter.State = st
		case "running":
			st.State = pb.ContainerState_CONTAINER_RUNNING
			filter.State = st
		case "stopped":
			st.State = pb.ContainerState_CONTAINER_EXITED
			filter.State = st
		default:
			log.Fatalf("--state should be one of created, running or stopped")
		}
	}
	if opts.labels != nil {
		filter.LabelSelector = opts.labels
	}
	r, err := client.ListContainers(context.Background(), &pb.ListContainersRequest{
		Filter: filter,
	})
	if err != nil {
		return err
	}
	for _, c := range r.GetContainers() {
		if opts.quiet {
			fmt.Println(c.Id)
			continue
		}
		fmt.Printf("ID: %s\n", c.Id)
		fmt.Printf("Pod: %s\n", c.PodSandboxId)
		if c.Metadata != nil {
			if c.Metadata.Name != "" {
				fmt.Printf("Name: %s\n", c.Metadata.Name)
			}
			fmt.Printf("Attempt: %v\n", c.Metadata.Attempt)
		}
		fmt.Printf("Status: %s\n", c.State)
		if c.Image != nil {
			fmt.Printf("Image: %s\n", c.Image.Image)
		}
		ctm := time.Unix(0, c.CreatedAt)
		fmt.Printf("Created: %v\n", ctm)
		if c.Labels != nil {
			fmt.Println("Labels:")
			for _, k := range getSortedKeys(c.Labels) {
				fmt.Printf("\t%s -> %s\n", k, c.Labels[k])
			}
		}
		if c.Annotations != nil {
			fmt.Println("Annotations:")
			for _, k := range getSortedKeys(c.Annotations) {
				fmt.Printf("\t%s -> %s\n", k, c.Annotations[k])
			}
		}
		fmt.Println()
	}
	return nil
}

var inspectContainerCommand = cli.Command{
	Name:  "inspect",
	Usage: "get container info from crio daemon",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "id",
			Value: "",
			Usage: "id of the container",
		},
	},
	Action: func(context *cli.Context) error {
		ID := context.String("id")
		if ID == "" {
			return fmt.Errorf("ID cannot be empty")
		}
		c, err := client.New(context.GlobalString("connect"))
		if err != nil {
			return err
		}

		cInfo, err := c.ContainerInfo(ID)
		if err != nil {
			return err
		}

		jsonBytes, err := json.MarshalIndent(cInfo, "", "    ")
		if err != nil {
			return err
		}
		fmt.Println(string(jsonBytes))
		return nil
	},
}
