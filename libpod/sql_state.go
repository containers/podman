package libpod

import (
	"database/sql"
	"encoding/json"
	"io/ioutil"
	"os"

	"github.com/containers/storage"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	// Use SQLite backend for sql package
	_ "github.com/mattn/go-sqlite3"
)

// DBSchema is the current DB schema version
// Increments every time a change is made to the database's tables
const DBSchema = 2

// SQLState is a state implementation backed by a persistent SQLite3 database
type SQLState struct {
	db       *sql.DB
	specsDir string
	runtime  *Runtime
	lock     storage.Locker
	valid    bool
}

// NewSQLState initializes a SQL-backed state, created the database if necessary
func NewSQLState(dbPath, lockPath, specsDir string, runtime *Runtime) (State, error) {
	state := new(SQLState)

	state.runtime = runtime

	// Make our lock file
	lock, err := storage.GetLockfile(lockPath)
	if err != nil {
		return nil, errors.Wrapf(err, "error creating lockfile for state")
	}
	state.lock = lock

	// Make the directory that will hold JSON copies of container runtime specs
	if err := os.MkdirAll(specsDir, 0750); err != nil {
		// The directory is allowed to exist
		if !os.IsExist(err) {
			return nil, errors.Wrapf(err, "error creating OCI specs dir %s", specsDir)
		}
	}
	state.specsDir = specsDir

	// Acquire the lock while we open the database and perform initial setup
	state.lock.Lock()
	defer state.lock.Unlock()

	// TODO add a separate temporary database for per-boot container
	// state

	// Open the database
	// Use loc=auto to get accurate locales for timestamps
	db, err := sql.Open("sqlite3", dbPath+"?_loc=auto")
	if err != nil {
		return nil, errors.Wrapf(err, "error opening database")
	}

	// Ensure connectivity
	if err := db.Ping(); err != nil {
		return nil, errors.Wrapf(err, "cannot establish connection to database")
	}

	// Prepare database
	if err := prepareDB(db); err != nil {
		return nil, err
	}

	// Ensure that the database matches our config
	if err := checkDB(db, runtime); err != nil {
		return nil, err
	}

	state.db = db

	state.valid = true

	return state, nil
}

// Close the state's database connection
func (s *SQLState) Close() error {
	s.lock.Lock()
	defer s.lock.Unlock()

	if !s.valid {
		return ErrDBClosed
	}

	s.valid = false

	if err := s.db.Close(); err != nil {
		return errors.Wrapf(err, "error closing database")
	}

	return nil
}

// Refresh clears the state after a reboot
// Resets mountpoint, PID, state for all containers
func (s *SQLState) Refresh() (err error) {
	const refresh = `UPDATE containerState SET
                             State=?,
                             Mountpoint=?,
                             Pid=?;`

	s.lock.Lock()
	defer s.lock.Unlock()

	if !s.valid {
		return ErrDBClosed
	}

	tx, err := s.db.Begin()
	if err != nil {
		return errors.Wrapf(err, "error beginning database transaction")
	}
	defer func() {
		if err != nil {
			if err2 := tx.Rollback(); err2 != nil {
				logrus.Errorf("Error rolling back transaction to refresh state: %v", err2)
			}
		}
	}()

	// Refresh container state
	// The constants could be moved into the SQL, but keeping them here
	// will keep us in sync in case ContainerStateConfigured ever changes in
	// the container state
	_, err = tx.Exec(refresh,
		ContainerStateConfigured,
		"",
		0)
	if err != nil {
		return errors.Wrapf(err, "error refreshing database state")
	}

	if err := tx.Commit(); err != nil {
		return errors.Wrapf(err, "error committing transaction to refresh database")
	}

	return nil
}

// Container retrieves a container from its full ID
func (s *SQLState) Container(id string) (*Container, error) {
	const query = `SELECT containers.*,
                              containerState.State,
                              containerState.ConfigPath,
                              containerState.RunDir,
                              containerState.MountPoint,
                              containerState.StartedTime,
                              containerState.FinishedTime,
                              containerState.ExitCode,
                              containerState.OomKilled,
                              containerState.Pid
                       FROM containers
                       INNER JOIN
                           containerState ON containers.Id = containerState.Id
                       WHERE containers.Id=?;`

	if id == "" {
		return nil, ErrEmptyID
	}

	if !s.valid {
		return nil, ErrDBClosed
	}

	row := s.db.QueryRow(query, id)

	ctr, err := ctrFromScannable(row, s.runtime, s.specsDir)
	if err != nil {
		return nil, errors.Wrapf(err, "error retrieving container %s from database", id)
	}

	return ctr, nil
}

