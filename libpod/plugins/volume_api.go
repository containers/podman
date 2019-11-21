package plugin

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/containers/libpod/libpod/define"
	"github.com/docker/go-plugins-helpers/sdk"
	"github.com/docker/go-plugins-helpers/volume"
	jsoniter "github.com/json-iterator/go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

// TODO: load plugins from plugin configs
// - https://docs.docker.com/engine/extend/config/
// TODO: Handle plugin .spec files
// - literal URL in a file
// - https://docs.docker.com/engine/extend/plugin_api/

// Copied from docker/go-plugins-helpers/volume/api.go - not exported, so we
// need to do this to get at them.
// These are well-established paths that should not change unless the plugin API
// version changes.
var (
	activatePath     = "/Plugin.Activate"
	createPath       = "/VolumeDriver.Create"
	getPath          = "/VolumeDriver.Get"
	listPath         = "/VolumeDriver.List"
	removePath       = "/VolumeDriver.Remove"
	hostVirtualPath  = "/VolumeDriver.Path"
	mountPath        = "/VolumeDriver.Mount"
	unmountPath      = "/VolumeDriver.Unmount"
	capabilitiesPath = "/VolumeDriver.Capabilities"
)

const (
	// TODO: should we sync this with Docker?
	defaultTimeout   = 5 * time.Second
	defaultPath      = "/run/docker/plugins"
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
	req, err := http.NewRequest("POST", activatePath, nil)
	if err != nil {
		return errors.Wrapf(err, "error making request to volume plugin %s activation endpoint", newPlugin.Name)
	}

	req.Header.Set("Host", p.getURI())
	req.Header.Set("Content-Type", sdk.DefaultContentTypeV1_1)

	client := new(http.Client)
	client.Timeout = defaultTimeout
	resp, err := client.Do(req)
	if err != nil {
		return errors.Wrapf(err, "error sending request to plugin %s activation endpoint", newPlugin.Name)
	}

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
		return errors.Wrapf(err, "error unmarshaling plugin %s activation response", newPlugin.Name)
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

	plugins[newPlugin.Name] = newPlugin

	return nil
}

// GetVolumePlugin gets a single volume plugin by path.
func GetVolumePlugin(name string) (*VolumePlugin, error) {
	pluginsLock.Lock()
	defer pluginsLock.Unlock()

	plugin, exists := plugins[name]
	if exists {
		return plugin, nil
	}

	// It's not cached. We need to get it.

	newPlugin := new(VolumePlugin)
	newPlugin.Name = name
	newPlugin.Path = filepath.Join(defaultPath, fmt.Sprintf("%s.sock", name))

	stat, err := os.Stat(newPlugin.Path)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot access plugin %s socket %q", name, newPlugin.Path)
	}
	if stat.Mode()&os.ModeSocket == 0 {
		return nil, errors.Wrapf(ErrNotPlugin, "volume %s path %q is not a unix socket", name, newPlugin.Path)
	}

	if err := validatePlugin(newPlugin); err != nil {
		return nil, err
	}

	return newPlugin, nil
}

