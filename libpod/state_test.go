package libpod

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
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
		"boltdb":    getEmptyBoltState,
	}
)

// Get an empty BoltDB state for use in tests
func getEmptyBoltState() (s State, p string, p2 string, err error) {
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
	lockDir := filepath.Join(tmpDir, "locks")

	runtime := new(Runtime)
	runtime.config = new(RuntimeConfig)
	runtime.config.StorageConfig = storage.StoreOptions{}

	state, err := NewBoltState(dbPath, lockDir, runtime)
	if err != nil {
		return nil, "", "", err
	}

	return state, tmpDir, lockDir, nil
}

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

func runForAllStates(t *testing.T, testFunc func(*testing.T, State, string)) {
	for stateName, stateFunc := range testedStates {
		state, path, lockPath, err := stateFunc()
		if err != nil {
			t.Fatalf("Error initializing state %s: %v", stateName, err)
		}
		defer os.RemoveAll(path)
		defer state.Close()

		success := t.Run(stateName, func(t *testing.T) {
			testFunc(t, state, lockPath)
		})
		if !success {
			t.Fail()
		}
	}
}

func getTestCtrN(n, lockPath string) (*Container, error) {
	return getTestContainer(strings.Repeat(n, 32), "test"+n, lockPath)
}

func getTestCtr1(lockPath string) (*Container, error) {
	return getTestCtrN("1", lockPath)
}

func getTestCtr2(lockPath string) (*Container, error) {
	return getTestCtrN("2", lockPath)
}

func getTestPodN(n, lockPath string) (*Pod, error) {
	return getTestPod(strings.Repeat(n, 32), "test"+n, lockPath)
}

func getTestPod1(lockPath string) (*Pod, error) {
	return getTestPodN("1", lockPath)
}

func getTestPod2(lockPath string) (*Pod, error) {
	return getTestPodN("2", lockPath)
}

func TestAddAndGetContainer(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testCtr, err := getTestCtr1(lockPath)
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
	})
}

func TestAddAndGetContainerFromMultiple(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testCtr1, err := getTestCtr1(lockPath)
		assert.NoError(t, err)
		testCtr2, err := getTestCtr2(lockPath)
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
	})
}

func TestGetContainerPodSameIDFails(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod, err := getTestPod1(lockPath)
		assert.NoError(t, err)

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		_, err = state.Container(testPod.ID())
		assert.Error(t, err)
	})
}

func TestAddInvalidContainerFails(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		err := state.AddContainer(&Container{config: &ContainerConfig{ID: "1234"}})
		assert.Error(t, err)
	})
}

func TestAddDuplicateCtrIDFails(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testCtr1, err := getTestCtr1(lockPath)
		assert.NoError(t, err)
		testCtr2, err := getTestContainer(testCtr1.ID(), "test2", lockPath)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr1)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr2)
		assert.Error(t, err)

		ctrs, err := state.AllContainers()
		assert.NoError(t, err)
		assert.Equal(t, 1, len(ctrs))
	})
}

func TestAddDuplicateCtrNameFails(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testCtr1, err := getTestCtr1(lockPath)
		assert.NoError(t, err)
		testCtr2, err := getTestContainer(strings.Repeat("2", 32), testCtr1.Name(), lockPath)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr1)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr2)
		assert.Error(t, err)

		ctrs, err := state.AllContainers()
		assert.NoError(t, err)
		assert.Equal(t, 1, len(ctrs))
	})
}

func TestAddCtrPodDupIDFails(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod, err := getTestPod1(lockPath)
		assert.NoError(t, err)
		testCtr, err := getTestContainer(testPod.ID(), "testCtr", lockPath)
		assert.NoError(t, err)

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr)
		assert.Error(t, err)

		ctrs, err := state.AllContainers()
		assert.NoError(t, err)
		assert.Equal(t, 0, len(ctrs))
	})
}

func TestAddCtrPodDupNameFails(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod, err := getTestPod1(lockPath)
		assert.NoError(t, err)
		testCtr, err := getTestContainer(strings.Repeat("2", 32), testPod.Name(), lockPath)
		assert.NoError(t, err)

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr)
		assert.Error(t, err)

		ctrs, err := state.AllContainers()
		assert.NoError(t, err)
		assert.Equal(t, 0, len(ctrs))
	})
}

func TestAddCtrInPodFails(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod, err := getTestPod1(lockPath)
		assert.NoError(t, err)

		testCtr, err := getTestCtr2(lockPath)
		assert.NoError(t, err)

		testCtr.config.Pod = testPod.ID()

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr)
		assert.Error(t, err)

		ctrs, err := state.AllContainers()
		assert.NoError(t, err)
		assert.Equal(t, 0, len(ctrs))
	})
}

func TestGetNonexistentContainerFails(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		_, err := state.Container("does not exist")
		assert.Error(t, err)
	})
}

func TestGetContainerWithEmptyIDFails(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		_, err := state.Container("")
		assert.Error(t, err)
	})
}

func TestLookupContainerWithEmptyIDFails(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		_, err := state.LookupContainer("")
		assert.Error(t, err)
	})
}

func TestLookupNonexistentContainerFails(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		_, err := state.LookupContainer("does not exist")
		assert.Error(t, err)
	})
}

func TestLookupContainerByFullID(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testCtr, err := getTestCtr1(lockPath)
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
	})
}

func TestLookupContainerByUniquePartialID(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testCtr, err := getTestCtr1(lockPath)
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
	})
}

func TestLookupContainerByNonUniquePartialIDFails(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testCtr1, err := getTestContainer(strings.Repeat("0", 32), "test1", lockPath)
		assert.NoError(t, err)
		testCtr2, err := getTestContainer(strings.Repeat("0", 31)+"1", "test2", lockPath)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr1)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr2)
		assert.NoError(t, err)

		_, err = state.LookupContainer(testCtr1.ID()[0:8])
		assert.Error(t, err)
	})
}

func TestLookupContainerByName(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testCtr, err := getTestCtr1(lockPath)
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
	})
}

func TestLookupCtrByPodNameFails(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod, err := getTestPod1(lockPath)
		assert.NoError(t, err)

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		_, err = state.LookupContainer(testPod.Name())
		assert.Error(t, err)
	})
}

