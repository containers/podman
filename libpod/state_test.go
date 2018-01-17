package libpod

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/containers/storage"
	"github.com/stretchr/testify/assert"
)

// Returns state, tmp directory containing all state files, locks directory
// (subdirectory of tmp dir), and error
// Closing the state and removing the given tmp directory should be sufficient
// to clean up
type emptyStateFunc func() (State, string, string, error)

const (
	tmpDirPrefix = "libpod_state_test_"
)

var (
	testedStates = map[string]emptyStateFunc{
		"sql":       getEmptySQLState,
		"in-memory": getEmptyInMemoryState,
	}
)

// Get an empty in-memory state for use in tests
func getEmptyInMemoryState() (s State, p string, p2 string, err error) {
	tmpDir, err := ioutil.TempDir("", tmpDirPrefix)
	if err != nil {
		return nil, "", "", err
	}
	defer func() {
		if err != nil {
			os.RemoveAll(tmpDir)
		}
	}()

	state, err := NewInMemoryState()
	if err != nil {
		return nil, "", "", err
	}

	// Don't need a separate locks dir as InMemoryState stores nothing on
	// disk
	return state, tmpDir, tmpDir, nil
}

// Get an empty SQL state for use in tests
// An empty Runtime is provided
func getEmptySQLState() (s State, p string, p2 string, err error) {
	tmpDir, err := ioutil.TempDir("", tmpDirPrefix)
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

func runForAllStates(t *testing.T, testName string, testFunc func(*testing.T, State, string)) {
	for stateName, stateFunc := range testedStates {
		state, path, lockPath, err := stateFunc()
		if err != nil {
			t.Fatalf("Error initializing state %s", stateName)
		}
		defer os.RemoveAll(path)
		defer state.Close()

		testName = testName + "-" + stateName

		success := t.Run(testName, func(t *testing.T) {
			testFunc(t, state, lockPath)
		})
		if !success {
			t.Fail()
			t.Logf("%s failed for state %s", testName, stateName)
		}
	}
}

func TestAddAndGetContainer(t *testing.T) {
	runForAllStates(t, "TestAddAndGetContainer", addAndGetContainer)
}

func addAndGetContainer(t *testing.T, state State, lockPath string) {
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
	runForAllStates(t, "TestAddAndGetContainerFromMultiple", addAndGetContainerFromMultiple)
}

func addAndGetContainerFromMultiple(t *testing.T, state State, lockPath string) {
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
	runForAllStates(t, "TestAddInvalidContainerFails", addInvalidContainerFails)
}

func addInvalidContainerFails(t *testing.T, state State, lockPath string) {
	err := state.AddContainer(&Container{config: &ContainerConfig{ID: "1234"}})
	assert.Error(t, err)
}

func TestAddDuplicateCtrIDFails(t *testing.T) {
	runForAllStates(t, "TestAddDuplicateCtrIDFails", addDuplicateCtrIDFails)
}

func addDuplicateCtrIDFails(t *testing.T, state State, lockPath string) {
	testCtr1, err := getTestContainer("11111111111111111111111111111111", "test1", lockPath)
	assert.NoError(t, err)
	testCtr2, err := getTestContainer(testCtr1.ID(), "test2", lockPath)
	assert.NoError(t, err)

	err = state.AddContainer(testCtr1)
	assert.NoError(t, err)

	err = state.AddContainer(testCtr2)
	assert.Error(t, err)
}

func TestAddDuplicateCtrNameFails(t *testing.T) {
	runForAllStates(t, "TestAddDuplicateCtrNameFails", addDuplicateCtrNameFails)
}

func addDuplicateCtrNameFails(t *testing.T, state State, lockPath string) {
	testCtr1, err := getTestContainer("11111111111111111111111111111111", "test1", lockPath)
	assert.NoError(t, err)
	testCtr2, err := getTestContainer("22222222222222222222222222222222", testCtr1.Name(), lockPath)
	assert.NoError(t, err)

	err = state.AddContainer(testCtr1)
	assert.NoError(t, err)

	err = state.AddContainer(testCtr2)
	assert.Error(t, err)
}

func TestGetNonexistentContainerFails(t *testing.T) {
	runForAllStates(t, "TestGetNonexistentContainerFails", getNonexistentContainerFails)
}

func getNonexistentContainerFails(t *testing.T, state State, lockPath string) {
	_, err := state.Container("does not exist")
	assert.Error(t, err)
}

func TestGetContainerWithEmptyIDFails(t *testing.T) {
	runForAllStates(t, "TestGetContainerWithEmptyIDFails", getContainerWithEmptyIDFails)
}

func getContainerWithEmptyIDFails(t *testing.T, state State, lockPath string) {
	_, err := state.Container("")
	assert.Error(t, err)
}

func TestLookupContainerWithEmptyIDFails(t *testing.T) {
	runForAllStates(t, "TestLookupContainerWithEmptyIDFails", lookupContainerWithEmptyIDFails)
}

func lookupContainerWithEmptyIDFails(t *testing.T, state State, lockPath string) {
	_, err := state.LookupContainer("")
	assert.Error(t, err)
}

func TestLookupNonexistentContainerFails(t *testing.T) {
	runForAllStates(t, "TestLookupNonexistantContainerFails", lookupNonexistentContainerFails)
}

func lookupNonexistentContainerFails(t *testing.T, state State, lockPath string) {
	_, err := state.LookupContainer("does not exist")
	assert.Error(t, err)
}

func TestLookupContainerByFullID(t *testing.T) {
	runForAllStates(t, "TestLookupContainerByFullID", lookupContainerByFullID)
}

func lookupContainerByFullID(t *testing.T, state State, lockPath string) {
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
	runForAllStates(t, "TestLookupContainerByUniquePartialID", lookupContainerByUniquePartialID)
}

func lookupContainerByUniquePartialID(t *testing.T, state State, lockPath string) {
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
	runForAllStates(t, "TestLookupContainerByNonUniquePartialIDFails", lookupContainerByNonUniquePartialIDFails)
}

func lookupContainerByNonUniquePartialIDFails(t *testing.T, state State, lockPath string) {
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
	runForAllStates(t, "TestLookupContainerByName", lookupContainerByName)
}

func lookupContainerByName(t *testing.T, state State, lockPath string) {
	state, path, lockPath, err := getEmptySQLState()
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
	runForAllStates(t, "TestHasContainerEmptyIDFails", hasContainerEmptyIDFails)
}

func hasContainerEmptyIDFails(t *testing.T, state State, lockPath string) {
	_, err := state.HasContainer("")
	assert.Error(t, err)
}

func TestHasContainerNoSuchContainerReturnsFalse(t *testing.T) {
	runForAllStates(t, "TestHasContainerNoSuchContainerReturnsFalse", hasContainerNoSuchContainerReturnsFalse)
}

func hasContainerNoSuchContainerReturnsFalse(t *testing.T, state State, lockPath string) {
	exists, err := state.HasContainer("does not exist")
	assert.NoError(t, err)
	assert.False(t, exists)
}

func TestHasContainerFindsContainer(t *testing.T) {
	runForAllStates(t, "TestHasContainerFindsContainer", hasContainerFindsContainer)
}

func hasContainerFindsContainer(t *testing.T, state State, lockPath string) {
	testCtr, err := getTestContainer("0123456789ABCDEF0123456789ABCDEF", "test", lockPath)
	assert.NoError(t, err)

	err = state.AddContainer(testCtr)
	assert.NoError(t, err)

	exists, err := state.HasContainer(testCtr.ID())
	assert.NoError(t, err)
	assert.True(t, exists)
}

func TestSaveAndUpdateContainer(t *testing.T) {
	runForAllStates(t, "TestSaveAndUpdateContainer", saveAndUpdateContainer)
}

func saveAndUpdateContainer(t *testing.T, state State, lockPath string) {
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
	runForAllStates(t, "TestUpdateContainerNotInDatabaseReturnsError", updateContainerNotInDatabaseReturnsError)
}

func updateContainerNotInDatabaseReturnsError(t *testing.T, state State, lockPath string) {
	testCtr, err := getTestContainer("0123456789ABCDEF0123456789ABCDEF", "test", lockPath)
	assert.NoError(t, err)

	err = state.UpdateContainer(testCtr)
	assert.Error(t, err)
	assert.False(t, testCtr.valid)
}

func TestUpdateInvalidContainerReturnsError(t *testing.T) {
	runForAllStates(t, "TestUpdateInvalidContainerReturnsError", updateInvalidContainerReturnsError)
}

func updateInvalidContainerReturnsError(t *testing.T, state State, lockPath string) {
	err := state.UpdateContainer(&Container{config: &ContainerConfig{ID: "1234"}})
	assert.Error(t, err)
}

func TestSaveInvalidContainerReturnsError(t *testing.T) {
	runForAllStates(t, "TestSaveInvalidContainerReturnsError", saveInvalidContainerReturnsError)
}

func saveInvalidContainerReturnsError(t *testing.T, state State, lockPath string) {
	err := state.SaveContainer(&Container{config: &ContainerConfig{ID: "1234"}})
	assert.Error(t, err)
}

func TestSaveContainerNotInStateReturnsError(t *testing.T) {
	runForAllStates(t, "TestSaveContainerNotInStateReturnsError", saveContainerNotInStateReturnsError)
}

func saveContainerNotInStateReturnsError(t *testing.T, state State, lockPath string) {
	testCtr, err := getTestContainer("0123456789ABCDEF0123456789ABCDEF", "test", lockPath)
	assert.NoError(t, err)

	err = state.SaveContainer(testCtr)
	assert.Error(t, err)
	assert.False(t, testCtr.valid)
}

func TestRemoveContainer(t *testing.T) {
	runForAllStates(t, "TestRemoveContainer", removeContainer)
}

func removeContainer(t *testing.T, state State, lockPath string) {
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
	runForAllStates(t, "TestRemoveNonexistantContainerFails", removeNonexistantContainerFails)
}

func removeNonexistantContainerFails(t *testing.T, state State, lockPath string) {
	testCtr, err := getTestContainer("0123456789ABCDEF0123456789ABCDEF", "test", lockPath)
	assert.NoError(t, err)

	err = state.RemoveContainer(testCtr)
	assert.Error(t, err)
}

func TestGetAllContainersOnNewStateIsEmpty(t *testing.T) {
	runForAllStates(t, "TestGetAllContainersOnNewStateIsEmpty", getAllContainersOnNewStateIsEmpty)
}

func getAllContainersOnNewStateIsEmpty(t *testing.T, state State, lockPath string) {
	ctrs, err := state.AllContainers()
	assert.NoError(t, err)
	assert.Equal(t, 0, len(ctrs))
}

func TestGetAllContainersWithOneContainer(t *testing.T) {
	runForAllStates(t, "TestGetAllContainersWithOneContainer", getAllContainersWithOneContainer)
}

func getAllContainersWithOneContainer(t *testing.T, state State, lockPath string) {
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
	runForAllStates(t, "TestGetAllContainersTwoContainers", getAllContainersTwoContainers)
}

func getAllContainersTwoContainers(t *testing.T, state State, lockPath string) {
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
}

func TestContainerInUseInvalidContainer(t *testing.T) {
	runForAllStates(t, "TestContainerInUseInvalidContainer", containerInUseInvalidContainer)
}

func containerInUseInvalidContainer(t *testing.T, state State, lockPath string) {
	_, err := state.ContainerInUse(&Container{})
	assert.Error(t, err)
}

func TestContainerInUseOneContainer(t *testing.T) {
	runForAllStates(t, "TestContainerInUseOneContainer", containerInUseOneContainer)
}

func containerInUseOneContainer(t *testing.T, state State, lockPath string) {
	testCtr1, err := getTestContainer("11111111111111111111111111111111", "test1", lockPath)
	assert.NoError(t, err)
	testCtr2, err := getTestContainer("22222222222222222222222222222222", "test2", lockPath)
	assert.NoError(t, err)

	testCtr2.config.UserNsCtr = testCtr1.config.ID

	err = state.AddContainer(testCtr1)
	assert.NoError(t, err)

	err = state.AddContainer(testCtr2)
	assert.NoError(t, err)

	ids, err := state.ContainerInUse(testCtr1)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(ids))
	assert.Equal(t, testCtr2.config.ID, ids[0])
}

