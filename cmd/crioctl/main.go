package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"google.golang.org/grpc"
	pb "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
)

// This is populated by the Makefile from the VERSION file
// in the repository
var version = ""

// gitCommit is the commit that the binary is being built from.
// It will be populated by the Makefile.
var gitCommit = ""

func getClientConnection(context *cli.Context) (*grpc.ClientConn, error) {
	conn, err := grpc.Dial(context.GlobalString("connect"), grpc.WithInsecure(), grpc.WithTimeout(context.GlobalDuration("timeout")),
		grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
			return net.DialTimeout("unix", addr, timeout)
		}))
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %v", err)
	}
	return conn, nil
}

func openFile(path string) (*os.File, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config at %s not found", path)
		}
		return nil, err
	}
	return f, nil
}

func loadPodSandboxConfig(path string) (*pb.PodSandboxConfig, error) {
	f, err := openFile(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var config pb.PodSandboxConfig
	if err := json.NewDecoder(f).Decode(&config); err != nil {
		return nil, err
	}
	return &config, nil
}

func loadContainerConfig(path string) (*pb.ContainerConfig, error) {
	f, err := openFile(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var config pb.ContainerConfig
	if err := json.NewDecoder(f).Decode(&config); err != nil {
		return nil, err
	}
	return &config, nil
}

func main() {
	app := cli.NewApp()
	var v []string
	if version != "" {
		v = append(v, version)
	}
	if gitCommit != "" {
		v = append(v, fmt.Sprintf("commit: %s", gitCommit))
	}

	app.Name = "crioctl"
	app.Usage = "client for crio"
	app.Version = strings.Join(v, "\n")

	app.Commands = []cli.Command{
		podSandboxCommand,
		containerCommand,
		runtimeVersionCommand,
		imageCommand,
		infoCommand,
	}

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "connect",
			Value: "/var/run/crio.sock",
			Usage: "Socket to connect to",
		},
		cli.DurationFlag{
			Name:  "timeout",
			Value: 10 * time.Second,
			Usage: "Timeout of connecting to server",
		},
	}

	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}
