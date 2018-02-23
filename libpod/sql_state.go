package libpod

import (
	"database/sql"
	"encoding/json"
	"os"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	// Use SQLite backend for sql package
	_ "github.com/mattn/go-sqlite3"
)

// DBSchema is the current DB schema version
// Increments every time a change is made to the database's tables
const DBSchema = 12

// SQLState is a state implementation backed by a persistent SQLite3 database
type SQLState struct {
	db       *sql.DB
	specsDir string
	lockDir  string
	runtime  *Runtime
	valid    bool
}

// NewSQLState initializes a SQL-backed state, created the database if necessary
func NewSQLState(dbPath, specsDir, lockDir string, runtime *Runtime) (State, error) {
	state := new(SQLState)

	state.runtime = runtime

	// Make the directory that will hold JSON copies of container runtime specs
	if err := os.MkdirAll(specsDir, 0750); err != nil {
		// The directory is allowed to exist
		if !os.IsExist(err) {
			return nil, errors.Wrapf(err, "error creating OCI specs dir %s", specsDir)
		}
	}
	state.specsDir = specsDir

	// Make the directory that will hold container lockfiles
	if err := os.MkdirAll(lockDir, 0750); err != nil {
		// The directory is allowed to exist
		if !os.IsExist(err) {
			return nil, errors.Wrapf(err, "error creating lockfiles dir %s", lockDir)
		}
	}
	state.lockDir = lockDir

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
// Resets mountpoint, PID, state, netns path for all containers
func (s *SQLState) Refresh() (err error) {
	const refresh = `UPDATE containerState SET
                             State=?,
                             Mountpoint=?,
                             Pid=?,
                             NetNSPath=?,
                             IPAddress=?,
                             SubnetMask=?,
                             ExecSessions=?;`

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
		0,
		"",
		"",
		"",
		"{}")
	if err != nil {
		return errors.Wrapf(err, "error refreshing database state")
	}

	committed = true

	if err := tx.Commit(); err != nil {
		return errors.Wrapf(err, "error committing transaction to refresh database")
	}

	return nil
}

// Container retrieves a container from its full ID
func (s *SQLState) Container(id string) (*Container, error) {
	const query = containerQuery + "WHERE containers.Id=?;"

	if id == "" {
		return nil, ErrEmptyID
	}

	if !s.valid {
		return nil, ErrDBClosed
	}

	row := s.db.QueryRow(query, id)

	ctr, err := s.ctrFromScannable(row)
	if err != nil {
		return nil, errors.Wrapf(err, "error retrieving container %s from database", id)
	}

	return ctr, nil
}

// LookupContainer retrieves a container by full or unique partial ID or name
func (s *SQLState) LookupContainer(idOrName string) (*Container, error) {
	const query = containerQuery + "WHERE (containers.Id LIKE ?) OR containers.Name=?;"

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
		ctr, err = s.ctrFromScannable(rows)
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
	if !ctr.valid {
		return ErrCtrRemoved
	}

	if ctr.config.Pod != "" {
		return errors.Wrapf(ErrPodExists, "cannot add container that belongs to a pod, use AddContainerToPod instead")
	}

	return s.addContainer(ctr, nil)
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
                              Pid,
                              NetNSPath,
                              IPAddress,
                              SubnetMask,
                              ExecSessions
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
		netNSPath          string
		ipAddress          string
		subnetMask         string
		execSessions       string
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
		&pid,
		&netNSPath,
		&ipAddress,
		&subnetMask,
	        &execSessions)
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

	newState := new(containerState)
	newState.State = ContainerStatus(state)
	newState.ConfigPath = configPath
	newState.RunDir = runDir
	newState.Mountpoint = mountpoint
	newState.ExitCode = exitCode
	newState.OOMKilled = boolFromSQL(oomKilled)
	newState.PID = pid
	newState.IPAddress = ipAddress
	newState.SubnetMask = subnetMask

	newState.ExecSessions = make(map[string]int)
	if err := json.Unmarshal([]byte(execSessions), &newState.ExecSessions); err != nil {
		return errors.Wrapf(err, "error parsing container %s exec sessions", ctr.ID())
	}

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

	// Do we need to replace the container's netns?
	if netNSPath != "" {
		// Check if the container's old state has a good netns
		if ctr.state.NetNS != nil && netNSPath == ctr.state.NetNS.Path() {
			newState.NetNS = ctr.state.NetNS
		} else {
			// Tear down the existing namespace
			if err := s.runtime.teardownNetNS(ctr); err != nil {
				return err
			}

			// Open the new network namespace
			ns, err := joinNetNS(netNSPath)
			if err != nil {
				return errors.Wrapf(err, "error joining network namespace for container %s", ctr.ID())
			}
			newState.NetNS = ns
		}
	} else {
		// The container no longer has a network namespace
		// Tear down the old one
		if err := s.runtime.teardownNetNS(ctr); err != nil {
			return err
		}
	}

	// New state compiled successfully, swap it into the current state
	ctr.state = newState

	return nil
}

