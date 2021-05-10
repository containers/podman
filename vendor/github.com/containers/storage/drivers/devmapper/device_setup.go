// +build linux,cgo

package devmapper

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type directLVMConfig struct {
	Device              string
	ThinpPercent        uint64
	ThinpMetaPercent    uint64
	AutoExtendPercent   uint64
	AutoExtendThreshold uint64
	MetaDataSize        string
}

var (
	errThinpPercentMissing = errors.New("must set both `dm.thinp_percent` and `dm.thinp_metapercent` if either is specified")
	errThinpPercentTooBig  = errors.New("combined `dm.thinp_percent` and `dm.thinp_metapercent` must not be greater than 100")
	errMissingSetupDevice  = errors.New("must provide device path in `dm.directlvm_device` in order to configure direct-lvm")
)

func validateLVMConfig(cfg directLVMConfig) error {
	if cfg.Device == "" {
		return errMissingSetupDevice
	}
	if (cfg.ThinpPercent > 0 && cfg.ThinpMetaPercent == 0) || cfg.ThinpMetaPercent > 0 && cfg.ThinpPercent == 0 {
		return errThinpPercentMissing
	}

	if cfg.ThinpPercent+cfg.ThinpMetaPercent > 100 {
		return errThinpPercentTooBig
	}
	return nil
}

func checkDevAvailable(dev string) error {
	lvmScan, err := exec.LookPath("lvmdiskscan")
	if err != nil {
		logrus.Debug("could not find lvmdiskscan")
		return nil
	}

	out, err := exec.Command(lvmScan).CombinedOutput()
	if err != nil {
		logrus.WithError(err).Error(string(out))
		return nil
	}

	if !bytes.Contains(out, []byte(dev)) {
		return errors.Errorf("%s is not available for use with devicemapper", dev)
	}
	return nil
}

func checkDevInVG(dev string) error {
	pvDisplay, err := exec.LookPath("pvdisplay")
	if err != nil {
		logrus.Debug("could not find pvdisplay")
		return nil
	}

	out, err := exec.Command(pvDisplay, dev).CombinedOutput()
	if err != nil {
		logrus.WithError(err).Error(string(out))
		return nil
	}

	scanner := bufio.NewScanner(bytes.NewReader(bytes.TrimSpace(out)))
	for scanner.Scan() {
		fields := strings.SplitAfter(strings.TrimSpace(scanner.Text()), "VG Name")
		if len(fields) > 1 {
			// got "VG Name" line"
			vg := strings.TrimSpace(fields[1])
			if len(vg) > 0 {
				return errors.Errorf("%s is already part of a volume group %q: must remove this device from any volume group or provide a different device", dev, vg)
			}
			logrus.Error(fields)
			break
		}
	}
	return nil
}

func checkDevHasFS(dev string) error {
	blkid, err := exec.LookPath("blkid")
	if err != nil {
		logrus.Debug("could not find blkid")
		return nil
	}

	out, err := exec.Command(blkid, dev).CombinedOutput()
	if err != nil {
		logrus.WithError(err).Error(string(out))
		return nil
	}

	fields := bytes.Fields(out)
	for _, f := range fields {
		kv := bytes.Split(f, []byte{'='})
		if bytes.Equal(kv[0], []byte("TYPE")) {
			v := bytes.Trim(kv[1], "\"")
			if len(v) > 0 {
				return errors.Errorf("%s has a filesystem already, use dm.directlvm_device_force=true if you want to wipe the device", dev)
			}
			return nil
		}
	}
	return nil
}

func verifyBlockDevice(dev string, force bool) error {
	absPath, err := filepath.Abs(dev)
	if err != nil {
		return errors.Errorf("unable to get absolute path for %s: %s", dev, err)
	}
	realPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return errors.Errorf("failed to canonicalise path for %s: %s", dev, err)
	}
	if err := checkDevAvailable(absPath); err != nil {
		logrus.Infof("block device '%s' not available, checking '%s'", absPath, realPath)
		if err := checkDevAvailable(realPath); err != nil {
			return errors.Errorf("neither '%s' nor '%s' are in the output of lvmdiskscan, can't use device.", absPath, realPath)
		}
	}
	if err := checkDevInVG(realPath); err != nil {
		return err
	}

	if force {
		return nil
	}

	if err := checkDevHasFS(realPath); err != nil {
		return err
	}
	return nil
}

