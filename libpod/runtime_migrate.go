//go:build !remote

package libpod

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/containers/podman/v5/libpod/define"
	"github.com/sirupsen/logrus"
	"go.podman.io/common/pkg/config"
	"go.podman.io/storage/pkg/fileutils"
)

// Migrate stops the rootless pause process and performs any necessary database
// migrations that are required. It can also migrate all containers to a new OCI
// runtime, if requested.
func (r *Runtime) Migrate(newRuntime string, migrateDB bool) error {
	// Acquire the alive lock and hold it.
	// Ensures that we don't let other Podman commands run while we are
	// rewriting things in the DB.
	aliveLock, err := r.getRuntimeAliveLock()
	if err != nil {
		return fmt.Errorf("retrieving alive lock: %w", err)
	}
	aliveLock.Lock()
	defer aliveLock.Unlock()

	if !r.valid {
		return define.ErrRuntimeStopped
	}

	runningContainers, err := r.GetRunningContainers()
	if err != nil {
		return err
	}

	allCtrs, err := r.state.AllContainers(false)
	if err != nil {
		return err
	}

	logrus.Infof("Stopping all containers")
	for _, ctr := range runningContainers {
		fmt.Printf("stopped %s\n", ctr.ID())
		if err := ctr.Stop(); err != nil {
			return fmt.Errorf("cannot stop container %s: %w", ctr.ID(), err)
		}
	}

	// Did the user request a new runtime?
	runtimeChangeRequested := newRuntime != ""
	var requestedRuntime OCIRuntime
	if runtimeChangeRequested {
		runtime, exists := r.ociRuntimes[newRuntime]
		if !exists {
			return fmt.Errorf("change to runtime %q requested but no such runtime is defined: %w", newRuntime, define.ErrInvalidArg)
		}
		requestedRuntime = runtime
	}

	for _, ctr := range allCtrs {
		needsWrite := false

		// Reset pause process location
		oldLocation := filepath.Join(ctr.state.RunDir, "conmon.pid")
		if ctr.config.ConmonPidFile == oldLocation {
			logrus.Infof("Changing conmon PID file for %s", ctr.ID())
			ctr.config.ConmonPidFile = filepath.Join(ctr.config.StaticDir, "conmon.pid")
			needsWrite = true
		}

		// Reset runtime
		if runtimeChangeRequested && ctr.config.OCIRuntime != newRuntime {
			logrus.Infof("Resetting container %s runtime to runtime %s", ctr.ID(), newRuntime)
			ctr.config.OCIRuntime = newRuntime
			ctr.ociRuntime = requestedRuntime

			needsWrite = true
		}

		if needsWrite {
			if err := r.state.RewriteContainerConfig(ctr, ctr.config); err != nil {
				return fmt.Errorf("rewriting config for container %s: %w", ctr.ID(), err)
			}
		}
	}

	if migrateDB {
		if err := r.checkCanMigrate(); err != nil {
			switch {
			case errors.Is(err, errCannotMigrateSqlite):
				fmt.Printf("No migration is necessary: %v", err)
				return r.stopPauseProcess()
			case errors.Is(err, errCannotMigrateHardcodedBolt):
				logrus.Errorf("In containers.conf, database_backend is manually set to \"boltdb\" - comment this line out and run `podman system migrate --migrate-db` to complete migration to SQLite")
				return fmt.Errorf("unable to migrate to SQLite database as database backend manually set")
			default:
				return err
			}
		}

		if err := r.migrateDB(); err != nil {
			return fmt.Errorf("migrating database from BoltDB to SQLite: %w", err)
		}
	}

	return r.stopPauseProcess()
}

var (
	errCannotMigrateSqlite        = errors.New("backing database is already SQLite")
	errCannotMigrateHardcodedBolt = errors.New("database_backend in containers.conf is manually set to \"boltdb\"")
)

