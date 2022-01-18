package plugin

import (
	"bytes"
	"context"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/containers/podman/v4/libpod/define"
	"github.com/docker/go-plugins-helpers/sdk"
	"github.com/docker/go-plugins-helpers/volume"
	jsoniter "github.com/json-iterator/go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

// TODO: We should add syntax for specifying plugins to containers.conf, and
// support for loading based on that.

// Copied from docker/go-plugins-helpers/volume/api.go - not exported, so we
// need to do this to get at them.
// These are well-established paths that should not change unless the plugin API
// version changes.
var (
	activatePath    = "/Plugin.Activate"
	createPath      = "/VolumeDriver.Create"
	getPath         = "/VolumeDriver.Get"
	listPath        = "/VolumeDriver.List"
	removePath      = "/VolumeDriver.Remove"
	hostVirtualPath = "/VolumeDriver.Path"
	mountPath       = "/VolumeDriver.Mount"
	unmountPath     = "/VolumeDriver.Unmount"
	// nolint
	capabilitiesPath = "/VolumeDriver.Capabilities"
)

const (
	defaultTimeout   = 5 * time.Second
	volumePluginType = "VolumeDriver"
)

var (
	ErrNotPlugin       = errors.New("target does not appear to be a valid plugin")
	ErrNotVolumePlugin = errors.New("plugin is not a volume plugin")
	ErrPluginRemoved   = errors.New("plugin is no longer available (shut down?)")

	// This stores available, initialized volume plugins.
	pluginsLock sync.Mutex
	plugins     map[string]*VolumePlugin
)

// VolumePlugin is a single volume plugin.
type VolumePlugin struct {
	// Name is the name of the volume plugin. This will be used to refer to
	// it.
	Name string
	// SocketPath is the unix socket at which the plugin is accessed.
	SocketPath string
	// Client is the HTTP client we use to connect to the plugin.
	Client *http.Client
}

// This is the response from the activate endpoint of the API.
type activateResponse struct {
	Implements []string
}

// Validate that the given plugin is good to use.
// Add it to available plugins if so.
func validatePlugin(newPlugin *VolumePlugin) error {
	// It's a socket. Is it a plugin?
	// Hit the Activate endpoint to find out if it is, and if so what kind
	req, err := http.NewRequest("POST", "http://plugin"+activatePath, nil)
	if err != nil {
		return errors.Wrapf(err, "error making request to volume plugin %s activation endpoint", newPlugin.Name)
	}

	req.Header.Set("Host", newPlugin.getURI())
	req.Header.Set("Content-Type", sdk.DefaultContentTypeV1_1)

	resp, err := newPlugin.Client.Do(req)
	if err != nil {
		return errors.Wrapf(err, "error sending request to plugin %s activation endpoint", newPlugin.Name)
	}
	defer resp.Body.Close()

	// Response code MUST be 200. Anything else, we have to assume it's not
	// a valid plugin.
	if resp.StatusCode != 200 {
		return errors.Wrapf(ErrNotPlugin, "got status code %d from activation endpoint for plugin %s", resp.StatusCode, newPlugin.Name)
	}

	// Read and decode the body so we can tell if this is a volume plugin.
	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrapf(err, "error reading activation response body from plugin %s", newPlugin.Name)
	}

	respStruct := new(activateResponse)
	if err := json.Unmarshal(respBytes, respStruct); err != nil {
		return errors.Wrapf(err, "error unmarshalling plugin %s activation response", newPlugin.Name)
	}

	foundVolume := false
	for _, pluginType := range respStruct.Implements {
		if pluginType == volumePluginType {
			foundVolume = true
			break
		}
	}

	if !foundVolume {
		return errors.Wrapf(ErrNotVolumePlugin, "plugin %s does not implement volume plugin, instead provides %s", newPlugin.Name, strings.Join(respStruct.Implements, ", "))
	}

	if plugins == nil {
		plugins = make(map[string]*VolumePlugin)
	}

	plugins[newPlugin.Name] = newPlugin

	return nil
}

// GetVolumePlugin gets a single volume plugin, with the given name, at the
// given path.
func GetVolumePlugin(name string, path string) (*VolumePlugin, error) {
	pluginsLock.Lock()
	defer pluginsLock.Unlock()

	plugin, exists := plugins[name]
	if exists {
		// This shouldn't be possible, but just in case...
		if plugin.SocketPath != filepath.Clean(path) {
			return nil, errors.Wrapf(define.ErrInvalidArg, "requested path %q for volume plugin %s does not match pre-existing path for plugin, %q", path, name, plugin.SocketPath)
		}

		return plugin, nil
	}

	// It's not cached. We need to get it.

	newPlugin := new(VolumePlugin)
	newPlugin.Name = name
	newPlugin.SocketPath = filepath.Clean(path)

	// Need an HTTP client to force a Unix connection.
	// And since we can reuse it, might as well cache it.
	client := new(http.Client)
	client.Timeout = defaultTimeout
	// This bit borrowed from pkg/bindings/connection.go
	client.Transport = &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", newPlugin.SocketPath)
		},
		DisableCompression: true,
	}
	newPlugin.Client = client

	stat, err := os.Stat(newPlugin.SocketPath)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot access plugin %s socket %q", name, newPlugin.SocketPath)
	}
	if stat.Mode()&os.ModeSocket == 0 {
		return nil, errors.Wrapf(ErrNotPlugin, "volume %s path %q is not a unix socket", name, newPlugin.SocketPath)
	}

	if err := validatePlugin(newPlugin); err != nil {
		return nil, err
	}

	return newPlugin, nil
}