// SaveContainer updates a container's state in the database
func (s *SQLState) SaveContainer(ctr *Container) (err error) {
	const update = `UPDATE containerState SET
                          State=?,
                          ConfigPath=?,
                          RunDir=?,
                          Mountpoint=?,
                          StartedTime=?,
                          FinishedTime=?,
                          ExitCode=?,
                          OomKilled=?,
                          Pid=?,
                          NetNSPath=?,
                          IPAddress=?,
                          SubnetMask=?,
                          ExecSessions=?
                       WHERE Id=?;`

	if !ctr.valid {
		return ErrCtrRemoved
	}

	execSessionsJSON, err := json.Marshal(ctr.state.ExecSessions)
	if err != nil {
		return errors.Wrapf(err, "error marshalling container %s exec sessions", ctr.ID())
	}

	netNSPath := ""
	if ctr.state.NetNS != nil {
		netNSPath = ctr.state.NetNS.Path()
	}

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
		netNSPath,
		ctr.state.IPAddress,
		ctr.state.SubnetMask,
		execSessionsJSON,
		ctr.ID())
	if err != nil {
		return errors.Wrapf(err, "error updating container %s state in database", ctr.ID())
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return errors.Wrapf(err, "error retrieving number of rows modified by update of container %s", ctr.ID())
	}
	if rows == 0 {
		// Container was probably removed elsewhere
		ctr.valid = false
		return ErrNoSuchCtr
	}

	committed = true

	if err := tx.Commit(); err != nil {
		return errors.Wrapf(err, "error committing transaction to update container %s", ctr.ID())
	}

	return nil
}

// ContainerInUse checks if other containers depend on the given container
// It returns the IDs of containers which depend on the given container
func (s *SQLState) ContainerInUse(ctr *Container) ([]string, error) {
	const inUseQuery = `SELECT Id FROM containers WHERE
                                IPCNsCtr=?   OR
                                MountNsCtr=? OR
                                NetNsCtr=?   OR
                                PIDNsCtr=?   OR
                                UserNsCtr=?  OR
                                UTSNsCtr=?   OR
                                CgroupNsCtr=?;`

	if !s.valid {
		return nil, ErrDBClosed
	}

	if !ctr.valid {
		return nil, ErrCtrRemoved
	}

	id := ctr.ID()

	rows, err := s.db.Query(inUseQuery, id, id, id, id, id, id, id)
	if err != nil {
		return nil, errors.Wrapf(err, "error querying database for containers that depend on container %s", id)
	}
	defer rows.Close()

	ids := []string{}

	for rows.Next() {
		var ctrID string
		if err := rows.Scan(&ctrID); err != nil {
			return nil, errors.Wrapf(err, "error scanning container IDs from db rows for container %s", id)
		}

		ids = append(ids, ctrID)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrapf(err, "error retrieving rows for container %s", id)
	}

	return ids, nil
}

// RemoveContainer removes the given container from the state
func (s *SQLState) RemoveContainer(ctr *Container) error {
	if ctr.config.Pod != "" {
		return errors.Wrapf(ErrPodExists, "container %s belongs to a pod, use RemoveContainerFromPod", ctr.ID())
	}

	return s.removeContainer(ctr, nil)
}

