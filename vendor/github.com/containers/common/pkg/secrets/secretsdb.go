package secrets

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"time"

	"github.com/pkg/errors"
)

type db struct {
	// Secrets maps a secret id to secret metadata
	Secrets map[string]Secret `json:"secrets"`
	// NameToID maps a secret name to a secret id
	NameToID map[string]string `json:"nametoid"`
	// IDToName maps a secret id to a secret name
	IDToName map[string]string `json:"idtoname"`
	// lastModified is the time when the database was last modified on the file system
	lastModified time.Time
}

// loadDB loads database data into the in-memory cache if it has been modified
func (s *SecretsManager) loadDB() error {
	// check if the db file exists
	fileInfo, err := os.Stat(secretsFile)
	if err != nil {
		if os.IsNotExist(err) {
			// if the file doesn't exist, then there's no reason to update the db cache
			// the db cache will show no entries anyway
			// the file will be created later on a store()
			return nil
		} else {
			return err
		}
	}

	// we check if the file has been modified after the last time it was loaded into the cache
	// if the file has been modified, then we know that our cache is not up-to-date, so we load
	// the db into the cache
	if !s.db.lastModified.Equal(fileInfo.ModTime()) {
		file, err := os.Open(s.secretsDBPath)
		if err != nil {
			return err
		}
		defer file.Close()
		if err != nil {
			return err
		}

		byteValue, err := ioutil.ReadAll(file)
		if err != nil {
			return err
		}
		unmarshalled := new(db)
		if err := json.Unmarshal(byteValue, unmarshalled); err != nil {
			return err
		}
		s.db = unmarshalled
		s.db.lastModified = fileInfo.ModTime()
	}
	return nil
}

// getNameAndID takes a secret's name or id and returns both its
// identifying name and id
func (s *SecretsManager) getNameAndID(nameOrID string) (name, id string, err error) {
	err = s.loadDB()
	if err != nil {
		return "", "", err
	}
	if id, ok := s.db.NameToID[nameOrID]; ok {
		name := nameOrID
		return name, id, nil
	}

	if name, ok := s.db.IDToName[nameOrID]; ok {
		id := nameOrID
		return name, id, nil
	}
	return "", "", errors.Wrapf(ErrNoSuchSecret, "No secret with name or id %s", nameOrID)
}

// secretExists checks if the secret exists
func (s *SecretsManager) secretExists(nameOrID string) (bool, error) {
	_, _, err := s.getNameAndID(nameOrID)
	if err != nil {
		if errors.Cause(err) == ErrNoSuchSecret {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// lookupAll gets all secrets stored
func (s *SecretsManager) lookupAll() (map[string]Secret, error) {
	err := s.loadDB()
	if err != nil {
		return nil, err
	}
	return s.db.Secrets, nil
}

// lookupSecret returns a secret with the given name or id
func (s *SecretsManager) lookupSecret(nameOrID string) (*Secret, error) {
	_, id, err := s.getNameAndID(nameOrID)
	if err != nil {
		return nil, err
	}
	allSecrets, err := s.lookupAll()
	if err != nil {
		return nil, err
	}
	if secret, ok := allSecrets[id]; ok {
		return &secret, nil
	}
	return nil, ErrNoSuchSecret
}

// Store creates a new secret in the secrets database
// it deals with only storing metadata, not data payload
func (s *SecretsManager) store(entry *Secret) error {
	err := s.loadDB()
	if err != nil {
		return err
	}

	s.db.Secrets[entry.ID] = *entry
	s.db.NameToID[entry.Name] = entry.ID
	s.db.IDToName[entry.ID] = entry.Name

	marshalled, err := json.MarshalIndent(s.db, "", "  ")
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(s.secretsDBPath, marshalled, 0644)
	if err != nil {
		return err
	}

	return nil

}

// delete deletes a secret from the secrets database
// it deals with only deleting metadata, not data payload
func (s *SecretsManager) delete(nameOrID string) error {
	name, id, err := s.getNameAndID(nameOrID)
	if err != nil {
		return err
	}
	err = s.loadDB()
	if err != nil {
		return err
	}
	delete(s.db.Secrets, id)
	delete(s.db.NameToID, name)
	delete(s.db.IDToName, id)
	marshalled, err := json.MarshalIndent(s.db, "", "  ")
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(s.secretsDBPath, marshalled, 0644)
	if err != nil {
		return err
	}
	return nil

}

// sync removes secret entries that do not have a matching entry
// in the driver, and vice versa
func (s *SecretsManager) sync() error {
	s.lockfile.Lock()
	defer s.lockfile.Unlock()
	secrets, err := s.lookupAll()
	if err != nil {
		return err
	}
	secretsIDs, err := s.driver.List()
	if err != nil {
		return err
	}

	// Delete entries from secret database if it does not have a matching driver entry
	for id := range secrets { // if it's in the secrets db
		_, err := s.driver.Lookup(id)
		if err == nil { // but not in the driver db
			if secrets[id].Driver == s.driver.DriverType() { // check if the driver matches
				s.delete(id)
			}

		}
	}

	// Delete entries from driver if it does not have matching entry in database
	for _, id := range secretsIDs {
		if _, ok := secrets[id]; !ok {
			s.driver.Delete(id)
		}
	}
	return nil

}