func (p *VolumePlugin) getURI() string {
	return "unix://" + p.SocketPath
}

// Verify the plugin is still available.
// TODO: Do we want to ping with an HTTP request? There's no ping endpoint so
// we'd need to hit Activate or Capabilities?
func (p *VolumePlugin) verifyReachable() error {
	if _, err := os.Stat(p.SocketPath); err != nil {
		if os.IsNotExist(err) {
			pluginsLock.Lock()
			defer pluginsLock.Unlock()
			delete(plugins, p.Name)
			return errors.Wrapf(ErrPluginRemoved, p.Name)
		}

		return errors.Wrapf(err, "error accessing plugin %s", p.Name)
	}
	return nil
}

// Send a request to the volume plugin for handling.
// Callers *MUST* close the response when they are done.
func (p *VolumePlugin) sendRequest(toJSON interface{}, hasBody bool, endpoint string) (*http.Response, error) {
	var (
		reqJSON []byte
		err     error
	)

	if hasBody {
		reqJSON, err = json.Marshal(toJSON)
		if err != nil {
			return nil, errors.Wrapf(err, "error marshalling request JSON for volume plugin %s endpoint %s", p.Name, endpoint)
		}
	}

	req, err := http.NewRequest("POST", "http://plugin"+endpoint, bytes.NewReader(reqJSON))
	if err != nil {
		return nil, errors.Wrapf(err, "error making request to volume plugin %s endpoint %s", p.Name, endpoint)
	}

	req.Header.Set("Host", p.getURI())
	req.Header.Set("Content-Type", sdk.DefaultContentTypeV1_1)

	resp, err := p.Client.Do(req)
	if err != nil {
		return nil, errors.Wrapf(err, "error sending request to volume plugin %s endpoint %s", p.Name, endpoint)
	}
	// We are *deliberately not closing* response here. It is the
	// responsibility of the caller to do so after reading the response.

	return resp, nil
}

// Turn an error response from a volume plugin into a well-formatted Go error.
func (p *VolumePlugin) makeErrorResponse(err, endpoint, volName string) error {
	if err == "" {
		err = "empty error from plugin"
	}
	if volName != "" {
		return errors.Wrapf(errors.New(err), "error on %s on volume %s in volume plugin %s", endpoint, volName, p.Name)
	}
	return errors.Wrapf(errors.New(err), "error on %s in volume plugin %s", endpoint, p.Name)
}

// Handle error responses from plugin
func (p *VolumePlugin) handleErrorResponse(resp *http.Response, endpoint, volName string) error {
	// The official plugin reference implementation uses HTTP 500 for
	// errors, but I don't think we can guarantee all plugins do that.
	// Let's interpret anything other than 200 as an error.
	// If there isn't an error, don't even bother decoding the response.
	if resp.StatusCode != 200 {
		errResp, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return errors.Wrapf(err, "error reading response body from volume plugin %s", p.Name)
		}

		errStruct := new(volume.ErrorResponse)
		if err := json.Unmarshal(errResp, errStruct); err != nil {
			return errors.Wrapf(err, "error unmarshalling JSON response from volume plugin %s", p.Name)
		}

		return p.makeErrorResponse(errStruct.Err, endpoint, volName)
	}

	return nil
}

// CreateVolume creates a volume in the plugin.
func (p *VolumePlugin) CreateVolume(req *volume.CreateRequest) error {
	if req == nil {
		return errors.Wrapf(define.ErrInvalidArg, "must provide non-nil request to CreateVolume")
	}

	if err := p.verifyReachable(); err != nil {
		return err
	}

	logrus.Infof("Creating volume %s using plugin %s", req.Name, p.Name)

	resp, err := p.sendRequest(req, true, createPath)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return p.handleErrorResponse(resp, createPath, req.Name)
}