// AllContainers retrieves all the containers presently in the state
func (s *SQLState) AllContainers() ([]*Container, error) {
	const query = containerQuery + ";"

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
		ctr, err := s.ctrFromScannable(rows)
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
	const query = "SELECT * FROM pods WHERE Id=?;"

	if !s.valid {
		return nil, ErrDBClosed
	}

	if id == "" {
		return nil, ErrEmptyID
	}

	row := s.db.QueryRow(query, id)

	pod, err := s.podFromScannable(row)
	if err != nil {
		return nil, errors.Wrapf(err, "error retrieving pod %s from database", id)
	}

	return pod, nil
}

// LookupPod retrieves a pot by full or unique partial ID or name
func (s *SQLState) LookupPod(idOrName string) (*Pod, error) {
	const query = "SELECT * FROM pods WHERE (Id LIKE ?) OR Name=?;"

	if idOrName == "" {
		return nil, ErrEmptyID
	}

	if !s.valid {
		return nil, ErrDBClosed
	}

	rows, err := s.db.Query(query, idOrName+"%", idOrName)
	if err != nil {
		return nil, errors.Wrapf(err, "error retrieving pod %s row from database", idOrName)
	}
	defer rows.Close()

	foundResult := false
	var pod *Pod
	for rows.Next() {
		if foundResult {
			return nil, errors.Wrapf(ErrCtrExists, "more than one result for ID or name %s", idOrName)
		}

		var err error
		pod, err = s.podFromScannable(rows)
		if err != nil {
			return nil, errors.Wrapf(err, "error retrieving pod %s from database", idOrName)
		}
		foundResult = true
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrapf(err, "error retrieving rows for pod ID or name %s", idOrName)
	}

	if !foundResult {
		return nil, errors.Wrapf(ErrNoSuchCtr, "no pod with ID or name %s found", idOrName)
	}

	return pod, nil
}

// HasPod checks if a pod exists given its full ID
func (s *SQLState) HasPod(id string) (bool, error) {
	if id == "" {
		return false, ErrEmptyID
	}

	if !s.valid {
		return false, ErrDBClosed
	}

	return s.podExists(id)
}

// PodHasContainer checks if the given pod containers a container with the given
// ID
func (s *SQLState) PodHasContainer(pod *Pod, ctrID string) (bool, error) {
	const query = "SELECT 1 FROM containers WHERE Id=? AND Pod=?;"

	if ctrID == "" {
		return false, ErrEmptyID
	}

	if !s.valid {
		return false, ErrDBClosed
	}

	if !pod.valid {
		return false, ErrPodRemoved
	}

	row := s.db.QueryRow(query, ctrID, pod.ID())

	var check int
	err := row.Scan(&check)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}

		return false, errors.Wrapf(err, "error questing database for existence of container %s", ctrID)
	} else if check != 1 {
		return false, errors.Wrapf(ErrInternal, "check digit for PodHasContainer query incorrect")
	}

	return true, nil
}

// PodContainersByID returns the container IDs of all containers in the given
// pod
func (s *SQLState) PodContainersByID(pod *Pod) ([]string, error) {
	const query = "SELECT Id FROM containers WHERE Pod=?;"

	if !s.valid {
		return nil, ErrDBClosed
	}

	if !pod.valid {
		return nil, ErrPodRemoved
	}

	// Check to make sure pod still exists in DB
	exists, err := s.podExists(pod.ID())
	if err != nil {
		return nil, err
	}
	if !exists {
		pod.valid = false
		return nil, ErrPodRemoved
	}

	// Get actual containers
	rows, err := s.db.Query(query, pod.ID())
	if err != nil {
		return nil, errors.Wrapf(err, "error retrieving containers from database")
	}
	defer rows.Close()

	containers := []string{}

	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			if err == sql.ErrNoRows {
				return nil, ErrNoSuchCtr
			}

			return nil, errors.Wrapf(err, "error parsing database row into container ID")
		}

		containers = append(containers, id)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrapf(err, "error retrieving container rows")
	}

	return containers, nil
}