func TestLookupCtrByPodIDFails(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod, err := getTestPod1(lockPath)
		assert.NoError(t, err)

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		_, err = state.LookupContainer(testPod.ID())
		assert.Error(t, err)
	})
}

func TestHasContainerEmptyIDFails(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		_, err := state.HasContainer("")
		assert.Error(t, err)
	})
}

func TestHasContainerNoSuchContainerReturnsFalse(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		exists, err := state.HasContainer("does not exist")
		assert.NoError(t, err)
		assert.False(t, exists)
	})
}

func TestHasContainerFindsContainer(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testCtr, err := getTestCtr1(lockPath)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr)
		assert.NoError(t, err)

		exists, err := state.HasContainer(testCtr.ID())
		assert.NoError(t, err)
		assert.True(t, exists)
	})
}

func TestHasContainerPodIDIsFalse(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod, err := getTestPod1(lockPath)
		assert.NoError(t, err)

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		exists, err := state.HasContainer(testPod.ID())
		assert.NoError(t, err)
		assert.False(t, exists)
	})
}

func TestSaveAndUpdateContainer(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testCtr, err := getTestCtr1(lockPath)
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
	})
}

func TestUpdateContainerNotInDatabaseReturnsError(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testCtr, err := getTestCtr1(lockPath)
		assert.NoError(t, err)

		err = state.UpdateContainer(testCtr)
		assert.Error(t, err)
		assert.False(t, testCtr.valid)
	})
}

func TestUpdateInvalidContainerReturnsError(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		err := state.UpdateContainer(&Container{config: &ContainerConfig{ID: "1234"}})
		assert.Error(t, err)
	})
}

func TestSaveInvalidContainerReturnsError(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		err := state.SaveContainer(&Container{config: &ContainerConfig{ID: "1234"}})
		assert.Error(t, err)
	})
}

func TestSaveContainerNotInStateReturnsError(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testCtr, err := getTestCtr1(lockPath)
		assert.NoError(t, err)

		err = state.SaveContainer(testCtr)
		assert.Error(t, err)
		assert.False(t, testCtr.valid)
	})
}

func TestRemoveContainer(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testCtr, err := getTestCtr1(lockPath)
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
	})
}

func TestRemoveNonexistantContainerFails(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testCtr, err := getTestCtr1(lockPath)
		assert.NoError(t, err)

		err = state.RemoveContainer(testCtr)
		assert.Error(t, err)
		assert.False(t, testCtr.valid)
	})
}

func TestGetAllContainersOnNewStateIsEmpty(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		ctrs, err := state.AllContainers()
		assert.NoError(t, err)
		assert.Equal(t, 0, len(ctrs))
	})
}

func TestGetAllContainersWithOneContainer(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testCtr, err := getTestCtr1(lockPath)
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
	})
}

func TestGetAllContainersTwoContainers(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testCtr1, err := getTestCtr1(lockPath)
		assert.NoError(t, err)
		testCtr2, err := getTestCtr2(lockPath)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr1)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr2)
		assert.NoError(t, err)

		ctrs, err := state.AllContainers()
		assert.NoError(t, err)
		assert.Equal(t, 2, len(ctrs))
	})
}

func TestContainerInUseInvalidContainer(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		_, err := state.ContainerInUse(&Container{})
		assert.Error(t, err)
	})
}

func TestContainerInUseOneContainer(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testCtr1, err := getTestCtr1(lockPath)
		assert.NoError(t, err)
		testCtr2, err := getTestCtr2(lockPath)
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
	})
}

func TestContainerInUseTwoContainers(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testCtr1, err := getTestCtr1(lockPath)
		assert.NoError(t, err)
		testCtr2, err := getTestCtr2(lockPath)
		assert.NoError(t, err)
		testCtr3, err := getTestCtrN("3", lockPath)
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
	})
}

func TestContainerInUseOneContainerMultipleDependencies(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testCtr1, err := getTestCtr1(lockPath)
		assert.NoError(t, err)
		testCtr2, err := getTestCtr2(lockPath)
		assert.NoError(t, err)

		testCtr2.config.UserNsCtr = testCtr1.config.ID
		testCtr2.config.IPCNsCtr = testCtr1.config.ID

		err = state.AddContainer(testCtr1)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr2)
		assert.NoError(t, err)

		ids, err := state.ContainerInUse(testCtr1)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(ids))
		assert.Equal(t, testCtr2.config.ID, ids[0])
	})
}

func TestCannotRemoveContainerWithDependency(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testCtr1, err := getTestCtr1(lockPath)
		assert.NoError(t, err)
		testCtr2, err := getTestCtr2(lockPath)
		assert.NoError(t, err)

		testCtr2.config.UserNsCtr = testCtr1.config.ID

		err = state.AddContainer(testCtr1)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr2)
		assert.NoError(t, err)

		err = state.RemoveContainer(testCtr1)
		assert.Error(t, err)

		ctrs, err := state.AllContainers()
		assert.NoError(t, err)
		assert.Equal(t, 2, len(ctrs))
	})
}

func TestCanRemoveContainerAfterDependencyRemoved(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testCtr1, err := getTestCtr1(lockPath)
		assert.NoError(t, err)
		testCtr2, err := getTestCtr2(lockPath)
		assert.NoError(t, err)

		testCtr2.config.UserNsCtr = testCtr1.ID()

		err = state.AddContainer(testCtr1)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr2)
		assert.NoError(t, err)

		err = state.RemoveContainer(testCtr2)
		assert.NoError(t, err)

		err = state.RemoveContainer(testCtr1)
		assert.NoError(t, err)

		ctrs, err := state.AllContainers()
		assert.NoError(t, err)
		assert.Equal(t, 0, len(ctrs))
	})
}

func TestCanRemoveContainerAfterDependencyRemovedDuplicate(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testCtr1, err := getTestCtr1(lockPath)
		assert.NoError(t, err)
		testCtr2, err := getTestCtr2(lockPath)
		assert.NoError(t, err)

		testCtr2.config.UserNsCtr = testCtr1.ID()
		testCtr2.config.IPCNsCtr = testCtr1.ID()

		err = state.AddContainer(testCtr1)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr2)
		assert.NoError(t, err)

		err = state.RemoveContainer(testCtr2)
		assert.NoError(t, err)

		err = state.RemoveContainer(testCtr1)
		assert.NoError(t, err)

		ctrs, err := state.AllContainers()
		assert.NoError(t, err)
		assert.Equal(t, 0, len(ctrs))
	})
}

