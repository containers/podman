package libpod

import (
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	goruntime "runtime"
	"time"

	"github.com/containers/common/libnetwork/types"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/rootless"
	"github.com/containers/storage"
	"github.com/sirupsen/logrus"

	// SQLite backend for database/sql
	_ "github.com/mattn/go-sqlite3"
)

const schemaVersion = 1

// SQLiteState is a state implementation backed by a SQLite database
type SQLiteState struct {
	valid   bool
	conn    *sql.DB
	runtime *Runtime
}

// NewBoltState creates a new bolt-backed state database
func NewSqliteState(runtime *Runtime) (_ State, defErr error) {
	state := new(SQLiteState)

	conn, err := sql.Open("sqlite3", filepath.Join(runtime.storageConfig.GraphRoot, "db.sql?_loc=auto"))
	if err != nil {
		return nil, fmt.Errorf("initializing sqlite database: %w", err)
	}
	defer func() {
		if defErr != nil {
			if err := conn.Close(); err != nil {
				logrus.Errorf("Error closing SQLite DB connection: %v", err)
			}
		}
	}()

	state.conn = conn

	if err := state.conn.Ping(); err != nil {
		return nil, fmt.Errorf("cannot connect to database: %w", err)
	}

	if _, err := state.conn.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		return nil, fmt.Errorf("enabling foreign key support in database: %w", err)
	}
	if _, err := state.conn.Exec("PRAGMA journal_mode = WAL;"); err != nil {
		return nil, fmt.Errorf("switching journal to WAL mode: %w", err)
	}
	if _, err := state.conn.Exec("PRAGMA synchronous = FULL;"); err != nil {
		return nil, fmt.Errorf("setting full fsync mode in db: %w", err)
	}

	if err := state.migrateSchemaIfNecessary(); err != nil {
		return nil, err
	}

	// Set up tables
	if err := sqliteInitTables(state.conn); err != nil {
		return nil, fmt.Errorf("creating tables: %w", err)
	}

	state.valid = true
	state.runtime = runtime

	return state, nil
}

// Close closes the state and prevents further use
func (s *SQLiteState) Close() error {
	if err := s.conn.Close(); err != nil {
		return err
	}

	s.valid = false
	return nil
}

// Refresh clears container and pod states after a reboot
func (s *SQLiteState) Refresh() (defErr error) {
	if !s.valid {
		return define.ErrDBClosed
	}

	// Retrieve all containers, pods, and volumes.
	// Maps are indexed by ID (or volume name) so we know which goes where,
	// and store the marshalled state JSON
	ctrStates := make(map[string]string)
	podStates := make(map[string]string)
	volumeStates := make(map[string]string)

	ctrRows, err := s.conn.Query("SELECT Id, Json FROM ContainerState;")
	if err != nil {
		return fmt.Errorf("querying for container states: %w", err)
	}
	defer ctrRows.Close()

	for ctrRows.Next() {
		var (
			id, stateJSON string
		)
		if err := ctrRows.Scan(&id, &stateJSON); err != nil {
			return fmt.Errorf("scanning container state row: %w", err)
		}

		ctrState := new(ContainerState)

		if err := json.Unmarshal([]byte(stateJSON), ctrState); err != nil {
			return fmt.Errorf("unmarshalling container state json: %w", err)
		}

		// Refresh the state
		resetContainerState(ctrState)

		newJSON, err := json.Marshal(ctrState)
		if err != nil {
			return fmt.Errorf("marshalling container state json: %w", err)
		}

		ctrStates[id] = string(newJSON)
	}

	podRows, err := s.conn.Query("SELECT Id, Json FROM PodState;")
	if err != nil {
		return fmt.Errorf("querying for pod states: %w", err)
	}
	defer podRows.Close()

	for podRows.Next() {
		var (
			id, stateJSON string
		)
		if err := podRows.Scan(&id, &stateJSON); err != nil {
			return fmt.Errorf("scanning pod state row: %w", err)
		}

		podState := new(podState)

		if err := json.Unmarshal([]byte(stateJSON), podState); err != nil {
			return fmt.Errorf("unmarshalling pod state json: %w", err)
		}

		// Refresh the state
		resetPodState(podState)

		newJSON, err := json.Marshal(podState)
		if err != nil {
			return fmt.Errorf("marshalling pod state json: %w", err)
		}

		podStates[id] = string(newJSON)
	}

	volRows, err := s.conn.Query("SELECT Name, Json FROM VolumeState;")
	if err != nil {
		return fmt.Errorf("querying for volume states: %w", err)
	}
	defer volRows.Close()

	for volRows.Next() {
		var (
			name, stateJSON string
		)

		if err := volRows.Scan(&name, &stateJSON); err != nil {
			return fmt.Errorf("scanning volume state row: %w", err)
		}

		volState := new(VolumeState)

		if err := json.Unmarshal([]byte(stateJSON), volState); err != nil {
			return fmt.Errorf("unmarshalling volume state json: %w", err)
		}

		// Refresh the state
		resetVolumeState(volState)

		newJSON, err := json.Marshal(volState)
		if err != nil {
			return fmt.Errorf("marshalling volume state json: %w", err)
		}

		volumeStates[name] = string(newJSON)
	}

	// Write updated states back to DB, and perform additional maintenance
	// (Remove exit codes and exec sessions)

	tx, err := s.conn.Begin()
	if err != nil {
		return fmt.Errorf("beginning refresh transaction: %w", err)
	}
	defer func() {
		if defErr != nil {
			if err := tx.Rollback(); err != nil {
				logrus.Errorf("Error rolling back transaction to refresh database state: %v", err)
			}
		}
	}()

	for id, json := range ctrStates {
		if _, err := tx.Exec("UPDATE TABLE ContainerState SET Json=? WHERE Id=?;", json, id); err != nil {
			return fmt.Errorf("updating container state: %w", err)
		}
	}
	for id, json := range podStates {
		if _, err := tx.Exec("UPDATE TABLE PodState SET Json=? WHERE Id=?;", json, id); err != nil {
			return fmt.Errorf("updating pod state: %w", err)
		}
	}
	for name, json := range volumeStates {
		if _, err := tx.Exec("UPDATE TABLE VolumeState SET Json=? WHERE Name=?;", json, name); err != nil {
			return fmt.Errorf("updating volume state: %w", err)
		}
	}

	if _, err := tx.Exec("DELETE FROM ContainerExitCode;"); err != nil {
		return fmt.Errorf("removing container exit codes: %w", err)
	}

	if _, err := tx.Exec("DELETE FROM ContainerExecSession;"); err != nil {
		return fmt.Errorf("removing container exec sessions: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}

// GetDBConfig retrieves runtime configuration fields that were created when
// the database was first initialized
func (s *SQLiteState) GetDBConfig() (*DBConfig, error) {
	if !s.valid {
		return nil, define.ErrDBClosed
	}

	cfg := new(DBConfig)
	var staticDir, tmpDir, graphRoot, runRoot, graphDriver, volumeDir string

	row := s.conn.QueryRow("SELECT StaticDir, TmpDir, GraphRoot, RunRoot, GraphDriver, VolumeDir FROM DBConfig;")

	if err := row.Scan(&staticDir, &tmpDir, &graphRoot, &runRoot, &graphDriver, &volumeDir); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return cfg, nil
		}
		return nil, fmt.Errorf("retrieving DB config: %w", err)
	}

	cfg.LibpodRoot = staticDir
	cfg.LibpodTmp = tmpDir
	cfg.StorageRoot = graphRoot
	cfg.StorageTmp = runRoot
	cfg.GraphDriver = graphDriver
	cfg.VolumePath = volumeDir

	return cfg, nil
}

// ValidateDBConfig validates paths in the given runtime against the database
func (s *SQLiteState) ValidateDBConfig(runtime *Runtime) (defErr error) {
	if !s.valid {
		return define.ErrDBClosed
	}

	storeOpts, err := storage.DefaultStoreOptions(rootless.IsRootless(), rootless.GetRootlessUID())
	if err != nil {
		return err
	}

	const createRow = `
        INSERT INTO DBconfig VALUES (
                ?, ?, ?,
                ?, ?, ?,
                ?, ?, ?
        );`

	var (
		os, staticDir, tmpDir, graphRoot, runRoot, graphDriver, volumePath string
		runtimeOS                                                          = goruntime.GOOS
		runtimeStaticDir                                                   = filepath.Clean(s.runtime.config.Engine.StaticDir)
		runtimeTmpDir                                                      = filepath.Clean(s.runtime.config.Engine.TmpDir)
		runtimeGraphRoot                                                   = filepath.Clean(s.runtime.StorageConfig().GraphRoot)
		runtimeRunRoot                                                     = filepath.Clean(s.runtime.StorageConfig().RunRoot)
		runtimeGraphDriver                                                 = s.runtime.StorageConfig().GraphDriverName
		runtimeVolumePath                                                  = filepath.Clean(s.runtime.config.Engine.VolumePath)
	)

	// Some fields may be empty, indicating they are set to the default.
	// If so, grab the default from c/storage for them.
	if runtimeGraphRoot == "" {
		runtimeGraphRoot = storeOpts.GraphRoot
	}
	if runtimeRunRoot == "" {
		runtimeRunRoot = storeOpts.RunRoot
	}
	if runtimeGraphDriver == "" {
		runtimeGraphDriver = storeOpts.GraphDriverName
	}

	row := s.conn.QueryRow("SELECT Os, StaticDir, TmpDir, GraphRoot, RunRoot, GraphDriver, VolumeDir FROM DBConfig;")

	if err := row.Scan(&os, &staticDir, &tmpDir, &graphRoot, &runRoot, &graphDriver, &volumePath); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Need to create runtime config info in DB
			tx, err := s.conn.Begin()
			if err != nil {
				return fmt.Errorf("beginning DB config transaction: %w", err)
			}
			defer func() {
				if defErr != nil {
					if err := tx.Rollback(); err != nil {
						logrus.Errorf("Error rolling back transaction to create DB config: %v", err)
					}
				}
			}()

			if _, err := tx.Exec(createRow, 1, schemaVersion, runtimeOS,
				runtimeStaticDir, runtimeTmpDir, runtimeGraphRoot,
				runtimeRunRoot, runtimeGraphDriver, runtimeVolumePath); err != nil {
				return fmt.Errorf("adding DB config row: %w", err)
			}

			if err := tx.Commit(); err != nil {
				return fmt.Errorf("committing DB config transaction: %w", err)
			}

			return nil
		}

		return fmt.Errorf("retrieving DB config: %w", err)
	}

	checkField := func(fieldName, dbVal, ourVal string) error {
		if dbVal != ourVal {
			return fmt.Errorf("database %s %q does not match our %s %q: %w", fieldName, dbVal, fieldName, ourVal, define.ErrDBBadConfig)
		}

		return nil
	}

	if err := checkField("os", os, runtimeOS); err != nil {
		return err
	}
	if err := checkField("static dir", staticDir, runtimeStaticDir); err != nil {
		return err
	}
	if err := checkField("tmp dir", tmpDir, runtimeTmpDir); err != nil {
		return err
	}
	if err := checkField("graph root", graphRoot, runtimeGraphRoot); err != nil {
		return err
	}
	if err := checkField("run root", runRoot, runtimeRunRoot); err != nil {
		return err
	}
	if err := checkField("graph driver", graphDriver, runtimeGraphDriver); err != nil {
		return err
	}
	if err := checkField("volume path", volumePath, runtimeVolumePath); err != nil {
		return err
	}

	return nil
}

