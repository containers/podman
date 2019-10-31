package config

import (
	"path/filepath"

	"github.com/containers/libpod/libpod/define"
	"github.com/sirupsen/logrus"
)

// Merge merges the other config into the current one.  Note that a field of the
// other config is only merged when it's not already set in the current one.
//
// Note that the StateType and the StorageConfig will NOT be changed.
func (c *Config) mergeConfig(other *Config) error {
	// strings
	c.CgroupManager = mergeStrings(c.CgroupManager, other.CgroupManager)
	c.CNIConfigDir = mergeStrings(c.CNIConfigDir, other.CNIConfigDir)
	c.CNIDefaultNetwork = mergeStrings(c.CNIDefaultNetwork, other.CNIDefaultNetwork)
	c.DefaultMountsFile = mergeStrings(c.DefaultMountsFile, other.DefaultMountsFile)
	c.DetachKeys = mergeStrings(c.DetachKeys, other.DetachKeys)
	c.EventsLogFilePath = mergeStrings(c.EventsLogFilePath, other.EventsLogFilePath)
	c.EventsLogger = mergeStrings(c.EventsLogger, other.EventsLogger)
	c.ImageDefaultTransport = mergeStrings(c.ImageDefaultTransport, other.ImageDefaultTransport)
	c.InfraCommand = mergeStrings(c.InfraCommand, other.InfraCommand)
	c.InfraImage = mergeStrings(c.InfraImage, other.InfraImage)
	c.InitPath = mergeStrings(c.InitPath, other.InitPath)
	c.LockType = mergeStrings(c.LockType, other.LockType)
	c.Namespace = mergeStrings(c.Namespace, other.Namespace)
	c.NetworkCmdPath = mergeStrings(c.NetworkCmdPath, other.NetworkCmdPath)
	c.OCIRuntime = mergeStrings(c.OCIRuntime, other.OCIRuntime)
	c.SignaturePolicyPath = mergeStrings(c.SignaturePolicyPath, other.SignaturePolicyPath)
	c.StaticDir = mergeStrings(c.StaticDir, other.StaticDir)
	c.TmpDir = mergeStrings(c.TmpDir, other.TmpDir)
	c.VolumePath = mergeStrings(c.VolumePath, other.VolumePath)

	// string map of slices
	c.OCIRuntimes = mergeStringMaps(c.OCIRuntimes, other.OCIRuntimes)

	// string slices
	c.CNIPluginDir = mergeStringSlices(c.CNIPluginDir, other.CNIPluginDir)
	c.ConmonEnvVars = mergeStringSlices(c.ConmonEnvVars, other.ConmonEnvVars)
	c.ConmonPath = mergeStringSlices(c.ConmonPath, other.ConmonPath)
	c.HooksDir = mergeStringSlices(c.HooksDir, other.HooksDir)
	c.RuntimePath = mergeStringSlices(c.RuntimePath, other.RuntimePath)
	c.RuntimeSupportsJSON = mergeStringSlices(c.RuntimeSupportsJSON, other.RuntimeSupportsJSON)
	c.RuntimeSupportsNoCgroups = mergeStringSlices(c.RuntimeSupportsNoCgroups, other.RuntimeSupportsNoCgroups)

	// int64s
	c.MaxLogSize = mergeInt64s(c.MaxLogSize, other.MaxLogSize)

	// uint32s
	c.NumLocks = mergeUint32s(c.NumLocks, other.NumLocks)

	// bools
	c.EnableLabeling = mergeBools(c.EnableLabeling, other.EnableLabeling)
	c.EnablePortReservation = mergeBools(c.EnablePortReservation, other.EnablePortReservation)
	c.NoPivotRoot = mergeBools(c.NoPivotRoot, other.NoPivotRoot)
	c.SDNotify = mergeBools(c.SDNotify, other.SDNotify)

	// state type
	if c.StateType == define.InvalidStateStore {
		c.StateType = other.StateType
	}

	// store options - need to check all fields since some configs might only
	// set it partially
	c.StorageConfig.RunRoot = mergeStrings(c.StorageConfig.RunRoot, other.StorageConfig.RunRoot)
	c.StorageConfig.GraphRoot = mergeStrings(c.StorageConfig.GraphRoot, other.StorageConfig.GraphRoot)
	c.StorageConfig.GraphDriverName = mergeStrings(c.StorageConfig.GraphDriverName, other.StorageConfig.GraphDriverName)
	c.StorageConfig.GraphDriverOptions = mergeStringSlices(c.StorageConfig.GraphDriverOptions, other.StorageConfig.GraphDriverOptions)
	if c.StorageConfig.UIDMap == nil {
		c.StorageConfig.UIDMap = other.StorageConfig.UIDMap
	}
	if c.StorageConfig.GIDMap == nil {
		c.StorageConfig.GIDMap = other.StorageConfig.GIDMap
	}

	// backwards compat *Set fields
	c.StorageConfigRunRootSet = mergeBools(c.StorageConfigRunRootSet, other.StorageConfigRunRootSet)
	c.StorageConfigGraphRootSet = mergeBools(c.StorageConfigGraphRootSet, other.StorageConfigGraphRootSet)
	c.StorageConfigGraphDriverNameSet = mergeBools(c.StorageConfigGraphDriverNameSet, other.StorageConfigGraphDriverNameSet)
	c.VolumePathSet = mergeBools(c.VolumePathSet, other.VolumePathSet)
	c.StaticDirSet = mergeBools(c.StaticDirSet, other.StaticDirSet)
	c.TmpDirSet = mergeBools(c.TmpDirSet, other.TmpDirSet)

	return nil
}