func TestCannotUsePodAsDependency(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testCtr, err := getTestCtr1(lockPath)
		assert.NoError(t, err)

		testPod, err := getTestPod2(lockPath)
		assert.NoError(t, err)

		testCtr.config.UserNsCtr = testPod.ID()

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr)
		assert.Error(t, err)

		ctrs, err := state.AllContainers()
		assert.NoError(t, err)
		assert.Equal(t, 0, len(ctrs))
	})
}

func TestCannotUseBadIDAsDependency(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testCtr, err := getTestCtr1(lockPath)
		assert.NoError(t, err)

		testCtr.config.UserNsCtr = strings.Repeat("5", 32)

		err = state.AddContainer(testCtr)
		assert.Error(t, err)

		ctrs, err := state.AllContainers()
		assert.NoError(t, err)
		assert.Equal(t, 0, len(ctrs))
	})
}

func TestGetPodDoesNotExist(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		_, err := state.Pod("doesnotexist")
		assert.Error(t, err)
	})
}

func TestGetPodEmptyID(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		_, err := state.Pod("")
		assert.Error(t, err)
	})
}

func TestGetPodOnePod(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod, err := getTestPod1(lockPath)
		assert.NoError(t, err)

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		statePod, err := state.Pod(testPod.ID())
		assert.NoError(t, err)

		// Use assert.EqualValues if the test fails to pretty print diff
		// between actual and expected
		if !testPodsEqual(testPod, statePod) {
			assert.EqualValues(t, testPod, statePod)
		}
	})
}

func TestGetOnePodFromTwo(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod1, err := getTestPod1(lockPath)
		assert.NoError(t, err)

		testPod2, err := getTestPod2(lockPath)
		assert.NoError(t, err)

		err = state.AddPod(testPod1)
		assert.NoError(t, err)

		err = state.AddPod(testPod2)
		assert.NoError(t, err)

		statePod, err := state.Pod(testPod1.ID())
		assert.NoError(t, err)

		// Use assert.EqualValues if the test fails to pretty print diff
		// between actual and expected
		if !testPodsEqual(testPod1, statePod) {
			assert.EqualValues(t, testPod1, statePod)
		}
	})
}

func TestGetNotExistPodWithPods(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod1, err := getTestPod1(lockPath)
		assert.NoError(t, err)

		testPod2, err := getTestPod2(lockPath)
		assert.NoError(t, err)

		err = state.AddPod(testPod1)
		assert.NoError(t, err)

		err = state.AddPod(testPod2)
		assert.NoError(t, err)

		_, err = state.Pod("notexist")
		assert.Error(t, err)
	})
}

func TestGetPodByCtrID(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testCtr, err := getTestCtr1(lockPath)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr)
		assert.NoError(t, err)

		_, err = state.Pod(testCtr.ID())
		assert.Error(t, err)
	})
}

func TestLookupPodEmptyID(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		_, err := state.LookupPod("")
		assert.Error(t, err)
	})
}

func TestLookupNotExistPod(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		_, err := state.LookupPod("doesnotexist")
		assert.Error(t, err)
	})
}

func TestLookupPodFullID(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod, err := getTestPod1(lockPath)
		assert.NoError(t, err)

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		statePod, err := state.LookupPod(testPod.ID())
		assert.NoError(t, err)

		// Use assert.EqualValues if the test fails to pretty print diff
		// between actual and expected
		if !testPodsEqual(testPod, statePod) {
			assert.EqualValues(t, testPod, statePod)
		}
	})
}

func TestLookupPodUniquePartialID(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod, err := getTestPod1(lockPath)
		assert.NoError(t, err)

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		statePod, err := state.LookupPod(testPod.ID()[0:8])
		assert.NoError(t, err)

		// Use assert.EqualValues if the test fails to pretty print diff
		// between actual and expected
		if !testPodsEqual(testPod, statePod) {
			assert.EqualValues(t, testPod, statePod)
		}
	})
}

func TestLookupPodNonUniquePartialID(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod1, err := getTestPod(strings.Repeat("1", 32), "test1", lockPath)
		assert.NoError(t, err)

		testPod2, err := getTestPod(strings.Repeat("1", 31)+"2", "test2", lockPath)
		assert.NoError(t, err)

		err = state.AddPod(testPod1)
		assert.NoError(t, err)

		err = state.AddPod(testPod2)
		assert.NoError(t, err)

		_, err = state.LookupPod(testPod1.ID()[0:8])
		assert.Error(t, err)
	})
}

func TestLookupPodByName(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod, err := getTestPod1(lockPath)
		assert.NoError(t, err)

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		statePod, err := state.LookupPod(testPod.Name())
		assert.NoError(t, err)

		// Use assert.EqualValues if the test fails to pretty print diff
		// between actual and expected
		if !testPodsEqual(testPod, statePod) {
			assert.EqualValues(t, testPod, statePod)
		}
	})
}

func TestLookupPodByCtrID(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testCtr, err := getTestCtr1(lockPath)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr)
		assert.NoError(t, err)

		_, err = state.LookupPod(testCtr.ID())
		assert.Error(t, err)
	})
}

func TestLookupPodByCtrName(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testCtr, err := getTestCtr1(lockPath)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr)
		assert.NoError(t, err)

		_, err = state.LookupPod(testCtr.Name())
		assert.Error(t, err)
	})
}

func TestHasPodEmptyIDErrors(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		_, err := state.HasPod("")
		assert.Error(t, err)
	})
}

func TestHasPodNoSuchPod(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		exist, err := state.HasPod("notexist")
		assert.NoError(t, err)
		assert.False(t, exist)
	})
}

func TestHasPodWrongIDFalse(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod, err := getTestPod1(lockPath)
		assert.NoError(t, err)

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		exist, err := state.HasPod(strings.Repeat("a", 32))
		assert.NoError(t, err)
		assert.False(t, exist)
	})
}

func TestHasPodRightIDTrue(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod, err := getTestPod1(lockPath)
		assert.NoError(t, err)

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		exist, err := state.HasPod(testPod.ID())
		assert.NoError(t, err)
		assert.True(t, exist)
	})
}