// GetContainerName returns the name of the container associated with a given
// ID. Returns ErrNoSuchCtr if the ID does not exist.
func (s *SQLiteState) GetContainerName(id string) (string, error) {
	if id == "" {
		return "", define.ErrEmptyID
	}

	if !s.valid {
		return "", define.ErrDBClosed
	}

	var name string

	row := s.conn.QueryRow("SELECT Name FROM ContainerConfig WHERE Id=?;", id)
	if err := row.Scan(&name); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", define.ErrNoSuchCtr
		}

		return "", fmt.Errorf("looking up container %s name: %w", id, err)
	}

	return name, nil
}

// GetPodName returns the name of the pod associated with a given ID.
// Returns ErrNoSuchPod if the ID does not exist.
func (s *SQLiteState) GetPodName(id string) (string, error) {
	if id == "" {
		return "", define.ErrEmptyID
	}

	if !s.valid {
		return "", define.ErrDBClosed
	}

	var name string

	row := s.conn.QueryRow("SELECT Name FROM PodConfig WHERE Id=?;", id)
	if err := row.Scan(&name); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", define.ErrNoSuchPod
		}

		return "", fmt.Errorf("looking up pod %s name: %w", id, err)
	}

	return name, nil
}

// Container retrieves a single container from the state by its full ID
func (s *SQLiteState) Container(id string) (*Container, error) {
	if id == "" {
		return nil, define.ErrEmptyID
	}

	if !s.valid {
		return nil, define.ErrDBClosed
	}

	ctrConfig, err := s.getCtrConfig(id)
	if err != nil {
		return nil, err
	}

	ctr := new(Container)
	ctr.config = ctrConfig
	ctr.state = new(ContainerState)
	ctr.runtime = s.runtime

	if err := finalizeCtrSqlite(ctr); err != nil {
		return nil, err
	}

	return ctr, nil
}

// LookupContainerID retrieves a container ID from the state by full or unique
// partial ID or name
func (s *SQLiteState) LookupContainerID(idOrName string) (string, error) {
	if idOrName == "" {
		return "", define.ErrEmptyID
	}

	if !s.valid {
		return "", define.ErrDBClosed
	}

	rows, err := s.conn.Query("SELECT Id FROM ContainerConfig WHERE ContainerConfig.Name=? OR (ContainerConfig.Id LIKE ?);", idOrName, idOrName)
	if err != nil {
		return "", fmt.Errorf("looking up container %q in database: %w", idOrName, err)
	}
	defer rows.Close()

	var id string
	foundResult := false
	for rows.Next() {
		if foundResult {
			return "", fmt.Errorf("more than one result for container %q: %w", idOrName, define.ErrCtrExists)
		}

		if err := rows.Scan(&id); err != nil {
			return "", fmt.Errorf("retrieving container %q ID from database: %w", idOrName, err)
		}
	}
	if !foundResult {
		return "", define.ErrNoSuchCtr
	}

	return id, nil
}

// LookupContainer retrieves a container from the state by full or unique
// partial ID or name
func (s *SQLiteState) LookupContainer(idOrName string) (*Container, error) {
	if idOrName == "" {
		return nil, define.ErrEmptyID
	}

	if !s.valid {
		return nil, define.ErrDBClosed
	}

	rows, err := s.conn.Query("SELECT Json FROM ContainerConfig WHERE ContainerConfig.Name=? OR (ContainerConfig.Id LIKE ?);", idOrName, idOrName)
	if err != nil {
		return nil, fmt.Errorf("looking up container %q in database: %w", idOrName, err)
	}
	defer rows.Close()

	var rawJSON string
	foundResult := false
	for rows.Next() {
		if foundResult {
			return nil, fmt.Errorf("more than one result for container %q: %w", idOrName, define.ErrCtrExists)
		}

		if err := rows.Scan(&rawJSON); err != nil {
			return nil, fmt.Errorf("error retrieving container %q ID from database: %w", idOrName, err)
		}
	}
	if !foundResult {
		return nil, define.ErrNoSuchCtr
	}

	ctr := new(Container)
	ctr.config = new(ContainerConfig)
	ctr.state = new(ContainerState)
	ctr.runtime = s.runtime

	if err := json.Unmarshal([]byte(rawJSON), ctr.config); err != nil {
		return nil, fmt.Errorf("unmarshalling container config JSON: %w", err)
	}

	if err := finalizeCtrSqlite(ctr); err != nil {
		return nil, err
	}

	return ctr, nil
}

// HasContainer checks if a container is present in the state
func (s *SQLiteState) HasContainer(id string) (bool, error) {
	if id == "" {
		return false, define.ErrEmptyID
	}

	if !s.valid {
		return false, define.ErrDBClosed
	}

	row := s.conn.QueryRow("SELECT 1 FROM ContainerConfig WHERE Id=?;", id)

	var check int
	if err := row.Scan(&check); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("looking up container %s in database: %w", id, err)
	} else if check != 1 {
		return false, fmt.Errorf("check digit for container %s lookup incorrect: %w", id, define.ErrInternal)
	}

	return true, nil
}

// AddContainer adds a container to the state
// The container being added cannot belong to a pod
func (s *SQLiteState) AddContainer(ctr *Container) error {
	if !s.valid {
		return define.ErrDBClosed
	}

	if !ctr.valid {
		return define.ErrCtrRemoved
	}

	if ctr.config.Pod != "" {
		return fmt.Errorf("cannot add a container that belongs to a pod with AddContainer - use AddContainerToPod: %w", define.ErrInvalidArg)
	}

	return s.addContainer(ctr)
}

// RemoveContainer removes a container from the state
// Only removes containers not in pods - for containers that are a member of a
// pod, use RemoveContainerFromPod
func (s *SQLiteState) RemoveContainer(ctr *Container) error {
	if !s.valid {
		return define.ErrDBClosed
	}

	if ctr.config.Pod != "" {
		return fmt.Errorf("container %s is part of a pod, use RemoveContainerFromPod instead: %w", ctr.ID(), define.ErrPodExists)
	}

	return s.removeContainer(ctr)
}

// UpdateContainer updates a container's state from the database
func (s *SQLiteState) UpdateContainer(ctr *Container) error {
	if !s.valid {
		return define.ErrDBClosed
	}

	if !ctr.valid {
		return define.ErrCtrRemoved
	}

	row := s.conn.QueryRow("SELECT Json FROM ContainerState WHERE Id=?;", ctr.ID())

	var rawJSON string
	if err := row.Scan(&rawJSON); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Container was removed
			ctr.valid = false
			return fmt.Errorf("no container with ID %s found in database: %w", ctr.ID(), define.ErrNoSuchCtr)
		}
	}

	newState := new(ContainerState)
	if err := json.Unmarshal([]byte(rawJSON), newState); err != nil {
		return fmt.Errorf("unmarshalling container %s state JSON: %w", ctr.ID(), err)
	}

	ctr.state = newState

	return nil
}

