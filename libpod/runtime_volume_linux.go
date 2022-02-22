// +build linux

package libpod

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/libpod/events"
	volplugin "github.com/containers/podman/v4/libpod/plugin"
	"github.com/containers/storage/drivers/quota"
	"github.com/containers/storage/pkg/stringid"
	pluginapi "github.com/docker/go-plugins-helpers/volume"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// NewVolume creates a new empty volume
func (r *Runtime) NewVolume(ctx context.Context, options ...VolumeCreateOption) (*Volume, error) {
	if !r.valid {
		return nil, define.ErrRuntimeStopped
	}
	return r.newVolume(ctx, options...)
}

// newVolume creates a new empty volume
func (r *Runtime) newVolume(ctx context.Context, options ...VolumeCreateOption) (_ *Volume, deferredErr error) {
	volume := newVolume(r)
	for _, option := range options {
		if err := option(volume); err != nil {
			return nil, errors.Wrapf(err, "running volume create option")
		}
	}

	if volume.config.Name == "" {
		volume.config.Name = stringid.GenerateNonCryptoID()
	}
	if volume.config.Driver == "" {
		volume.config.Driver = define.VolumeDriverLocal
	}
	volume.config.CreatedTime = time.Now()

	// Check if volume with given name exists.
	exists, err := r.state.HasVolume(volume.config.Name)
	if err != nil {
		return nil, errors.Wrapf(err, "checking if volume with name %s exists", volume.config.Name)
	}
	if exists {
		return nil, errors.Wrapf(define.ErrVolumeExists, "volume with name %s already exists", volume.config.Name)
	}

	// Plugin can be nil if driver is local, but that's OK - superfluous
	// assignment doesn't hurt much.
	plugin, err := r.getVolumePlugin(volume.config.Driver)
	if err != nil {
		return nil, errors.Wrapf(err, "volume %s uses volume plugin %s but it could not be retrieved", volume.config.Name, volume.config.Driver)
	}
	volume.plugin = plugin

	if volume.config.Driver == define.VolumeDriverLocal {
		logrus.Debugf("Validating options for local driver")
		// Validate options
		for key, val := range volume.config.Options {
			switch strings.ToLower(key) {
			case "device":
				if strings.ToLower(volume.config.Options["type"]) == "bind" {
					if _, err := os.Stat(val); err != nil {
						return nil, errors.Wrapf(err, "invalid volume option %s for driver 'local'", key)
					}
				}
			case "o", "type", "uid", "gid", "size", "inodes":
				// Do nothing, valid keys
			default:
				return nil, errors.Wrapf(define.ErrInvalidArg, "invalid mount option %s for driver 'local'", key)
			}
		}
	}

	// Now we get conditional: we either need to make the volume in the
	// volume plugin, or on disk if not using a plugin.
	if volume.plugin != nil {
		// We can't chown, or relabel, or similar the path the volume is
		// using, because it's not managed by us.
		// TODO: reevaluate this once we actually have volume plugins in
		// use in production - it may be safe, but I can't tell without
		// knowing what the actual plugin does...
		if err := makeVolumeInPluginIfNotExist(volume.config.Name, volume.config.Options, volume.plugin); err != nil {
			return nil, err
		}
	} else {
		// Create the mountpoint of this volume
		volPathRoot := filepath.Join(r.config.Engine.VolumePath, volume.config.Name)
		if err := os.MkdirAll(volPathRoot, 0700); err != nil {
			return nil, errors.Wrapf(err, "creating volume directory %q", volPathRoot)
		}
		if err := os.Chown(volPathRoot, volume.config.UID, volume.config.GID); err != nil {
			return nil, errors.Wrapf(err, "chowning volume directory %q to %d:%d", volPathRoot, volume.config.UID, volume.config.GID)
		}
		fullVolPath := filepath.Join(volPathRoot, "_data")
		if err := os.MkdirAll(fullVolPath, 0755); err != nil {
			return nil, errors.Wrapf(err, "creating volume directory %q", fullVolPath)
		}
		if err := os.Chown(fullVolPath, volume.config.UID, volume.config.GID); err != nil {
			return nil, errors.Wrapf(err, "chowning volume directory %q to %d:%d", fullVolPath, volume.config.UID, volume.config.GID)
		}
		if err := LabelVolumePath(fullVolPath); err != nil {
			return nil, err
		}
		projectQuotaSupported := false

		q, err := quota.NewControl(r.config.Engine.VolumePath)
		if err == nil {
			projectQuotaSupported = true
		}
		quota := quota.Quota{}
		if volume.config.Size > 0 || volume.config.Inodes > 0 {
			if !projectQuotaSupported {
				return nil, errors.New("Volume options size and inodes not supported. Filesystem does not support Project Quota")
			}
			quota.Size = volume.config.Size
			quota.Inodes = volume.config.Inodes
		}
		if projectQuotaSupported {
			if err := q.SetQuota(fullVolPath, quota); err != nil {
				return nil, errors.Wrapf(err, "failed to set size quota size=%d inodes=%d for volume directory %q", volume.config.Size, volume.config.Inodes, fullVolPath)
			}
		}

		volume.config.MountPoint = fullVolPath
	}

	lock, err := r.lockManager.AllocateLock()
	if err != nil {
		return nil, errors.Wrapf(err, "allocating lock for new volume")
	}
	volume.lock = lock
	volume.config.LockID = volume.lock.ID()

	defer func() {
		if deferredErr != nil {
			if err := volume.lock.Free(); err != nil {
				logrus.Errorf("Freeing volume lock after failed creation: %v", err)
			}
		}
	}()

	volume.valid = true

	// Add the volume to state
	if err := r.state.AddVolume(volume); err != nil {
		return nil, errors.Wrapf(err, "adding volume to state")
	}
	defer volume.newVolumeEvent(events.Create)
	return volume, nil
}