// MergeDBConfig merges the configuration from the database.
func (c *Config) MergeDBConfig(dbConfig *DBConfig) error {

	if !c.StorageConfigRunRootSet && dbConfig.StorageTmp != "" {
		if c.StorageConfig.RunRoot != dbConfig.StorageTmp &&
			c.StorageConfig.RunRoot != "" {
			logrus.Debugf("Overriding run root %q with %q from database",
				c.StorageConfig.RunRoot, dbConfig.StorageTmp)
		}
		c.StorageConfig.RunRoot = dbConfig.StorageTmp
	}

	if !c.StorageConfigGraphRootSet && dbConfig.StorageRoot != "" {
		if c.StorageConfig.GraphRoot != dbConfig.StorageRoot &&
			c.StorageConfig.GraphRoot != "" {
			logrus.Debugf("Overriding graph root %q with %q from database",
				c.StorageConfig.GraphRoot, dbConfig.StorageRoot)
		}
		c.StorageConfig.GraphRoot = dbConfig.StorageRoot
	}

	if !c.StorageConfigGraphDriverNameSet && dbConfig.GraphDriver != "" {
		if c.StorageConfig.GraphDriverName != dbConfig.GraphDriver &&
			c.StorageConfig.GraphDriverName != "" {
			logrus.Errorf("User-selected graph driver %q overwritten by graph driver %q from database - delete libpod local files to resolve",
				c.StorageConfig.GraphDriverName, dbConfig.GraphDriver)
		}
		c.StorageConfig.GraphDriverName = dbConfig.GraphDriver
	}

	if !c.StaticDirSet && dbConfig.LibpodRoot != "" {
		if c.StaticDir != dbConfig.LibpodRoot && c.StaticDir != "" {
			logrus.Debugf("Overriding static dir %q with %q from database", c.StaticDir, dbConfig.LibpodRoot)
		}
		c.StaticDir = dbConfig.LibpodRoot
	}

	if !c.TmpDirSet && dbConfig.LibpodTmp != "" {
		if c.TmpDir != dbConfig.LibpodTmp && c.TmpDir != "" {
			logrus.Debugf("Overriding tmp dir %q with %q from database", c.TmpDir, dbConfig.LibpodTmp)
		}
		c.TmpDir = dbConfig.LibpodTmp
		c.EventsLogFilePath = filepath.Join(dbConfig.LibpodTmp, "events", "events.log")
	}

	if !c.VolumePathSet && dbConfig.VolumePath != "" {
		if c.VolumePath != dbConfig.VolumePath && c.VolumePath != "" {
			logrus.Debugf("Overriding volume path %q with %q from database", c.VolumePath, dbConfig.VolumePath)
		}
		c.VolumePath = dbConfig.VolumePath
	}
	return nil
}

func mergeStrings(a, b string) string {
	if a == "" {
		return b
	}
	return a
}

func mergeStringSlices(a, b []string) []string {
	if len(a) == 0 && b != nil {
		return b
	}
	return a
}

func mergeStringMaps(a, b map[string][]string) map[string][]string {
	if len(a) == 0 && b != nil {
		return b
	}
	return a
}

func mergeInt64s(a, b int64) int64 {
	if a == 0 {
		return b
	}
	return a
}

func mergeUint32s(a, b uint32) uint32 {
	if a == 0 {
		return b
	}
	return a
}

func mergeBools(a, b bool) bool {
	if !a {
		return b
	}
	return a
}
