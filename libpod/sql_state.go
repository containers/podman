package libpod

import (
	"database/sql"
	"encoding/json"
	"io/ioutil"
	"os"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	// Use SQLite backend for sql package
	_ "github.com/mattn/go-sqlite3"
)

// DBSchema is the current DB schema version
// Increments every time a change is made to the database's tables
const DBSchema = 8

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
                             SubnetMask=?;`

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
		0,
		"",
		"",
		"")
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
                              containerState.Pid,
                              containerState.NetNSPath,
                              containerState.IPAddress,
                              containerState.SubnetMask
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

	ctr, err := s.ctrFromScannable(row)
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
                              containerState.Pid,
                              containerState.NetNSPath,
                              containerState.IPAddress,
                              containerState.SubnetMask
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
	const (
		addCtr = `INSERT INTO containers VALUES (
                    ?, ?, ?, ?, ?,
                    ?, ?, ?, ?, ?,
                    ?, ?, ?, ?, ?,
                    ?, ?, ?, ?, ?,
                    ?, ?, ?, ?, ?,
                    ?, ?, ?, ?, ?,
                    ?, ?, ?, ?
                );`
		addCtrState = `INSERT INTO containerState VALUES (
                    ?, ?, ?, ?, ?,
                    ?, ?, ?, ?, ?,
                    ?, ?, ?
                );`
	)

	if !s.valid {
		return ErrDBClosed
	}

	if !ctr.valid {
		return ErrCtrRemoved
	}

	mounts, err := json.Marshal(ctr.config.Mounts)
	if err != nil {
		return errors.Wrapf(err, "error marshaling container %s mounts to JSON", ctr.ID())
	}

	dnsServerJSON, err := json.Marshal(ctr.config.DNSServer)
	if err != nil {
		return errors.Wrapf(err, "error marshaling container %s DNS servers to JSON", ctr.ID())
	}

	dnsSearchJSON, err := json.Marshal(ctr.config.DNSSearch)
	if err != nil {
		return errors.Wrapf(err, "error marshaling container %s DNS search domains to JSON", ctr.ID())
	}

	dnsOptionJSON, err := json.Marshal(ctr.config.DNSOption)
	if err != nil {
		return errors.Wrapf(err, "error marshaling container %s DNS options to JSON", ctr.ID())
	}

	hostAddJSON, err := json.Marshal(ctr.config.HostAdd)
	if err != nil {
		return errors.Wrapf(err, "error marshaling container %s hosts to JSON", ctr.ID())
	}

	labelsJSON, err := json.Marshal(ctr.config.Labels)
	if err != nil {
		return errors.Wrapf(err, "error marshaling container %s labels to JSON", ctr.ID())
	}

	netNSPath := ""
	if ctr.state.NetNS != nil {
		netNSPath = ctr.state.NetNS.Path()
	}

	specJSON, err := json.Marshal(ctr.config.Spec)
	if err != nil {
		return errors.Wrapf(err, "error marshalling container %s spec to JSON", ctr.ID())
	}

	portsJSON := []byte{}
	if len(ctr.config.PortMappings) > 0 {
		portsJSON, err = json.Marshal(&ctr.config.PortMappings)
		if err != nil {
			return errors.Wrapf(err, "error marshalling container %s port mappings to JSON", ctr.ID())
		}
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

	// Add static container information
	_, err = tx.Exec(addCtr,
		ctr.ID(),
		ctr.Name(),
		stringToNullString(ctr.PodID()),

		ctr.config.RootfsImageID,
		ctr.config.RootfsImageName,
		boolToSQL(ctr.config.ImageVolumes),
		boolToSQL(ctr.config.ReadOnly),
		ctr.config.ShmDir,
		ctr.config.ShmSize,
		ctr.config.StaticDir,
		string(mounts),

		boolToSQL(ctr.config.Privileged),
		boolToSQL(ctr.config.NoNewPrivs),
		ctr.config.ProcessLabel,
		ctr.config.MountLabel,
		ctr.config.User,

		stringToNullString(ctr.config.IPCNsCtr),
		stringToNullString(ctr.config.MountNsCtr),
		stringToNullString(ctr.config.NetNsCtr),
		stringToNullString(ctr.config.PIDNsCtr),
		stringToNullString(ctr.config.UserNsCtr),
		stringToNullString(ctr.config.UTSNsCtr),
		stringToNullString(ctr.config.CgroupNsCtr),

		boolToSQL(ctr.config.CreateNetNS),
		string(dnsServerJSON),
		string(dnsSearchJSON),
		string(dnsOptionJSON),
		string(hostAddJSON),

		boolToSQL(ctr.config.Stdin),
		string(labelsJSON),
		ctr.config.StopSignal,
		ctr.config.StopTimeout,
		timeToSQL(ctr.config.CreatedTime),
		ctr.config.CgroupParent)
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
		ctr.state.PID,
		netNSPath,
		ctr.state.IPAddress,
		ctr.state.SubnetMask)
	if err != nil {
		return errors.Wrapf(err, "error adding container %s state to database", ctr.ID())
	}

	// Save the container's runtime spec to disk
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

	// If the container has port mappings, save them to disk
	if len(ctr.config.PortMappings) > 0 {
		portPath := getPortsPath(s.specsDir, ctr.ID())
		if err := ioutil.WriteFile(portPath, portsJSON, 0750); err != nil {
			return errors.Wrapf(err, "error saving container %s port JSON to disk", ctr.ID())
		}
		defer func() {
			if err != nil {
				if err2 := os.Remove(portPath); err2 != nil {
					logrus.Errorf("Error removing container %s JSON ports from state: %v", ctr.ID(), err2)
				}
			}
		}()
	}

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
                              Pid,
                              NetNSPath,
                              IPAddress,
                              SubnetMask
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
		&subnetMask)
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
	newState.IPAddress = ipAddress
	newState.SubnetMask = subnetMask

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
                          Pid=?,
                          NetNSPath=?,
                          IPAddress=?,
                          SubnetMask=?
                       WHERE Id=?;`

	if !ctr.valid {
		return ErrCtrRemoved
	}

	netNSPath := ""
	if ctr.state.NetNS != nil {
		netNSPath = ctr.state.NetNS.Path()
	}

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

// RemoveContainer removes the container from the state
func (s *SQLState) RemoveContainer(ctr *Container) error {
	const (
		removeCtr   = "DELETE FROM containers WHERE Id=?;"
		removeState = "DELETE FROM containerState WHERE ID=?;"
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

	// Remove containers ports JSON from disk
	// May not exist, so ignore os.IsNotExist
	portsPath := getPortsPath(s.specsDir, ctr.ID())
	if err := os.Remove(portsPath); err != nil {
		if !os.IsNotExist(err) {
			return errors.Wrapf(err, "error removing JSON ports from state for container %s", ctr.ID())
		}
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
                              containerState.Pid,
                              containerState.NetNSPath,
                              containerState.IPAddress,
                              containerState.SubnetMask
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

// PodContainers returns all the containers in a pod given the pod's full ID
func (s *SQLState) PodContainers(id string) ([]*Container, error) {
	return nil, ErrNotImplemented
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