// SaveContainer saves a container's current state in the database
func (s *SQLiteState) SaveContainer(ctr *Container) (defErr error) {
	if !s.valid {
		return define.ErrDBClosed
	}

	if !ctr.valid {
		return define.ErrCtrRemoved
	}

	stateJSON, err := json.Marshal(ctr.state)
	if err != nil {
		return fmt.Errorf("marshalling container %s state JSON: %w", ctr.ID(), err)
	}

	tx, err := s.conn.Begin()
	if err != nil {
		return fmt.Errorf("beginning container %s save transaction: %w", ctr.ID(), err)
	}
	defer func() {
		if defErr != nil {
			if err := tx.Rollback(); err != nil {
				logrus.Errorf("Error rolling back transaction to save container %s state: %v", ctr.ID(), err)
			}
		}
	}()

	result, err := tx.Exec("UPDATE ContainerState SET Json=? WHERE Id=?;", stateJSON, ctr.ID())
	if err != nil {
		return fmt.Errorf("writing container %s state: %w", ctr.ID(), err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("retrieving container %s save rows affected: %w", ctr.ID(), err)
	}
	if rows == 0 {
		ctr.valid = false
		return define.ErrNoSuchCtr
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing container %s state: %w", ctr.ID(), err)
	}

	return nil
}

// ContainerInUse checks if other containers depend on the given container
// It returns a slice of the IDs of the containers depending on the given
// container. If the slice is empty, no containers depend on the given container
func (s *SQLiteState) ContainerInUse(ctr *Container) ([]string, error) {
	if !s.valid {
		return nil, define.ErrDBClosed
	}

	if !ctr.valid {
		return nil, define.ErrCtrRemoved
	}

	rows, err := s.conn.Query("SELECT Id FROM ContainerDependency WHERE DependencyID=?;", ctr.ID())
	if err != nil {
		return nil, fmt.Errorf("retrieving containers that depend on container %s: %w", ctr.ID(), err)
	}
	defer rows.Close()

	deps := []string{}
	for rows.Next() {
		var dep string
		if err := rows.Scan(&dep); err != nil {
			return nil, fmt.Errorf("reading containers that depend on %s: %w", ctr.ID(), err)
		}
		deps = append(deps, dep)
	}

	return deps, nil
}

// AllContainers retrieves all the containers in the database
// If `loadState` is set, the containers' state will be loaded as well.
func (s *SQLiteState) AllContainers(loadState bool) ([]*Container, error) {
	if !s.valid {
		return nil, define.ErrDBClosed
	}

	ctrs := []*Container{}

	if loadState {
		rows, err := s.conn.Query("SELECT ContainerConfig.Json, ContainerState.Json AS StateJson INNER JOIN ContainerState ON ContainerConfig.Id = ContainerState.Id;")
		if err != nil {
			return nil, fmt.Errorf("retrieving all containers from database: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var configJSON, stateJSON string
			if err := rows.Scan(&configJSON, &stateJSON); err != nil {
				return nil, fmt.Errorf("scanning container from database: %w", err)
			}

			ctr := new(Container)
			ctr.config = new(ContainerConfig)
			ctr.state = new(ContainerState)
			ctr.runtime = s.runtime

			if err := json.Unmarshal([]byte(configJSON), ctr.config); err != nil {
				return nil, fmt.Errorf("unmarshalling container config: %w", err)
			}
			if err := json.Unmarshal([]byte(stateJSON), ctr.config); err != nil {
				return nil, fmt.Errorf("unmarshalling container %s config: %w", ctr.ID(), err)
			}

			ctrs = append(ctrs, ctr)
		}
	} else {
		rows, err := s.conn.Query("SELECT Json FROM ContainerConfig;")
		if err != nil {
			return nil, fmt.Errorf("retrieving all containers from database: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var rawJSON string
			if err := rows.Scan(&rawJSON); err != nil {
				return nil, fmt.Errorf("scanning container from database: %w", err)
			}

			ctr := new(Container)
			ctr.config = new(ContainerConfig)
			ctr.state = new(ContainerState)
			ctr.runtime = s.runtime

			if err := json.Unmarshal([]byte(rawJSON), ctr.config); err != nil {
				return nil, fmt.Errorf("unmarshalling container config: %w", err)
			}

			ctrs = append(ctrs, ctr)
		}
	}

	for _, ctr := range ctrs {
		if err := finalizeCtrSqlite(ctr); err != nil {
			return nil, err
		}
	}

	return ctrs, nil
}

// GetNetworks returns the networks this container is a part of.
func (s *SQLiteState) GetNetworks(ctr *Container) (map[string]types.PerNetworkOptions, error) {
	if !s.valid {
		return nil, define.ErrDBClosed
	}

	if !ctr.valid {
		return nil, define.ErrCtrRemoved
	}

	// if the network mode is not bridge return no networks
	if !ctr.config.NetMode.IsBridge() {
		return nil, nil
	}

	cfg, err := s.getCtrConfig(ctr.ID())
	if err != nil {
		if errors.Is(err, define.ErrNoSuchCtr) {
			ctr.valid = false
		}
		return nil, err
	}

	return cfg.Networks, nil
}

// NetworkConnect adds the given container to the given network. If aliases are
// specified, those will be added to the given network.
func (s *SQLiteState) NetworkConnect(ctr *Container, network string, opts types.PerNetworkOptions) error {
	return s.networkModify(ctr, network, opts, true)
}

// NetworkModify will allow you to set new options on an existing connected network
func (s *SQLiteState) NetworkModify(ctr *Container, network string, opts types.PerNetworkOptions) error {
	return s.networkModify(ctr, network, opts, false)
}

// networkModify allows you to modify or add a new network, to add a new network use the new bool
func (s *SQLiteState) networkModify(ctr *Container, network string, opts types.PerNetworkOptions, new bool) error {
	if !s.valid {
		return define.ErrDBClosed
	}

	if !ctr.valid {
		return define.ErrCtrRemoved
	}

	if network == "" {
		return fmt.Errorf("network names must not be empty: %w", define.ErrInvalidArg)
	}

	// Grab a fresh copy of the config, in case anything changed
	newCfg, err := s.getCtrConfig(ctr.ID())
	if err != nil && errors.Is(err, define.ErrNoSuchCtr) {
		ctr.valid = false
		return define.ErrNoSuchCtr
	}

	_, ok := newCfg.Networks[network]
	if new && ok {
		return fmt.Errorf("container %s is already connected to network %s: %w", ctr.ID(), network, define.ErrInvalidArg)
	}
	if !new && !ok {
		return fmt.Errorf("container %s is not connected to network %s: %w", ctr.ID(), network, define.ErrInvalidArg)
	}

	newCfg.Networks[network] = opts

	if err := s.rewriteContainerConfig(ctr, newCfg); err != nil {
		return err
	}

	ctr.config = newCfg

	return nil
}

// NetworkDisconnect disconnects the container from the given network, also
// removing any aliases in the network.
// TODO TODO TODO
func (s *SQLiteState) NetworkDisconnect(ctr *Container, network string) error {
	if !s.valid {
		return define.ErrDBClosed
	}

	if !ctr.valid {
		return define.ErrCtrRemoved
	}

	if network == "" {
		return fmt.Errorf("network names must not be empty: %w", define.ErrInvalidArg)
	}

	return define.ErrNotImplemented

	// ctrID := []byte(ctr.ID())

	// db, err := s.getDBCon()
	// if err != nil {
	// 	return err
	// }
	// defer s.deferredCloseDBCon(db)

	// return db.Update(func(tx *bolt.Tx) error {
	// 	ctrBucket, err := getCtrBucket(tx)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	dbCtr := ctrBucket.Bucket(ctrID)
	// 	if dbCtr == nil {
	// 		ctr.valid = false
	// 		return fmt.Errorf("container %s does not exist in database: %w", ctr.ID(), define.ErrNoSuchCtr)
	// 	}

	// 	ctrAliasesBkt := dbCtr.Bucket(aliasesBkt)
	// 	ctrNetworksBkt := dbCtr.Bucket(networksBkt)
	// 	if ctrNetworksBkt == nil {
	// 		return fmt.Errorf("container %s is not connected to any networks, so cannot disconnect: %w", ctr.ID(), define.ErrNoSuchNetwork)
	// 	}
	// 	netConnected := ctrNetworksBkt.Get([]byte(network))
	// 	if netConnected == nil {
	// 		return fmt.Errorf("container %s is not connected to network %q: %w", ctr.ID(), network, define.ErrNoSuchNetwork)
	// 	}

	// 	if err := ctrNetworksBkt.Delete([]byte(network)); err != nil {
	// 		return fmt.Errorf("removing container %s from network %s: %w", ctr.ID(), network, err)
	// 	}

	// 	if ctrAliasesBkt != nil {
	// 		bktExists := ctrAliasesBkt.Bucket([]byte(network))
	// 		if bktExists == nil {
	// 			return nil
	// 		}

	// 		if err := ctrAliasesBkt.DeleteBucket([]byte(network)); err != nil {
	// 			return fmt.Errorf("removing container %s network aliases for network %s: %w", ctr.ID(), network, err)
	// 		}
	// 	}

	// 	return nil
	// })
}

// GetContainerConfig returns a container config from the database by full ID
func (s *SQLiteState) GetContainerConfig(id string) (*ContainerConfig, error) {
	if len(id) == 0 {
		return nil, define.ErrEmptyID
	}

	if !s.valid {
		return nil, define.ErrDBClosed
	}

	return s.getCtrConfig(id)
}

// AddContainerExitCode adds the exit code for the specified container to the database.
func (s *SQLiteState) AddContainerExitCode(id string, exitCode int32) (defErr error) {
	if len(id) == 0 {
		return define.ErrEmptyID
	}

	if !s.valid {
		return define.ErrDBClosed
	}

	tx, err := s.conn.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction to add exit code: %w", err)
	}
	defer func() {
		if defErr != nil {
			if err := tx.Rollback(); err != nil {
				logrus.Errorf("Error rolling back transaction to add exit code: %v", err)
			}
		}
	}()

	if _, err := tx.Exec("INSERT INTO ContainerExitCode VALUES (?, ?, ?);", id, time.Now().Unix(), exitCode); err != nil {
		return fmt.Errorf("adding container %s exit code: %w", id, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction to add exit code: %w", err)
	}

	return nil
}

// GetContainerExitCode returns the exit code for the specified container.
func (s *SQLiteState) GetContainerExitCode(id string) (int32, error) {
	if len(id) == 0 {
		return -1, define.ErrEmptyID
	}

	if !s.valid {
		return -1, define.ErrDBClosed
	}

	row := s.conn.QueryRow("SELECT ExitCode FROM ContainerExitCode WHERE Id=?;", id)

	var exitCode int32
	if err := row.Scan(&exitCode); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return -1, fmt.Errorf("getting exit code of container %s from DB: %w", id, define.ErrNoSuchExitCode)
		}
		return -1, fmt.Errorf("scanning exit code of container %s: %w", id, err)
	}

	return exitCode, nil
}

// GetContainerExitCodeTimeStamp returns the time stamp when the exit code of
// the specified container was added to the database.
func (s *SQLiteState) GetContainerExitCodeTimeStamp(id string) (*time.Time, error) {
	if len(id) == 0 {
		return nil, define.ErrEmptyID
	}

	if !s.valid {
		return nil, define.ErrDBClosed
	}

	row := s.conn.QueryRow("SELECT Timestamp FROM ContainerExitCode WHERE Id=?;", id)

	var timestamp int64
	if err := row.Scan(&timestamp); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("getting timestamp for exit code of container %s from DB: %w", id, define.ErrNoSuchExitCode)
		}
		return nil, fmt.Errorf("scanning exit timestamp of container %s: %w", id, err)
	}

	result := time.Unix(timestamp, 0)

	return &result, nil
}

// PruneExitCodes removes exit codes older than 5 minutes.
func (s *SQLiteState) PruneContainerExitCodes() (defErr error) {
	if !s.valid {
		return define.ErrDBClosed
	}

	fiveMinsAgo := time.Now().Add(-5 * time.Minute).Unix()

	tx, err := s.conn.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction to remove old timestamps: %w", err)
	}
	defer func() {
		if defErr != nil {
			if err := tx.Rollback(); err != nil {
				logrus.Errorf("Error rolling back transaction to remove old timestamps: %v", err)
			}
		}
	}()

	if _, err := tx.Exec("DELETE FROM ContainerExitCode WHERE (Timestamp <= ?);", fiveMinsAgo); err != nil {
		return fmt.Errorf("removing exit codes with timestamps older than 5 minutes: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction to remove old timestamps: %w", err)
	}

	return nil
}

// AddExecSession adds an exec session to the state.
// TODO TODO TODO
func (s *SQLiteState) AddExecSession(ctr *Container, session *ExecSession) error {
	if !s.valid {
		return define.ErrDBClosed
	}

	if !ctr.valid {
		return define.ErrCtrRemoved
	}

	return define.ErrNotImplemented

	// db, err := s.getDBCon()
	// if err != nil {
	// 	return err
	// }
	// defer s.deferredCloseDBCon(db)

	// ctrID := []byte(ctr.ID())
	// sessionID := []byte(session.ID())

	// err = db.Update(func(tx *bolt.Tx) error {
	// 	execBucket, err := getExecBucket(tx)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	ctrBucket, err := getCtrBucket(tx)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	dbCtr := ctrBucket.Bucket(ctrID)
	// 	if dbCtr == nil {
	// 		ctr.valid = false
	// 		return fmt.Errorf("container %s is not present in the database: %w", ctr.ID(), define.ErrNoSuchCtr)
	// 	}

	// 	ctrExecSessionBucket, err := dbCtr.CreateBucketIfNotExists(execBkt)
	// 	if err != nil {
	// 		return fmt.Errorf("creating exec sessions bucket for container %s: %w", ctr.ID(), err)
	// 	}

	// 	execExists := execBucket.Get(sessionID)
	// 	if execExists != nil {
	// 		return fmt.Errorf("an exec session with ID %s already exists: %w", session.ID(), define.ErrExecSessionExists)
	// 	}

	// 	if err := execBucket.Put(sessionID, ctrID); err != nil {
	// 		return fmt.Errorf("adding exec session %s to DB: %w", session.ID(), err)
	// 	}

	// 	if err := ctrExecSessionBucket.Put(sessionID, ctrID); err != nil {
	// 		return fmt.Errorf("adding exec session %s to container %s in DB: %w", session.ID(), ctr.ID(), err)
	// 	}

	// 	return nil
	// })
	// return err
}

