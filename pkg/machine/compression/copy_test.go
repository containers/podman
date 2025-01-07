package compression

import (
	"os"
	"testing"

	crcOs "github.com/crc-org/crc/v2/pkg/os"
)

func TestCopyFile(t *testing.T) {
	testStr := "test-machine"

	tmpDir := t.TempDir()

	srcFile, err := os.CreateTemp(tmpDir, "machine-test-")
	if err != nil {
		t.Fatal(err)
	}

	_, _ = srcFile.Write([]byte(testStr)) //nolint:mirror
	srcFile.Close()

	destFile, err := os.CreateTemp(tmpDir, "machine-copy-test-")
	if err != nil {
		t.Fatal(err)
	}

	destFile.Close()

	if err := crcOs.CopyFile(srcFile.Name(), destFile.Name()); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(destFile.Name())
	if err != nil {
		t.Fatal(err)
	}

	if string(data) != testStr {
		t.Fatalf("expected data \"%s\"; received \"%s\"", testStr, string(data))
	}
}
