package libpod

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/containers/storage"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/stretchr/testify/assert"
)

func getTestContainer(id, name, locksDir string) (*Container, error) {
	ctr := &Container{
		config: &ContainerConfig{
			ID:              id,
			Name:            name,
			RootfsImageID:   id,
			RootfsImageName: "testimg",
			UseImageConfig:  true,
			StaticDir:       "/does/not/exist/",
			Stdin:           true,
			Labels:          make(map[string]string),
			StopSignal:      0,
			StopTimeout:     0,
			CreatedTime:     time.Now(),
		},
		state: &containerRuntimeInfo{
			State:      ContainerStateRunning,
			ConfigPath: "/does/not/exist/specs/" + id,
			RunDir:     "/does/not/exist/tmp/",
			Mounted:    true,
			Mountpoint: "/does/not/exist/tmp/" + id,
			PID:        1234,
		},
		valid: true,
	}

	g := generate.New()
	ctr.config.Spec = g.Spec()

	ctr.config.Labels["test"] = "testing"

	// Must make lockfile or container will error on being retrieved from DB
	lockPath := filepath.Join(locksDir, id)
	lock, err := storage.GetLockfile(lockPath)
	if err != nil {
		return nil, err
	}
	ctr.lock = lock

	return ctr, nil
}

// This horrible hack tests if containers are equal in a way that should handle
// empty arrays being dropped to nil pointers in the spec JSON
func testContainersEqual(a, b *Container) bool {
	if a == nil && b == nil {
		return true
	} else if a == nil || b == nil {
		return false
	}

	if a.valid != b.valid {
		return false
	}

	aConfigJSON, err := json.Marshal(a.config)
	if err != nil {
		return false
	}

	bConfigJSON, err := json.Marshal(b.config)
	if err != nil {
		return false
	}

	if !reflect.DeepEqual(aConfigJSON, bConfigJSON) {
		return false
	}

	aStateJSON, err := json.Marshal(a.state)
	if err != nil {
		return false
	}

	bStateJSON, err := json.Marshal(b.state)
	if err != nil {
		return false
	}

	return reflect.DeepEqual(aStateJSON, bStateJSON)
}

// Get an empty state for use in tests
// An empty Runtime is provided
func getEmptyState() (s State, p string, p2 string, err error) {
	tmpDir, err := ioutil.TempDir("", "libpod_state_test_")
	if err != nil {
		return nil, "", "", err
	}
	defer func() {
		if err != nil {
			os.RemoveAll(tmpDir)
		}
	}()

	dbPath := filepath.Join(tmpDir, "db.sql")
	specsDir := filepath.Join(tmpDir, "specs")
	lockDir := filepath.Join(tmpDir, "locks")

	runtime := new(Runtime)
	runtime.config = new(RuntimeConfig)
	runtime.config.StorageConfig = storage.StoreOptions{}

	state, err := NewSQLState(dbPath, specsDir, lockDir, runtime)
	if err != nil {
		return nil, "", "", err
	}

	return state, tmpDir, lockDir, nil
}

func TestAddAndGetContainer(t *testing.T) {
	state, path, lockPath, err := getEmptyState()
	assert.NoError(t, err)
	defer os.RemoveAll(path)
	defer state.Close()

	testCtr, err := getTestContainer("0123456789ABCDEF0123456789ABCDEF", "test", lockPath)
	assert.NoError(t, err)

	err = state.AddContainer(testCtr)
	assert.NoError(t, err)

	retrievedCtr, err := state.Container(testCtr.ID())
	assert.NoError(t, err)

	// Use assert.EqualValues if the test fails to pretty print diff
	// between actual and expected
	if !testContainersEqual(testCtr, retrievedCtr) {
		assert.EqualValues(t, testCtr, retrievedCtr)
	}
}

func TestAddAndGetContainerFromMultiple(t *testing.T) {
	state, path, lockPath, err := getEmptyState()
	assert.NoError(t, err)
	defer os.RemoveAll(path)
	defer state.Close()

	testCtr1, err := getTestContainer("11111111111111111111111111111111", "test1", lockPath)
	assert.NoError(t, err)
	testCtr2, err := getTestContainer("22222222222222222222222222222222", "test2", lockPath)
	assert.NoError(t, err)

	err = state.AddContainer(testCtr1)
	assert.NoError(t, err)

	err = state.AddContainer(testCtr2)
	assert.NoError(t, err)

	retrievedCtr, err := state.Container(testCtr1.ID())
	assert.NoError(t, err)

	// Use assert.EqualValues if the test fails to pretty print diff
	// between actual and expected
	if !testContainersEqual(testCtr1, retrievedCtr) {
		assert.EqualValues(t, testCtr1, retrievedCtr)
	}
}