// GetExecSession returns the ID of the container an exec session is associated
// with.
func (s *SQLiteState) GetExecSession(id string) (string, error) {
	if !s.valid {
		return "", define.ErrDBClosed
	}

	if id == "" {
		return "", define.ErrEmptyID
	}

	return "", define.ErrNotImplemented

	// db, err := s.getDBCon()
	// if err != nil {
	// 	return "", err
	// }
	// defer s.deferredCloseDBCon(db)

	// ctrID := ""
	// err = db.View(func(tx *bolt.Tx) error {
	// 	execBucket, err := getExecBucket(tx)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	ctr := execBucket.Get([]byte(id))
	// 	if ctr == nil {
	// 		return fmt.Errorf("no exec session with ID %s found: %w", id, define.ErrNoSuchExecSession)
	// 	}
	// 	ctrID = string(ctr)
	// 	return nil
	// })
	// return ctrID, err
}

// RemoveExecSession removes references to the given exec session in the
// database.
func (s *SQLiteState) RemoveExecSession(session *ExecSession) error {
	if !s.valid {
		return define.ErrDBClosed
	}

	return define.ErrNotImplemented

	// db, err := s.getDBCon()
	// if err != nil {
	// 	return err
	// }
	// defer s.deferredCloseDBCon(db)

	// sessionID := []byte(session.ID())
	// containerID := []byte(session.ContainerID())
	// err = db.Update(func(tx *bolt.Tx) error {
	// 	execBucket, err := getExecBucket(tx)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	ctrBucket, err := getCtrBucket(tx)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	sessionExists := execBucket.Get(sessionID)
	// 	if sessionExists == nil {
	// 		return define.ErrNoSuchExecSession
	// 	}
	// 	// Check that container ID matches
	// 	if string(sessionExists) != session.ContainerID() {
	// 		return fmt.Errorf("database inconsistency: exec session %s points to container %s in state but %s in database: %w", session.ID(), session.ContainerID(), string(sessionExists), define.ErrInternal)
	// 	}

	// 	if err := execBucket.Delete(sessionID); err != nil {
	// 		return fmt.Errorf("removing exec session %s from database: %w", session.ID(), err)
	// 	}

	// 	dbCtr := ctrBucket.Bucket(containerID)
	// 	if dbCtr == nil {
	// 		// State is inconsistent. We refer to a container that
	// 		// is no longer in the state.
	// 		// Return without error, to attempt to recover.
	// 		return nil
	// 	}

	// 	ctrExecBucket := dbCtr.Bucket(execBkt)
	// 	if ctrExecBucket == nil {
	// 		// Again, state is inconsistent. We should have an exec
	// 		// bucket, and it should have this session.
	// 		// Again, nothing we can do, so proceed and try to
	// 		// recover.
	// 		return nil
	// 	}

	// 	ctrSessionExists := ctrExecBucket.Get(sessionID)
	// 	if ctrSessionExists != nil {
	// 		if err := ctrExecBucket.Delete(sessionID); err != nil {
	// 			return fmt.Errorf("removing exec session %s from container %s in database: %w", session.ID(), session.ContainerID(), err)
	// 		}
	// 	}

	// 	return nil
	// })
	// return err
}

// GetContainerExecSessions retrieves the IDs of all exec sessions running in a
// container that the database is aware of (IE, were added via AddExecSession).
func (s *SQLiteState) GetContainerExecSessions(ctr *Container) ([]string, error) {
	if !s.valid {
		return nil, define.ErrDBClosed
	}

	if !ctr.valid {
		return nil, define.ErrCtrRemoved
	}

	return nil, define.ErrNotImplemented

	// db, err := s.getDBCon()
	// if err != nil {
	// 	return nil, err
	// }
	// defer s.deferredCloseDBCon(db)

	// ctrID := []byte(ctr.ID())
	// sessions := []string{}
	// err = db.View(func(tx *bolt.Tx) error {
	// 	ctrBucket, err := getCtrBucket(tx)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	dbCtr := ctrBucket.Bucket(ctrID)
	// 	if dbCtr == nil {
	// 		ctr.valid = false
	// 		return define.ErrNoSuchCtr
	// 	}

	// 	ctrExecSessions := dbCtr.Bucket(execBkt)
	// 	if ctrExecSessions == nil {
	// 		return nil
	// 	}

	// 	return ctrExecSessions.ForEach(func(id, unused []byte) error {
	// 		sessions = append(sessions, string(id))
	// 		return nil
	// 	})
	// })
	// if err != nil {
	// 	return nil, err
	// }

	// return sessions, nil
}

// RemoveContainerExecSessions removes all exec sessions attached to a given
// container.
func (s *SQLiteState) RemoveContainerExecSessions(ctr *Container) error {
	if !s.valid {
		return define.ErrDBClosed
	}

	if !ctr.valid {
		return define.ErrCtrRemoved
	}

	return define.ErrNotImplemented

	// db, err := s.getDBCon()
	// if err != nil {
	// 	return err
	// }
	// defer s.deferredCloseDBCon(db)

	// ctrID := []byte(ctr.ID())
	// sessions := []string{}

	// err = db.Update(func(tx *bolt.Tx) error {
	// 	execBucket, err := getExecBucket(tx)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	ctrBucket, err := getCtrBucket(tx)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	dbCtr := ctrBucket.Bucket(ctrID)
	// 	if dbCtr == nil {
	// 		ctr.valid = false
	// 		return define.ErrNoSuchCtr
	// 	}

	// 	ctrExecSessions := dbCtr.Bucket(execBkt)
	// 	if ctrExecSessions == nil {
	// 		return nil
	// 	}

	// 	err = ctrExecSessions.ForEach(func(id, unused []byte) error {
	// 		sessions = append(sessions, string(id))
	// 		return nil
	// 	})
	// 	if err != nil {
	// 		return err
	// 	}

	// 	for _, session := range sessions {
	// 		if err := ctrExecSessions.Delete([]byte(session)); err != nil {
	// 			return fmt.Errorf("removing container %s exec session %s from database: %w", ctr.ID(), session, err)
	// 		}
	// 		// Check if the session exists in the global table
	// 		// before removing. It should, but in cases where the DB
	// 		// has become inconsistent, we should try and proceed
	// 		// so we can recover.
	// 		sessionExists := execBucket.Get([]byte(session))
	// 		if sessionExists == nil {
	// 			continue
	// 		}
	// 		if string(sessionExists) != ctr.ID() {
	// 			return fmt.Errorf("database mismatch: exec session %s is associated with containers %s and %s: %w", session, ctr.ID(), string(sessionExists), define.ErrInternal)
	// 		}
	// 		if err := execBucket.Delete([]byte(session)); err != nil {
	// 			return fmt.Errorf("removing container %s exec session %s from exec sessions: %w", ctr.ID(), session, err)
	// 		}
	// 	}

	// 	return nil
	// })
	// return err
}

// RewriteContainerConfig rewrites a container's configuration.
// DO NOT USE TO: Change container dependencies, change pod membership, change
// container ID.
// WARNING: This function is DANGEROUS. Do not use without reading the full
// comment on this function in state.go.
// TODO: Once BoltDB is removed, this can be combined with SafeRewriteContainerConfig.
func (s *SQLiteState) RewriteContainerConfig(ctr *Container, newCfg *ContainerConfig) error {
	if !s.valid {
		return define.ErrDBClosed
	}

	if !ctr.valid {
		return define.ErrCtrRemoved
	}

	return s.rewriteContainerConfig(ctr, newCfg)
}

// SafeRewriteContainerConfig rewrites a container's configuration in a more
// limited fashion than RewriteContainerConfig. It is marked as safe to use
// under most circumstances, unlike RewriteContainerConfig.
// DO NOT USE TO: Change container dependencies, change pod membership, change
// locks, change container ID.
// TODO: Once BoltDB is removed, this can be combined with RewriteContainerConfig.
func (s *SQLiteState) SafeRewriteContainerConfig(ctr *Container, oldName, newName string, newCfg *ContainerConfig) error {
	if !s.valid {
		return define.ErrDBClosed
	}

	if !ctr.valid {
		return define.ErrCtrRemoved
	}

	if newName != "" && newCfg.Name != newName {
		return fmt.Errorf("new name %s for container %s must match name in given container config: %w", newName, ctr.ID(), define.ErrInvalidArg)
	}
	if newName != "" && oldName == "" {
		return fmt.Errorf("must provide old name for container if a new name is given: %w", define.ErrInvalidArg)
	}

	return s.rewriteContainerConfig(ctr, newCfg)
}

// RewritePodConfig rewrites a pod's configuration.
// WARNING: This function is DANGEROUS. Do not use without reading the full
// comment on this function in state.go.
// TODO TODO TODO
func (s *SQLiteState) RewritePodConfig(pod *Pod, newCfg *PodConfig) error {
	if !s.valid {
		return define.ErrDBClosed
	}

	if !pod.valid {
		return define.ErrPodRemoved
	}

	return define.ErrNotImplemented

	// newCfgJSON, err := json.Marshal(newCfg)
	// if err != nil {
	// 	return fmt.Errorf("marshalling new configuration JSON for pod %s: %w", pod.ID(), err)
	// }

	// db, err := s.getDBCon()
	// if err != nil {
	// 	return err
	// }
	// defer s.deferredCloseDBCon(db)

	// err = db.Update(func(tx *bolt.Tx) error {
	// 	podBkt, err := getPodBucket(tx)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	podDB := podBkt.Bucket([]byte(pod.ID()))
	// 	if podDB == nil {
	// 		pod.valid = false
	// 		return fmt.Errorf("no pod with ID %s found in DB: %w", pod.ID(), define.ErrNoSuchPod)
	// 	}

	// 	if err := podDB.Put(configKey, newCfgJSON); err != nil {
	// 		return fmt.Errorf("updating pod %s config JSON: %w", pod.ID(), err)
	// 	}

	// 	return nil
	// })
	// return err
}

