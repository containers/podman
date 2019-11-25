package config

// Merge merges the other config into the current one.  Note that a field of the
// other config is only merged when it's not already set in the current one.
//
// Note that the StateType and the StorageConfig will NOT be changed.
func (c *Config) mergeConfig(other *Config) error {
	// strings
	c.ApparmorProfile = mergeStrings(c.ApparmorProfile, other.ApparmorProfile)
	c.CgroupManager = mergeStrings(c.CgroupManager, other.CgroupManager)
	c.NetworkDir = mergeStrings(c.NetworkDir, other.NetworkDir)
	c.DefaultNetwork = mergeStrings(c.DefaultNetwork, other.DefaultNetwork)
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
	c.SeccompProfile = mergeStrings(c.SeccompProfile, other.SeccompProfile)
	c.ShmSize = mergeStrings(c.ShmSize, other.ShmSize)
	c.SignaturePolicyPath = mergeStrings(c.SignaturePolicyPath, other.SignaturePolicyPath)
	c.StaticDir = mergeStrings(c.StaticDir, other.StaticDir)
	c.TmpDir = mergeStrings(c.TmpDir, other.TmpDir)
	c.VolumePath = mergeStrings(c.VolumePath, other.VolumePath)

	// string map of slices
	c.OCIRuntimes = mergeStringMaps(c.OCIRuntimes, other.OCIRuntimes)

	// string slices
	c.AdditionalDevices = mergeStringSlices(c.AdditionalDevices, other.AdditionalDevices)
	c.DefaultCapabilities = mergeStringSlices(c.DefaultCapabilities, other.DefaultCapabilities)
	c.DefaultSysctls = mergeStringSlices(c.DefaultSysctls, other.DefaultSysctls)
	c.DefaultUlimits = mergeStringSlices(c.DefaultUlimits, other.DefaultUlimits)
	c.PluginDirs = mergeStringSlices(c.PluginDirs, other.PluginDirs)
	c.Env = mergeStringSlices(c.Env, other.Env)
	c.ConmonPath = mergeStringSlices(c.ConmonPath, other.ConmonPath)
	c.HooksDir = mergeStringSlices(c.HooksDir, other.HooksDir)
	c.HTTPProxy = mergeStringSlices(c.HTTPProxy, other.HTTPProxy)
	c.RuntimePath = mergeStringSlices(c.RuntimePath, other.RuntimePath)
	c.RuntimeSupportsJSON = mergeStringSlices(c.RuntimeSupportsJSON, other.RuntimeSupportsJSON)
	c.RuntimeSupportsNoCgroups = mergeStringSlices(c.RuntimeSupportsNoCgroups, other.RuntimeSupportsNoCgroups)

	// int64s
	c.LogSizeMax = mergeInt64s(c.LogSizeMax, other.LogSizeMax)
	c.PidsLimit = mergeInt64s(c.PidsLimit, other.PidsLimit)

	// uint32s
	c.NumLocks = mergeUint32s(c.NumLocks, other.NumLocks)

	// bools
	c.EnableLabeling = mergeBools(c.EnableLabeling, other.EnableLabeling)
	c.EnablePortReservation = mergeBools(c.EnablePortReservation, other.EnablePortReservation)
	c.Init = mergeBools(c.Init, other.Init)
	c.NoPivotRoot = mergeBools(c.NoPivotRoot, other.NoPivotRoot)
	c.SDNotify = mergeBools(c.SDNotify, other.SDNotify)

	// state type
	if c.StateType == InvalidStateStore {
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