func TestAddInvalidContainerFails(t *testing.T) {
	state, path, _, err := getEmptyState()
	assert.NoError(t, err)
	defer os.RemoveAll(path)
	defer state.Close()

	err = state.AddContainer(&Container{})
	assert.Error(t, err)
}

func TestAddDuplicateIDFails(t *testing.T) {
	state, path, lockPath, err := getEmptyState()
	assert.NoError(t, err)
	defer os.RemoveAll(path)
	defer state.Close()

	testCtr1, err := getTestContainer("11111111111111111111111111111111", "test1", lockPath)
	assert.NoError(t, err)
	testCtr2, err := getTestContainer(testCtr1.ID(), "test2", lockPath)
	assert.NoError(t, err)

	err = state.AddContainer(testCtr1)
	assert.NoError(t, err)

	err = state.AddContainer(testCtr2)
	assert.Error(t, err)
}

func TestAddDuplicateNameFails(t *testing.T) {
	state, path, lockPath, err := getEmptyState()
	assert.NoError(t, err)
	defer os.RemoveAll(path)
	defer state.Close()

	testCtr1, err := getTestContainer("11111111111111111111111111111111", "test1", lockPath)
	assert.NoError(t, err)
	testCtr2, err := getTestContainer("22222222222222222222222222222222", testCtr1.Name(), lockPath)
	assert.NoError(t, err)

	err = state.AddContainer(testCtr1)
	assert.NoError(t, err)

	err = state.AddContainer(testCtr2)
	assert.Error(t, err)
}

func TestGetNonexistantContainerFails(t *testing.T) {
	state, path, _, err := getEmptyState()
	assert.NoError(t, err)
	defer os.RemoveAll(path)
	defer state.Close()

	_, err = state.Container("does not exist")
	assert.Error(t, err)
}

func TestGetContainerWithEmptyIDFails(t *testing.T) {
	state, path, _, err := getEmptyState()
	assert.NoError(t, err)
	defer os.RemoveAll(path)
	defer state.Close()

	_, err = state.Container("")
	assert.Error(t, err)
}

func TestLookupContainerWithEmptyIDFails(t *testing.T) {
	state, path, _, err := getEmptyState()
	assert.NoError(t, err)
	defer os.RemoveAll(path)
	defer state.Close()

	_, err = state.LookupContainer("")
	assert.Error(t, err)
}

func TestLookupNonexistantContainerFails(t *testing.T) {
	state, path, _, err := getEmptyState()
	assert.NoError(t, err)
	defer os.RemoveAll(path)

	_, err = state.LookupContainer("does not exist")
	assert.Error(t, err)
}

func TestLookupContainerByFullID(t *testing.T) {
	state, path, lockPath, err := getEmptyState()
	assert.NoError(t, err)
	defer os.RemoveAll(path)
	defer state.Close()

	testCtr, err := getTestContainer("0123456789ABCDEF0123456789ABCDEF", "test", lockPath)
	assert.NoError(t, err)

	err = state.AddContainer(testCtr)
	assert.NoError(t, err)

	retrievedCtr, err := state.LookupContainer(testCtr.ID())
	assert.NoError(t, err)

	// Use assert.EqualValues if the test fails to pretty print diff
	// between actual and expected
	if !testContainersEqual(testCtr, retrievedCtr) {
		assert.EqualValues(t, testCtr, retrievedCtr)
	}
}

func TestLookupContainerByUniquePartialID(t *testing.T) {
	state, path, lockPath, err := getEmptyState()
	assert.NoError(t, err)
	defer os.RemoveAll(path)
	defer state.Close()

	testCtr, err := getTestContainer("0123456789ABCDEF0123456789ABCDEF", "test", lockPath)
	assert.NoError(t, err)

	err = state.AddContainer(testCtr)
	assert.NoError(t, err)

	retrievedCtr, err := state.LookupContainer(testCtr.ID()[0:8])
	assert.NoError(t, err)

	// Use assert.EqualValues if the test fails to pretty print diff
	// between actual and expected
	if !testContainersEqual(testCtr, retrievedCtr) {
		assert.EqualValues(t, testCtr, retrievedCtr)
	}
}

