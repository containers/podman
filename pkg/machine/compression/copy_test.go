package compression

import (
	"os"
	"path/filepath"
	"testing"

	crcOs "github.com/crc-org/crc/v2/pkg/os"
)

func TestCopyFile(t *testing.T) {
	testStr := "test-machine"

	srcFile, err := os.CreateTemp("", "machine-test-")
	if err != nil {
		t.Fatal(err)
	}
	srcFi, err := srcFile.Stat()
	if err != nil {
		t.Fatal(err)
	}

	_, _ = srcFile.Write([]byte(testStr)) //nolint:mirror
	srcFile.Close()

	srcFilePath := filepath.Join(os.TempDir(), srcFi.Name())

	destFile, err := os.CreateTemp("", "machine-copy-test-")
	if err != nil {
		t.Fatal(err)
	}

	destFi, err := destFile.Stat()
	if err != nil {
		t.Fatal(err)
	}

	destFile.Close()

	destFilePath := filepath.Join(os.TempDir(), destFi.Name())

	if err := crcOs.CopyFile(srcFilePath, destFilePath); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(destFilePath)
	if err != nil {
		t.Fatal(err)
	}

	if string(data) != testStr {
		t.Fatalf("expected data \"%s\"; received \"%s\"", testStr, string(data))
	}
}
