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
