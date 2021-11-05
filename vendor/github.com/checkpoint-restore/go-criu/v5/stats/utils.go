package stats

import (
	"encoding/binary"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/checkpoint-restore/go-criu/v5/magic"
	"google.golang.org/protobuf/proto"
)

func readStatisticsFile(imgDir *os.File, fileName string) (*StatsEntry, error) {
	buf, err := ioutil.ReadFile(filepath.Join(imgDir.Name(), fileName))
	if err != nil {
		return nil, err
	}

	if binary.LittleEndian.Uint32(buf[magic.PrimaryMagicOffset:magic.SecondaryMagicOffset]) != magic.ImgServiceMagic {
		return nil, errors.New("Primary magic not found")
	}

	if binary.LittleEndian.Uint32(buf[magic.SecondaryMagicOffset:magic.SizeOffset]) != magic.StatsMagic {
		return nil, errors.New("Secondary magic not found")
	}

	payloadSize := binary.LittleEndian.Uint32(buf[magic.SizeOffset:magic.PayloadOffset])

	st := &StatsEntry{}
	if err := proto.Unmarshal(buf[magic.PayloadOffset:magic.PayloadOffset+payloadSize], st); err != nil {
		return nil, err
	}

	return st, nil
}

func CriuGetDumpStats(imgDir *os.File) (*DumpStatsEntry, error) {
	st, err := readStatisticsFile(imgDir, StatsDump)
	if err != nil {
		return nil, err
	}

	return st.GetDump(), nil
}

func CriuGetRestoreStats(imgDir *os.File) (*RestoreStatsEntry, error) {
	st, err := readStatisticsFile(imgDir, StatsRestore)
	if err != nil {
		return nil, err
	}

	return st.GetRestore(), nil
}
