package e2e_test

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/containers/podman/v5/pkg/machine/compression"
	"github.com/containers/podman/v5/pkg/machine/define"
	"github.com/containers/podman/v5/pkg/machine/ocipull"
)

func pullOCITestDisk(finalDir string, vmType define.VMType) error {
	imageCacheDir, err := define.NewMachineFile(finalDir, nil)
	if err != nil {
		return err
	}
	unusedFinalPath, err := imageCacheDir.AppendToNewVMFile(fmt.Sprintf("machinetest-%s", runtime.GOOS), nil)
	if err != nil {
		return err
	}
	dirs := define.MachineDirs{ImageCacheDir: imageCacheDir}
	ociArtPull, err := ocipull.NewOCIArtifactPull(context.Background(), &dirs, "", "e2emachine", vmType, unusedFinalPath)
	if err != nil {
		return err
	}
	_, err = ociArtPull.GetNoCompress()
	if err != nil {
		return err
	}
	fp, originalName := ociArtPull.OriginalFileName()
	// Rename the download to something we recognize
	compressionExt := filepath.Ext(fp)
	fqImageName = filepath.Join(tmpDir, strings.TrimSuffix(originalName, compressionExt))
	suiteImageName = filepath.Base(fqImageName)
	compressedImage, err := define.NewMachineFile(fp, nil)
	if err != nil {
		return err
	}
	return compression.Decompress(compressedImage, fqImageName)
}
