package libpod

import (
	"database/sql"
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"time"

	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	// Use SQLite backend for sql package
	_ "github.com/mattn/go-sqlite3"
)

// Performs database setup including by not limited to initializing tables in
// the database
func prepareDB(db *sql.DB) (err error) {
	// TODO create pod tables
	// TODO add Pod ID to CreateStaticContainer as a FOREIGN KEY referencing podStatic(Id)
	// TODO add ctr shared namespaces information - A separate table, probably? So we can FOREIGN KEY the ID
	// TODO schema migration might be necessary and should be handled here
	// TODO add a table for the runtime, and refuse to load the database if the runtime configuration
	// does not match the one in the database

	// Enable foreign keys in SQLite
	if _, err := db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		return errors.Wrapf(err, "error enabling foreign key support in database")
	}

	// Create a table for unchanging container data
	const createCtr = `
        CREATE TABLE IF NOT EXISTS containers(
            Id TEXT NOT NULL PRIMARY KEY,
            Name TEXT NOT NULL UNIQUE,
            MountLabel TEXT NOT NULL,
            StaticDir TEXT NOT NULL,
            Stdin INTEGER NOT NULL,
            LabelsJSON TEXT NOT NULL,
            StopSignal INTEGER NOT NULL,
            CreatedTime TEXT NOT NULL,
            RootfsImageID TEXT NOT NULL,
            RootfsImageName TEXT NOT NULL,
            UseImageConfig INTEGER NOT NULL,
            CHECK (Stdin IN (0, 1)),
            CHECK (UseImageConfig IN (0, 1)),
            CHECK (StopSignal>=0)
        );
        `

	// Create a table for changing container state
	const createCtrState = `
        CREATE TABLE IF NOT EXISTS containerState(
            Id TEXT NOT NULL PRIMARY KEY,
            State INTEGER NOT NULL,
            ConfigPath TEXT NOT NULL,
            RunDir TEXT NOT NULL,
            Mountpoint TEXT NOT NULL,
            StartedTime TEXT NUT NULL,
            FinishedTime TEXT NOT NULL,
            ExitCode INTEGER NOT NULL,
            OomKilled INTEGER NOT NULL,
            Pid INTEGER NOT NULL,
            CHECK (State>0),
            CHECK (OomKilled IN (0, 1)),
            FOREIGN KEY (Id) REFERENCES containers(Id) DEFERRABLE INITIALLY DEFERRED
        );
        `

	// Create the tables
	tx, err := db.Begin()
	if err != nil {
		return errors.Wrapf(err, "error beginning database transaction")
	}
	defer func() {
		if err != nil {
			if err2 := tx.Rollback(); err2 != nil {
				logrus.Errorf("Error rolling back transaction to create tables: %v", err2)
			}
		}

	}()

	if _, err := tx.Exec(createCtr); err != nil {
		return errors.Wrapf(err, "error creating containers table in database")
	}
	if _, err := tx.Exec(createCtrState); err != nil {
		return errors.Wrapf(err, "error creating container state table in database")
	}

	if err := tx.Commit(); err != nil {
		return errors.Wrapf(err, "error committing table creation transaction in database")
	}

	return nil
}

// Get filename for OCI spec on disk
func getSpecPath(specsDir, id string) string {
	return filepath.Join(specsDir, id)
}

// Convert a bool into SQL-readable format
func boolToSQL(b bool) int {
	if b {
		return 1
	}

	return 0
}

// Convert a bool from SQL-readable format
func boolFromSQL(i int) bool {
	return i != 0
}

// Convert a time.Time into SQL-readable format
func timeToSQL(t time.Time) string {
	return t.Format(time.RFC3339Nano)
}

// Convert a SQL-readable time back to a time.Time
func timeFromSQL(s string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, s)
}

// Interface to abstract sql.Rows and sql.Row so they can both be used
type scannable interface {
	Scan(dest ...interface{}) error
}

// Read a single container from a single row result in the database
func ctrFromScannable(row scannable, runtime *Runtime, specsDir string) (*Container, error) {
	var (
		id                 string
		name               string
		mountLabel         string
		staticDir          string
		stdin              int
		labelsJSON         string
		stopSignal         uint
		createdTimeString  string
		rootfsImageID      string
		rootfsImageName    string
		useImageConfig     int
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

	err := row.Scan(
		&id,
		&name,
		&mountLabel,
		&staticDir,
		&stdin,
		&labelsJSON,
		&stopSignal,
		&createdTimeString,
		&rootfsImageID,
		&rootfsImageName,
		&useImageConfig,
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
		if err == sql.ErrNoRows {
			return nil, ErrNoSuchCtr
		}

		return nil, errors.Wrapf(err, "error parsing database row into container")
	}

	ctr := new(Container)
	ctr.config = new(ContainerConfig)
	ctr.state = new(containerRuntimeInfo)

	ctr.config.ID = id
	ctr.config.Name = name
	ctr.config.RootfsImageID = rootfsImageID
	ctr.config.RootfsImageName = rootfsImageName
	ctr.config.UseImageConfig = boolFromSQL(useImageConfig)
	ctr.config.MountLabel = mountLabel
	ctr.config.StaticDir = staticDir
	ctr.config.Stdin = boolFromSQL(stdin)
	ctr.config.StopSignal = stopSignal

	ctr.state.State = ContainerState(state)
	ctr.state.ConfigPath = configPath
	ctr.state.RunDir = runDir
	ctr.state.Mountpoint = mountpoint
	ctr.state.ExitCode = exitCode
	ctr.state.OOMKilled = boolFromSQL(oomKilled)
	ctr.state.PID = pid

	// TODO should we store this in the database separately instead?
	if ctr.state.Mountpoint != "" {
		ctr.state.Mounted = true
	}

	labels := make(map[string]string)
	if err := json.Unmarshal([]byte(labelsJSON), &labels); err != nil {
		return nil, errors.Wrapf(err, "error parsing container %s labels JSON", id)
	}
	ctr.config.Labels = labels

	createdTime, err := timeFromSQL(createdTimeString)
	if err != nil {
		return nil, errors.Wrapf(err, "error parsing container %s created time", id)
	}
	ctr.config.CreatedTime = createdTime

	startedTime, err := timeFromSQL(startedTimeString)
	if err != nil {
		return nil, errors.Wrapf(err, "error parsing container %s started time", id)
	}
	ctr.state.StartedTime = startedTime

	finishedTime, err := timeFromSQL(finishedTimeString)
	if err != nil {
		return nil, errors.Wrapf(err, "error parsing container %s finished time", id)
	}
	ctr.state.FinishedTime = finishedTime

	ctr.valid = true
	ctr.runtime = runtime

	// Retrieve the spec from disk
	ociSpec := new(spec.Spec)
	specPath := getSpecPath(specsDir, id)
	fileContents, err := ioutil.ReadFile(specPath)
	if err != nil {
		return nil, errors.Wrapf(err, "error reading container %s OCI spec", id)
	}
	if err := json.Unmarshal(fileContents, ociSpec); err != nil {
		return nil, errors.Wrapf(err, "error parsing container %s OCI spec", id)
	}
	ctr.config.Spec = ociSpec

	return ctr, nil
}
