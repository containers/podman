package filedriver

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/containers/storage/pkg/lockfile"
	"github.com/pkg/errors"
)

// secretsDataFile is the file where secrets data/payload will be stored
var secretsDataFile = "secretsdata.json"

// ErrNoSecretData indicates that there is not data associated with an id
var ErrNoSecretData = errors.New("No secret data with ID")

// ErrNoSecretData indicates that there is secret data already associated with an id
var ErrSecretIDExists = errors.New("Secret data with ID already exists")

// fileDriver is the filedriver object
type FileDriver struct {
	// filePath is the path to the secretsfile
	secretsDataFilePath string
	// drivertype is the string representation of a filedriver
	drivertype string
	// lockfile is the filedriver lockfile
	lockfile lockfile.Locker
	// secretData is an in-memory rep of secrets data
	secretData map[string][]byte
	// lastModified is the time when the database was last modified in memory
	lastModified time.Time
}

// NewFileDriver creates a new file driver
func NewFileDriver(dirPath string) (*FileDriver, error) {
	fileDriver := new(FileDriver)
	fileDriver.secretsDataFilePath = filepath.Join(dirPath, secretsDataFile)
	fileDriver.drivertype = "file"
	lock, err := lockfile.GetLockfile(filepath.Join(dirPath, "secretsdata.lock"))
	if err != nil {
		return nil, err
	}
	fileDriver.lockfile = lock
	fileDriver.secretData = make(map[string][]byte)

	return fileDriver, nil
}

// DriverType returns the string represntation of the filedriver
func (d *FileDriver) DriverType() string {
	return d.drivertype
}

// List returns all secret id's
func (d *FileDriver) List() ([]string, error) {
	d.lockfile.Lock()
	defer d.lockfile.Unlock()
	err := d.loadData()
	if err != nil {
		return nil, err
	}
	var allID []string
	for k := range d.secretData {
		allID = append(allID, k)
	}

	return allID, err
}

// Lookup returns the bytes associated with a secret id
func (d *FileDriver) Lookup(id string) ([]byte, error) {

	d.lockfile.Lock()
	defer d.lockfile.Unlock()

	err := d.loadData()
	if err != nil {
		return nil, err
	}
	if data, ok := d.secretData[id]; ok {
		return data, nil
	} else {
		return nil, errors.Wrapf(ErrNoSecretData, "%s", id)
	}

}

// Store stores the bytes associated with an id
func (d *FileDriver) Store(id string, data []byte) error {
	d.lockfile.Lock()
	defer d.lockfile.Unlock()

	err := d.loadData()
	if err != nil {
		return err
	}
	if _, ok := d.secretData[id]; ok {
		return errors.Wrapf(ErrSecretIDExists, "%s", id)
	}
	d.secretData[id] = data
	marshalled, err := json.MarshalIndent(d.secretData, "", "  ")
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(d.secretsDataFilePath, marshalled, 0644)
	if err != nil {
		return err
	}
	return nil

}

// Delete deletes a secret's data associated with an id
func (d *FileDriver) Delete(id string) error {
	d.lockfile.Lock()
	defer d.lockfile.Unlock()
	err := d.loadData()
	if err != nil {
		return err
	}
	if _, ok := d.secretData[id]; ok {
		delete(d.secretData, id)
	} else {
		return errors.Wrap(ErrNoSecretData, id)
	}
	marshalled, err := json.MarshalIndent(d.secretData, "", "  ")
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(d.secretsDataFilePath, marshalled, 0644)
	if err != nil {
		return err
	}
	return nil
}

// load loads the secret data into memory if it has been modified
func (d *FileDriver) loadData() error {
	fileInfo, err := os.Stat(d.secretsDataFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		} else {
			return err
		}
	}

	if !d.lastModified.Equal(fileInfo.ModTime()) {
		file, err := os.Open(d.secretsDataFilePath)
		if err != nil {
			return err
		}
		defer file.Close()

		byteValue, err := ioutil.ReadAll(file)
		if err != nil {
			return err
		}
		json.Unmarshal([]byte(byteValue), &d.secretData)
		d.lastModified = fileInfo.ModTime()
	}
	return nil

}
