package libpod

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/containers/podman/v2/libpod/define"
	"github.com/containers/podman/v2/pkg/rootless"
	"github.com/containers/podman/v2/pkg/util"
	"github.com/containers/storage"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Reset removes all storage
func (r *Runtime) Reset(ctx context.Context) error {

	pods, err := r.GetAllPods()
	if err != nil {
		return err
	}
	for _, p := range pods {
		if err := r.RemovePod(ctx, p, true, true); err != nil {
			if errors.Cause(err) == define.ErrNoSuchPod {
				continue
			}
			logrus.Errorf("Error removing Pod %s: %v", p.ID(), err)
		}
	}

	ctrs, err := r.GetAllContainers()
	if err != nil {
		return err
	}

	for _, c := range ctrs {
		if err := r.RemoveContainer(ctx, c, true, true); err != nil {
			if err := r.RemoveStorageContainer(c.ID(), true); err != nil {
				if errors.Cause(err) == define.ErrNoSuchCtr {
					continue
				}
				logrus.Errorf("Error removing container %s: %v", c.ID(), err)
			}
		}
	}

	if err := r.stopPauseProcess(); err != nil {
		logrus.Errorf("Error stopping pause process: %v", err)
	}

	ir := r.ImageRuntime()
	images, err := ir.GetImages()
	if err != nil {
		return err
	}

	for _, i := range images {
		if err := i.Remove(ctx, true); err != nil {
			if errors.Cause(err) == define.ErrNoSuchImage {
				continue
			}
			logrus.Errorf("Error removing image %s: %v", i.ID(), err)
		}
	}
	volumes, err := r.state.AllVolumes()
	if err != nil {
		return err
	}
	for _, v := range volumes {
		if err := r.RemoveVolume(ctx, v, true); err != nil {
			if errors.Cause(err) == define.ErrNoSuchVolume {
				continue
			}
			logrus.Errorf("Error removing volume %s: %v", v.config.Name, err)
		}
	}

	_, prevError := r.store.Shutdown(true)
	if err := os.RemoveAll(r.store.GraphRoot()); err != nil {
		if prevError != nil {
			logrus.Error(prevError)
		}
		prevError = err
	}
	if err := os.RemoveAll(r.store.RunRoot()); err != nil {
		if prevError != nil {
			logrus.Error(prevError)
		}
		prevError = err
	}
	runtimeDir, err := util.GetRuntimeDir()
	if err != nil {
		return err
	}
	tempDir := r.config.Engine.TmpDir
	if tempDir == runtimeDir {
		tempDir = filepath.Join(tempDir, "containers")
	}
	if err := os.RemoveAll(tempDir); err != nil {
		if prevError != nil {
			logrus.Error(prevError)
		}
		prevError = err
	}

	if storageConfPath, err := storage.DefaultConfigFile(rootless.IsRootless()); err == nil {
		if _, err = os.Stat(storageConfPath); err == nil {
			fmt.Printf("A storage.conf file exists at %s\n", storageConfPath)
			fmt.Println("You should remove this file if you did not modified the configuration.")
		}
	} else {
		if prevError != nil {
			logrus.Error(prevError)
		}
		prevError = err
	}

	return prevError
}