func TestHasPodCtrIDFalse(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testCtr, err := getTestCtr1(lockPath)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr)
		assert.NoError(t, err)

		exist, err := state.HasPod(testCtr.ID())
		assert.NoError(t, err)
		assert.False(t, exist)
	})
}

func TestAddPodInvalidPodErrors(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		err := state.AddPod(&Pod{})
		assert.Error(t, err)
	})
}

func TestAddPodValidPodSucceeds(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod, err := getTestPod1(lockPath)
		assert.NoError(t, err)

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		allPods, err := state.AllPods()
		assert.NoError(t, err)
		assert.Equal(t, 1, len(allPods))

		// Use assert.EqualValues if the test fails to pretty print diff
		// between actual and expected
		if !testPodsEqual(testPod, allPods[0]) {
			assert.EqualValues(t, testPod, allPods[0])
		}
	})
}

func TestAddPodDuplicateIDFails(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod1, err := getTestPod1(lockPath)
		assert.NoError(t, err)

		testPod2, err := getTestPod(testPod1.ID(), "testpod2", lockPath)
		assert.NoError(t, err)

		err = state.AddPod(testPod1)
		assert.NoError(t, err)

		err = state.AddPod(testPod2)
		assert.Error(t, err)

		allPods, err := state.AllPods()
		assert.NoError(t, err)
		assert.Equal(t, 1, len(allPods))
	})
}

func TestAddPodDuplicateNameFails(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod1, err := getTestPod1(lockPath)
		assert.NoError(t, err)

		testPod2, err := getTestPod(strings.Repeat("2", 32), testPod1.Name(), lockPath)
		assert.NoError(t, err)

		err = state.AddPod(testPod1)
		assert.NoError(t, err)

		err = state.AddPod(testPod2)
		assert.Error(t, err)

		allPods, err := state.AllPods()
		assert.NoError(t, err)
		assert.Equal(t, 1, len(allPods))
	})
}

func TestAddPodNonDuplicateSucceeds(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod1, err := getTestPod1(lockPath)
		assert.NoError(t, err)

		testPod2, err := getTestPod2(lockPath)
		assert.NoError(t, err)

		err = state.AddPod(testPod1)
		assert.NoError(t, err)

		err = state.AddPod(testPod2)
		assert.NoError(t, err)

		allPods, err := state.AllPods()
		assert.NoError(t, err)
		assert.Equal(t, 2, len(allPods))
	})
}

func TestAddPodCtrIDConflictFails(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testCtr, err := getTestCtr1(lockPath)
		assert.NoError(t, err)

		testPod, err := getTestPod(testCtr.ID(), "testpod1", lockPath)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr)
		assert.NoError(t, err)

		err = state.AddPod(testPod)
		assert.Error(t, err)

		allPods, err := state.AllPods()
		assert.NoError(t, err)
		assert.Equal(t, 0, len(allPods))
	})
}

func TestAddPodCtrNameConflictFails(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testCtr, err := getTestCtr1(lockPath)
		assert.NoError(t, err)

		testPod, err := getTestPod(strings.Repeat("3", 32), testCtr.Name(), lockPath)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr)
		assert.NoError(t, err)

		err = state.AddPod(testPod)
		assert.Error(t, err)

		allPods, err := state.AllPods()
		assert.NoError(t, err)
		assert.Equal(t, 0, len(allPods))
	})
}

func TestRemovePodInvalidPodErrors(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		err := state.RemovePod(&Pod{})
		assert.Error(t, err)
	})
}

func TestRemovePodNotInStateFails(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod, err := getTestPod1(lockPath)
		assert.NoError(t, err)

		err = state.RemovePod(testPod)
		assert.Error(t, err)
		assert.False(t, testPod.valid)
	})
}

func TestRemovePodSucceeds(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod, err := getTestPod1(lockPath)
		assert.NoError(t, err)

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		err = state.RemovePod(testPod)
		assert.NoError(t, err)

		allPods, err := state.AllPods()
		assert.NoError(t, err)
		assert.Equal(t, 0, len(allPods))
	})
}

func TestRemovePodFromPods(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod1, err := getTestPod1(lockPath)
		assert.NoError(t, err)

		testPod2, err := getTestPod2(lockPath)
		assert.NoError(t, err)

		err = state.AddPod(testPod1)
		assert.NoError(t, err)

		err = state.AddPod(testPod2)
		assert.NoError(t, err)

		err = state.RemovePod(testPod1)
		assert.NoError(t, err)

		allPods, err := state.AllPods()
		assert.NoError(t, err)
		assert.Equal(t, 1, len(allPods))

		// Use assert.EqualValues if the test fails to pretty print diff
		// between actual and expected
		if !testPodsEqual(testPod2, allPods[0]) {
			assert.EqualValues(t, testPod2, allPods[0])
		}
	})
}

func TestRemovePodNotEmptyFails(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod, err := getTestPod1(lockPath)
		assert.NoError(t, err)

		testCtr, err := getTestCtr2(lockPath)
		assert.NoError(t, err)
		testCtr.config.Pod = testPod.ID()

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		err = state.AddContainerToPod(testPod, testCtr)
		assert.NoError(t, err)

		err = state.RemovePod(testPod)
		assert.Error(t, err)

		allPods, err := state.AllPods()
		assert.NoError(t, err)
		assert.Equal(t, 1, len(allPods))
	})
}

func TestRemovePodAfterEmptySucceeds(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod, err := getTestPod1(lockPath)
		assert.NoError(t, err)

		testCtr, err := getTestCtr2(lockPath)
		assert.NoError(t, err)
		testCtr.config.Pod = testPod.ID()

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		err = state.AddContainerToPod(testPod, testCtr)
		assert.NoError(t, err)

		err = state.RemoveContainerFromPod(testPod, testCtr)
		assert.NoError(t, err)

		err = state.RemovePod(testPod)
		assert.NoError(t, err)

		allPods, err := state.AllPods()
		assert.NoError(t, err)
		assert.Equal(t, 0, len(allPods))
	})
}

func TestAllPodsEmptyOnEmptyState(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		allPods, err := state.AllPods()
		assert.NoError(t, err)
		assert.Equal(t, 0, len(allPods))
	})
}

