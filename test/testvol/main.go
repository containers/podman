package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/docker/go-plugins-helpers/volume"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "testvol",
	Short: "testvol - volume plugin for Podman",
	Long:  `Creates simple directory volumes using the Volume Plugin API for testing volume plugin functionality`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return startServer(config.sockName)
	},
	PersistentPreRunE: before,
}

// Configuration for the volume plugin
type cliConfig struct {
	logLevel string
	sockName string
	path     string
}

// Default configuration is stored here. Will be overwritten by flags.
var config = cliConfig{
	logLevel: "error",
	sockName: "test-volume-plugin",
}

func init() {
	rootCmd.Flags().StringVar(&config.sockName, "sock-name", config.sockName, "Name of unix socket for plugin")
	rootCmd.Flags().StringVar(&config.path, "path", "", "Path to initialize state and mount points")
	rootCmd.PersistentFlags().StringVar(&config.logLevel, "log-level", config.logLevel, "Log messages including and over the specified level: debug, info, warn, error, fatal, panic")
}

func before(cmd *cobra.Command, args []string) error {
	if config.logLevel == "" {
		config.logLevel = "error"
	}

	level, err := logrus.ParseLevel(config.logLevel)
	if err != nil {
		return err
	}

	logrus.SetLevel(level)

	return nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		logrus.Errorf("Running volume plugin: %v", err)
		os.Exit(1)
	}

	os.Exit(0)
}

// startServer runs the HTTP server and responds to requests
func startServer(socketPath string) error {
	logrus.Debugf("Starting server...")

	if config.path == "" {
		path, err := ioutil.TempDir("", "test_volume_plugin")
		if err != nil {
			return errors.Wrapf(err, "error getting directory for plugin")
		}
		config.path = path
	} else {
		pathStat, err := os.Stat(config.path)
		if err != nil {
			return errors.Wrapf(err, "unable to access requested plugin state directory")
		}
		if !pathStat.IsDir() {
			return errors.Errorf("cannot use %v as plugin state dir as it is not a directory", config.path)
		}
	}

	handle := makeDirDriver(config.path)
	logrus.Infof("Using %s for volume path", config.path)

	server := volume.NewHandler(handle)
	if err := server.ServeUnix(socketPath, 0); err != nil {
		return errors.Wrapf(err, "error starting server")
	}
	return nil
}

// DirDriver is a trivial volume driver implementation.
// the volumes field maps name to volume
type DirDriver struct {
	lock        sync.Mutex
	volumesPath string
	volumes     map[string]*dirVol
}

type dirVol struct {
	name       string
	path       string
	options    map[string]string
	mounts     map[string]bool
	createTime time.Time
}

// Make a new DirDriver.
func makeDirDriver(path string) volume.Driver {
	drv := new(DirDriver)
	drv.volumesPath = path
	drv.volumes = make(map[string]*dirVol)

	return drv
}

// Capabilities returns the capabilities of the driver.
func (d *DirDriver) Capabilities() *volume.CapabilitiesResponse {
	logrus.Infof("Hit Capabilities() endpoint")

	return &volume.CapabilitiesResponse{
		Capabilities: volume.Capability{
			Scope: "local",
		},
	}
}

// Create creates a volume.
func (d *DirDriver) Create(opts *volume.CreateRequest) error {
	d.lock.Lock()
	defer d.lock.Unlock()

	logrus.Infof("Hit Create() endpoint")

	if _, exists := d.volumes[opts.Name]; exists {
		return errors.Errorf("volume with name %s already exists", opts.Name)
	}

	newVol := new(dirVol)
	newVol.name = opts.Name
	newVol.mounts = make(map[string]bool)
	newVol.options = make(map[string]string)
	newVol.createTime = time.Now()
	for k, v := range opts.Options {
		newVol.options[k] = v
	}

	volPath := filepath.Join(d.volumesPath, opts.Name)
	if err := os.Mkdir(volPath, 0755); err != nil {
		return errors.Wrapf(err, "error making volume directory")
	}
	newVol.path = volPath

	d.volumes[opts.Name] = newVol

	logrus.Debugf("Made volume with name %s and path %s", newVol.name, newVol.path)

	return nil
}