// PodContainers returns all the containers in a pod given the pod's full ID
func (s *SQLState) PodContainers(pod *Pod) ([]*Container, error) {
	const query = containerQuery + "WHERE containers.Pod=?;"

	if !s.valid {
		return nil, ErrDBClosed
	}

	if !pod.valid {
		return nil, ErrPodRemoved
	}

	// Check to make sure pod still exists in DB
	exists, err := s.podExists(pod.ID())
	if err != nil {
		return nil, err
	}
	if !exists {
		pod.valid = false
		return nil, ErrPodRemoved
	}

	// Get actual containers
	rows, err := s.db.Query(query, pod.ID())
	if err != nil {
		return nil, errors.Wrapf(err, "error retrieving containers from database")
	}
	defer rows.Close()

	containers := []*Container{}

	for rows.Next() {
		ctr, err := s.ctrFromScannable(rows)
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

// AddPod adds a pod to the state
// Only empty pods can be added to the state
func (s *SQLState) AddPod(pod *Pod) (err error) {
	const (
		podQuery      = "INSERT INTO pods VALUES (?, ?, ?);"
		registryQuery = "INSERT INTO registry VALUES (?, ?);"
	)

	if !s.valid {
		return ErrDBClosed
	}

	if !pod.valid {
		return ErrPodRemoved
	}

	labelsJSON, err := json.Marshal(pod.config.Labels)
	if err != nil {
		return errors.Wrapf(err, "error marshaling pod %s labels to JSON", pod.ID())
	}

	committed := false

	tx, err := s.db.Begin()
	if err != nil {
		return errors.Wrapf(err, "error beginning database transaction")
	}
	defer func() {
		if err != nil && !committed {
			if err2 := tx.Rollback(); err2 != nil {
				logrus.Errorf("Error rolling back transaction to add pod %s: %v", pod.ID(), err2)
			}
		}
	}()

	if _, err := tx.Exec(registryQuery, pod.ID(), pod.Name()); err != nil {
		return errors.Wrapf(err, "error adding pod %s to name/ID registry", pod.ID())
	}

	if _, err = tx.Exec(podQuery, pod.ID(), pod.Name(), string(labelsJSON)); err != nil {
		return errors.Wrapf(err, "error adding pod %s to database", pod.ID())
	}

	committed = true

	if err := tx.Commit(); err != nil {
		return errors.Wrapf(err, "error committing transaction to add pod %s", pod.ID())
	}

	return nil
}

// RemovePod removes a pod from the state
// Only empty pods can be removed
func (s *SQLState) RemovePod(pod *Pod) (err error) {
	const (
		removePod      = "DELETE FROM pods WHERE ID=?;"
		removeRegistry = "DELETE FROM registry WHERE Id=?;"
	)

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
				logrus.Errorf("Error rolling back transaction to remove pod %s: %v", pod.ID(), err2)
			}
		}
	}()

	// Check rows acted on for the first statement, verify we actually removed something
	result, err := tx.Exec(removePod, pod.ID())
	if err != nil {
		return errors.Wrapf(err, "error removing pod %s from pods table", pod.ID())
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return errors.Wrapf(err, "error retrieving number of rows in transaction removing pod %s", pod.ID())
	} else if rows == 0 {
		pod.valid = false
		return ErrNoSuchPod
	}

	// We know it exists, remove it from registry
	if _, err := tx.Exec(removeRegistry, pod.ID()); err != nil {
		return errors.Wrapf(err, "error removing pod %s from name/ID registry", pod.ID())
	}

	committed = true

	if err := tx.Commit(); err != nil {
		return errors.Wrapf(err, "error committing transaction to remove pod %s", pod.ID())
	}

	return nil
}