func TestAllPodsFindsPod(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod, err := getTestPod1(lockPath)
		assert.NoError(t, err)

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		allPods, err := state.AllPods()
		assert.NoError(t, err)
		assert.Equal(t, 1, len(allPods))

		// Use assert.EqualValues if the test fails to pretty print diff
		// between actual and expected
		if !testPodsEqual(testPod, allPods[0]) {
			assert.EqualValues(t, testPod, allPods[0])
		}
	})
}

func TestAllPodsMultiplePods(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod1, err := getTestPod1(lockPath)
		assert.NoError(t, err)

		testPod2, err := getTestPod2(lockPath)
		assert.NoError(t, err)

		testPod3, err := getTestPodN("3", lockPath)
		assert.NoError(t, err)

		allPods1, err := state.AllPods()
		assert.NoError(t, err)
		assert.Equal(t, 0, len(allPods1))

		err = state.AddPod(testPod1)
		assert.NoError(t, err)

		allPods2, err := state.AllPods()
		assert.NoError(t, err)
		assert.Equal(t, 1, len(allPods2))

		err = state.AddPod(testPod2)
		assert.NoError(t, err)

		allPods3, err := state.AllPods()
		assert.NoError(t, err)
		assert.Equal(t, 2, len(allPods3))

		err = state.AddPod(testPod3)
		assert.NoError(t, err)

		allPods4, err := state.AllPods()
		assert.NoError(t, err)
		assert.Equal(t, 3, len(allPods4))
	})
}

func TestPodHasContainerNoSuchPod(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		_, err := state.PodHasContainer(&Pod{}, strings.Repeat("0", 32))
		assert.Error(t, err)
	})
}

func TestPodHasContainerEmptyCtrID(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod, err := getTestPod1(lockPath)
		assert.NoError(t, err)

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		_, err = state.PodHasContainer(testPod, "")
		assert.Error(t, err)
	})
}

func TestPodHasContainerNoSuchCtr(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod, err := getTestPod1(lockPath)
		assert.NoError(t, err)

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		exist, err := state.PodHasContainer(testPod, strings.Repeat("2", 32))
		assert.NoError(t, err)
		assert.False(t, exist)
	})
}

func TestPodHasContainerCtrNotInPod(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod, err := getTestPod1(lockPath)
		assert.NoError(t, err)

		testCtr, err := getTestCtr2(lockPath)
		assert.NoError(t, err)

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr)
		assert.NoError(t, err)

		exist, err := state.PodHasContainer(testPod, testCtr.ID())
		assert.NoError(t, err)
		assert.False(t, exist)
	})
}

func TestPodHasContainerSucceeds(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod, err := getTestPod1(lockPath)
		assert.NoError(t, err)

		testCtr, err := getTestCtr2(lockPath)
		assert.NoError(t, err)

		testCtr.config.Pod = testPod.ID()

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		err = state.AddContainerToPod(testPod, testCtr)
		assert.NoError(t, err)

		exist, err := state.PodHasContainer(testPod, testCtr.ID())
		assert.NoError(t, err)
		assert.True(t, exist)
	})
}

func TestPodContainersByIDInvalidPod(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		_, err := state.PodContainersByID(&Pod{})
		assert.Error(t, err)
	})
}

func TestPodContainerdByIDPodNotInState(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod, err := getTestPod1(lockPath)
		assert.NoError(t, err)

		_, err = state.PodContainersByID(testPod)
		assert.Error(t, err)
		assert.False(t, testPod.valid)
	})
}

func TestPodContainersByIDEmptyPod(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod, err := getTestPod1(lockPath)
		assert.NoError(t, err)

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		ctrs, err := state.PodContainersByID(testPod)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(ctrs))
	})
}

func TestPodContainersByIDOneContainer(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod, err := getTestPod1(lockPath)
		assert.NoError(t, err)

		testCtr, err := getTestCtr2(lockPath)
		assert.NoError(t, err)

		testCtr.config.Pod = testPod.ID()

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		err = state.AddContainerToPod(testPod, testCtr)
		assert.NoError(t, err)

		ctrs, err := state.PodContainersByID(testPod)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(ctrs))
		assert.Equal(t, testCtr.ID(), ctrs[0])
	})
}

func TestPodContainersByIDMultipleContainers(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod, err := getTestPod1(lockPath)
		assert.NoError(t, err)

		testCtr1, err := getTestCtr2(lockPath)
		assert.NoError(t, err)
		testCtr1.config.Pod = testPod.ID()

		testCtr2, err := getTestCtrN("3", lockPath)
		assert.NoError(t, err)
		testCtr2.config.Pod = testPod.ID()

		testCtr3, err := getTestCtrN("4", lockPath)
		assert.NoError(t, err)
		testCtr3.config.Pod = testPod.ID()

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		ctrs0, err := state.PodContainersByID(testPod)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(ctrs0))

		err = state.AddContainerToPod(testPod, testCtr1)
		assert.NoError(t, err)

		ctrs1, err := state.PodContainersByID(testPod)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(ctrs1))

		err = state.AddContainerToPod(testPod, testCtr2)
		assert.NoError(t, err)

		ctrs2, err := state.PodContainersByID(testPod)
		assert.NoError(t, err)
		assert.Equal(t, 2, len(ctrs2))

		err = state.AddContainerToPod(testPod, testCtr3)
		assert.NoError(t, err)

		ctrs3, err := state.PodContainersByID(testPod)
		assert.NoError(t, err)
		assert.Equal(t, 3, len(ctrs3))
	})
}

func TestPodContainersInvalidPod(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		_, err := state.PodContainers(&Pod{})
		assert.Error(t, err)
	})
}

func TestPodContainerdPodNotInState(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod, err := getTestPod1(lockPath)
		assert.NoError(t, err)

		_, err = state.PodContainers(testPod)
		assert.Error(t, err)
		assert.False(t, testPod.valid)
	})
}

func TestPodContainersEmptyPod(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod, err := getTestPod1(lockPath)
		assert.NoError(t, err)

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		ctrs, err := state.PodContainers(testPod)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(ctrs))
	})
}

func TestPodContainersOneContainer(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod, err := getTestPod1(lockPath)
		assert.NoError(t, err)

		testCtr, err := getTestCtr2(lockPath)
		assert.NoError(t, err)

		testCtr.config.Pod = testPod.ID()

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		err = state.AddContainerToPod(testPod, testCtr)
		assert.NoError(t, err)

		ctrs, err := state.PodContainers(testPod)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(ctrs))

		// Use assert.EqualValues if the test fails to pretty print diff
		// between actual and expected
		if !testContainersEqual(testCtr, ctrs[0]) {
			assert.EqualValues(t, testCtr, ctrs[0])
		}
	})
}

