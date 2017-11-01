package driver

import cstorage "github.com/containers/storage"

// Data handles the data for a storage driver
type Data struct {
	Name string
	Data map[string]string
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
