package bindings

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

/*
	All methods still need error handling defined based on the http response codes.
*/

func (c Connection) MountContainer(nameOrID string) (string, error) {
	var (
		path string
	)
	response, err := http.Get(c.makeEndpoint(fmt.Sprintf("containers/%s/mount", nameOrID)))
	if err != nil {
		return path, err
	}
	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return path, err
	}
	err = json.Unmarshal(data, &path)
	return path, err
}

func (c Connection) GetMountedContainerPaths() (map[string]string, error) {
	response, err := http.Get(c.makeEndpoint("containers/showmounted"))
	if err != nil {
		return nil, err
	}
	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	mounts := make(map[string]string)
	err = json.Unmarshal(data, &mounts)
	return mounts, err
}