func (r *Runtime) checkCanMigrate() error {
	if r.state.Type() == "sqlite" {
		return errCannotMigrateSqlite
	}

	// Necessary as database configuration is overwritten when the state is set up.
	// So we need a completely new state from disk to see what the user set.
	newCfg, err := config.New(nil)
	if err != nil {
		return fmt.Errorf("reloading configuration to check database backend in use")
	}
	backend, err := config.ParseDBBackend(newCfg.Engine.DBBackend)
	if err != nil {
		return fmt.Errorf("invalid DB backend configured - please change containers.conf database_backend to \"sqlite\"")
	}
	if backend == config.DBBackendBoltDB {
		return errCannotMigrateHardcodedBolt
	}

	return nil
}

func (r *Runtime) migrateDB() error {
	// Does the SQLite state already exist?
	dbBasePath, dbFilename := sqliteStatePath(r)
	dbPath := filepath.Join(dbBasePath, dbFilename)
	if err := fileutils.Exists(dbPath); err == nil {
		return fmt.Errorf("a SQLite database already exists at %s, refusing to overwrite", dbPath)
	}

	sqlState, err := NewSqliteState(r)
	if err != nil {
		return fmt.Errorf("creating SQLite database: %w", err)
	}

	// Recreate the cached paths in the new database
	if err := sqlState.ValidateDBConfig(r); err != nil {
		return fmt.Errorf("recreating config table: %w", err)
	}

	// Migrate volumes, then pods, then containers.
	// Containers must be last as the pods they are part of and volumes they use must already exist.
	allVolumes, err := r.state.AllVolumes()
	if err != nil {
		return fmt.Errorf("retrieving volumes from boltdb: %w", err)
	}
	for _, vol := range allVolumes {
		if err := sqlState.AddVolume(vol); err != nil {
			return err
		}
		if err := vol.update(); err != nil {
			return err
		}
		if err := sqlState.SaveVolume(vol); err != nil {
			return err
		}
	}

	allPods, err := r.state.AllPods()
	if err != nil {
		return fmt.Errorf("retrieving pods from boltdb: %w", err)
	}
	for _, pod := range allPods {
		if err := sqlState.AddPod(pod); err != nil {
			return err
		}
		if err := pod.updatePod(); err != nil {
			return err
		}
		if err := sqlState.SavePod(pod); err != nil {
			return err
		}
	}

	// Containers must be done as a graph due to dependencies.
	// The state will error if we add a container before its dependencies.
	allCtrs, err := r.state.AllContainers(true)
	if err != nil {
		return fmt.Errorf("retrieving containers from boltdb: %w", err)
	}

	// BoltDB doesn't actually populate container networks on initial pull
	// from the database, that needs to be done separately.
	for _, ctr := range allCtrs {
		ctrNetworks, err := r.state.GetNetworks(ctr)
		if err != nil {
			return err
		}
		ctr.config.Networks = ctrNetworks
	}

	graph, err := BuildContainerGraph(allCtrs)
	if err != nil {
		return err
	}

	ctrErrors := make(map[string]error)
	ctrsVisited := make(map[string]bool)

	for _, node := range graph.noDepNodes {
		migrateNodeDatabase(node, false, ctrErrors, ctrsVisited, sqlState)
	}
	var ctrError error
	for id, err := range ctrErrors {
		if ctrError != nil {
			logrus.Errorf("Migrating containers to SQLite: %v", ctrError)
		}
		ctrError = fmt.Errorf("migrating container %s: %w", id, err)
	}
	if ctrError != nil {
		return ctrError
	}

	boltState := r.state
	r.state = sqlState

	// Move the Bolt database so it is not reused, but preserve so data is not lost.
	boltDBPath := getBoltDBPath(r)
	newBoltDBPath := fmt.Sprintf("%s-old", boltDBPath)
	if err := os.Rename(boltDBPath, newBoltDBPath); err != nil {
		// Restore the old state. Migration has failed.
		r.state = boltState

		return fmt.Errorf("renaming old database %s to %s: %w", boltDBPath, newBoltDBPath, err)
	}
	fmt.Printf("Old database has been renamed to %s and will no longer be used\n", newBoltDBPath)

	// Only after we handle all failure cases that could require a revert.
	boltState.Close()

	return nil
}