// RewriteVolumeConfig rewrites a volume's configuration.
// WARNING: This function is DANGEROUS. Do not use without reading the full
// comment on this function in state.go.
// TODO TODO TODO
func (s *SQLiteState) RewriteVolumeConfig(volume *Volume, newCfg *VolumeConfig) error {
	if !s.valid {
		return define.ErrDBClosed
	}

	if !volume.valid {
		return define.ErrVolumeRemoved
	}

	return define.ErrNotImplemented

	// newCfgJSON, err := json.Marshal(newCfg)
	// if err != nil {
	// 	return fmt.Errorf("marshalling new configuration JSON for volume %q: %w", volume.Name(), err)
	// }

	// db, err := s.getDBCon()
	// if err != nil {
	// 	return err
	// }
	// defer s.deferredCloseDBCon(db)

	// err = db.Update(func(tx *bolt.Tx) error {
	// 	volBkt, err := getVolBucket(tx)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	volDB := volBkt.Bucket([]byte(volume.Name()))
	// 	if volDB == nil {
	// 		volume.valid = false
	// 		return fmt.Errorf("no volume with name %q found in DB: %w", volume.Name(), define.ErrNoSuchVolume)
	// 	}

	// 	if err := volDB.Put(configKey, newCfgJSON); err != nil {
	// 		return fmt.Errorf("updating volume %q config JSON: %w", volume.Name(), err)
	// 	}

	// 	return nil
	// })
	// return err
}

// Pod retrieves a pod given its full ID
// TODO TODO TODO
func (s *SQLiteState) Pod(id string) (*Pod, error) {
	if id == "" {
		return nil, define.ErrEmptyID
	}

	if !s.valid {
		return nil, define.ErrDBClosed
	}

	return nil, define.ErrNotImplemented

	// podID := []byte(id)

	// pod := new(Pod)
	// pod.config = new(PodConfig)
	// pod.state = new(podState)

	// db, err := s.getDBCon()
	// if err != nil {
	// 	return nil, err
	// }
	// defer s.deferredCloseDBCon(db)

	// err = db.View(func(tx *bolt.Tx) error {
	// 	podBkt, err := getPodBucket(tx)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	return s.getPodFromDB(podID, pod, podBkt)
	// })
	// if err != nil {
	// 	return nil, err
	// }

	// return pod, nil
}

// LookupPod retrieves a pod from a full or unique partial ID, or a name.
func (s *SQLiteState) LookupPod(idOrName string) (*Pod, error) {
	if idOrName == "" {
		return nil, define.ErrEmptyID
	}

	if !s.valid {
		return nil, define.ErrDBClosed
	}

	rows, err := s.conn.Query("SELECT Json FROM PodConfig WHERE PodConfig.Name=? OR (PodConfig.Id LIKE ?);", idOrName, idOrName)
	if err != nil {
		return nil, fmt.Errorf("looking up pod %q in database: %w", idOrName, err)
	}
	defer rows.Close()

	var rawJSON string
	foundResult := false
	for rows.Next() {
		if foundResult {
			return nil, fmt.Errorf("more than one result for pod %q: %w", idOrName, define.ErrCtrExists)
		}

		if err := rows.Scan(&rawJSON); err != nil {
			return nil, fmt.Errorf("error retrieving pod %q ID from database: %w", idOrName, err)
		}
	}
	if !foundResult {
		return nil, define.ErrNoSuchPod
	}

	pod := new(Pod)
	pod.config = new(PodConfig)
	pod.state = new(podState)
	pod.runtime = s.runtime

	if err := json.Unmarshal([]byte(rawJSON), pod.config); err != nil {
		return nil, fmt.Errorf("unmarshalling pod JSON: %w", err)
	}

	if err := finalizePodSqlite(pod); err != nil {
		return nil, err
	}

	return pod, nil
}

// HasPod checks if a pod with the given ID exists in the state
// TODO TODO TODO
func (s *SQLiteState) HasPod(id string) (bool, error) {
	if id == "" {
		return false, define.ErrEmptyID
	}

	if !s.valid {
		return false, define.ErrDBClosed
	}

	return false, define.ErrNotImplemented

	// podID := []byte(id)

	// exists := false

	// db, err := s.getDBCon()
	// if err != nil {
	// 	return false, err
	// }
	// defer s.deferredCloseDBCon(db)

	// err = db.View(func(tx *bolt.Tx) error {
	// 	podBkt, err := getPodBucket(tx)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	podDB := podBkt.Bucket(podID)
	// 	if podDB != nil {
	// 		if s.namespaceBytes != nil {
	// 			podNS := podDB.Get(namespaceKey)
	// 			if bytes.Equal(s.namespaceBytes, podNS) {
	// 				exists = true
	// 			}
	// 		} else {
	// 			exists = true
	// 		}
	// 	}

	// 	return nil
	// })
	// if err != nil {
	// 	return false, err
	// }

	// return exists, nil
}

// PodHasContainer checks if the given pod has a container with the given ID
// TODO TODO TODO
func (s *SQLiteState) PodHasContainer(pod *Pod, id string) (bool, error) {
	if id == "" {
		return false, define.ErrEmptyID
	}

	if !s.valid {
		return false, define.ErrDBClosed
	}

	if !pod.valid {
		return false, define.ErrPodRemoved
	}

	return false, define.ErrNotImplemented

	// ctrID := []byte(id)
	// podID := []byte(pod.ID())

	// exists := false

	// db, err := s.getDBCon()
	// if err != nil {
	// 	return false, err
	// }
	// defer s.deferredCloseDBCon(db)

	// err = db.View(func(tx *bolt.Tx) error {
	// 	podBkt, err := getPodBucket(tx)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	// Get pod itself
	// 	podDB := podBkt.Bucket(podID)
	// 	if podDB == nil {
	// 		pod.valid = false
	// 		return fmt.Errorf("pod %s not found in database: %w", pod.ID(), define.ErrNoSuchPod)
	// 	}

	// 	// Get pod containers bucket
	// 	podCtrs := podDB.Bucket(containersBkt)
	// 	if podCtrs == nil {
	// 		return fmt.Errorf("pod %s missing containers bucket in DB: %w", pod.ID(), define.ErrInternal)
	// 	}

	// 	// Don't bother with a namespace check on the container -
	// 	// We maintain the invariant that container namespaces must
	// 	// match the namespace of the pod they join.
	// 	// We already checked the pod namespace, so we should be fine.

	// 	ctr := podCtrs.Get(ctrID)
	// 	if ctr != nil {
	// 		exists = true
	// 	}

	// 	return nil
	// })
	// if err != nil {
	// 	return false, err
	// }

	// return exists, nil
}

// PodContainersByID returns the IDs of all containers present in the given pod
// TODO TODO TODO
func (s *SQLiteState) PodContainersByID(pod *Pod) ([]string, error) {
	if !s.valid {
		return nil, define.ErrDBClosed
	}

	if !pod.valid {
		return nil, define.ErrPodRemoved
	}

	return nil, define.ErrNotImplemented

	// if s.namespace != "" && s.namespace != pod.config.Namespace {
	// 	return nil, fmt.Errorf("pod %s is in namespace %q but we are in namespace %q: %w", pod.ID(), pod.config.Namespace, s.namespace, define.ErrNSMismatch)
	// }

	// podID := []byte(pod.ID())

	// ctrs := []string{}

	// db, err := s.getDBCon()
	// if err != nil {
	// 	return nil, err
	// }
	// defer s.deferredCloseDBCon(db)

	// err = db.View(func(tx *bolt.Tx) error {
	// 	podBkt, err := getPodBucket(tx)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	// Get pod itself
	// 	podDB := podBkt.Bucket(podID)
	// 	if podDB == nil {
	// 		pod.valid = false
	// 		return fmt.Errorf("pod %s not found in database: %w", pod.ID(), define.ErrNoSuchPod)
	// 	}

	// 	// Get pod containers bucket
	// 	podCtrs := podDB.Bucket(containersBkt)
	// 	if podCtrs == nil {
	// 		return fmt.Errorf("pod %s missing containers bucket in DB: %w", pod.ID(), define.ErrInternal)
	// 	}

	// 	// Iterate through all containers in the pod
	// 	err = podCtrs.ForEach(func(id, val []byte) error {
	// 		ctrs = append(ctrs, string(id))

	// 		return nil
	// 	})
	// 	if err != nil {
	// 		return err
	// 	}

	// 	return nil
	// })
	// if err != nil {
	// 	return nil, err
	// }

	// return ctrs, nil
}

// PodContainers returns all the containers present in the given pod
// TODO TODO TODO
func (s *SQLiteState) PodContainers(pod *Pod) ([]*Container, error) {
	if !s.valid {
		return nil, define.ErrDBClosed
	}

	if !pod.valid {
		return nil, define.ErrPodRemoved
	}

	return nil, define.ErrNotImplemented

	// podID := []byte(pod.ID())

	// ctrs := []*Container{}

	// db, err := s.getDBCon()
	// if err != nil {
	// 	return nil, err
	// }
	// defer s.deferredCloseDBCon(db)

	// err = db.View(func(tx *bolt.Tx) error {
	// 	podBkt, err := getPodBucket(tx)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	ctrBkt, err := getCtrBucket(tx)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	// Get pod itself
	// 	podDB := podBkt.Bucket(podID)
	// 	if podDB == nil {
	// 		pod.valid = false
	// 		return fmt.Errorf("pod %s not found in database: %w", pod.ID(), define.ErrNoSuchPod)
	// 	}

	// 	// Get pod containers bucket
	// 	podCtrs := podDB.Bucket(containersBkt)
	// 	if podCtrs == nil {
	// 		return fmt.Errorf("pod %s missing containers bucket in DB: %w", pod.ID(), define.ErrInternal)
	// 	}

	// 	// Iterate through all containers in the pod
	// 	err = podCtrs.ForEach(func(id, val []byte) error {
	// 		newCtr := new(Container)
	// 		newCtr.config = new(ContainerConfig)
	// 		newCtr.state = new(ContainerState)
	// 		ctrs = append(ctrs, newCtr)

	// 		return s.getContainerFromDB(id, newCtr, ctrBkt, false)
	// 	})
	// 	if err != nil {
	// 		return err
	// 	}

	// 	return nil
	// })
	// if err != nil {
	// 	return nil, err
	// }

	// return ctrs, nil
}