func TestPodContainersMultipleContainers(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod, err := getTestPod1(lockPath)
		assert.NoError(t, err)

		testCtr1, err := getTestCtr2(lockPath)
		assert.NoError(t, err)
		testCtr1.config.Pod = testPod.ID()

		testCtr2, err := getTestCtrN("3", lockPath)
		assert.NoError(t, err)
		testCtr2.config.Pod = testPod.ID()

		testCtr3, err := getTestCtrN("4", lockPath)
		assert.NoError(t, err)
		testCtr3.config.Pod = testPod.ID()

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		ctrs0, err := state.PodContainers(testPod)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(ctrs0))

		err = state.AddContainerToPod(testPod, testCtr1)
		assert.NoError(t, err)

		ctrs1, err := state.PodContainers(testPod)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(ctrs1))

		err = state.AddContainerToPod(testPod, testCtr2)
		assert.NoError(t, err)

		ctrs2, err := state.PodContainers(testPod)
		assert.NoError(t, err)
		assert.Equal(t, 2, len(ctrs2))

		err = state.AddContainerToPod(testPod, testCtr3)
		assert.NoError(t, err)

		ctrs3, err := state.PodContainers(testPod)
		assert.NoError(t, err)
		assert.Equal(t, 3, len(ctrs3))
	})
}

func TestRemovePodContainersInvalidPod(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		err := state.RemovePodContainers(&Pod{})
		assert.Error(t, err)
	})
}

func TestRemovePodContainersPodNotInState(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod, err := getTestPod1(lockPath)
		assert.NoError(t, err)

		err = state.RemovePodContainers(testPod)
		assert.Error(t, err)
		assert.False(t, testPod.valid)
	})
}

func TestRemovePodContainersNoContainers(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod, err := getTestPod1(lockPath)
		assert.NoError(t, err)

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		err = state.RemovePodContainers(testPod)
		assert.NoError(t, err)

		ctrs, err := state.PodContainersByID(testPod)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(ctrs))
	})
}

func TestRemovePodContainersOneContainer(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod, err := getTestPod1(lockPath)
		assert.NoError(t, err)

		testCtr, err := getTestCtr2(lockPath)
		assert.NoError(t, err)
		testCtr.config.Pod = testPod.ID()

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		err = state.AddContainerToPod(testPod, testCtr)
		assert.NoError(t, err)

		err = state.RemovePodContainers(testPod)
		assert.NoError(t, err)

		ctrs, err := state.PodContainersByID(testPod)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(ctrs))
	})
}

func TestRemovePodContainersPreservesCtrOutsidePod(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod, err := getTestPod1(lockPath)
		assert.NoError(t, err)

		testCtr1, err := getTestCtr2(lockPath)
		assert.NoError(t, err)
		testCtr1.config.Pod = testPod.ID()

		testCtr2, err := getTestCtrN("3", lockPath)
		assert.NoError(t, err)

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		err = state.AddContainerToPod(testPod, testCtr1)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr2)
		assert.NoError(t, err)

		err = state.RemovePodContainers(testPod)
		assert.NoError(t, err)

		ctrs, err := state.PodContainersByID(testPod)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(ctrs))

		allCtrs, err := state.AllContainers()
		assert.NoError(t, err)
		assert.Equal(t, 1, len(allCtrs))
	})
}

func TestRemovePodContainersTwoContainers(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod, err := getTestPod1(lockPath)
		assert.NoError(t, err)

		testCtr1, err := getTestCtr2(lockPath)
		assert.NoError(t, err)
		testCtr1.config.Pod = testPod.ID()

		testCtr2, err := getTestCtrN("3", lockPath)
		assert.NoError(t, err)
		testCtr2.config.Pod = testPod.ID()

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		err = state.AddContainerToPod(testPod, testCtr1)
		assert.NoError(t, err)

		err = state.AddContainerToPod(testPod, testCtr2)
		assert.NoError(t, err)

		err = state.RemovePodContainers(testPod)
		assert.NoError(t, err)

		ctrs, err := state.PodContainersByID(testPod)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(ctrs))
	})
}

func TestRemovePodContainerDependencyInPod(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod, err := getTestPod1(lockPath)
		assert.NoError(t, err)

		testCtr1, err := getTestCtr2(lockPath)
		assert.NoError(t, err)
		testCtr1.config.Pod = testPod.ID()

		testCtr2, err := getTestCtrN("3", lockPath)
		assert.NoError(t, err)
		testCtr2.config.Pod = testPod.ID()
		testCtr2.config.IPCNsCtr = testCtr1.ID()

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		err = state.AddContainerToPod(testPod, testCtr1)
		assert.NoError(t, err)

		err = state.AddContainerToPod(testPod, testCtr2)
		assert.NoError(t, err)

		err = state.RemovePodContainers(testPod)
		assert.NoError(t, err)

		ctrs, err := state.PodContainersByID(testPod)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(ctrs))
	})
}

func TestRemovePodContainerDependencyNotInPod(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod, err := getTestPod1(lockPath)
		assert.NoError(t, err)

		testCtr1, err := getTestCtr2(lockPath)
		assert.NoError(t, err)
		testCtr1.config.Pod = testPod.ID()

		testCtr2, err := getTestCtrN("3", lockPath)
		assert.NoError(t, err)
		testCtr2.config.IPCNsCtr = testCtr1.ID()

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		err = state.AddContainerToPod(testPod, testCtr1)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr2)
		assert.NoError(t, err)

		err = state.RemovePodContainers(testPod)
		t.Logf("Err %v", err)
		assert.Error(t, err)

		ctrs, err := state.PodContainersByID(testPod)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(ctrs))
	})
}

func TestAddContainerToPodInvalidPod(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testCtr, err := getTestCtr1(lockPath)
		assert.NoError(t, err)

		err = state.AddContainerToPod(&Pod{}, testCtr)
		assert.Error(t, err)
	})
}

func TestAddContainerToPodInvalidCtr(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod, err := getTestPod1(lockPath)
		assert.NoError(t, err)

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		err = state.AddContainerToPod(testPod, &Container{config: &ContainerConfig{ID: "1234"}})
		assert.Error(t, err)

		ctrs, err := state.PodContainersByID(testPod)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(ctrs))
	})
}