// ListVolumes lists volumes available in the plugin.
func (p *VolumePlugin) ListVolumes() ([]*volume.Volume, error) {
	if err := p.verifyReachable(); err != nil {
		return nil, err
	}

	logrus.Infof("Listing volumes using plugin %s", p.Name)

	resp, err := p.sendRequest(nil, false, listPath)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := p.handleErrorResponse(resp, listPath, ""); err != nil {
		return nil, err
	}

	// TODO: Can probably unify response reading under a helper
	volumeRespBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrapf(err, "error reading response body from volume plugin %s", p.Name)
	}

	volumeResp := new(volume.ListResponse)
	if err := json.Unmarshal(volumeRespBytes, volumeResp); err != nil {
		return nil, errors.Wrapf(err, "error unmarshalling volume plugin %s list response", p.Name)
	}

	return volumeResp.Volumes, nil
}

// GetVolume gets a single volume from the plugin.
func (p *VolumePlugin) GetVolume(req *volume.GetRequest) (*volume.Volume, error) {
	if req == nil {
		return nil, errors.Wrapf(define.ErrInvalidArg, "must provide non-nil request to GetVolume")
	}

	if err := p.verifyReachable(); err != nil {
		return nil, err
	}

	logrus.Infof("Getting volume %s using plugin %s", req.Name, p.Name)

	resp, err := p.sendRequest(req, true, getPath)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := p.handleErrorResponse(resp, getPath, req.Name); err != nil {
		return nil, err
	}

	getRespBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrapf(err, "error reading response body from volume plugin %s", p.Name)
	}

	getResp := new(volume.GetResponse)
	if err := json.Unmarshal(getRespBytes, getResp); err != nil {
		return nil, errors.Wrapf(err, "error unmarshalling volume plugin %s get response", p.Name)
	}

	return getResp.Volume, nil
}

// RemoveVolume removes a single volume from the plugin.
func (p *VolumePlugin) RemoveVolume(req *volume.RemoveRequest) error {
	if req == nil {
		return errors.Wrapf(define.ErrInvalidArg, "must provide non-nil request to RemoveVolume")
	}

	if err := p.verifyReachable(); err != nil {
		return err
	}

	logrus.Infof("Removing volume %s using plugin %s", req.Name, p.Name)

	resp, err := p.sendRequest(req, true, removePath)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return p.handleErrorResponse(resp, removePath, req.Name)
}

// GetVolumePath gets the path the given volume is mounted at.
func (p *VolumePlugin) GetVolumePath(req *volume.PathRequest) (string, error) {
	if req == nil {
		return "", errors.Wrapf(define.ErrInvalidArg, "must provide non-nil request to GetVolumePath")
	}

	if err := p.verifyReachable(); err != nil {
		return "", err
	}

	logrus.Infof("Getting volume %s path using plugin %s", req.Name, p.Name)

	resp, err := p.sendRequest(req, true, hostVirtualPath)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if err := p.handleErrorResponse(resp, hostVirtualPath, req.Name); err != nil {
		return "", err
	}

	pathRespBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Wrapf(err, "error reading response body from volume plugin %s", p.Name)
	}

	pathResp := new(volume.PathResponse)
	if err := json.Unmarshal(pathRespBytes, pathResp); err != nil {
		return "", errors.Wrapf(err, "error unmarshalling volume plugin %s path response", p.Name)
	}

	return pathResp.Mountpoint, nil
}

// MountVolume mounts the given volume. The ID argument is the ID of the
// mounting container, used for internal record-keeping by the plugin. Returns
// the path the volume has been mounted at.
func (p *VolumePlugin) MountVolume(req *volume.MountRequest) (string, error) {
	if req == nil {
		return "", errors.Wrapf(define.ErrInvalidArg, "must provide non-nil request to MountVolume")
	}

	if err := p.verifyReachable(); err != nil {
		return "", err
	}

	logrus.Infof("Mounting volume %s using plugin %s for container %s", req.Name, p.Name, req.ID)

	resp, err := p.sendRequest(req, true, mountPath)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if err := p.handleErrorResponse(resp, mountPath, req.Name); err != nil {
		return "", err
	}

	mountRespBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Wrapf(err, "error reading response body from volume plugin %s", p.Name)
	}

	mountResp := new(volume.MountResponse)
	if err := json.Unmarshal(mountRespBytes, mountResp); err != nil {
		return "", errors.Wrapf(err, "error unmarshalling volume plugin %s path response", p.Name)
	}

	return mountResp.Mountpoint, nil
}

// UnmountVolume unmounts the given volume. The ID argument is the ID of the
// container that is unmounting, used for internal record-keeping by the plugin.
func (p *VolumePlugin) UnmountVolume(req *volume.UnmountRequest) error {
	if req == nil {
		return errors.Wrapf(define.ErrInvalidArg, "must provide non-nil request to UnmountVolume")
	}

	if err := p.verifyReachable(); err != nil {
		return err
	}

	logrus.Infof("Unmounting volume %s using plugin %s for container %s", req.Name, p.Name, req.ID)

	resp, err := p.sendRequest(req, true, unmountPath)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return p.handleErrorResponse(resp, unmountPath, req.Name)
}