// AddPod adds the given pod to the state.
// TODO TODO TODO
func (s *SQLiteState) AddPod(pod *Pod) error {
	if !s.valid {
		return define.ErrDBClosed
	}

	if !pod.valid {
		return define.ErrPodRemoved
	}

	return define.ErrNotImplemented

	// podID := []byte(pod.ID())
	// podName := []byte(pod.Name())

	// var podNamespace []byte
	// if pod.config.Namespace != "" {
	// 	podNamespace = []byte(pod.config.Namespace)
	// }

	// podConfigJSON, err := json.Marshal(pod.config)
	// if err != nil {
	// 	return fmt.Errorf("marshalling pod %s config to JSON: %w", pod.ID(), err)
	// }

	// podStateJSON, err := json.Marshal(pod.state)
	// if err != nil {
	// 	return fmt.Errorf("marshalling pod %s state to JSON: %w", pod.ID(), err)
	// }

	// db, err := s.getDBCon()
	// if err != nil {
	// 	return err
	// }
	// defer s.deferredCloseDBCon(db)

	// err = db.Update(func(tx *bolt.Tx) error {
	// 	podBkt, err := getPodBucket(tx)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	allPodsBkt, err := getAllPodsBucket(tx)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	idsBkt, err := getIDBucket(tx)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	namesBkt, err := getNamesBucket(tx)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	nsBkt, err := getNSBucket(tx)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	// Check if we already have something with the given ID and name
	// 	idExist := idsBkt.Get(podID)
	// 	if idExist != nil {
	// 		err = define.ErrPodExists
	// 		if allPodsBkt.Get(idExist) == nil {
	// 			err = define.ErrCtrExists
	// 		}
	// 		return fmt.Errorf("ID \"%s\" is in use: %w", pod.ID(), err)
	// 	}
	// 	nameExist := namesBkt.Get(podName)
	// 	if nameExist != nil {
	// 		err = define.ErrPodExists
	// 		if allPodsBkt.Get(nameExist) == nil {
	// 			err = define.ErrCtrExists
	// 		}
	// 		return fmt.Errorf("name \"%s\" is in use: %w", pod.Name(), err)
	// 	}

	// 	// We are good to add the pod
	// 	// Make a bucket for it
	// 	newPod, err := podBkt.CreateBucket(podID)
	// 	if err != nil {
	// 		return fmt.Errorf("creating bucket for pod %s: %w", pod.ID(), err)
	// 	}

	// 	// Make a subbucket for pod containers
	// 	if _, err := newPod.CreateBucket(containersBkt); err != nil {
	// 		return fmt.Errorf("creating bucket for pod %s containers: %w", pod.ID(), err)
	// 	}

	// 	if err := newPod.Put(configKey, podConfigJSON); err != nil {
	// 		return fmt.Errorf("storing pod %s configuration in DB: %w", pod.ID(), err)
	// 	}

	// 	if err := newPod.Put(stateKey, podStateJSON); err != nil {
	// 		return fmt.Errorf("storing pod %s state JSON in DB: %w", pod.ID(), err)
	// 	}

	// 	if podNamespace != nil {
	// 		if err := newPod.Put(namespaceKey, podNamespace); err != nil {
	// 			return fmt.Errorf("storing pod %s namespace in DB: %w", pod.ID(), err)
	// 		}
	// 		if err := nsBkt.Put(podID, podNamespace); err != nil {
	// 			return fmt.Errorf("storing pod %s namespace in DB: %w", pod.ID(), err)
	// 		}
	// 	}

	// 	// Add us to the ID and names buckets
	// 	if err := idsBkt.Put(podID, podName); err != nil {
	// 		return fmt.Errorf("storing pod %s ID in DB: %w", pod.ID(), err)
	// 	}
	// 	if err := namesBkt.Put(podName, podID); err != nil {
	// 		return fmt.Errorf("storing pod %s name in DB: %w", pod.Name(), err)
	// 	}
	// 	if err := allPodsBkt.Put(podID, podName); err != nil {
	// 		return fmt.Errorf("storing pod %s in all pods bucket in DB: %w", pod.ID(), err)
	// 	}

	// 	return nil
	// })
	// if err != nil {
	// 	return err
	// }

	// return nil
}

// RemovePod removes the given pod from the state.
// Only empty pods can be removed.
// TODO TODO TODO
func (s *SQLiteState) RemovePod(pod *Pod) error {
	if !s.valid {
		return define.ErrDBClosed
	}

	if !pod.valid {
		return define.ErrPodRemoved
	}

	return define.ErrNotImplemented

	// podID := []byte(pod.ID())
	// podName := []byte(pod.Name())

	// db, err := s.getDBCon()
	// if err != nil {
	// 	return err
	// }
	// defer s.deferredCloseDBCon(db)

	// err = db.Update(func(tx *bolt.Tx) error {
	// 	podBkt, err := getPodBucket(tx)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	allPodsBkt, err := getAllPodsBucket(tx)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	idsBkt, err := getIDBucket(tx)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	namesBkt, err := getNamesBucket(tx)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	nsBkt, err := getNSBucket(tx)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	// Check if the pod exists
	// 	podDB := podBkt.Bucket(podID)
	// 	if podDB == nil {
	// 		pod.valid = false
	// 		return fmt.Errorf("pod %s does not exist in DB: %w", pod.ID(), define.ErrNoSuchPod)
	// 	}

	// 	// Check if pod is empty
	// 	// This should never be nil
	// 	// But if it is, we can assume there are no containers in the
	// 	// pod.
	// 	// So let's eject the malformed pod without error.
	// 	podCtrsBkt := podDB.Bucket(containersBkt)
	// 	if podCtrsBkt != nil {
	// 		cursor := podCtrsBkt.Cursor()
	// 		if id, _ := cursor.First(); id != nil {
	// 			return fmt.Errorf("pod %s is not empty: %w", pod.ID(), define.ErrCtrExists)
	// 		}
	// 	}

	// 	// Pod is empty, and ready for removal
	// 	// Let's kick it out
	// 	if err := idsBkt.Delete(podID); err != nil {
	// 		return fmt.Errorf("removing pod %s ID from DB: %w", pod.ID(), err)
	// 	}
	// 	if err := namesBkt.Delete(podName); err != nil {
	// 		return fmt.Errorf("removing pod %s name (%s) from DB: %w", pod.ID(), pod.Name(), err)
	// 	}
	// 	if err := nsBkt.Delete(podID); err != nil {
	// 		return fmt.Errorf("removing pod %s namespace from DB: %w", pod.ID(), err)
	// 	}
	// 	if err := allPodsBkt.Delete(podID); err != nil {
	// 		return fmt.Errorf("removing pod %s ID from all pods bucket in DB: %w", pod.ID(), err)
	// 	}
	// 	if err := podBkt.DeleteBucket(podID); err != nil {
	// 		return fmt.Errorf("removing pod %s from DB: %w", pod.ID(), err)
	// 	}

	// 	return nil
	// })
	// if err != nil {
	// 	return err
	// }

	// return nil
}

// RemovePodContainers removes all containers in a pod.
// TODO TODO TODO
func (s *SQLiteState) RemovePodContainers(pod *Pod) error {
	if !s.valid {
		return define.ErrDBClosed
	}

	if !pod.valid {
		return define.ErrPodRemoved
	}

	return define.ErrNotImplemented

	// podID := []byte(pod.ID())

	// db, err := s.getDBCon()
	// if err != nil {
	// 	return err
	// }
	// defer s.deferredCloseDBCon(db)

	// err = db.Update(func(tx *bolt.Tx) error {
	// 	podBkt, err := getPodBucket(tx)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	ctrBkt, err := getCtrBucket(tx)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	allCtrsBkt, err := getAllCtrsBucket(tx)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	idsBkt, err := getIDBucket(tx)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	namesBkt, err := getNamesBucket(tx)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	// Check if the pod exists
	// 	podDB := podBkt.Bucket(podID)
	// 	if podDB == nil {
	// 		pod.valid = false
	// 		return fmt.Errorf("pod %s does not exist in DB: %w", pod.ID(), define.ErrNoSuchPod)
	// 	}

	// 	podCtrsBkt := podDB.Bucket(containersBkt)
	// 	if podCtrsBkt == nil {
	// 		return fmt.Errorf("pod %s does not have a containers bucket: %w", pod.ID(), define.ErrInternal)
	// 	}

	// 	// Traverse all containers in the pod with a cursor
	// 	// for-each has issues with data mutation
	// 	err = podCtrsBkt.ForEach(func(id, name []byte) error {
	// 		// Get the container so we can check dependencies
	// 		ctr := ctrBkt.Bucket(id)
	// 		if ctr == nil {
	// 			// This should never happen
	// 			// State is inconsistent
	// 			return fmt.Errorf("pod %s referenced nonexistent container %s: %w", pod.ID(), string(id), define.ErrNoSuchCtr)
	// 		}
	// 		ctrDeps := ctr.Bucket(dependenciesBkt)
	// 		// This should never be nil, but if it is, we're
	// 		// removing it anyways, so continue if it is
	// 		if ctrDeps != nil {
	// 			err = ctrDeps.ForEach(func(depID, name []byte) error {
	// 				exists := podCtrsBkt.Get(depID)
	// 				if exists == nil {
	// 					return fmt.Errorf("container %s has dependency %s outside of pod %s: %w", string(id), string(depID), pod.ID(), define.ErrCtrExists)
	// 				}
	// 				return nil
	// 			})
	// 			if err != nil {
	// 				return err
	// 			}
	// 		}

	// 		// Dependencies are set, we're clear to remove

	// 		if err := ctrBkt.DeleteBucket(id); err != nil {
	// 			return fmt.Errorf("deleting container %s from DB: %w", string(id), define.ErrInternal)
	// 		}

	// 		if err := idsBkt.Delete(id); err != nil {
	// 			return fmt.Errorf("deleting container %s ID in DB: %w", string(id), err)
	// 		}

	// 		if err := namesBkt.Delete(name); err != nil {
	// 			return fmt.Errorf("deleting container %s name in DB: %w", string(id), err)
	// 		}

	// 		if err := allCtrsBkt.Delete(id); err != nil {
	// 			return fmt.Errorf("deleting container %s ID from all containers bucket in DB: %w", string(id), err)
	// 		}

	// 		return nil
	// 	})
	// 	if err != nil {
	// 		return err
	// 	}

	// 	// Delete and recreate the bucket to empty it
	// 	if err := podDB.DeleteBucket(containersBkt); err != nil {
	// 		return fmt.Errorf("removing pod %s containers bucket: %w", pod.ID(), err)
	// 	}
	// 	if _, err := podDB.CreateBucket(containersBkt); err != nil {
	// 		return fmt.Errorf("recreating pod %s containers bucket: %w", pod.ID(), err)
	// 	}

	// 	return nil
	// })
	// if err != nil {
	// 	return err
	// }

	// return nil
}

// AddContainerToPod adds the given container to an existing pod
// The container will be added to the state and the pod
func (s *SQLiteState) AddContainerToPod(pod *Pod, ctr *Container) error {
	if !s.valid {
		return define.ErrDBClosed
	}

	if !pod.valid {
		return define.ErrPodRemoved
	}

	if !ctr.valid {
		return define.ErrCtrRemoved
	}

	if ctr.config.Pod != pod.ID() {
		return fmt.Errorf("container %s is not part of pod %s: %w", ctr.ID(), pod.ID(), define.ErrNoSuchCtr)
	}

	return s.addContainer(ctr)
}

