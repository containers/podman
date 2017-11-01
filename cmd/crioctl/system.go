package main

import (
	"fmt"

	"github.com/urfave/cli"
	"golang.org/x/net/context"
	pb "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
)

var runtimeVersionCommand = cli.Command{
	Name:  "runtimeversion",
	Usage: "get runtime version information",
	Action: func(context *cli.Context) error {
		// Set up a connection to the server.
		conn, err := getClientConnection(context)
		if err != nil {
			return fmt.Errorf("failed to connect: %v", err)
		}
		defer conn.Close()
		client := pb.NewRuntimeServiceClient(conn)

		// Test RuntimeServiceClient.Version
		version := "v1alpha1"
		err = Version(client, version)
		if err != nil {
			return fmt.Errorf("Getting the runtime version failed: %v", err)
		}
		return nil
	},
}

// Version sends a VersionRequest to the server, and parses the returned VersionResponse.
func Version(client pb.RuntimeServiceClient, version string) error {
	r, err := client.Version(context.Background(), &pb.VersionRequest{Version: version})
	if err != nil {
		return err
	}
	fmt.Printf("VersionResponse: Version: %s, RuntimeName: %s, RuntimeVersion: %s, RuntimeApiVersion: %s\n", r.Version, r.RuntimeName, r.RuntimeVersion, r.RuntimeApiVersion)
	return nil
}
