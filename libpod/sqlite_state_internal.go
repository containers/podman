package libpod

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/containers/podman/v4/libpod/define"
	"github.com/sirupsen/logrus"

	_ "github.com/mattn/go-sqlite3"
)

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
		"DBConfig": dbConfig,
		"IDNamespace": idNamespace,
		"ContainerConfig": containerConfig,
		"ContainerState": containerState,
		"ContainerExecSession": containerExecSession,
		"ContainerDependency": containerDependency,
		"ContainerVolume": containerVolume,
		"ContainerExitCode": containerExitCode,
		"PodConfig": podConfig,
		"PodState": podState,
		"VolumeConfig": volumeConfig,
		"volumeState": volumeState,
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
