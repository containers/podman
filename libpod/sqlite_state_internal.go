package libpod

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/containers/common/libnetwork/types"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/sirupsen/logrus"

	// SQLite backend for database/sql
	_ "github.com/mattn/go-sqlite3"
)

func (s *SQLiteState) migrateSchemaIfNecessary() (defErr error) {
	row := s.conn.QueryRow("SELECT SchemaVersion FROM DBConfig;")
	var schemaVer int
	if err := row.Scan(&schemaVer); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Brand-new, unpopulated DB.
			// Schema was just created, so it has to be the latest.
			return nil
		}
	}

	// If the schema version 0 or less, it's invalid
	if schemaVer <= 0 {
		return fmt.Errorf("database schema version %d is invalid: %w", schemaVer, define.ErrInternal)
	}

	if schemaVer != schemaVersion {
		// If the DB is a later schema than we support, we have to error
		if schemaVer > schemaVersion {
			return fmt.Errorf("database has schema version %d while this libpod version only supports version %d: %w",
				schemaVer, schemaVersion, define.ErrInternal)
		}

		// Perform schema migration here, one version at a time.
	}

	return nil
}

// Initialize all required tables for the SQLite state
func sqliteInitTables(conn *sql.DB) (defErr error) {
	// Technically we could split the "CREATE TABLE IF NOT EXISTS" and ");"
	// bits off each command and add them in the for loop where we actually
	// run the SQL, but that seems unnecessary.
	const dbConfig = `
        CREATE TABLE IF NOT EXISTS DBConfig(
                Id            INTEGER PRIMARY KEY NOT NULL,
                SchemaVersion INTEGER NOT NULL,
                Os            TEXT    NOT NULL,
                StaticDir     TEXT    NOT NULL,
                TmpDir        TEXT    NOT NULL,
                GraphRoot     TEXT    NOT NULL,
                RunRoot       TEXT    NOT NULL,
                GraphDriver   TEXT    NOT NULL,
                VolumeDir     TEXT    NOT NULL,
                CHECK (Id IN (1))
        );`

	const idNamespace = `
        CREATE TABLE IF NOT EXISTS IDNamespace(
                Id TEXT PRIMARY KEY NOT NULL
        );`

	const containerConfig = `
        CREATE TABLE IF NOT EXISTS ContainerConfig(
                Id              TEXT    PRIMARY KEY NOT NULL,
                Name            TEXT    UNIQUE NOT NULL,
                PodID           TEXT,
                Json            TEXT    NOT NULL,
                FOREIGN KEY (Id)    REFERENCES IDNamespace(Id)    DEFERRABLE INITIALLY DEFERRED,
                FOREIGN KEY (Id)    REFERENCES ContainerState(Id) DEFERRABLE INITIALLY DEFERRED,
                FOREIGN KEY (PodID) REFERENCES PodConfig(Id)
        );`

	const containerState = `
        CREATE TABLE IF NOT EXISTS ContainerState(
                Id       TEXT    PRIMARY KEY NOT NULL,
                State    INTEGER NOT NULL,
                ExitCode INTEGER,
                Json     TEXT    NOT NULL,
                FOREIGN KEY (Id) REFERENCES ContainerConfig(Id) DEFERRABLE INITIALLY DEFERRED,
                CHECK (ExitCode BETWEEN 0 AND 255)
        );`

	const containerExecSession = `
        CREATE TABLE IF NOT EXISTS ContainerExecSession(
                Id          TEXT PRIMARY KEY NOT NULL,
                ContainerID TEXT NOT NULL,
                Json        TEXT NOT NULL,
                FOREIGN KEY (ContainerID) REFERENCES ContainerConfig(Id)
        );`

	const containerDependency = `
        CREATE TABLE IF NOT EXISTS ContainerDependency(
                Id           TEXT NOT NULL,
                DependencyID TEXT NOT NULL,
                PRIMARY KEY (Id, DependencyID),
                FOREIGN KEY (Id)           REFERENCES ContainerConfig(Id) DEFERRABLE INITIALLY DEFERRED,
                FOREIGN KEY (DependencyID) REFERENCES ContainerConfig(Id),
                CHECK (Id <> DependencyID)
        );`

	const containerVolume = `
        CREATE TABLE IF NOT EXISTS ContainerVolume(
                ContainerID TEXT NOT NULL,
                VolumeName  TEXT NOT NULL,
                PRIMARY KEY (ContainerID, VolumeName),
                FOREIGN KEY (ContainerID) REFERENCES ContainerConfig(Id) DEFERRABLE INITIALLY DEFERRED,
                FOREIGN KEY (VolumeName)  REFERENCES VolumeConfig(Name)
        );`

	const containerExitCode = `
        CREATE TABLE IF NOT EXISTS ContainerExitCode(
                Id        TEXT    PRIMARY KEY NOT NULL,
                Timestamp INTEGER NOT NULL,
                ExitCode  INTEGER NOT NULL,
                CHECK (ExitCode BETWEEN 0 AND 255)
        );`

	const podConfig = `
        CREATE TABLE IF NOT EXISTS PodConfig(
                Id              TEXT    PRIMARY KEY NOT NULL,
                Name            TEXT    UNIQUE NOT NULL,
                Json            TEXT    NOT NULL,
                FOREIGN KEY (Id) REFERENCES IDNamespace(Id) DEFERRABLE INITIALLY DEFERRED,
                FOREIGN KEY (Id) REFERENCES PodState(Id)    DEFERRABLE INITIALLY DEFERRED
        );`

	const podState = `
        CREATE TABLE IF NOT EXISTS PodState(
                Id               TEXT PRIMARY KEY NOT NULL,
                InfraContainerId TEXT,
                Json             TEXT NOT NULL,
                FOREIGN KEY (Id)               REFERENCES PodConfig(Id)       DEFERRABLE INITIALLY DEFERRED,
                FOREIGN KEY (InfraContainerId) REFERENCES ContainerConfig(Id) DEFERRABLE INITIALLY DEFERRED
        );`

	const volumeConfig = `
        CREATE TABLE IF NOT EXISTS VolumeConfig(
                Name            TEXT    PRIMARY KEY NOT NULL,
                StorageID       TEXT,
                Json            TEXT    NOT NULL,
                FOREIGN KEY (Name) REFERENCES VolumeState(Name) DEFERRABLE INITIALLY DEFERRED
        );`

	const volumeState = `
        CREATE TABLE IF NOT EXISTS VolumeState(
                Name TEXT PRIMARY KEY NOT NULL,
                Json TEXT NOT NULL,
                FOREIGN KEY (Name) REFERENCES VolumeConfig(Name) DEFERRABLE INITIALLY DEFERRED
        );`

	tables := map[string]string{
		"DBConfig":             dbConfig,
		"IDNamespace":          idNamespace,
		"ContainerConfig":      containerConfig,
		"ContainerState":       containerState,
		"ContainerExecSession": containerExecSession,
		"ContainerDependency":  containerDependency,
		"ContainerVolume":      containerVolume,
		"ContainerExitCode":    containerExitCode,
		"PodConfig":            podConfig,
		"PodState":             podState,
		"VolumeConfig":         volumeConfig,
		"volumeState":          volumeState,
	}

	tx, err := conn.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() {
		if defErr != nil {
			if err := tx.Rollback(); err != nil {
				logrus.Errorf("Error rolling back transaction to create tables: %v", err)
			}
		}
	}()

	for tblName, cmd := range tables {
		if _, err := tx.Exec(cmd); err != nil {
			return fmt.Errorf("creating table %s: %w", tblName, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}

// Get the config of a container with the given ID from the database
func (s *SQLiteState) getCtrConfig(id string) (*ContainerConfig, error) {
	row := s.conn.QueryRow("SELECT Json FROM ContainerConfig WHERE Id=?;", id)

	var rawJSON string
	if err := row.Scan(&rawJSON); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, define.ErrNoSuchCtr
		}
		return nil, fmt.Errorf("retrieving container %s config from DB: %w", id, err)
	}

	ctrCfg := new(ContainerConfig)

	if err := json.Unmarshal([]byte(rawJSON), ctrCfg); err != nil {
		return nil, fmt.Errorf("unmarshalling container %s config: %w", id, err)
	}

	return ctrCfg, nil
}

// Finalize a container that was pulled out of the database.
func finalizeCtrSqlite(ctr *Container) error {
	// Get the lock
	lock, err := ctr.runtime.lockManager.RetrieveLock(ctr.config.LockID)
	if err != nil {
		return fmt.Errorf("retrieving lock for container %s: %w", ctr.ID(), err)
	}
	ctr.lock = lock

	// Get the OCI runtime
	if ctr.config.OCIRuntime == "" {
		ctr.ociRuntime = ctr.runtime.defaultOCIRuntime
	} else {
		// Handle legacy containers which might use a literal path for
		// their OCI runtime name.
		runtimeName := ctr.config.OCIRuntime
		ociRuntime, ok := ctr.runtime.ociRuntimes[runtimeName]
		if !ok {
			runtimeSet := false

			// If the path starts with a / and exists, make a new
			// OCI runtime for it using the full path.
			if strings.HasPrefix(runtimeName, "/") {
				if stat, err := os.Stat(runtimeName); err == nil && !stat.IsDir() {
					newOCIRuntime, err := newConmonOCIRuntime(runtimeName, []string{runtimeName}, ctr.runtime.conmonPath, ctr.runtime.runtimeFlags, ctr.runtime.config)
					if err == nil {
						// TODO: There is a potential risk of concurrent map modification here.
						// This is an unlikely case, though.
						ociRuntime = newOCIRuntime
						ctr.runtime.ociRuntimes[runtimeName] = ociRuntime
						runtimeSet = true
					}
				}
			}

			if !runtimeSet {
				// Use a MissingRuntime implementation
				ociRuntime = getMissingRuntime(runtimeName, ctr.runtime)
			}
		}
		ctr.ociRuntime = ociRuntime
	}

	ctr.valid = true

	return nil
}

// Finalize a pod that was pulled out of the database.
func finalizePodSqlite(pod *Pod) error {
	// Get the lock
	lock, err := pod.runtime.lockManager.RetrieveLock(pod.config.LockID)
	if err != nil {
		return fmt.Errorf("retrieving lock for pod %s: %w", pod.ID(), err)
	}
	pod.lock = lock

	pod.valid = true

	return nil
}

func (s *SQLiteState) rewriteContainerConfig(ctr *Container, newCfg *ContainerConfig) (defErr error) {
	json, err := json.Marshal(newCfg)
	if err != nil {
		return fmt.Errorf("error marshalling container %s new config JSON: %w", ctr.ID(), err)
	}

	tx, err := s.conn.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction to rewrite container %s config: %w", ctr.ID(), err)
	}
	defer func() {
		if defErr != nil {
			if err := tx.Rollback(); err != nil {
				logrus.Errorf("Error rolling back transaction to rewrite container %s config: %v", ctr.ID(), err)
			}
		}
	}()

	results, err := tx.Exec("UPDATE TABLE ContainerConfig SET Name=?, Json=? WHERE Id=?;", newCfg.Name, json, ctr.ID())
	if err != nil {
		return fmt.Errorf("updating container config table with new configuration for container %s: %w", ctr.ID(), err)
	}
	rows, err := results.RowsAffected()
	if err != nil {
		return fmt.Errorf("retrieving container %s config rewrite rows affected: %w", ctr.ID(), err)
	}
	if rows == 0 {
		ctr.valid = false
		return define.ErrNoSuchCtr
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction to rewrite container %s config: %w", ctr.ID(), err)
	}

	return nil
}