// makeVolumeInPluginIfNotExist makes a volume in the given volume plugin if it
// does not already exist.
func makeVolumeInPluginIfNotExist(name string, options map[string]string, plugin *volplugin.VolumePlugin) error {
	// Ping the volume plugin to see if it exists first.
	// If it does, use the existing volume in the plugin.
	// Options may not match exactly, but not much we can do about
	// that. Not complaining avoids a lot of the sync issues we see
	// with c/storage and libpod DB.
	needsCreate := true
	getReq := new(pluginapi.GetRequest)
	getReq.Name = name
	if resp, err := plugin.GetVolume(getReq); err == nil {
		// TODO: What do we do if we get a 200 response, but the
		// Volume is nil? The docs on the Plugin API are very
		// nonspecific, so I don't know if this is valid or
		// not...
		if resp != nil {
			needsCreate = false
			logrus.Infof("Volume %q already exists in plugin %q, using existing volume", name, plugin.Name)
		}
	}
	if needsCreate {
		createReq := new(pluginapi.CreateRequest)
		createReq.Name = name
		createReq.Options = options
		if err := plugin.CreateVolume(createReq); err != nil {
			return errors.Wrapf(err, "creating volume %q in plugin %s", name, plugin.Name)
		}
	}

	return nil
}

// removeVolume removes the specified volume from state as well tears down its mountpoint and storage
func (r *Runtime) removeVolume(ctx context.Context, v *Volume, force bool, timeout *uint) error {
	if !v.valid {
		if ok, _ := r.state.HasVolume(v.Name()); !ok {
			return nil
		}
		return define.ErrVolumeRemoved
	}

	v.lock.Lock()
	defer v.lock.Unlock()

	// Update volume status to pick up a potential removal from state
	if err := v.update(); err != nil {
		return err
	}

	deps, err := r.state.VolumeInUse(v)
	if err != nil {
		return err
	}
	if len(deps) != 0 {
		depsStr := strings.Join(deps, ", ")
		if !force {
			return errors.Wrapf(define.ErrVolumeBeingUsed, "volume %s is being used by the following container(s): %s", v.Name(), depsStr)
		}

		// We need to remove all containers using the volume
		for _, dep := range deps {
			ctr, err := r.state.Container(dep)
			if err != nil {
				// If the container's removed, no point in
				// erroring.
				if errors.Cause(err) == define.ErrNoSuchCtr || errors.Cause(err) == define.ErrCtrRemoved {
					continue
				}

				return errors.Wrapf(err, "removing container %s that depends on volume %s", dep, v.Name())
			}

			logrus.Debugf("Removing container %s (depends on volume %q)", ctr.ID(), v.Name())

			if err := r.removeContainer(ctx, ctr, force, false, false, timeout); err != nil {
				return errors.Wrapf(err, "removing container %s that depends on volume %s", ctr.ID(), v.Name())
			}
		}
	}

	// If the volume is still mounted - force unmount it
	if err := v.unmount(true); err != nil {
		if force {
			// If force is set, evict the volume, even if errors
			// occur. Otherwise we'll never be able to get rid of
			// them.
			logrus.Errorf("Unmounting volume %s: %v", v.Name(), err)
		} else {
			return errors.Wrapf(err, "unmounting volume %s", v.Name())
		}
	}

	// Set volume as invalid so it can no longer be used
	v.valid = false

	var removalErr error

	// If we use a volume plugin, we need to remove from the plugin.
	if v.UsesVolumeDriver() {
		canRemove := true

		// Do we have a volume driver?
		if v.plugin == nil {
			canRemove = false
			removalErr = errors.Wrapf(define.ErrMissingPlugin, "cannot remove volume %s from plugin %s, but it has been removed from Podman", v.Name(), v.Driver())
		} else {
			// Ping the plugin first to verify the volume still
			// exists.
			// We're trying to be very tolerant of missing volumes
			// in the backend, to avoid the problems we see with
			// sync between c/storage and the Libpod DB.
			getReq := new(pluginapi.GetRequest)
			getReq.Name = v.Name()
			if _, err := v.plugin.GetVolume(getReq); err != nil {
				canRemove = false
				removalErr = errors.Wrapf(err, "volume %s could not be retrieved from plugin %s, but it has been removed from Podman", v.Name(), v.Driver())
			}
		}
		if canRemove {
			req := new(pluginapi.RemoveRequest)
			req.Name = v.Name()
			if err := v.plugin.RemoveVolume(req); err != nil {
				return errors.Wrapf(err, "volume %s could not be removed from plugin %s", v.Name(), v.Driver())
			}
		}
	}

	// Remove the volume from the state
	if err := r.state.RemoveVolume(v); err != nil {
		if removalErr != nil {
			logrus.Errorf("Removing volume %s from plugin %s: %v", v.Name(), v.Driver(), removalErr)
		}
		return errors.Wrapf(err, "removing volume %s", v.Name())
	}

	// Free the volume's lock
	if err := v.lock.Free(); err != nil {
		if removalErr == nil {
			removalErr = errors.Wrapf(err, "freeing lock for volume %s", v.Name())
		} else {
			logrus.Errorf("Freeing lock for volume %q: %v", v.Name(), err)
		}
	}

	// Delete the mountpoint path of the volume, that is delete the volume
	// from /var/lib/containers/storage/volumes
	if err := v.teardownStorage(); err != nil {
		if removalErr == nil {
			removalErr = errors.Wrapf(err, "cleaning up volume storage for %q", v.Name())
		} else {
			logrus.Errorf("Cleaning up volume storage for volume %q: %v", v.Name(), err)
		}
	}

	defer v.newVolumeEvent(events.Remove)
	logrus.Debugf("Removed volume %s", v.Name())
	return removalErr
}
