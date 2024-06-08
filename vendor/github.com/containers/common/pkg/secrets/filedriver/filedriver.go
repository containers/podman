package filedriver

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/containers/storage/pkg/lockfile"
)

// secretsDataFile is the file where secrets data/payload will be stored
var secretsDataFile = "secretsdata.json"

// errNoSecretData indicates that there is not data associated with an id
var errNoSecretData = errors.New("no secret data with ID")

// errNoSecretData indicates that there is secret data already associated with an id
var errSecretIDExists = errors.New("secret data with ID already exists")

// Driver is the filedriver object
type Driver struct {
	// secretsDataFilePath is the path to the secretsfile
	secretsDataFilePath string
	// lockfile is the filedriver lockfile
	lockfile *lockfile.LockFile
}

// NewDriver creates a new file driver.
// rootPath is the directory where the secrets data file resides.
func NewDriver(rootPath string) (*Driver, error) {
	fileDriver := new(Driver)
	fileDriver.secretsDataFilePath = filepath.Join(rootPath, secretsDataFile)
	// the lockfile functions require that the rootPath dir is executable
	if err := os.MkdirAll(rootPath, 0o700); err != nil {
		return nil, err
	}

	lock, err := lockfile.GetLockFile(filepath.Join(rootPath, "secretsdata.lock"))
	if err != nil {
		return nil, err
	}
	fileDriver.lockfile = lock

	return fileDriver, nil
}

// List returns all secret IDs
func (d *Driver) List() ([]string, error) {
	d.lockfile.Lock()
	defer d.lockfile.Unlock()
	secretData, err := d.getAllData()
	if err != nil {
		return nil, err
	}
	allID := make([]string, 0, len(secretData))
	for k := range secretData {
		allID = append(allID, k)
	}
	sort.Strings(allID)
	return allID, err
}

// Lookup returns the bytes associated with a secret ID
func (d *Driver) Lookup(id string) ([]byte, error) {
	d.lockfile.Lock()
	defer d.lockfile.Unlock()

	secretData, err := d.getAllData()
	if err != nil {
		return nil, err
	}
	if data, ok := secretData[id]; ok {
		return data, nil
	}
	return nil, fmt.Errorf("%s: %w", id, errNoSecretData)
}

// Store stores the bytes associated with an ID. An error is returned if the ID arleady exists
func (d *Driver) Store(id string, data []byte) error {
	d.lockfile.Lock()
	defer d.lockfile.Unlock()

	secretData, err := d.getAllData()
	if err != nil {
		return err
	}
	if _, ok := secretData[id]; ok {
		return fmt.Errorf("%s: %w", id, errSecretIDExists)
	}
	secretData[id] = data
	marshalled, err := json.MarshalIndent(secretData, "", "  ")
	if err != nil {
		return err
	}
	err = os.WriteFile(d.secretsDataFilePath, marshalled, 0o600)
	if err != nil {
		return err
	}
	return nil
}

// Delete deletes the secret associated with the specified ID.  An error is returned if no matching secret is found.
func (d *Driver) Delete(id string) error {
	d.lockfile.Lock()
	defer d.lockfile.Unlock()
	secretData, err := d.getAllData()
	if err != nil {
		return err
	}
	if _, ok := secretData[id]; ok {
		delete(secretData, id)
	} else {
		return fmt.Errorf("%s: %w", id, errNoSecretData)
	}
	marshalled, err := json.MarshalIndent(secretData, "", "  ")
	if err != nil {
		return err
	}
	err = os.WriteFile(d.secretsDataFilePath, marshalled, 0o600)
	if err != nil {
		return err
	}
	return nil
}

// getAllData reads the data file and returns all data
func (d *Driver) getAllData() (map[string][]byte, error) {
	// check if the db file exists
	_, err := os.Stat(d.secretsDataFilePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// the file will be created later on a store()
			return make(map[string][]byte), nil
		}
		return nil, err
	}

	file, err := os.Open(d.secretsDataFilePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	byteValue, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	secretData := new(map[string][]byte)
	err = json.Unmarshal([]byte(byteValue), secretData)
	if err != nil {
		return nil, err
	}
	return *secretData, nil
}