func TestLookupContainerByNonUniquePartialIDFails(t *testing.T) {
	state, path, lockPath, err := getEmptyState()
	assert.NoError(t, err)
	defer os.RemoveAll(path)
	defer state.Close()

	testCtr1, err := getTestContainer("00000000000000000000000000000000", "test1", lockPath)
	assert.NoError(t, err)
	testCtr2, err := getTestContainer("00000000000000000000000000000001", "test2", lockPath)
	assert.NoError(t, err)

	err = state.AddContainer(testCtr1)
	assert.NoError(t, err)

	err = state.AddContainer(testCtr2)
	assert.NoError(t, err)

	_, err = state.LookupContainer(testCtr1.ID()[0:8])
	assert.Error(t, err)
}

func TestLookupContainerByName(t *testing.T) {
	state, path, lockPath, err := getEmptyState()
	assert.NoError(t, err)
	defer os.RemoveAll(path)
	defer state.Close()

	testCtr, err := getTestContainer("0123456789ABCDEF0123456789ABCDEF", "test", lockPath)
	assert.NoError(t, err)

	err = state.AddContainer(testCtr)
	assert.NoError(t, err)

	retrievedCtr, err := state.LookupContainer(testCtr.Name())
	assert.NoError(t, err)

	// Use assert.EqualValues if the test fails to pretty print diff
	// between actual and expected
	if !testContainersEqual(testCtr, retrievedCtr) {
		assert.EqualValues(t, testCtr, retrievedCtr)
	}
}

func TestHasContainerEmptyIDFails(t *testing.T) {
	state, path, _, err := getEmptyState()
	assert.NoError(t, err)
	defer os.RemoveAll(path)
	defer state.Close()

	_, err = state.HasContainer("")
	assert.Error(t, err)
}

func TestHasContainerNoSuchContainerReturnsFalse(t *testing.T) {
	state, path, _, err := getEmptyState()
	assert.NoError(t, err)
	defer os.RemoveAll(path)
	defer state.Close()

	exists, err := state.HasContainer("does not exist")
	assert.NoError(t, err)
	assert.False(t, exists)
}

func TestHasContainerFindsContainer(t *testing.T) {
	state, path, lockPath, err := getEmptyState()
	assert.NoError(t, err)
	defer os.RemoveAll(path)
	defer state.Close()

	testCtr, err := getTestContainer("0123456789ABCDEF0123456789ABCDEF", "test", lockPath)
	assert.NoError(t, err)

	err = state.AddContainer(testCtr)
	assert.NoError(t, err)

	exists, err := state.HasContainer(testCtr.ID())
	assert.NoError(t, err)
	assert.True(t, exists)
}

func TestSaveAndUpdateContainer(t *testing.T) {
	state, path, lockPath, err := getEmptyState()
	assert.NoError(t, err)
	defer os.RemoveAll(path)
	defer state.Close()

	testCtr, err := getTestContainer("0123456789ABCDEF0123456789ABCDEF", "test", lockPath)
	assert.NoError(t, err)

	err = state.AddContainer(testCtr)
	assert.NoError(t, err)

	retrievedCtr, err := state.Container(testCtr.ID())
	assert.NoError(t, err)

	retrievedCtr.state.State = ContainerStateStopped
	retrievedCtr.state.ExitCode = 127
	retrievedCtr.state.FinishedTime = time.Now()

	err = state.SaveContainer(retrievedCtr)
	assert.NoError(t, err)

	err = state.UpdateContainer(testCtr)
	assert.NoError(t, err)

	// Use assert.EqualValues if the test fails to pretty print diff
	// between actual and expected
	if !testContainersEqual(testCtr, retrievedCtr) {
		assert.EqualValues(t, testCtr, retrievedCtr)
	}
}

func TestUpdateContainerNotInDatabaseReturnsError(t *testing.T) {
	state, path, lockPath, err := getEmptyState()
	assert.NoError(t, err)
	defer os.RemoveAll(path)
	defer state.Close()

	testCtr, err := getTestContainer("0123456789ABCDEF0123456789ABCDEF", "test", lockPath)
	assert.NoError(t, err)

	err = state.UpdateContainer(testCtr)
	assert.Error(t, err)
	assert.False(t, testCtr.valid)
}