// LookupContainer retrieves a container by full or unique partial ID or name
func (s *SQLState) LookupContainer(idOrName string) (*Container, error) {
	const query = `SELECT containers.*,
                              containerState.State,
                              containerState.ConfigPath,
                              containerState.RunDir,
                              containerState.MountPoint,
                              containerState.StartedTime,
                              containerState.FinishedTime,
                              containerState.ExitCode,
                              containerState.OomKilled,
                              containerState.Pid
                       FROM containers
                       INNER JOIN
                           containerState ON containers.Id = containerState.Id
                       WHERE (containers.Id LIKE ?) OR containers.Name=?;`

	if idOrName == "" {
		return nil, ErrEmptyID
	}

	if !s.valid {
		return nil, ErrDBClosed
	}

	rows, err := s.db.Query(query, idOrName+"%", idOrName)
	if err != nil {
		return nil, errors.Wrapf(err, "error retrieving container %s row from database", idOrName)
	}
	defer rows.Close()

	foundResult := false
	var ctr *Container
	for rows.Next() {
		if foundResult {
			return nil, errors.Wrapf(ErrCtrExists, "more than one result for ID or name %s", idOrName)
		}

		var err error
		ctr, err = ctrFromScannable(rows, s.runtime, s.specsDir)
		if err != nil {
			return nil, errors.Wrapf(err, "error retrieving container %s from database", idOrName)
		}
		foundResult = true
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrapf(err, "error retrieving rows for container ID or name %s", idOrName)
	}

	if !foundResult {
		return nil, errors.Wrapf(ErrNoSuchCtr, "no container with ID or name %s found", idOrName)
	}

	return ctr, nil
}

// HasContainer checks if the given container is present in the state
// It accepts a full ID
func (s *SQLState) HasContainer(id string) (bool, error) {
	const query = "SELECT 1 FROM containers WHERE Id=?;"

	if id == "" {
		return false, ErrEmptyID
	}

	if !s.valid {
		return false, ErrDBClosed
	}

	row := s.db.QueryRow(query, id)

	var check int
	err := row.Scan(&check)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}

		return false, errors.Wrapf(err, "error questing database for existence of container %s", id)
	} else if check != 1 {
		return false, errors.Wrapf(ErrInternal, "check digit for HasContainer query incorrect")
	}

	return true, nil
}

// AddContainer adds the given container to the state
// If the container belongs to a pod, that pod must already be present in the
// state, and the container will be added to the pod
func (s *SQLState) AddContainer(ctr *Container) (err error) {
	const (
		addCtr = `INSERT INTO containers VALUES (
                    ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
                );`
		addCtrState = `INSERT INTO containerState VALUES (
                    ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
                );`
	)

	if !s.valid {
		return ErrDBClosed
	}

	if !ctr.valid {
		return ErrCtrRemoved
	}

	labelsJSON, err := json.Marshal(ctr.config.Labels)
	if err != nil {
		return errors.Wrapf(err, "error marshaling container %s labels to JSON", ctr.ID())
	}

	s.lock.Lock()
	defer s.lock.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return errors.Wrapf(err, "error beginning database transaction")
	}
	defer func() {
		if err != nil {
			if err2 := tx.Rollback(); err2 != nil {
				logrus.Errorf("Error rolling back transaction to add container %s: %v", ctr.ID(), err2)
			}
		}
	}()

	mounts, err := json.Marshal(ctr.config.Mounts)
	if err != nil {
		return errors.Wrapf(err, "error marshaling container %s monunts to JSON", ctr.ID())
	}
	// Add static container information
	_, err = tx.Exec(addCtr,
		ctr.ID(),
		ctr.Name(),
		ctr.config.ProcessLabel,
		ctr.config.MountLabel,
		string(mounts),
		ctr.config.ShmDir,
		ctr.config.StaticDir,
		boolToSQL(ctr.config.Stdin),
		string(labelsJSON),
		ctr.config.StopSignal,
		timeToSQL(ctr.config.CreatedTime),
		ctr.config.RootfsImageID,
		ctr.config.RootfsImageName,
		boolToSQL(ctr.config.UseImageConfig))
	if err != nil {
		return errors.Wrapf(err, "error adding static information for container %s to database", ctr.ID())
	}

	// Add container state to the database
	_, err = tx.Exec(addCtrState,
		ctr.ID(),
		ctr.state.State,
		ctr.state.ConfigPath,
		ctr.state.RunDir,
		ctr.state.Mountpoint,
		timeToSQL(ctr.state.StartedTime),
		timeToSQL(ctr.state.FinishedTime),
		ctr.state.ExitCode,
		boolToSQL(ctr.state.OOMKilled),
		ctr.state.PID)
	if err != nil {
		return errors.Wrapf(err, "error adding container %s state to database", ctr.ID())
	}

	// Save the container's runtime spec to disk
	specJSON, err := json.Marshal(ctr.config.Spec)
	if err != nil {
		return errors.Wrapf(err, "error marshalling container %s spec to JSON", ctr.ID())
	}
	specPath := getSpecPath(s.specsDir, ctr.ID())
	if err := ioutil.WriteFile(specPath, specJSON, 0750); err != nil {
		return errors.Wrapf(err, "error saving container %s spec JSON to disk", ctr.ID())
	}
	defer func() {
		if err != nil {
			if err2 := os.Remove(specPath); err2 != nil {
				logrus.Errorf("Error removing container %s JSON spec from state: %v", ctr.ID(), err2)
			}
		}
	}()

	if err := tx.Commit(); err != nil {
		return errors.Wrapf(err, "error committing transaction to add container %s", ctr.ID())
	}

	return nil
}