// RemoveContainerFromPod removes a container from an existing pod
// The container will also be removed from the state
func (s *SQLiteState) RemoveContainerFromPod(pod *Pod, ctr *Container) error {
	if !s.valid {
		return define.ErrDBClosed
	}

	if !pod.valid {
		return define.ErrPodRemoved
	}

	if ctr.config.Pod == "" {
		return fmt.Errorf("container %s is not part of a pod, use RemoveContainer instead: %w", ctr.ID(), define.ErrNoSuchPod)
	}

	if ctr.config.Pod != pod.ID() {
		return fmt.Errorf("container %s is not part of pod %s: %w", ctr.ID(), pod.ID(), define.ErrInvalidArg)
	}

	return s.removeContainer(ctr)
}

// UpdatePod updates a pod's state from the database.
// TODO TODO TODO
func (s *SQLiteState) UpdatePod(pod *Pod) error {
	if !s.valid {
		return define.ErrDBClosed
	}

	if !pod.valid {
		return define.ErrPodRemoved
	}

	return define.ErrNotImplemented

	// newState := new(podState)

	// db, err := s.getDBCon()
	// if err != nil {
	// 	return err
	// }
	// defer s.deferredCloseDBCon(db)

	// podID := []byte(pod.ID())

	// err = db.View(func(tx *bolt.Tx) error {
	// 	podBkt, err := getPodBucket(tx)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	podDB := podBkt.Bucket(podID)
	// 	if podDB == nil {
	// 		pod.valid = false
	// 		return fmt.Errorf("no pod with ID %s found in database: %w", pod.ID(), define.ErrNoSuchPod)
	// 	}

	// 	// Get the pod state JSON
	// 	podStateBytes := podDB.Get(stateKey)
	// 	if podStateBytes == nil {
	// 		return fmt.Errorf("pod %s is missing state key in DB: %w", pod.ID(), define.ErrInternal)
	// 	}

	// 	if err := json.Unmarshal(podStateBytes, newState); err != nil {
	// 		return fmt.Errorf("unmarshalling pod %s state JSON: %w", pod.ID(), err)
	// 	}

	// 	return nil
	// })
	// if err != nil {
	// 	return err
	// }

	// pod.state = newState

	// return nil
}

// SavePod saves a pod's state to the database.
// TODO TODO TODO
func (s *SQLiteState) SavePod(pod *Pod) error {
	if !s.valid {
		return define.ErrDBClosed
	}

	if !pod.valid {
		return define.ErrPodRemoved
	}

	return define.ErrNotImplemented

	// stateJSON, err := json.Marshal(pod.state)
	// if err != nil {
	// 	return fmt.Errorf("marshalling pod %s state to JSON: %w", pod.ID(), err)
	// }

	// db, err := s.getDBCon()
	// if err != nil {
	// 	return err
	// }
	// defer s.deferredCloseDBCon(db)

	// podID := []byte(pod.ID())

	// err = db.Update(func(tx *bolt.Tx) error {
	// 	podBkt, err := getPodBucket(tx)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	podDB := podBkt.Bucket(podID)
	// 	if podDB == nil {
	// 		pod.valid = false
	// 		return fmt.Errorf("no pod with ID %s found in database: %w", pod.ID(), define.ErrNoSuchPod)
	// 	}

	// 	// Set the pod state JSON
	// 	if err := podDB.Put(stateKey, stateJSON); err != nil {
	// 		return fmt.Errorf("updating pod %s state in database: %w", pod.ID(), err)
	// 	}

	// 	return nil
	// })
	// if err != nil {
	// 	return err
	// }

	// return nil
}

// AllPods returns all pods present in the state.
// TODO TODO TODO
func (s *SQLiteState) AllPods() ([]*Pod, error) {
	if !s.valid {
		return nil, define.ErrDBClosed
	}

	return nil, define.ErrNotImplemented

	// pods := []*Pod{}

	// db, err := s.getDBCon()
	// if err != nil {
	// 	return nil, err
	// }
	// defer s.deferredCloseDBCon(db)

	// err = db.View(func(tx *bolt.Tx) error {
	// 	allPodsBucket, err := getAllPodsBucket(tx)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	podBucket, err := getPodBucket(tx)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	err = allPodsBucket.ForEach(func(id, name []byte) error {
	// 		podExists := podBucket.Bucket(id)
	// 		// This check can be removed if performance becomes an
	// 		// issue, but much less helpful errors will be produced
	// 		if podExists == nil {
	// 			return fmt.Errorf("inconsistency in state - pod %s is in all pods bucket but pod not found: %w", string(id), define.ErrInternal)
	// 		}

	// 		pod := new(Pod)
	// 		pod.config = new(PodConfig)
	// 		pod.state = new(podState)

	// 		if err := s.getPodFromDB(id, pod, podBucket); err != nil {
	// 			if !errors.Is(err, define.ErrNSMismatch) {
	// 				logrus.Errorf("Retrieving pod %s from the database: %v", string(id), err)
	// 			}
	// 		} else {
	// 			pods = append(pods, pod)
	// 		}

	// 		return nil
	// 	})
	// 	return err
	// })
	// if err != nil {
	// 	return nil, err
	// }

	// return pods, nil
}

// AddVolume adds the given volume to the state. It also adds ctrDepID to
// the sub bucket holding the container dependencies that this volume has
// TODO TODO TODO
func (s *SQLiteState) AddVolume(volume *Volume) error {
	if !s.valid {
		return define.ErrDBClosed
	}

	if !volume.valid {
		return define.ErrVolumeRemoved
	}

	return define.ErrNotImplemented

	// volName := []byte(volume.Name())

	// volConfigJSON, err := json.Marshal(volume.config)
	// if err != nil {
	// 	return fmt.Errorf("marshalling volume %s config to JSON: %w", volume.Name(), err)
	// }

	// // Volume state is allowed to not exist
	// var volStateJSON []byte
	// if volume.state != nil {
	// 	volStateJSON, err = json.Marshal(volume.state)
	// 	if err != nil {
	// 		return fmt.Errorf("marshalling volume %s state to JSON: %w", volume.Name(), err)
	// 	}
	// }

	// db, err := s.getDBCon()
	// if err != nil {
	// 	return err
	// }
	// defer s.deferredCloseDBCon(db)

	// err = db.Update(func(tx *bolt.Tx) error {
	// 	volBkt, err := getVolBucket(tx)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	allVolsBkt, err := getAllVolsBucket(tx)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	volCtrsBkt, err := getVolumeContainersBucket(tx)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	// Check if we already have a volume with the given name
	// 	volExists := allVolsBkt.Get(volName)
	// 	if volExists != nil {
	// 		return fmt.Errorf("name %s is in use: %w", volume.Name(), define.ErrVolumeExists)
	// 	}

	// 	// We are good to add the volume
	// 	// Make a bucket for it
	// 	newVol, err := volBkt.CreateBucket(volName)
	// 	if err != nil {
	// 		return fmt.Errorf("creating bucket for volume %s: %w", volume.Name(), err)
	// 	}

	// 	// Make a subbucket for the containers using the volume. Dependent container IDs will be addedremoved to
	// 	// this bucket in addcontainer/removeContainer
	// 	if _, err := newVol.CreateBucket(volDependenciesBkt); err != nil {
	// 		return fmt.Errorf("creating bucket for containers using volume %s: %w", volume.Name(), err)
	// 	}

	// 	if err := newVol.Put(configKey, volConfigJSON); err != nil {
	// 		return fmt.Errorf("storing volume %s configuration in DB: %w", volume.Name(), err)
	// 	}

	// 	if volStateJSON != nil {
	// 		if err := newVol.Put(stateKey, volStateJSON); err != nil {
	// 			return fmt.Errorf("storing volume %s state in DB: %w", volume.Name(), err)
	// 		}
	// 	}

	// 	if volume.config.StorageID != "" {
	// 		if err := volCtrsBkt.Put([]byte(volume.config.StorageID), volName); err != nil {
	// 			return fmt.Errorf("storing volume %s container ID in DB: %w", volume.Name(), err)
	// 		}
	// 	}

	// 	if err := allVolsBkt.Put(volName, volName); err != nil {
	// 		return fmt.Errorf("storing volume %s in all volumes bucket in DB: %w", volume.Name(), err)
	// 	}

	// 	return nil
	// })
	// return err
}

// RemoveVolume removes the given volume from the state
// TODO TODO TODO
func (s *SQLiteState) RemoveVolume(volume *Volume) error {
	if !s.valid {
		return define.ErrDBClosed
	}

	return define.ErrNotImplemented

	// volName := []byte(volume.Name())

	// db, err := s.getDBCon()
	// if err != nil {
	// 	return err
	// }
	// defer s.deferredCloseDBCon(db)

	// err = db.Update(func(tx *bolt.Tx) error {
	// 	volBkt, err := getVolBucket(tx)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	allVolsBkt, err := getAllVolsBucket(tx)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	ctrBkt, err := getCtrBucket(tx)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	volCtrIDBkt, err := getVolumeContainersBucket(tx)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	// Check if the volume exists
	// 	volDB := volBkt.Bucket(volName)
	// 	if volDB == nil {
	// 		volume.valid = false
	// 		return fmt.Errorf("volume %s does not exist in DB: %w", volume.Name(), define.ErrNoSuchVolume)
	// 	}

	// 	// Check if volume is not being used by any container
	// 	// This should never be nil
	// 	// But if it is, we can assume that no containers are using
	// 	// the volume.
	// 	volCtrsBkt := volDB.Bucket(volDependenciesBkt)
	// 	if volCtrsBkt != nil {
	// 		var deps []string
	// 		err = volCtrsBkt.ForEach(func(id, value []byte) error {
	// 			// Alright, this is ugly.
	// 			// But we need it to work around the change in
	// 			// volume dependency handling, to make sure that
	// 			// older Podman versions don't cause DB
	// 			// corruption.
	// 			// Look up all dependencies and see that they
	// 			// still exist before appending.
	// 			ctrExists := ctrBkt.Bucket(id)
	// 			if ctrExists == nil {
	// 				return nil
	// 			}

	// 			deps = append(deps, string(id))
	// 			return nil
	// 		})
	// 		if err != nil {
	// 			return fmt.Errorf("getting list of dependencies from dependencies bucket for volumes %q: %w", volume.Name(), err)
	// 		}
	// 		if len(deps) > 0 {
	// 			return fmt.Errorf("volume %s is being used by container(s) %s: %w", volume.Name(), strings.Join(deps, ","), define.ErrVolumeBeingUsed)
	// 		}
	// 	}

	// 	// volume is ready for removal
	// 	// Let's kick it out
	// 	if err := allVolsBkt.Delete(volName); err != nil {
	// 		return fmt.Errorf("removing volume %s from all volumes bucket in DB: %w", volume.Name(), err)
	// 	}
	// 	if err := volBkt.DeleteBucket(volName); err != nil {
	// 		return fmt.Errorf("removing volume %s from DB: %w", volume.Name(), err)
	// 	}
	// 	if volume.config.StorageID != "" {
	// 		if err := volCtrIDBkt.Delete([]byte(volume.config.StorageID)); err != nil {
	// 			return fmt.Errorf("removing volume %s container ID from DB: %w", volume.Name(), err)
	// 		}
	// 	}

	// 	return nil
	// })
	// return err
}

