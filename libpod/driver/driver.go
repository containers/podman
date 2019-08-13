package driver

import (
	cstorage "github.com/containers/storage"
)

// Data handles the data for a storage driver
type Data struct {
	Name string            `json:"Name"`
	Data map[string]string `json:"Data"`
}

// GetDriverName returns the name of the driver for the given store
func GetDriverName(store cstorage.Store) (string, error) {
	driver, err := store.GraphDriver()
	if err != nil {
		return "", err
	}
	return driver.String(), nil
}

// GetDriverMetadata returns the metadata regarding the driver for the layer in the given store
func GetDriverMetadata(store cstorage.Store, layerID string) (map[string]string, error) {
	driver, err := store.GraphDriver()
	if err != nil {
		return nil, err
	}
	return driver.Metadata(layerID)
}

// GetDriverData returns the Data struct with information of the driver used by the store
func GetDriverData(store cstorage.Store, layerID string) (*Data, error) {
	name, err := GetDriverName(store)
	if err != nil {
		return nil, err
	}
	metaData, err := GetDriverMetadata(store, layerID)
	if err != nil {
		return nil, err
	}
	if mountTimes, err := store.Mounted(layerID); mountTimes == 0 || err != nil {
		delete(metaData, "MergedDir")
	}

	return &Data{
		Name: name,
		Data: metaData,
	}, nil
}