// UpdateContainer updates a container's state from the database
func (s *SQLState) UpdateContainer(ctr *Container) error {
	const query = `SELECT State,
                              ConfigPath,
                              RunDir,
                              Mountpoint,
                              StartedTime,
                              FinishedTime,
                              ExitCode,
                              OomKilled,
                              Pid
                       FROM containerState WHERE ID=?;`

	var (
		state              int
		configPath         string
		runDir             string
		mountpoint         string
		startedTimeString  string
		finishedTimeString string
		exitCode           int32
		oomKilled          int
		pid                int
	)

	if !s.valid {
		return ErrDBClosed
	}

	if !ctr.valid {
		return ErrCtrRemoved
	}

	row := s.db.QueryRow(query, ctr.ID())
	err := row.Scan(
		&state,
		&configPath,
		&runDir,
		&mountpoint,
		&startedTimeString,
		&finishedTimeString,
		&exitCode,
		&oomKilled,
		&pid)
	if err != nil {
		// The container may not exist in the database
		if err == sql.ErrNoRows {
			// Assume that the container was removed by another process
			// As such make it invalid
			ctr.valid = false

			return errors.Wrapf(ErrNoSuchCtr, "no container with ID %s found in database", ctr.ID())
		}

		return errors.Wrapf(err, "error parsing database state for container %s", ctr.ID())
	}

	newState := new(containerRuntimeInfo)
	newState.State = ContainerState(state)
	newState.ConfigPath = configPath
	newState.RunDir = runDir
	newState.Mountpoint = mountpoint
	newState.ExitCode = exitCode
	newState.OOMKilled = boolFromSQL(oomKilled)
	newState.PID = pid

	if newState.Mountpoint != "" {
		newState.Mounted = true
	}

	startedTime, err := timeFromSQL(startedTimeString)
	if err != nil {
		return errors.Wrapf(err, "error parsing container %s started time", ctr.ID())
	}
	newState.StartedTime = startedTime

	finishedTime, err := timeFromSQL(finishedTimeString)
	if err != nil {
		return errors.Wrapf(err, "error parsing container %s finished time", ctr.ID())
	}
	newState.FinishedTime = finishedTime

	// New state compiled successfully, swap it into the current state
	ctr.state = newState

	return nil
}

// SaveContainer updates a container's state in the database
func (s *SQLState) SaveContainer(ctr *Container) error {
	const update = `UPDATE containerState SET
                          State=?,
                          ConfigPath=?,
                          RunDir=?,
                          Mountpoint=?,
                          StartedTime=?,
                          FinishedTime=?,
                          ExitCode=?,
                          OomKilled=?,
                          Pid=?
                       WHERE Id=?;`

	s.lock.Lock()
	defer s.lock.Unlock()

	if !s.valid {
		return ErrDBClosed
	}

	if !ctr.valid {
		return ErrCtrRemoved
	}

	tx, err := s.db.Begin()
	if err != nil {
		return errors.Wrapf(err, "error beginning database transaction")
	}
	defer func() {
		if err != nil {
			if err2 := tx.Rollback(); err2 != nil {
				logrus.Errorf("Error rolling back transaction to add container %s: %v", ctr.ID(), err2)
			}
		}
	}()

	// Add container state to the database
	result, err := tx.Exec(update,
		ctr.state.State,
		ctr.state.ConfigPath,
		ctr.state.RunDir,
		ctr.state.Mountpoint,
		timeToSQL(ctr.state.StartedTime),
		timeToSQL(ctr.state.FinishedTime),
		ctr.state.ExitCode,
		boolToSQL(ctr.state.OOMKilled),
		ctr.state.PID,
		ctr.ID())
	if err != nil {
		return errors.Wrapf(err, "error updating container %s state in database", ctr.ID())
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return errors.Wrapf(err, "error retrieving number of rows modified by update of container %s", ctr.ID())
	}
	if rows == 0 {
		return ErrNoSuchCtr
	}

	if err := tx.Commit(); err != nil {
		return errors.Wrapf(err, "error committing transaction to update container %s", ctr.ID())
	}

	return nil
}