// GetPluginsFromDirectory populates the list of available volume plugins using
// the given directory. All unix sockets will be discovered and added as plugins.
// A list of names of disovered plugins is returned; the actual plugins can be
// retrieved via GetVolumePlugin.
// Errors retrieving plugins will be ignored; the only errors returned will be
// those that prevent any discovery from taking place.
func GetPluginsFromDirectory(dir string) ([]string, error) {
	stat, err := os.Stat(dir)
	if err != nil {
		// If the dir doesn't exist, don't report an error for legacy
		// compat. Instead, just report no plugins available.
		if os.IsNotExist(err) {
			return []string{}, nil
		}

		return errors.Wrapf(err, "error running stat on plugin directory %s", dir)
	}
	if !stat.IsDir() {
		return errors.Wrapf(define.ErrInvalidArg, "plugin directory %s is not a directory", dir)
	}

	// Find everything in the folder that's a unix socket
	contents, err := ioutil.ReadDir(dir)
	if err != nil {
		return errors.Wrapf(err, "error reading directory %s contents", dir)
	}

	goodPlugins := []string{}

	for _, file := range contents {
		// Only look at unix sockets
		if file.Mode()&os.ModeSocket != 0 {
			newPlugin := new(VolumePlugin)
			newPlugin.Name = strings.TrimSuffix(filepath.Base(file.Name()), ".sock")

			sockPath, err = filepath.Abs(filepath.Join(dir, filepath.Base(file.Name())))
			if err != nil {
				logrus.Errorf("Error getting absolute path of potential plugin %s socket", file.Name())
				continue
			}
			newPlugin.SocketPath = sockPath

			if err := validatePlugin(newPlugin); err != nil {
				logrus.Warnf("Error validating plugin %s: %v", newPlugin.Name, err)
			}

			goodPlugins = append(goodPlugins. newPlugin.Name)
		}
	}

	return goodPlugins, nil
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

	req, err := http.NewRequest("POST", endpoint, reqJSON)
	if err != nil {
		return nil, errors.Wrapf(err, "error making request to volume plugin %s endpoint %s", p.Name, endpoint)
	}

	req.Header.Set("Host", p.getURI())
	req.Header.Set("Content-Type", sdk.DefaultContentTypeV1_1)

	client := new(http.Client)
	client.Timeout = defaultTimeout
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrapf(err, "error sending request to volume plugin %s endpoint %s", p.Name, endpoint)
	}

	return resp, nil
}

// Turn an error response from a volume plugin into a well-formatted Go error.
func (p *VolumePlugin) makeErrorResponse(err string) error {
	if err == nil {
		err = "empty error from plugin"
	}
	return errors.Wrapf(errors.New(err), "error creating volume %s in volume plugin %s", req.Name, p.Name)
}

// Handle error responses from plugin
func (p *VolumePlugin) handleErrorResponse(resp *http.Response) error {
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

		return p.makeErrorResponse(errStruct.Err)
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

	return p.handleErrorResponse(resp)
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

	if err := p.handleErrorResponse(resp); err != nil {
		return nil, err
	}

	// TODO: Can probably unify response reading under a helper
	volumeRespBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrapf(err, "error reading response body from volume plugin %s", p.Name)
	}

	volumeResp := new(volume.ListResponse)
	if err := json.Unmarshal(volumeRespBytes, volumeResp); err != nil {
		return nil, errors.Wrapf(err, "error unmarshaling volume plugin %s list response", p.Name)
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

	if err := p.handleErrorResponse(resp); err != nil {
		return nil, err
	}

	getRespBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrapf(err, "error reading response body from volume plugin %s", p.Name)
	}

	getResp := new(volume.GetResponse)
	if err := json.Unmarshal(getRespBytes, getResp); err != nil {
		return nil, errors.Wrapf(err, "error unmarshaling volume plugin %s get response", p.Name)
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

	return p.handleErrorResponse(resp)
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

	if err := p.handleErrorResponse(resp); err != nil {
		return "", err
	}

	pathRespBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Wrapf(err, "error reading response body from volume plugin %s", p.Name)
	}

	pathResp := new(volume.PathResponse)
	if err := json.Unmarshal(pathRespBytes, pathResp); err != nil {
		return "", errors.Wrapf(err, "error unmarshaling volume plugin %s path response", p.Name)
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

	if err := p.handleErrorResponse(resp); err != nil {
		return "", err
	}

	mountRespBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Wrapf(err, "error reading response body from volume plugin %s", p.Name)
	}

	mountResp := new(volume.MountResponse)
	if err := json.Unmarshal(mountRespBytes, mountResp); err != nil {
		return "", errors.Wrapf(err, "error unmarshaling volume plugin %s path response", p.Name)
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

	return p.handleErrorResponse(resp)
}