// UpdateVolume updates the volume's state from the database.
// TODO TODO TODO
func (s *SQLiteState) UpdateVolume(volume *Volume) error {
	if !s.valid {
		return define.ErrDBClosed
	}

	if !volume.valid {
		return define.ErrVolumeRemoved
	}

	return define.ErrNotImplemented

	// newState := new(VolumeState)
	// volumeName := []byte(volume.Name())

	// db, err := s.getDBCon()
	// if err != nil {
	// 	return err
	// }
	// defer s.deferredCloseDBCon(db)

	// err = db.View(func(tx *bolt.Tx) error {
	// 	volBucket, err := getVolBucket(tx)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	volToUpdate := volBucket.Bucket(volumeName)
	// 	if volToUpdate == nil {
	// 		volume.valid = false
	// 		return fmt.Errorf("no volume with name %s found in database: %w", volume.Name(), define.ErrNoSuchVolume)
	// 	}

	// 	stateBytes := volToUpdate.Get(stateKey)
	// 	if stateBytes == nil {
	// 		// Having no state is valid.
	// 		// Return nil, use the empty state.
	// 		return nil
	// 	}

	// 	if err := json.Unmarshal(stateBytes, newState); err != nil {
	// 		return fmt.Errorf("unmarshalling volume %s state: %w", volume.Name(), err)
	// 	}

	// 	return nil
	// })
	// if err != nil {
	// 	return err
	// }

	// volume.state = newState

	// return nil
}

// SaveVolume saves the volume's state to the database.
func (s *SQLiteState) SaveVolume(volume *Volume) error {
	if !s.valid {
		return define.ErrDBClosed
	}

	if !volume.valid {
		return define.ErrVolumeRemoved
	}

	return define.ErrNotImplemented

	// volumeName := []byte(volume.Name())

	// var newStateJSON []byte
	// if volume.state != nil {
	// 	stateJSON, err := json.Marshal(volume.state)
	// 	if err != nil {
	// 		return fmt.Errorf("marshalling volume %s state to JSON: %w", volume.Name(), err)
	// 	}
	// 	newStateJSON = stateJSON
	// }

	// db, err := s.getDBCon()
	// if err != nil {
	// 	return err
	// }
	// defer s.deferredCloseDBCon(db)

	// err = db.Update(func(tx *bolt.Tx) error {
	// 	volBucket, err := getVolBucket(tx)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	volToUpdate := volBucket.Bucket(volumeName)
	// 	if volToUpdate == nil {
	// 		volume.valid = false
	// 		return fmt.Errorf("no volume with name %s found in database: %w", volume.Name(), define.ErrNoSuchVolume)
	// 	}

	// 	return volToUpdate.Put(stateKey, newStateJSON)
	// })
	// return err
}

// AllVolumes returns all volumes present in the state
// TODO TODO TODO
func (s *SQLiteState) AllVolumes() ([]*Volume, error) {
	if !s.valid {
		return nil, define.ErrDBClosed
	}

	return nil, define.ErrNotImplemented

	// volumes := []*Volume{}

	// db, err := s.getDBCon()
	// if err != nil {
	// 	return nil, err
	// }
	// defer s.deferredCloseDBCon(db)

	// err = db.View(func(tx *bolt.Tx) error {
	// 	allVolsBucket, err := getAllVolsBucket(tx)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	volBucket, err := getVolBucket(tx)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	err = allVolsBucket.ForEach(func(id, name []byte) error {
	// 		volExists := volBucket.Bucket(id)
	// 		// This check can be removed if performance becomes an
	// 		// issue, but much less helpful errors will be produced
	// 		if volExists == nil {
	// 			return fmt.Errorf("inconsistency in state - volume %s is in all volumes bucket but volume not found: %w", string(id), define.ErrInternal)
	// 		}

	// 		volume := new(Volume)
	// 		volume.config = new(VolumeConfig)
	// 		volume.state = new(VolumeState)

	// 		if err := s.getVolumeFromDB(id, volume, volBucket); err != nil {
	// 			if !errors.Is(err, define.ErrNSMismatch) {
	// 				logrus.Errorf("Retrieving volume %s from the database: %v", string(id), err)
	// 			}
	// 		} else {
	// 			volumes = append(volumes, volume)
	// 		}

	// 		return nil
	// 	})
	// 	return err
	// })
	// if err != nil {
	// 	return nil, err
	// }

	// return volumes, nil
}

// Volume retrieves a volume from full name
// TODO TODO TODO
func (s *SQLiteState) Volume(name string) (*Volume, error) {
	if name == "" {
		return nil, define.ErrEmptyID
	}

	if !s.valid {
		return nil, define.ErrDBClosed
	}

	return nil, define.ErrNotImplemented

	// volName := []byte(name)

	// volume := new(Volume)
	// volume.config = new(VolumeConfig)
	// volume.state = new(VolumeState)

	// db, err := s.getDBCon()
	// if err != nil {
	// 	return nil, err
	// }
	// defer s.deferredCloseDBCon(db)

	// err = db.View(func(tx *bolt.Tx) error {
	// 	volBkt, err := getVolBucket(tx)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	return s.getVolumeFromDB(volName, volume, volBkt)
	// })
	// if err != nil {
	// 	return nil, err
	// }

	// return volume, nil
}

// LookupVolume locates a volume from a partial name.
// TODO TODO TODO
func (s *SQLiteState) LookupVolume(name string) (*Volume, error) {
	if name == "" {
		return nil, define.ErrEmptyID
	}

	if !s.valid {
		return nil, define.ErrDBClosed
	}

	return nil, define.ErrNotImplemented

	// volName := []byte(name)

	// volume := new(Volume)
	// volume.config = new(VolumeConfig)
	// volume.state = new(VolumeState)

	// db, err := s.getDBCon()
	// if err != nil {
	// 	return nil, err
	// }
	// defer s.deferredCloseDBCon(db)

	// err = db.View(func(tx *bolt.Tx) error {
	// 	volBkt, err := getVolBucket(tx)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	allVolsBkt, err := getAllVolsBucket(tx)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	// Check for exact match on name
	// 	volDB := volBkt.Bucket(volName)
	// 	if volDB != nil {
	// 		return s.getVolumeFromDB(volName, volume, volBkt)
	// 	}

	// 	// No exact match. Search all names.
	// 	foundMatch := false
	// 	err = allVolsBkt.ForEach(func(checkName, checkName2 []byte) error {
	// 		if strings.HasPrefix(string(checkName), name) {
	// 			if foundMatch {
	// 				return fmt.Errorf("more than one result for volume name %q: %w", name, define.ErrVolumeExists)
	// 			}
	// 			foundMatch = true
	// 			volName = checkName
	// 		}
	// 		return nil
	// 	})
	// 	if err != nil {
	// 		return err
	// 	}

	// 	if !foundMatch {
	// 		return fmt.Errorf("no volume with name %q found: %w", name, define.ErrNoSuchVolume)
	// 	}

	// 	return s.getVolumeFromDB(volName, volume, volBkt)
	// })
	// if err != nil {
	// 	return nil, err
	// }

	// return volume, nil
}

// HasVolume returns true if the given volume exists in the state, otherwise it returns false
// TODO TODO TODO
func (s *SQLiteState) HasVolume(name string) (bool, error) {
	if name == "" {
		return false, define.ErrEmptyID
	}

	if !s.valid {
		return false, define.ErrDBClosed
	}

	return false, define.ErrNotImplemented

	// volName := []byte(name)

	// exists := false

	// db, err := s.getDBCon()
	// if err != nil {
	// 	return false, err
	// }
	// defer s.deferredCloseDBCon(db)

	// err = db.View(func(tx *bolt.Tx) error {
	// 	volBkt, err := getVolBucket(tx)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	volDB := volBkt.Bucket(volName)
	// 	if volDB != nil {
	// 		exists = true
	// 	}

	// 	return nil
	// })
	// if err != nil {
	// 	return false, err
	// }

	// return exists, nil
}

// VolumeInUse checks if any container is using the volume
// It returns a slice of the IDs of the containers using the given
// volume. If the slice is empty, no containers use the given volume
// TODO TODO TODO
func (s *SQLiteState) VolumeInUse(volume *Volume) ([]string, error) {
	if !s.valid {
		return nil, define.ErrDBClosed
	}

	if !volume.valid {
		return nil, define.ErrVolumeRemoved
	}

	return nil, define.ErrNotImplemented

	// depCtrs := []string{}

	// db, err := s.getDBCon()
	// if err != nil {
	// 	return nil, err
	// }
	// defer s.deferredCloseDBCon(db)

	// err = db.View(func(tx *bolt.Tx) error {
	// 	volBucket, err := getVolBucket(tx)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	ctrBucket, err := getCtrBucket(tx)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	volDB := volBucket.Bucket([]byte(volume.Name()))
	// 	if volDB == nil {
	// 		volume.valid = false
	// 		return fmt.Errorf("no volume with name %s found in DB: %w", volume.Name(), define.ErrNoSuchVolume)
	// 	}

	// 	dependsBkt := volDB.Bucket(volDependenciesBkt)
	// 	if dependsBkt == nil {
	// 		return fmt.Errorf("volume %s has no dependencies bucket: %w", volume.Name(), define.ErrInternal)
	// 	}

	// 	// Iterate through and add dependencies
	// 	err = dependsBkt.ForEach(func(id, value []byte) error {
	// 		// Look up all dependencies and see that they
	// 		// still exist before appending.
	// 		ctrExists := ctrBucket.Bucket(id)
	// 		if ctrExists == nil {
	// 			return nil
	// 		}

	// 		depCtrs = append(depCtrs, string(id))

	// 		return nil
	// 	})
	// 	if err != nil {
	// 		return err
	// 	}

	// 	return nil
	// })
	// if err != nil {
	// 	return nil, err
	// }

	// return depCtrs, nil
}

// ContainerIDIsVolume checks if the given c/storage container ID is used as
// backing storage for a volume.
// TODO TODO TODO
func (s *SQLiteState) ContainerIDIsVolume(id string) (bool, error) {
	if !s.valid {
		return false, define.ErrDBClosed
	}

	return false, define.ErrNotImplemented

	// isVol := false

	// db, err := s.getDBCon()
	// if err != nil {
	// 	return false, err
	// }
	// defer s.deferredCloseDBCon(db)

	// err = db.View(func(tx *bolt.Tx) error {
	// 	volCtrsBkt, err := getVolumeContainersBucket(tx)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	volName := volCtrsBkt.Get([]byte(id))
	// 	if volName != nil {
	// 		isVol = true
	// 	}

	// 	return nil
	// })
	// return isVol, err
}