// List lists all volumes available.
func (d *DirDriver) List() (*volume.ListResponse, error) {
	d.lock.Lock()
	defer d.lock.Unlock()

	logrus.Infof("Hit List() endpoint")

	vols := new(volume.ListResponse)
	vols.Volumes = []*volume.Volume{}

	for _, vol := range d.volumes {
		newVol := new(volume.Volume)
		newVol.Name = vol.name
		newVol.Mountpoint = vol.path
		newVol.CreatedAt = vol.createTime.String()
		vols.Volumes = append(vols.Volumes, newVol)
		logrus.Debugf("Adding volume %s to list response", newVol.Name)
	}

	return vols, nil
}

// Get retrieves a single volume.
func (d *DirDriver) Get(req *volume.GetRequest) (*volume.GetResponse, error) {
	d.lock.Lock()
	defer d.lock.Unlock()

	logrus.Infof("Hit Get() endpoint")

	vol, exists := d.volumes[req.Name]
	if !exists {
		logrus.Debugf("Did not find volume %s", req.Name)
		return nil, errors.Errorf("no volume with name %s found", req.Name)
	}

	logrus.Debugf("Found volume %s", req.Name)

	resp := new(volume.GetResponse)
	resp.Volume = new(volume.Volume)
	resp.Volume.Name = vol.name
	resp.Volume.Mountpoint = vol.path
	resp.Volume.CreatedAt = vol.createTime.String()

	return resp, nil
}

// Remove removes a single volume.
func (d *DirDriver) Remove(req *volume.RemoveRequest) error {
	d.lock.Lock()
	defer d.lock.Unlock()

	logrus.Infof("Hit Remove() endpoint")

	vol, exists := d.volumes[req.Name]
	if !exists {
		logrus.Debugf("Did not find volume %s", req.Name)
		return errors.Errorf("no volume with name %s found", req.Name)
	}
	logrus.Debugf("Found volume %s", req.Name)

	if len(vol.mounts) > 0 {
		logrus.Debugf("Cannot remove %s, is mounted", req.Name)
		return errors.Errorf("volume %s is mounted and cannot be removed", req.Name)
	}

	delete(d.volumes, req.Name)

	if err := os.RemoveAll(vol.path); err != nil {
		return errors.Wrapf(err, "error removing mountpoint of volume %s", req.Name)
	}

	logrus.Debugf("Removed volume %s", req.Name)

	return nil
}

// Path returns the path a single volume is mounted at.
func (d *DirDriver) Path(req *volume.PathRequest) (*volume.PathResponse, error) {
	d.lock.Lock()
	defer d.lock.Unlock()

	logrus.Infof("Hit Path() endpoint")

	// TODO: Should we return error if not mounted?

	vol, exists := d.volumes[req.Name]
	if !exists {
		logrus.Debugf("Cannot locate volume %s", req.Name)
		return nil, errors.Errorf("no volume with name %s found", req.Name)
	}

	return &volume.PathResponse{
		Mountpoint: vol.path,
	}, nil
}

// Mount mounts the volume.
func (d *DirDriver) Mount(req *volume.MountRequest) (*volume.MountResponse, error) {
	d.lock.Lock()
	defer d.lock.Unlock()

	logrus.Infof("Hit Mount() endpoint")

	vol, exists := d.volumes[req.Name]
	if !exists {
		logrus.Debugf("Cannot locate volume %s", req.Name)
		return nil, errors.Errorf("no volume with name %s found", req.Name)
	}

	vol.mounts[req.ID] = true

	return &volume.MountResponse{
		Mountpoint: vol.path,
	}, nil
}

// Unmount unmounts the volume.
func (d *DirDriver) Unmount(req *volume.UnmountRequest) error {
	d.lock.Lock()
	defer d.lock.Unlock()

	logrus.Infof("Hit Unmount() endpoint")

	vol, exists := d.volumes[req.Name]
	if !exists {
		logrus.Debugf("Cannot locate volume %s", req.Name)
		return errors.Errorf("no volume with name %s found", req.Name)
	}

	mount := vol.mounts[req.ID]
	if !mount {
		logrus.Debugf("Volume %s is not mounted by %s", req.Name, req.ID)
		return errors.Errorf("volume %s is not mounted by %s", req.Name, req.ID)
	}

	delete(vol.mounts, req.ID)

	return nil
}
