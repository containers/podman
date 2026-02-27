package crutils

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCRRemoveDeletedFiles(t *testing.T) {
	containerRoot := t.TempDir()

	checkpointDir := t.TempDir()

	legitFile := filepath.Join(containerRoot, "legit.txt")
	if err := os.WriteFile(legitFile, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	targetFile := filepath.Join(t.TempDir(), "target.txt")
	if err := os.WriteFile(targetFile, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}

	relPath, err := filepath.Rel(containerRoot, targetFile)
	if err != nil {
		t.Fatal(err)
	}

	deletedFiles := []string{
		"/legit.txt",
		relPath,
	}

	deletedFilesContent, err := json.Marshal(deletedFiles)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(checkpointDir, "deleted.files"), deletedFilesContent, 0o644); err != nil {
		t.Fatal(err)
	}

	err = CRRemoveDeletedFiles("test-container", checkpointDir, containerRoot)
	if err != nil {
		t.Fatalf("CRRemoveDeletedFiles returned unexpected error: %v", err)
	}

	if _, err := os.Stat(legitFile); !os.IsNotExist(err) {
		t.Errorf("legitFile was not deleted")
	}

	if _, err := os.Stat(targetFile); os.IsNotExist(err) {
		t.Errorf("SECURITY FAIL: targetFile was deleted! Path traversal successful.")
	}
}

func TestCRRemoveDeletedFilesPathTraversal(t *testing.T) {
	containerRoot := t.TempDir()

	checkpointDir := t.TempDir()

	safeFile := filepath.Join(containerRoot, "safe.txt")
	if err := os.WriteFile(safeFile, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	externalFile := filepath.Join(t.TempDir(), "external.txt")
	if err := os.WriteFile(externalFile, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}

	relPath, err := filepath.Rel(containerRoot, externalFile)
	if err != nil {
		t.Fatal(err)
	}

	deletedFiles := []string{relPath}

	deletedFilesContent, err := json.Marshal(deletedFiles)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(checkpointDir, "deleted.files"), deletedFilesContent, 0o644); err != nil {
		t.Fatal(err)
	}

	err = CRRemoveDeletedFiles("test-container", checkpointDir, containerRoot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(safeFile); os.IsNotExist(err) {
		t.Errorf("safeFile was incorrectly deleted")
	}

	if _, err := os.Stat(externalFile); os.IsNotExist(err) {
		t.Errorf("SECURITY FAIL: externalFile was deleted! Path traversal successful.")
	}
}