func readLVMConfig(root string) (directLVMConfig, error) {
	var cfg directLVMConfig

	p := filepath.Join(root, "setup-config.json")
	b, err := ioutil.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, errors.Wrap(err, "error reading existing setup config")
	}

	// check if this is just an empty file, no need to produce a json error later if so
	if len(b) == 0 {
		return cfg, nil
	}

	err = json.Unmarshal(b, &cfg)
	return cfg, errors.Wrap(err, "error unmarshaling previous device setup config")
}

func writeLVMConfig(root string, cfg directLVMConfig) error {
	p := filepath.Join(root, "setup-config.json")
	b, err := json.Marshal(cfg)
	if err != nil {
		return errors.Wrap(err, "error marshalling direct lvm config")
	}
	err = ioutil.WriteFile(p, b, 0600)
	return errors.Wrap(err, "error writing direct lvm config to file")
}

func setupDirectLVM(cfg directLVMConfig) error {
	lvmProfileDir := "/etc/lvm/profile"
	binaries := []string{"pvcreate", "vgcreate", "lvcreate", "lvconvert", "lvchange", "thin_check"}

	for _, bin := range binaries {
		if _, err := exec.LookPath(bin); err != nil {
			return errors.Wrap(err, "error looking up command `"+bin+"` while setting up direct lvm")
		}
	}

	err := os.MkdirAll(lvmProfileDir, 0755)
	if err != nil {
		return errors.Wrap(err, "error creating lvm profile directory")
	}

	if cfg.AutoExtendPercent == 0 {
		cfg.AutoExtendPercent = 20
	}

	if cfg.AutoExtendThreshold == 0 {
		cfg.AutoExtendThreshold = 80
	}

	if cfg.ThinpPercent == 0 {
		cfg.ThinpPercent = 95
	}
	if cfg.ThinpMetaPercent == 0 {
		cfg.ThinpMetaPercent = 1
	}
	if cfg.MetaDataSize == "" {
		cfg.MetaDataSize = "128k"
	}

	out, err := exec.Command("pvcreate", "--metadatasize", cfg.MetaDataSize, "-f", cfg.Device).CombinedOutput()
	if err != nil {
		return errors.Wrap(err, string(out))
	}

	out, err = exec.Command("vgcreate", "storage", cfg.Device).CombinedOutput()
	if err != nil {
		return errors.Wrap(err, string(out))
	}

	out, err = exec.Command("lvcreate", "--wipesignatures", "y", "-n", "thinpool", "storage", "--extents", fmt.Sprintf("%d%%VG", cfg.ThinpPercent)).CombinedOutput()
	if err != nil {
		return errors.Wrap(err, string(out))
	}
	out, err = exec.Command("lvcreate", "--wipesignatures", "y", "-n", "thinpoolmeta", "storage", "--extents", fmt.Sprintf("%d%%VG", cfg.ThinpMetaPercent)).CombinedOutput()
	if err != nil {
		return errors.Wrap(err, string(out))
	}

	out, err = exec.Command("lvconvert", "-y", "--zero", "n", "-c", "512K", "--thinpool", "storage/thinpool", "--poolmetadata", "storage/thinpoolmeta").CombinedOutput()
	if err != nil {
		return errors.Wrap(err, string(out))
	}

	profile := fmt.Sprintf("activation{\nthin_pool_autoextend_threshold=%d\nthin_pool_autoextend_percent=%d\n}", cfg.AutoExtendThreshold, cfg.AutoExtendPercent)
	err = ioutil.WriteFile(lvmProfileDir+"/storage-thinpool.profile", []byte(profile), 0600)
	if err != nil {
		return errors.Wrap(err, "error writing storage thinp autoextend profile")
	}

	out, err = exec.Command("lvchange", "--metadataprofile", "storage-thinpool", "storage/thinpool").CombinedOutput()
	return errors.Wrap(err, string(out))
}
