package layers

import cstorage "github.com/containers/storage"

// FullID gets the full id of a layer given a partial id or name
func FullID(store cstorage.Store, id string) (string, error) {
	layer, err := store.Layer(id)
	if err != nil {
		return "", err
	}
	return layer.ID, nil
}
