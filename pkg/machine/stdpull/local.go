package stdpull

import (
	"github.com/containers/podman/v5/pkg/machine/compression"
	"github.com/containers/podman/v5/pkg/machine/define"
	"github.com/containers/storage/pkg/fileutils"
	"github.com/sirupsen/logrus"
)

type StdDiskPull struct {
	finalPath *define.VMFile
	inputPath *define.VMFile
}

func NewStdDiskPull(inputPath string, finalpath *define.VMFile) (*StdDiskPull, error) {
	ip, err := define.NewMachineFile(inputPath, nil)
	if err != nil {
		return nil, err
	}
	return &StdDiskPull{inputPath: ip, finalPath: finalpath}, nil
}

func (s *StdDiskPull) Get() error {
	if err := fileutils.Exists(s.inputPath.GetPath()); err != nil {
		// could not find disk
		return err
	}
	logrus.Debugf("decompressing (if needed) %s to %s", s.inputPath.GetPath(), s.finalPath.GetPath())
	return compression.Decompress(s.inputPath, s.finalPath.GetPath())
}