func TestAddContainerToPodPodNotInState(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod, err := getTestPod1(lockPath)
		assert.NoError(t, err)

		testCtr, err := getTestCtr2(lockPath)
		assert.NoError(t, err)
		testCtr.config.Pod = testPod.ID()

		err = state.AddContainerToPod(testPod, testCtr)
		assert.Error(t, err)
		assert.False(t, testPod.valid)
	})
}

func TestAddContainerToPodSucceeds(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod, err := getTestPod1(lockPath)
		assert.NoError(t, err)

		testCtr, err := getTestCtr2(lockPath)
		assert.NoError(t, err)
		testCtr.config.Pod = testPod.ID()

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		err = state.AddContainerToPod(testPod, testCtr)
		assert.NoError(t, err)

		ctrs, err := state.PodContainers(testPod)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(ctrs))

		allCtrs, err := state.AllContainers()
		assert.NoError(t, err)
		assert.Equal(t, 1, len(allCtrs))

		// Use assert.EqualValues if the test fails to pretty print diff
		// between actual and expected
		if !testContainersEqual(testCtr, ctrs[0]) {
			assert.EqualValues(t, testCtr, ctrs[0])
		}
		if !testContainersEqual(allCtrs[0], ctrs[0]) {
			assert.EqualValues(t, allCtrs[0], ctrs[0])
		}
	})
}

func TestAddContainerToPodTwoContainers(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod, err := getTestPod1(lockPath)
		assert.NoError(t, err)

		testCtr1, err := getTestCtr2(lockPath)
		assert.NoError(t, err)
		testCtr1.config.Pod = testPod.ID()

		testCtr2, err := getTestCtrN("3", lockPath)
		assert.NoError(t, err)
		testCtr2.config.Pod = testPod.ID()

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		err = state.AddContainerToPod(testPod, testCtr1)
		assert.NoError(t, err)

		err = state.AddContainerToPod(testPod, testCtr2)
		assert.NoError(t, err)

		ctrs, err := state.PodContainers(testPod)
		assert.NoError(t, err)
		assert.Equal(t, 2, len(ctrs))

		allCtrs, err := state.AllContainers()
		assert.NoError(t, err)
		assert.Equal(t, 2, len(allCtrs))
	})
}

func TestAddContainerToPodWithAddContainer(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod, err := getTestPod1(lockPath)
		assert.NoError(t, err)

		testCtr1, err := getTestCtr2(lockPath)
		assert.NoError(t, err)
		testCtr1.config.Pod = testPod.ID()

		testCtr2, err := getTestCtrN("3", lockPath)
		assert.NoError(t, err)

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		err = state.AddContainerToPod(testPod, testCtr1)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr2)
		assert.NoError(t, err)

		ctrs, err := state.PodContainers(testPod)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(ctrs))

		allCtrs, err := state.AllContainers()
		assert.NoError(t, err)
		assert.Equal(t, 2, len(allCtrs))

		// Use assert.EqualValues if the test fails to pretty print diff
		// between actual and expected
		if !testContainersEqual(testCtr1, ctrs[0]) {
			assert.EqualValues(t, testCtr1, ctrs[0])
		}
	})
}

func TestAddContainerToPodCtrIDConflict(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod, err := getTestPod1(lockPath)
		assert.NoError(t, err)

		testCtr1, err := getTestCtr2(lockPath)
		assert.NoError(t, err)

		testCtr2, err := getTestContainer(testCtr1.ID(), "testCtr3", lockPath)
		assert.NoError(t, err)
		testCtr2.config.Pod = testPod.ID()

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr1)
		assert.NoError(t, err)

		err = state.AddContainerToPod(testPod, testCtr2)
		assert.Error(t, err)

		ctrs, err := state.PodContainers(testPod)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(ctrs))

		allCtrs, err := state.AllContainers()
		assert.NoError(t, err)
		assert.Equal(t, 1, len(allCtrs))
	})
}

func TestAddContainerToPodCtrNameConflict(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod, err := getTestPod1(lockPath)
		assert.NoError(t, err)

		testCtr1, err := getTestCtr2(lockPath)
		assert.NoError(t, err)

		testCtr2, err := getTestContainer(strings.Repeat("4", 32), testCtr1.Name(), lockPath)
		assert.NoError(t, err)
		testCtr2.config.Pod = testPod.ID()

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr1)
		assert.NoError(t, err)

		err = state.AddContainerToPod(testPod, testCtr2)
		assert.Error(t, err)

		ctrs, err := state.PodContainers(testPod)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(ctrs))

		allCtrs, err := state.AllContainers()
		assert.NoError(t, err)
		assert.Equal(t, 1, len(allCtrs))
	})
}

func TestAddContainerToPodPodIDConflict(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod, err := getTestPod1(lockPath)
		assert.NoError(t, err)

		testCtr, err := getTestContainer(testPod.ID(), "testCtr", lockPath)
		assert.NoError(t, err)
		testCtr.config.Pod = testPod.ID()

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		err = state.AddContainerToPod(testPod, testCtr)
		assert.Error(t, err)

		ctrs, err := state.PodContainers(testPod)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(ctrs))

		allCtrs, err := state.AllContainers()
		assert.NoError(t, err)
		assert.Equal(t, 0, len(allCtrs))
	})
}

func TestAddContainerToPodPodNameConflict(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod, err := getTestPod1(lockPath)
		assert.NoError(t, err)

		testCtr, err := getTestContainer(strings.Repeat("2", 32), testPod.Name(), lockPath)
		assert.NoError(t, err)
		testCtr.config.Pod = testPod.ID()

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		err = state.AddContainerToPod(testPod, testCtr)
		assert.Error(t, err)

		ctrs, err := state.PodContainers(testPod)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(ctrs))

		allCtrs, err := state.AllContainers()
		assert.NoError(t, err)
		assert.Equal(t, 0, len(allCtrs))
	})
}

