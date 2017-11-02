package libkpod

import (
	"fmt"

	cstorage "github.com/containers/storage"
	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/libkpod/sandbox"
	"github.com/projectatomic/libpod/oci"
	"github.com/projectatomic/libpod/pkg/registrar"
)

// GetStorageContainer searches for a container with the given name or ID in the given store
func (c *ContainerServer) GetStorageContainer(container string) (*cstorage.Container, error) {
	ociCtr, err := c.LookupContainer(container)
	if err != nil {
		return nil, err
	}
	return c.store.Container(ociCtr.ID())
}

// GetContainerTopLayerID gets the ID of the top layer of the given container
func (c *ContainerServer) GetContainerTopLayerID(containerID string) (string, error) {
	ctr, err := c.GetStorageContainer(containerID)
	if err != nil {
		return "", err
	}
	return ctr.LayerID, nil
}

// GetContainerRwSize Gets the size of the mutable top layer of the container
func (c *ContainerServer) GetContainerRwSize(containerID string) (int64, error) {
	container, err := c.store.Container(containerID)
	if err != nil {
		return 0, err
	}

	// Get the size of the top layer by calculating the size of the diff
	// between the layer and its parent.  The top layer of a container is
	// the only RW layer, all others are immutable
	layer, err := c.store.Layer(container.LayerID)
	if err != nil {
		return 0, err
	}
	return c.store.DiffSize(layer.Parent, layer.ID)
}

// GetContainerRootFsSize gets the size of the container's root filesystem
// A container FS is split into two parts.  The first is the top layer, a
// mutable layer, and the rest is the RootFS: the set of immutable layers
// that make up the image on which the container is based
func (c *ContainerServer) GetContainerRootFsSize(containerID string) (int64, error) {
	container, err := c.store.Container(containerID)
	if err != nil {
		return 0, err
	}

	// Ignore the size of the top layer.   The top layer is a mutable RW layer
	// and is not considered a part of the rootfs
	rwLayer, err := c.store.Layer(container.LayerID)
	if err != nil {
		return 0, err
	}
	layer, err := c.store.Layer(rwLayer.Parent)
	if err != nil {
		return 0, err
	}

	size := int64(0)
	for layer.Parent != "" {
		layerSize, err := c.store.DiffSize(layer.Parent, layer.ID)
		if err != nil {
			return size, errors.Wrapf(err, "getting diffsize of layer %q and its parent %q", layer.ID, layer.Parent)
		}
		size += layerSize
		layer, err = c.store.Layer(layer.Parent)
		if err != nil {
			return 0, err
		}
	}
	// Get the size of the last layer.  Has to be outside of the loop
	// because the parent of the last layer is "", andlstore.Get("")
	// will return an error
	layerSize, err := c.store.DiffSize(layer.Parent, layer.ID)
	return size + layerSize, err
}

// GetContainerFromRequest gets an oci container matching the specified full or partial id
func (c *ContainerServer) GetContainerFromRequest(cid string) (*oci.Container, error) {
	if cid == "" {
		return nil, fmt.Errorf("container ID should not be empty")
	}

	containerID, err := c.ctrIDIndex.Get(cid)
	if err != nil {
		return nil, fmt.Errorf("container with ID starting with %s not found: %v", cid, err)
	}

	ctr := c.GetContainer(containerID)
	if ctr == nil {
		return nil, fmt.Errorf("specified container not found: %s", containerID)
	}
	return ctr, nil
}

func (c *ContainerServer) getSandboxFromRequest(pid string) (*sandbox.Sandbox, error) {
	if pid == "" {
		return nil, fmt.Errorf("pod ID should not be empty")
	}

	podID, err := c.podIDIndex.Get(pid)
	if err != nil {
		return nil, fmt.Errorf("pod with ID starting with %s not found: %v", pid, err)
	}

	sb := c.GetSandbox(podID)
	if sb == nil {
		return nil, fmt.Errorf("specified pod not found: %s", podID)
	}
	return sb, nil
}

// LookupContainer returns the container with the given name or full or partial id
func (c *ContainerServer) LookupContainer(idOrName string) (*oci.Container, error) {
	if idOrName == "" {
		return nil, fmt.Errorf("container ID or name should not be empty")
	}

	ctrID, err := c.ctrNameIndex.Get(idOrName)
	if err != nil {
		if err == registrar.ErrNameNotReserved {
			ctrID = idOrName
		} else {
			return nil, err
		}
	}

	return c.GetContainerFromRequest(ctrID)
}

// LookupSandbox returns the pod sandbox with the given name or full or partial id
func (c *ContainerServer) LookupSandbox(idOrName string) (*sandbox.Sandbox, error) {
	if idOrName == "" {
		return nil, fmt.Errorf("container ID or name should not be empty")
	}

	podID, err := c.podNameIndex.Get(idOrName)
	if err != nil {
		if err == registrar.ErrNameNotReserved {
			podID = idOrName
		} else {
			return nil, err
		}
	}

	return c.getSandboxFromRequest(podID)
}