// RemoveContainer removes the container from the state
func (s *SQLState) RemoveContainer(ctr *Container) error {
	const (
		removeCtr   = "DELETE FROM containers WHERE Id=?;"
		removeState = "DELETE FROM containerState WHERE ID=?;"
	)

	s.lock.Lock()
	defer s.lock.Unlock()

	if !s.valid {
		return ErrDBClosed
	}

	committed := false

	tx, err := s.db.Begin()
	if err != nil {
		return errors.Wrapf(err, "error beginning database transaction")
	}
	defer func() {
		if err != nil && !committed {
			if err2 := tx.Rollback(); err2 != nil {
				logrus.Errorf("Error rolling back transaction to add container %s: %v", ctr.ID(), err2)
			}
		}
	}()

	// Check rows acted on for the first transaction, verify we actually removed something
	result, err := tx.Exec(removeCtr, ctr.ID())
	if err != nil {
		return errors.Wrapf(err, "error removing container %s from containers table", ctr.ID())
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return errors.Wrapf(err, "error retrieving number of rows in transaction removing container %s", ctr.ID())
	} else if rows == 0 {
		return ErrNoSuchCtr
	}

	if _, err := tx.Exec(removeState, ctr.ID()); err != nil {
		return errors.Wrapf(err, "error removing container %s from state table", ctr.ID())
	}

	if err := tx.Commit(); err != nil {
		return errors.Wrapf(err, "error committing transaction to remove container %s", ctr.ID())
	}

	committed = true

	// Remove the container's JSON from disk
	jsonPath := getSpecPath(s.specsDir, ctr.ID())
	if err := os.Remove(jsonPath); err != nil {
		return errors.Wrapf(err, "error removing JSON spec from state for container %s", ctr.ID())
	}

	ctr.valid = false

	return nil
}

// AllContainers retrieves all the containers presently in the state
func (s *SQLState) AllContainers() ([]*Container, error) {
	// TODO maybe do an ORDER BY here?
	const query = `SELECT containers.*,
                              containerState.State,
                              containerState.ConfigPath,
                              containerState.RunDir,
                              containerState.MountPoint,
                              containerState.StartedTime,
                              containerState.FinishedTime,
                              containerState.ExitCode,
                              containerState.OomKilled,
                              containerState.Pid
                      FROM containers
                      INNER JOIN
                          containerState ON containers.Id = containerState.Id
                      ORDER BY containers.CreatedTime DESC;`

	if !s.valid {
		return nil, ErrDBClosed
	}

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, errors.Wrapf(err, "error retrieving containers from database")
	}
	defer rows.Close()

	containers := []*Container{}

	for rows.Next() {
		ctr, err := ctrFromScannable(rows, s.runtime, s.specsDir)
		if err != nil {
			return nil, err
		}

		containers = append(containers, ctr)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrapf(err, "error retrieving container rows")
	}

	return containers, nil
}

// Pod retrieves a pod by its full ID
func (s *SQLState) Pod(id string) (*Pod, error) {
	return nil, ErrNotImplemented
}

// LookupPod retrieves a pot by full or unique partial ID or name
func (s *SQLState) LookupPod(idOrName string) (*Pod, error) {
	return nil, ErrNotImplemented
}

// HasPod checks if a pod exists given its full ID
func (s *SQLState) HasPod(id string) (bool, error) {
	return false, ErrNotImplemented
}

// AddPod adds a pod to the state
// Only empty pods can be added to the state
func (s *SQLState) AddPod(pod *Pod) error {
	return ErrNotImplemented
}

// RemovePod removes a pod from the state
// Only empty pods can be removed
func (s *SQLState) RemovePod(pod *Pod) error {
	return ErrNotImplemented
}

// AllPods retrieves all pods presently in the state
func (s *SQLState) AllPods() ([]*Pod, error) {
	return nil, ErrNotImplemented
}
