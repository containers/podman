package main

import (
	"fmt"

	"github.com/urfave/cli"
	"golang.org/x/net/context"
	pb "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
)

var imageCommand = cli.Command{
	Name: "image",
	Subcommands: []cli.Command{
		pullImageCommand,
		listImageCommand,
		imageStatusCommand,
		removeImageCommand,
	},
}

var pullImageCommand = cli.Command{
	Name:  "pull",
	Usage: "pull an image",
	Action: func(context *cli.Context) error {
		// Set up a connection to the server.
		conn, err := getClientConnection(context)
		if err != nil {
			return fmt.Errorf("failed to connect: %v", err)
		}
		defer conn.Close()
		client := pb.NewImageServiceClient(conn)

		_, err = PullImage(client, context.Args().Get(0))
		if err != nil {
			return fmt.Errorf("pulling image failed: %v", err)
		}
		return nil
	},
}

var listImageCommand = cli.Command{
	Name:  "list",
	Usage: "list images",
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "quiet",
			Usage: "list only image IDs",
		},
	},
	Action: func(context *cli.Context) error {
		// Set up a connection to the server.
		conn, err := getClientConnection(context)
		if err != nil {
			return fmt.Errorf("failed to connect: %v", err)
		}
		defer conn.Close()
		client := pb.NewImageServiceClient(conn)

		r, err := ListImages(client, context.Args().Get(0))
		if err != nil {
			return fmt.Errorf("listing images failed: %v", err)
		}
		quiet := context.Bool("quiet")
		for _, image := range r.Images {
			if quiet {
				fmt.Printf("%s\n", image.Id)
				continue
			}
			fmt.Printf("ID: %s\n", image.Id)
			for _, tag := range image.RepoTags {
				fmt.Printf("Tag: %s\n", tag)
			}
			for _, digest := range image.RepoDigests {
				fmt.Printf("Digest: %s\n", digest)
			}
			if image.Size_ != 0 {
				fmt.Printf("Size: %d\n", image.Size_)
			}
		}
		return nil
	},
}

var imageStatusCommand = cli.Command{
	Name:  "status",
	Usage: "return the status of an image",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "id",
			Usage: "id of the image",
		},
	},
	Action: func(context *cli.Context) error {
		// Set up a connection to the server.
		conn, err := getClientConnection(context)
		if err != nil {
			return fmt.Errorf("failed to connect: %v", err)
		}
		defer conn.Close()
		client := pb.NewImageServiceClient(conn)

		r, err := ImageStatus(client, context.String("id"))
		if err != nil {
			return fmt.Errorf("image status request failed: %v", err)
		}
		image := r.Image
		if image == nil {
			return fmt.Errorf("no such image present")
		}
		fmt.Printf("ID: %s\n", image.Id)
		for _, tag := range image.RepoTags {
			fmt.Printf("Tag: %s\n", tag)
		}
		for _, digest := range image.RepoDigests {
			fmt.Printf("Digest: %s\n", digest)
		}
		fmt.Printf("Size: %d\n", image.Size_)
		return nil
	},
}
var removeImageCommand = cli.Command{
	Name:  "remove",
	Usage: "remove an image",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "id",
			Value: "",
			Usage: "id of the image",
		},
	},
	Action: func(context *cli.Context) error {
		// Set up a connection to the server.
		conn, err := getClientConnection(context)
		if err != nil {
			return fmt.Errorf("failed to connect: %v", err)
		}
		defer conn.Close()
		client := pb.NewImageServiceClient(conn)

		_, err = RemoveImage(client, context.String("id"))
		if err != nil {
			return fmt.Errorf("removing the image failed: %v", err)
		}
		return nil
	},
}

// PullImage sends a PullImageRequest to the server, and parses
// the returned PullImageResponse.
func PullImage(client pb.ImageServiceClient, image string) (*pb.PullImageResponse, error) {
	return client.PullImage(context.Background(), &pb.PullImageRequest{Image: &pb.ImageSpec{Image: image}})
}

// ListImages sends a ListImagesRequest to the server, and parses
// the returned ListImagesResponse.
func ListImages(client pb.ImageServiceClient, image string) (*pb.ListImagesResponse, error) {
	return client.ListImages(context.Background(), &pb.ListImagesRequest{Filter: &pb.ImageFilter{Image: &pb.ImageSpec{Image: image}}})
}

// ImageStatus sends an ImageStatusRequest to the server, and parses
// the returned ImageStatusResponse.
func ImageStatus(client pb.ImageServiceClient, image string) (*pb.ImageStatusResponse, error) {
	return client.ImageStatus(context.Background(), &pb.ImageStatusRequest{Image: &pb.ImageSpec{Image: image}})
}

// RemoveImage sends a RemoveImageRequest to the server, and parses
// the returned RemoveImageResponse.
func RemoveImage(client pb.ImageServiceClient, image string) (*pb.RemoveImageResponse, error) {
	if image == "" {
		return nil, fmt.Errorf("ID cannot be empty")
	}
	return client.RemoveImage(context.Background(), &pb.RemoveImageRequest{Image: &pb.ImageSpec{Image: image}})
}