// RemovePodContainers removes all containers in a pod simultaneously
// This can avoid issues with dependencies within the pod
// The operation will fail if any container in the pod has a dependency from
// outside the pod
func (s *SQLState) RemovePodContainers(pod *Pod) (err error) {
	const (
		getPodCtrs     = "SELECT Id FROM containers WHERE pod=?;"
		removeCtr      = "DELETE FROM containers WHERE pod=?;"
		removeCtrState = "DELETE FROM containerState WHERE ID IN (SELECT Id FROM containers WHERE pod=?);"
	)

	if !s.valid {
		return ErrDBClosed
	}

	if !pod.valid {
		return ErrPodRemoved
	}

	committed := false

	tx, err := s.db.Begin()
	if err != nil {
		return errors.Wrapf(err, "error beginning database transaction")
	}
	defer func() {
		if err != nil && !committed {
			if err2 := tx.Rollback(); err2 != nil {
				logrus.Errorf("Error rolling back transaction to remove pod %s containers: %v", pod.ID(), err2)
			}
		}
	}()

	// Check if the pod exists
	exists, err := podExistsTx(pod.ID(), tx)
	if err != nil {
		return err
	}
	if !exists {
		pod.valid = false
		return ErrNoSuchPod
	}

	// First get all containers in the pod
	rows, err := tx.Query(getPodCtrs, pod.ID())
	if err != nil {
		return errors.Wrapf(err, "error retrieving containers from database")
	}
	defer rows.Close()

	containers := []string{}

	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			if err == sql.ErrNoRows {
				return ErrNoSuchCtr
			}

			return errors.Wrapf(err, "error parsing database row into container ID")
		}

		containers = append(containers, id)
	}
	if err := rows.Err(); err != nil {
		return errors.Wrapf(err, "error retrieving container rows")
	}

	// Have container IDs, now exec SQL to remove containers in the pod
	// Remove state first, as it needs the subquery on containers
	// Don't bother checking if we actually removed anything, we just want to
	// empty the pod
	if _, err := tx.Exec(removeCtrState, pod.ID()); err != nil {
		return errors.Wrapf(err, "error removing pod %s containers from state table", pod.ID())
	}

	if _, err := tx.Exec(removeCtr, pod.ID()); err != nil {
		return errors.Wrapf(err, "error removing pod %s containers from containers table", pod.ID())
	}

	committed = true

	if err := tx.Commit(); err != nil {
		return errors.Wrapf(err, "error committing transaction remove pod %s containers", pod.ID())
	}

	// Remove JSON files from the containers in question
	hasError := false
	for _, ctr := range containers {
		jsonPath := getSpecPath(s.specsDir, ctr)
		if err := os.Remove(jsonPath); err != nil {
			logrus.Errorf("Error removing spec JSON for container %s: %v", ctr, err)
			hasError = true
		}

		portsPath := getPortsPath(s.specsDir, ctr)
		if err := os.Remove(portsPath); err != nil {
			if !os.IsNotExist(err) {
				logrus.Errorf("Error removing ports JSON for container %s: %v", ctr, err)
				hasError = true
			}
		}
	}
	if hasError {
		return errors.Wrapf(ErrInternal, "error removing JSON state for some containers")
	}

	return nil
}

// AddContainerToPod adds a container to the given pod
func (s *SQLState) AddContainerToPod(pod *Pod, ctr *Container) error {
	if !pod.valid {
		return ErrPodRemoved
	}

	if !ctr.valid {
		return ErrCtrRemoved
	}

	if ctr.config.Pod != pod.ID() {
		return errors.Wrapf(ErrInvalidArg, "container's pod ID does not match given pod's ID")
	}

	return s.addContainer(ctr, pod)
}

// RemoveContainerFromPod removes a container from the given pod
func (s *SQLState) RemoveContainerFromPod(pod *Pod, ctr *Container) error {
	if ctr.config.Pod != pod.ID() {
		return errors.Wrapf(ErrInvalidArg, "container %s is not in pod %s", ctr.ID(), pod.ID())
	}

	return s.removeContainer(ctr, pod)
}

// AllPods retrieves all pods presently in the state
func (s *SQLState) AllPods() ([]*Pod, error) {
	const query = "SELECT * FROM pods;"

	if !s.valid {
		return nil, ErrDBClosed
	}

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, errors.Wrapf(err, "error querying database for all pods")
	}
	defer rows.Close()

	pods := []*Pod{}

	for rows.Next() {
		pod, err := s.podFromScannable(rows)
		if err != nil {
			return nil, err
		}

		pods = append(pods, pod)
	}

	if err := rows.Err(); err != nil {
		return nil, errors.Wrapf(err, "error retrieving pod rows")
	}

	return pods, nil
}
