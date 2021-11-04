package libpod

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/containers/common/libimage"
	"github.com/containers/common/libnetwork/types"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/errorhandling"
	"github.com/containers/podman/v4/pkg/rootless"
	"github.com/containers/podman/v4/pkg/util"
	"github.com/containers/storage"
	storageTypes "github.com/containers/storage/types"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Reset removes all storage
func (r *Runtime) Reset(ctx context.Context, updateConf bool) error {
	var timeout *uint
	pods, err := r.GetAllPods()
	if err != nil {
		return err
	}
	for _, p := range pods {
		if err := r.RemovePod(ctx, p, true, true, timeout); err != nil {
			if errors.Cause(err) == define.ErrNoSuchPod {
				continue
			}
			logrus.Errorf("Removing Pod %s: %v", p.ID(), err)
		}
	}

	ctrs, err := r.GetAllContainers()
	if err != nil {
		return err
	}

	for _, c := range ctrs {
		if err := r.RemoveContainer(ctx, c, true, true, timeout); err != nil {
			if err := r.RemoveStorageContainer(c.ID(), true); err != nil {
				if errors.Cause(err) == define.ErrNoSuchCtr {
					continue
				}
				logrus.Errorf("Removing container %s: %v", c.ID(), err)
			}
		}
	}

	if err := r.stopPauseProcess(); err != nil {
		logrus.Errorf("Stopping pause process: %v", err)
	}

	rmiOptions := &libimage.RemoveImagesOptions{Filters: []string{"readonly=false"}}
	if _, rmiErrors := r.LibimageRuntime().RemoveImages(ctx, nil, rmiOptions); rmiErrors != nil {
		return errorhandling.JoinErrors(rmiErrors)
	}

	volumes, err := r.state.AllVolumes()
	if err != nil {
		return err
	}
	for _, v := range volumes {
		if err := r.RemoveVolume(ctx, v, true, timeout); err != nil {
			if errors.Cause(err) == define.ErrNoSuchVolume {
				continue
			}
			logrus.Errorf("Removing volume %s: %v", v.config.Name, err)
		}
	}

	// remove all networks
	nets, err := r.network.NetworkList()
	if err != nil {
		return err
	}
	for _, net := range nets {
		// do not delete the default network
		if net.Name == r.network.DefaultNetworkName() {
			continue
		}
		// ignore not exists errors because of the TOCTOU problem
		if err := r.network.NetworkRemove(net.Name); err != nil && !errors.Is(err, types.ErrNoSuchNetwork) {
			logrus.Errorf("Removing network %s: %v", net.Name, err)
		}
	}

	xdgRuntimeDir := filepath.Clean(os.Getenv("XDG_RUNTIME_DIR"))
	_, prevError := r.store.Shutdown(true)
	graphRoot := filepath.Clean(r.store.GraphRoot())
	if graphRoot == xdgRuntimeDir {
		if prevError != nil {
			logrus.Error(prevError)
		}
		prevError = errors.Errorf("failed to remove runtime graph root dir %s, since it is the same as XDG_RUNTIME_DIR", graphRoot)
	} else {
		if err := os.RemoveAll(graphRoot); err != nil {
			if prevError != nil {
				logrus.Error(prevError)
			}
			prevError = err
		}
	}
	runRoot := filepath.Clean(r.store.RunRoot())
	if runRoot == xdgRuntimeDir {
		if prevError != nil {
			logrus.Error(prevError)
		}
		prevError = errors.Errorf("failed to remove runtime root dir %s, since it is the same as XDG_RUNTIME_DIR", runRoot)
	} else {
		if err := os.RemoveAll(runRoot); err != nil {
			if prevError != nil {
				logrus.Error(prevError)
			}
			prevError = err
		}
	}
	runtimeDir, err := util.GetRuntimeDir()
	if err != nil {
		return err
	}
	tempDir := r.config.Engine.TmpDir
	if tempDir == runtimeDir {
		tempDir = filepath.Join(tempDir, "containers")
	}
	if filepath.Clean(tempDir) == xdgRuntimeDir {
		if prevError != nil {
			logrus.Error(prevError)
		}
		prevError = errors.Errorf("failed to remove runtime tmpdir %s, since it is the same as XDG_RUNTIME_DIR", tempDir)
	} else {
		if err := os.RemoveAll(tempDir); err != nil {
			if prevError != nil {
				logrus.Error(prevError)
			}
			prevError = err
		}
	}
	if storageConfPath, err := storage.DefaultConfigFile(rootless.IsRootless()); err == nil {
		if _, err := os.Stat(storageConfPath); err == nil {
			fmt.Printf("A storage.conf file exists at %s\n", storageConfPath)
			fmt.Println("you can remove it or override it with the --run-root and --graph-root options.")
			if updateConf { // only update the config if we have a storage.conf, we do not want to create a new one
				store, err := storageTypes.StorageConfig(rootless.IsRootless())
				if err != nil {
					return err
				}
				store.Storage.RunRoot = r.store.RunRoot()
				store.Storage.GraphRoot = r.store.GraphRoot()
				store.Storage.Driver = r.store.GraphDriverName()

				err = storageTypes.Save(*store, rootless.IsRootless())
				if err != nil {
					return err
				}

				fmt.Println("Wrote new storage config to", storageConfPath)
			}
		} else {
			logrus.Warnf("storage.conf path %s does not exist. Please create a storage.conf file before attempting to modify", storageConfPath)
		}
	} else {
		if prevError != nil {
			logrus.Error(prevError)
		}
		prevError = err
	}

	return prevError
}
