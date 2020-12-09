package secrets

import (
	"path/filepath"
	"time"

	"github.com/containers/storage/pkg/lockfile"
	"github.com/containers/storage/pkg/stringid"
	"github.com/pkg/errors"
)

// ErrInvalidPath indicates that the secrets path is invalid
var ErrInvalidPath = errors.New("Invalid secrets path")

// ErrNoSuchSecret indicates that the the secret does not exist
var ErrNoSuchSecret = errors.New("no such secret")

// ErrSecretNameInUse indicates that the secret name is already in use
var ErrSecretNameInUse = errors.New("secret name in use")

// ErrInvalidSecretName indicates that the secret name is invalid
var ErrInvalidSecretName = errors.New("invalid secret name")

// secretsFile is the name of the file that the secrets database will be stored in
var secretsFile = "secrets.json"

// SecretsManager holds information on handling secrets
type SecretsManager struct {
	// secretsPath is the path to the db file where secrets are stored
	secretsDBPath string
	// driver is the secrets data driver
	driver SecretsDriver
	// lockfile is the locker for the secrets file
	lockfile lockfile.Locker
	// db is an in-memory cache of the database of secrets
	db *db
}

// Secret defines a secret
type Secret struct {
	// Name is the name of the secret
	Name string `json:"name"`
	// ID is the unique secret ID
	ID string `json:"id"`
	// Metadata stores other metadata on the secret
	Metadata map[string]string `json:"metadata,omitempty"`
	// CreatedAt is when the secret was created
	CreatedAt time.Time
	// Driver is the driver used to store secret data
	Driver string
}

// SecretsDriver interfaces with the secrets data store.
// The driver stores the actual bytes of secret data, as opposed to
// the secret metadata.
// Currently only the unencrypted filedriver and an in-memory driver
// used for testing are implemented.
type SecretsDriver interface {
	// List lists all secret ids in the secrets data store
	List() ([]string, error)
	// Lookup gets the secret's data bytes
	Lookup(id string) ([]byte, error)
	// Store stores the secret's data bytes
	Store(id string, data []byte) error
	// Delete deletes a secret's data from the driver
	Delete(id string) error
	// DriverType returns the driver type
	DriverType() string
}

// NewManager creates a new secrets manager
func NewManager(dirPath string, driver SecretsDriver) (*SecretsManager, error) {
	manager := new(SecretsManager)

	if !filepath.IsAbs(dirPath) {
		return nil, errors.Wrapf(ErrInvalidPath, "path must be absolute: %s", dirPath)
	}
	lock, err := lockfile.GetLockfile(filepath.Join(dirPath, "secrets.lock"))
	if err != nil {
		return nil, err
	}
	manager.lockfile = lock
	manager.driver = driver
	manager.secretsDBPath = filepath.Join(dirPath, secretsFile)
	manager.db = new(db)
	manager.db.Secrets = make(map[string]Secret)
	manager.db.NameToID = make(map[string]string)
	manager.db.IDToName = make(map[string]string)
	return manager, nil
}

// Store creates a secret and stores it, given a name.
// It stores secret metadata as well as the secret payload
func (s *SecretsManager) Store(name string, data []byte) (string, error) {
	err := validateSecretName(name)
	if err != nil {
		return "", err
	}

	s.lockfile.Lock()
	defer s.lockfile.Unlock()

	exist, err := s.secretExists(name)
	if err != nil {
		return "", err
	}
	if exist {
		return "", errors.Wrapf(ErrSecretNameInUse, name)
	}

	secr := new(Secret)
	secr.Name = name

	for {
		newID := stringid.GenerateNonCryptoID()
		_, err := s.lookupSecret(newID)
		if err != nil {
			if errors.Cause(err) == ErrNoSuchSecret {
				secr.ID = newID
				break
			} else {
				return "", err
			}
		}
	}

	secr.Driver = s.driver.DriverType()
	secr.Metadata = make(map[string]string)
	secr.CreatedAt = time.Now()

	err = s.driver.Store(secr.ID, data)
	if err != nil {
		return "", errors.Wrapf(err, "error creating secret %s", name)
	}

	err = s.store(secr)
	if err != nil {
		return "", errors.Wrapf(err, "error creating secret %s", name)
	}

	return secr.ID, nil
}

// Delete removes a secret
// It removes secret metadata as well as the secret data
func (s *SecretsManager) Delete(nameOrID string) (string, error) {
	err := validateSecretName(nameOrID)
	if err != nil {
		return "", err
	}

	s.lockfile.Lock()
	defer s.lockfile.Unlock()

	_, id, err := s.getNameAndID(nameOrID)
	if err != nil {
		return "", err
	}

	err = s.driver.Delete(id)
	if err != nil {
		return "", errors.Wrapf(err, "error deleting secret %s", nameOrID)
	}

	err = s.delete(id)
	if err != nil {
		return "", errors.Wrapf(err, "error deleting secret %s", nameOrID)
	}
	return id, nil
}

// Lookup gives a secret's metadata
func (s *SecretsManager) Lookup(namesOrIDs []string) ([]*Secret, error) {
	if len(namesOrIDs) == 0 {
		return nil, ErrInvalidSecretName
	}
	var lookups []*Secret
	for _, nameOrID := range namesOrIDs {
		secret, err := s.lookupSecret(nameOrID)
		if err != nil {
			return nil, err
		}
		lookups = append(lookups, secret)
	}

	return lookups, nil
}

// List lists all secrets
func (s *SecretsManager) List() ([]Secret, error) {
	s.lockfile.Lock()
	defer s.lockfile.Unlock()

	secrets, err := s.lookupAll()
	if err != nil {
		return nil, err
	}
	var ls []Secret
	for _, v := range secrets {
		ls = append(ls, v)

	}
	return ls, nil

}

// validateSecretName checks if the secret name is valid
func validateSecretName(name string) error {
	if name == "" {
		return errors.Wrap(ErrInvalidSecretName, name)
	}
	return nil
}
