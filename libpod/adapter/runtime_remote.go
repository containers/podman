// +build remoteclient

package adapter

import (
	"context"
	"fmt"
	"strings"
	"time"

	iopodman "github.com/containers/libpod/cmd/podman/varlink"
	digest "github.com/opencontainers/go-digest"
	"github.com/urfave/cli"
	"github.com/varlink/go/varlink"
)

// ImageRuntime is wrapper for image runtime
type RemoteImageRuntime struct{}

// RemoteRuntime describes a wrapper runtime struct
type RemoteRuntime struct {
}

// LocalRuntime describes a typical libpod runtime
type LocalRuntime struct {
	Runtime *RemoteRuntime
	Remote  bool
	Conn    *varlink.Connection
}

// GetRuntime returns a LocalRuntime struct with the actual runtime embedded in it
func GetRuntime(c *cli.Context) (*LocalRuntime, error) {
	runtime := RemoteRuntime{}
	conn, err := runtime.Connect()
	if err != nil {
		return nil, err
	}
	return &LocalRuntime{
		Runtime: &runtime,
		Remote:  true,
		Conn:    conn,
	}, nil
}

// Shutdown is a bogus wrapper for compat with the libpod runtime
func (r RemoteRuntime) Shutdown(force bool) error {
	return nil
}

// ContainerImage
type ContainerImage struct {
	remoteImage
}

type remoteImage struct {
	ID          string
	Labels      map[string]string
	RepoTags    []string
	RepoDigests []string
	Parent      string
	Size        int64
	Created     time.Time
	InputName   string
	Names       []string
	Digest      digest.Digest
	isParent    bool
	Runtime     *LocalRuntime
}

// GetImages returns a slice of containerimages over a varlink connection
func (r *LocalRuntime) GetImages() ([]*ContainerImage, error) {
	var newImages []*ContainerImage
	images, err := iopodman.ListImages().Call(r.Conn)
	if err != nil {
		return nil, err
	}
	for _, i := range images {
		name := i.Id
		if len(i.RepoTags) > 1 {
			name = i.RepoTags[0]
		}
		newImage, err := imageInListToContainerImage(i, name, r)
		if err != nil {
			return nil, err
		}
		newImages = append(newImages, newImage)
	}
	return newImages, nil
}

func imageInListToContainerImage(i iopodman.ImageInList, name string, runtime *LocalRuntime) (*ContainerImage, error) {
	created, err := splitStringDate(i.Created)
	if err != nil {
		return nil, err
	}
	ri := remoteImage{
		InputName:   name,
		ID:          i.Id,
		Labels:      i.Labels,
		RepoTags:    i.RepoTags,
		RepoDigests: i.RepoTags,
		Parent:      i.ParentId,
		Size:        i.Size,
		Created:     created,
		Names:       i.RepoTags,
		isParent:    i.IsParent,
		Runtime:     runtime,
	}
	return &ContainerImage{ri}, nil
}

// NewImageFromLocal returns a container image representation of a image over varlink
func (r *LocalRuntime) NewImageFromLocal(name string) (*ContainerImage, error) {
	img, err := iopodman.GetImage().Call(r.Conn, name)
	if err != nil {
		return nil, err
	}
	return imageInListToContainerImage(img, name, r)

}

func splitStringDate(d string) (time.Time, error) {
	fields := strings.Fields(d)
	t := fmt.Sprintf("%sT%sZ", fields[0], fields[1])
	return time.ParseInLocation(time.RFC3339Nano, t, time.UTC)
}

// IsParent goes through the layers in the store and checks if i.TopLayer is
// the parent of any other layer in store. Double check that image with that
// layer exists as well.
func (ci *ContainerImage) IsParent() (bool, error) {
	return ci.remoteImage.isParent, nil
}

// ID returns the image ID as a string
func (ci *ContainerImage) ID() string {
	return ci.remoteImage.ID
}

// Names returns a string array of names associated with the image
func (ci *ContainerImage) Names() []string {
	return ci.remoteImage.Names
}

// Created returns the time the image was created
func (ci *ContainerImage) Created() time.Time {
	return ci.remoteImage.Created
}

// Size returns the size of the image
func (ci *ContainerImage) Size(ctx context.Context) (*uint64, error) {
	usize := uint64(ci.remoteImage.Size)
	return &usize, nil
}

// Digest returns the image's digest
func (ci *ContainerImage) Digest() digest.Digest {
	return ci.remoteImage.Digest
}

// Labels returns a map of the image's labels
func (ci *ContainerImage) Labels(ctx context.Context) (map[string]string, error) {
	return ci.remoteImage.Labels, nil
}

// Dangling returns a bool if the image is "dangling"
func (ci *ContainerImage) Dangling() bool {
	return len(ci.Names()) == 0
}

// TagImage ...
func (ci *ContainerImage) TagImage(tag string) error {
	_, err := iopodman.TagImage().Call(ci.Runtime.Conn, ci.ID(), tag)
	return err
}

func (r RemoteRuntime) RemoveImage(force bool) error {
	return nil
}