func TestUpdateInvalidContainerReturnsError(t *testing.T) {
	state, path, _, err := getEmptyState()
	assert.NoError(t, err)
	defer os.RemoveAll(path)
	defer state.Close()

	err = state.UpdateContainer(&Container{})
	assert.Error(t, err)
}

func TestSaveInvalidContainerReturnsError(t *testing.T) {
	state, path, _, err := getEmptyState()
	assert.NoError(t, err)
	defer os.RemoveAll(path)
	defer state.Close()

	err = state.SaveContainer(&Container{})
	assert.Error(t, err)
}

func TestSaveContainerNotInStateReturnsError(t *testing.T) {
	state, path, lockPath, err := getEmptyState()
	assert.NoError(t, err)
	defer os.RemoveAll(path)
	defer state.Close()

	testCtr, err := getTestContainer("0123456789ABCDEF0123456789ABCDEF", "test", lockPath)
	assert.NoError(t, err)

	err = state.SaveContainer(testCtr)
	assert.Error(t, err)
}

func TestRemoveContainer(t *testing.T) {
	state, path, lockPath, err := getEmptyState()
	assert.NoError(t, err)
	defer os.RemoveAll(path)
	defer state.Close()

	testCtr, err := getTestContainer("0123456789ABCDEF0123456789ABCDEF", "test", lockPath)
	assert.NoError(t, err)

	err = state.AddContainer(testCtr)
	assert.NoError(t, err)

	ctrs, err := state.AllContainers()
	assert.NoError(t, err)
	assert.Equal(t, 1, len(ctrs))

	err = state.RemoveContainer(testCtr)
	assert.NoError(t, err)

	ctrs2, err := state.AllContainers()
	assert.NoError(t, err)
	assert.Equal(t, 0, len(ctrs2))
}

func TestRemoveNonexistantContainerFails(t *testing.T) {
	state, path, lockPath, err := getEmptyState()
	assert.NoError(t, err)
	defer os.RemoveAll(path)
	defer state.Close()

	testCtr, err := getTestContainer("0123456789ABCDEF0123456789ABCDEF", "test", lockPath)
	assert.NoError(t, err)

	err = state.RemoveContainer(testCtr)
	assert.Error(t, err)
}

func TestGetAllContainersOnNewStateIsEmpty(t *testing.T) {
	state, path, _, err := getEmptyState()
	assert.NoError(t, err)
	defer os.RemoveAll(path)
	defer state.Close()

	ctrs, err := state.AllContainers()
	assert.NoError(t, err)
	assert.Equal(t, 0, len(ctrs))
}

func TestGetAllContainersWithOneContainer(t *testing.T) {
	state, path, lockPath, err := getEmptyState()
	assert.NoError(t, err)
	defer os.RemoveAll(path)
	defer state.Close()

	testCtr, err := getTestContainer("0123456789ABCDEF0123456789ABCDEF", "test", lockPath)
	assert.NoError(t, err)

	err = state.AddContainer(testCtr)
	assert.NoError(t, err)

	ctrs, err := state.AllContainers()
	assert.NoError(t, err)
	assert.Equal(t, 1, len(ctrs))

	// Use assert.EqualValues if the test fails to pretty print diff
	// between actual and expected
	if !testContainersEqual(testCtr, ctrs[0]) {
		assert.EqualValues(t, testCtr, ctrs[0])
	}
}

func TestGetAllContainersTwoContainers(t *testing.T) {
	state, path, lockPath, err := getEmptyState()
	assert.NoError(t, err)
	defer os.RemoveAll(path)
	defer state.Close()

	testCtr1, err := getTestContainer("11111111111111111111111111111111", "test1", lockPath)
	assert.NoError(t, err)
	testCtr2, err := getTestContainer("22222222222222222222222222222222", "test2", lockPath)
	assert.NoError(t, err)

	err = state.AddContainer(testCtr1)
	assert.NoError(t, err)

	err = state.AddContainer(testCtr2)
	assert.NoError(t, err)

	ctrs, err := state.AllContainers()
	assert.NoError(t, err)
	assert.Equal(t, 2, len(ctrs))

	// Containers should be ordered by creation time

	// Use assert.EqualValues if the test fails to pretty print diff
	// between actual and expected
	if !testContainersEqual(testCtr2, ctrs[0]) {
		assert.EqualValues(t, testCtr2, ctrs[0])
	}
	if !testContainersEqual(testCtr1, ctrs[1]) {
		assert.EqualValues(t, testCtr1, ctrs[1])
	}
}