func (s *SQLiteState) addContainer(ctr *Container) (defErr error) {
	configJSON, err := json.Marshal(ctr.config)
	if err != nil {
		return fmt.Errorf("marshalling container config json: %w", err)
	}

	stateJSON, err := json.Marshal(ctr.state)
	if err != nil {
		return fmt.Errorf("marshalling container state json: %w", err)
	}
	deps := ctr.Dependencies()

	pod := sql.NullString{}
	if ctr.config.Pod != "" {
		pod.Valid = true
		pod.String = ctr.config.Pod
	}

	tx, err := s.conn.Begin()
	if err != nil {
		return fmt.Errorf("beginning container create transaction: %w", err)
	}
	defer func() {
		if defErr != nil {
			if err := tx.Rollback(); err != nil {
				logrus.Errorf("Error rolling back transaction to create container: %v", err)
			}
		}
	}()

	if _, err := tx.Exec("INSERT INTO IDNamespace VALUES (?);", ctr.ID()); err != nil {
		return fmt.Errorf("adding container id to database: %w", err)
	}
	if _, err := tx.Exec("INSERT INTO ContainerConfig VALUES (?, ?, ?, ?);", ctr.ID(), ctr.Name(), pod, configJSON); err != nil {
		return fmt.Errorf("adding container config to database: %w", err)
	}
	if _, err := tx.Exec("INSERT INTO ContainerState VALUES (?, ?, ?, ?);", ctr.ID(), int(ctr.state.State), ctr.state.ExitCode, stateJSON); err != nil {
		return fmt.Errorf("adding container state to database: %w", err)
	}
	for _, dep := range deps {
		// Check if the dependency is in the same pod
		var depPod sql.NullString
		row := tx.QueryRow("SELECT PodID FROM ContainerConfig WHERE Id=?;", dep)
		if err := row.Scan(&depPod); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("container dependency %s does not exist in database: %w", dep, define.ErrNoSuchCtr)
			}
		}
		switch {
		case ctr.config.Pod == "" && depPod.Valid:
			return fmt.Errorf("container dependency %s is part of a pod, but container is not: %w", dep, define.ErrInvalidArg)
		case ctr.config.Pod != "" && !depPod.Valid:
			return fmt.Errorf("container dependency %s is not part of pod, but this container belongs to pod %s: %w", dep, ctr.config.Pod, define.ErrInvalidArg)
		case ctr.config.Pod != "" && depPod.String != ctr.config.Pod:
			return fmt.Errorf("container dependency %s is part of pod %s but container is part of pod %s, pods must match: %w", dep, depPod.String, ctr.config.Pod, define.ErrInvalidArg)
		}

		if _, err := tx.Exec("INSERT INTO ContainerDependency VALUES (?, ?);", ctr.ID(), dep); err != nil {
			return fmt.Errorf("adding container dependency %s to database: %w", dep, err)
		}
	}
	for _, vol := range ctr.config.NamedVolumes {
		if _, err := tx.Exec("INSERT INTO ContainerVolume VALUES (?, ?);", ctr.ID(), vol.Name); err != nil {
			return fmt.Errorf("adding container volume %s to database: %w", vol.Name, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}

func (s *SQLiteState) removeContainer(ctr *Container) (defErr error) {
	tx, err := s.conn.Begin()
	if err != nil {
		return fmt.Errorf("beginning container %s removal transaction: %w", ctr.ID(), err)
	}
	defer func() {
		if defErr != nil {
			if err := tx.Rollback(); err != nil {
				logrus.Errorf("Error rolling back transaction to remove container %s: %v", ctr.ID(), err)
			}
		}
	}()

	if _, err := tx.Exec("DELETE FROM IDNamespace WHERE Id=?;", ctr.ID()); err != nil {
		return fmt.Errorf("removing container %s id from database: %w", ctr.ID(), err)
	}
	if _, err := tx.Exec("DELETE FROM ContainerConfig WHERE Id=?;", ctr.ID()); err != nil {
		return fmt.Errorf("removing container %s config from database: %w", ctr.ID(), err)
	}
	if _, err := tx.Exec("DELETE FROM ContainerState WHERE Id=?;", ctr.ID()); err != nil {
		return fmt.Errorf("removing container %s state from database: %w", ctr.ID(), err)
	}
	if _, err := tx.Exec("DELETE FROM ContainerDependency WHERE Id=?;", ctr.ID()); err != nil {
		return fmt.Errorf("removing container %s dependencies from database: %w", ctr.ID(), err)
	}
	if _, err := tx.Exec("DELETE FROM ContainerVolume WHERE ContainerID=?;", ctr.ID()); err != nil {
		return fmt.Errorf("removing container %s volumes from database: %w", ctr.ID(), err)
	}
	if _, err := tx.Exec("DELETE FROM ContainerExecSession WHERE ContainerID=?;", ctr.ID()); err != nil {
		return fmt.Errorf("removing container %s exec sessions from database: %w", ctr.ID(), err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing container %s removal transaction: %w", ctr.ID(), err)
	}

	return nil
}

// networkModify allows you to modify or add a new network, to add a new network use the new bool
func (s *SQLiteState) networkModify(ctr *Container, network string, opts types.PerNetworkOptions, new, disconnect bool) error {
	if !s.valid {
		return define.ErrDBClosed
	}

	if !ctr.valid {
		return define.ErrCtrRemoved
	}

	if network == "" {
		return fmt.Errorf("network names must not be empty: %w", define.ErrInvalidArg)
	}

	if new && disconnect {
		return fmt.Errorf("new and disconnect are mutually exclusive: %w", define.ErrInvalidArg)
	}

	// Grab a fresh copy of the config, in case anything changed
	newCfg, err := s.getCtrConfig(ctr.ID())
	if err != nil && errors.Is(err, define.ErrNoSuchCtr) {
		ctr.valid = false
		return define.ErrNoSuchCtr
	}

	_, ok := newCfg.Networks[network]
	if new && ok {
		return fmt.Errorf("container %s is already connected to network %s: %w", ctr.ID(), network, define.ErrNoSuchNetwork)
	}
	if !ok && (!new || disconnect) {
		return fmt.Errorf("container %s is not connected to network %s: %w", ctr.ID(), network, define.ErrNoSuchNetwork)
	}

	if !disconnect {
		newCfg.Networks[network] = opts
	} else {
		delete(newCfg.Networks, network)
	}

	if err := s.rewriteContainerConfig(ctr, newCfg); err != nil {
		return err
	}

	ctr.config = newCfg

	return nil
}
