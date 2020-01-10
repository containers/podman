package bindings

import (
	"fmt"
	"net/http"
)

func (c Connection) MountContainer(nameOrID string) (string, error) {
	var (
		path string
	)
	response, err := c.newRequest(http.MethodPost, fmt.Sprintf("/containers/%s/mount", nameOrID), nil, nil)
	if err != nil {
		return path, err
	}
	return path, response.Process(&path)
}

func (c Connection) GetMountedContainerPaths() (map[string]string, error) {
	mounts := make(map[string]string)
	response, err := c.newRequest(http.MethodGet, "/containers/showmounted", nil, nil)
	if err != nil {
		return mounts, err
	}
	return mounts, response.Process(&mounts)
}