func TestAddContainerToPodAddsDependencies(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod, err := getTestPod1(lockPath)
		assert.NoError(t, err)

		testCtr1, err := getTestCtr2(lockPath)
		assert.NoError(t, err)
		testCtr1.config.Pod = testPod.ID()

		testCtr2, err := getTestCtrN("3", lockPath)
		assert.NoError(t, err)
		testCtr2.config.Pod = testPod.ID()
		testCtr2.config.IPCNsCtr = testCtr1.ID()

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		err = state.AddContainerToPod(testPod, testCtr1)
		assert.NoError(t, err)

		err = state.AddContainerToPod(testPod, testCtr2)
		assert.NoError(t, err)

		deps, err := state.ContainerInUse(testCtr1)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(deps))
		assert.Equal(t, testCtr2.ID(), deps[0])
	})
}

func TestAddContainerToPodPodDependencyFails(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod, err := getTestPod1(lockPath)
		assert.NoError(t, err)

		testCtr, err := getTestCtr2(lockPath)
		assert.NoError(t, err)
		testCtr.config.Pod = testPod.ID()
		testCtr.config.IPCNsCtr = testPod.ID()

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		err = state.AddContainerToPod(testPod, testCtr)
		assert.Error(t, err)

		ctrs, err := state.PodContainers(testPod)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(ctrs))
	})
}

func TestAddContainerToPodBadDependencyFails(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod, err := getTestPod1(lockPath)
		assert.NoError(t, err)

		testCtr, err := getTestCtr2(lockPath)
		assert.NoError(t, err)
		testCtr.config.Pod = testPod.ID()
		testCtr.config.IPCNsCtr = strings.Repeat("8", 32)

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		err = state.AddContainerToPod(testPod, testCtr)
		assert.Error(t, err)

		ctrs, err := state.PodContainers(testPod)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(ctrs))
	})
}

func TestRemoveContainerFromPodBadPodFails(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testCtr, err := getTestCtr1(lockPath)
		assert.NoError(t, err)

		err = state.RemoveContainerFromPod(&Pod{}, testCtr)
		assert.Error(t, err)
	})
}

func TestRemoveContainerFromPodPodNotInStateFails(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod, err := getTestPod1(lockPath)
		assert.NoError(t, err)

		testCtr, err := getTestCtr2(lockPath)
		assert.NoError(t, err)
		testCtr.config.Pod = testPod.ID()

		err = state.RemoveContainerFromPod(testPod, testCtr)
		assert.Error(t, err)

		assert.False(t, testPod.valid)
	})
}

func TestRemoveContainerFromPodCtrNotInStateFails(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod, err := getTestPod1(lockPath)
		assert.NoError(t, err)

		testCtr, err := getTestCtr2(lockPath)
		assert.NoError(t, err)
		testCtr.config.Pod = testPod.ID()

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		err = state.RemoveContainerFromPod(testPod, testCtr)
		assert.Error(t, err)

		assert.False(t, testCtr.valid)
	})
}

func TestRemoveContainerFromPodCtrNotInPodFails(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod, err := getTestPod1(lockPath)
		assert.NoError(t, err)

		testCtr, err := getTestCtr2(lockPath)
		assert.NoError(t, err)

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		err = state.AddContainer(testCtr)
		assert.NoError(t, err)

		err = state.RemoveContainerFromPod(testPod, testCtr)
		assert.Error(t, err)

		assert.True(t, testCtr.valid)

		ctrs, err := state.AllContainers()
		assert.NoError(t, err)
		assert.Equal(t, 1, len(ctrs))
	})
}

func TestRemoveContainerFromPodSucceeds(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod, err := getTestPod1(lockPath)
		assert.NoError(t, err)

		testCtr, err := getTestCtr2(lockPath)
		assert.NoError(t, err)
		testCtr.config.Pod = testPod.ID()

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		err = state.AddContainerToPod(testPod, testCtr)
		assert.NoError(t, err)

		err = state.RemoveContainerFromPod(testPod, testCtr)
		assert.NoError(t, err)

		ctrs, err := state.PodContainers(testPod)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(ctrs))

		allCtrs, err := state.AllContainers()
		assert.NoError(t, err)
		assert.Equal(t, 0, len(allCtrs))
	})
}

func TestRemoveContainerFromPodWithDependencyFails(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod, err := getTestPod1(lockPath)
		assert.NoError(t, err)

		testCtr1, err := getTestCtr2(lockPath)
		assert.NoError(t, err)
		testCtr1.config.Pod = testPod.ID()

		testCtr2, err := getTestCtrN("3", lockPath)
		assert.NoError(t, err)
		testCtr2.config.Pod = testPod.ID()
		testCtr2.config.IPCNsCtr = testCtr1.ID()

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		err = state.AddContainerToPod(testPod, testCtr1)
		assert.NoError(t, err)

		err = state.AddContainerToPod(testPod, testCtr2)
		assert.NoError(t, err)

		err = state.RemoveContainerFromPod(testPod, testCtr1)
		assert.Error(t, err)

		ctrs, err := state.PodContainers(testPod)
		assert.NoError(t, err)
		assert.Equal(t, 2, len(ctrs))

		allCtrs, err := state.AllContainers()
		assert.NoError(t, err)
		assert.Equal(t, 2, len(allCtrs))
	})
}

func TestRemoveContainerFromPodWithDependencySucceedsAfterDepRemoved(t *testing.T) {
	runForAllStates(t, func(t *testing.T, state State, lockPath string) {
		testPod, err := getTestPod1(lockPath)
		assert.NoError(t, err)

		testCtr1, err := getTestCtr2(lockPath)
		assert.NoError(t, err)
		testCtr1.config.Pod = testPod.ID()

		testCtr2, err := getTestCtrN("3", lockPath)
		assert.NoError(t, err)
		testCtr2.config.Pod = testPod.ID()
		testCtr2.config.IPCNsCtr = testCtr1.ID()

		err = state.AddPod(testPod)
		assert.NoError(t, err)

		err = state.AddContainerToPod(testPod, testCtr1)
		assert.NoError(t, err)

		err = state.AddContainerToPod(testPod, testCtr2)
		assert.NoError(t, err)

		err = state.RemoveContainerFromPod(testPod, testCtr2)
		assert.NoError(t, err)

		err = state.RemoveContainerFromPod(testPod, testCtr1)
		assert.NoError(t, err)

		ctrs, err := state.PodContainers(testPod)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(ctrs))

		allCtrs, err := state.AllContainers()
		assert.NoError(t, err)
		assert.Equal(t, 0, len(allCtrs))
	})
}
