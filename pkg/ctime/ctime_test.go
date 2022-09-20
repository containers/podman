package ctime

import (
	"os"
	"testing"
	"time"
)

func TestCreated(t *testing.T) {
	before := time.Now()

	fileA, err := os.CreateTemp("", "ctime-test-")
	if err != nil {
		t.Error(err)
	}
	defer os.Remove(fileA.Name())

	fileB, err := os.CreateTemp("", "ctime-test-")
	if err != nil {
		t.Error(err)
	}
	defer os.Remove(fileB.Name())

	after := time.Now()

	infoA, err := fileA.Stat()
	if err != nil {
		t.Error(err)
	}

	err = fileA.Close()
	if err != nil {
		t.Error(err)
	}

	infoB, err := fileB.Stat()
	if err != nil {
		t.Error(err)
	}

	err = fileB.Close()
	if err != nil {
		t.Error(err)
	}

	createdA := Created(infoA)
	beforeToCreateA := createdA.Sub(before)
	if beforeToCreateA.Nanoseconds() < -1000000000 {
		t.Errorf("created file A %s is %v nanoseconds before %s", createdA, -beforeToCreateA.Nanoseconds(), before)
	}

	createdB := Created(infoB)
	createAToCreateB := createdB.Sub(createdA)
	if createAToCreateB.Nanoseconds() < 0 {
		t.Errorf("created file B %s is %v nanoseconds before file A %s", createdB, -createAToCreateB.Nanoseconds(), createdA)
	}

	createBToAfter := after.Sub(createdB)
	if createBToAfter.Nanoseconds() < 0 {
		t.Errorf("created file B %s is %v nanoseconds after %s", createdB, -createBToAfter.Nanoseconds(), after)
	}
}