func TestContainerInUseTwoContainers(t *testing.T) {
	runForAllStates(t, "TestContainerInUseTwoContainers", containerInUseTwoContainers)
}

func containerInUseTwoContainers(t *testing.T, state State, lockPath string) {
	testCtr1, err := getTestContainer("11111111111111111111111111111111", "test1", lockPath)
	assert.NoError(t, err)
	testCtr2, err := getTestContainer("22222222222222222222222222222222", "test2", lockPath)
	assert.NoError(t, err)
	testCtr3, err := getTestContainer("33333333333333333333333333333333", "test3", lockPath)
	assert.NoError(t, err)

	testCtr2.config.UserNsCtr = testCtr1.config.ID
	testCtr3.config.IPCNsCtr = testCtr1.config.ID

	err = state.AddContainer(testCtr1)
	assert.NoError(t, err)

	err = state.AddContainer(testCtr2)
	assert.NoError(t, err)

	err = state.AddContainer(testCtr3)
	assert.NoError(t, err)

	ids, err := state.ContainerInUse(testCtr1)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(ids))
}

func TestCannotRemoveContainerWithDependency(t *testing.T) {
	runForAllStates(t, "TestCannotRemoveContainerWithDependency", cannotRemoveContainerWithDependency)
}

func cannotRemoveContainerWithDependency(t *testing.T, state State, lockPath string) {
	testCtr1, err := getTestContainer("11111111111111111111111111111111", "test1", lockPath)
	assert.NoError(t, err)
	testCtr2, err := getTestContainer("22222222222222222222222222222222", "test2", lockPath)
	assert.NoError(t, err)

	testCtr2.config.UserNsCtr = testCtr1.config.ID

	err = state.AddContainer(testCtr1)
	assert.NoError(t, err)

	err = state.AddContainer(testCtr2)
	assert.NoError(t, err)

	err = state.RemoveContainer(testCtr1)
	assert.Error(t, err)
}

func TestCanRemoveContainerAfterDependencyRemoved(t *testing.T) {
	runForAllStates(t, "TestCanRemoveContainerAfterDependencyRemoved", canRemoveContainerAfterDependencyRemoved)
}

func canRemoveContainerAfterDependencyRemoved(t *testing.T, state State, lockPath string) {
	testCtr1, err := getTestContainer("11111111111111111111111111111111", "test1", lockPath)
	assert.NoError(t, err)
	testCtr2, err := getTestContainer("22222222222222222222222222222222", "test2", lockPath)
	assert.NoError(t, err)

	testCtr2.config.UserNsCtr = testCtr1.config.ID

	err = state.AddContainer(testCtr1)
	assert.NoError(t, err)

	err = state.AddContainer(testCtr2)
	assert.NoError(t, err)

	err = state.RemoveContainer(testCtr2)
	assert.NoError(t, err)

	err = state.RemoveContainer(testCtr1)
	assert.NoError(t, err)
}
