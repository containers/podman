package configfile

import (
	"errors"
	"io"
	"io/fs"
	"iter"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/sirupsen/logrus"
)

const _configPathName = "containers"

var (
	// systemConfigPath is the location for the default config files shipped by the distro/vendor.
	//
	// This can be overridden at build time with the following go linker flag:
	// -ldflags '-X go.podman.io/storage/pkg/configfile.systemConfigPath=$your_path'
	systemConfigPath = builtinSystemConfigPath

	// adminOverrideConfigPath is the location for admin local override config files.
	//
	// This can be overridden at build time with the following go linker flag:
	// -ldflags '-X go.podman.io/storage/pkg/configfile.adminOverrideConfigPath=$your_path'
	adminOverrideConfigPath = getAdminOverrideConfigPath()
)

type File struct {
	// The name of the config file WITHOUT the extension (i.e. no .conf).
	// Must not be empty and must not contain the path separator.
	Name string

	// Extension is the file extension of the config file, i.e. "conf" or "yaml".
	// Must not be empty and must not contain the path separator.
	Extension string

	// EnvironmentName is the name of environment variable that can be set to specify the override.
	// Optional.
	EnvironmentName string

	// RootForImplicitAbsolutePaths is the path to an alternate root
	// If not "", prefixed to any absolute paths used by default in the package.
	// NOTE: This does NOT affect paths starting by $HOME or environment variables paths.
	RootForImplicitAbsolutePaths string

	// DoNotLoadMainFiles should be set if only the Drop In files should be loaded.
	DoNotLoadMainFiles bool

	// DoNotLoadDropInFiles should be set if only the main files should be loaded.
	DoNotLoadDropInFiles bool

	// DoNotUseExtensionForConfigName makes it so that the extension is only consulted for the drop in
	// file names but not the main config file name search path.
	DoNotUseExtensionForConfigName bool

	// UserId is the id of the user running this. Used to know where to search in the
	// different "rootful" and "rootless" drop in lookup paths.
	UserId int

	// Modules is a list of names of full paths which are loaded after all the other files.
	// Note the modules concept exists only for containers.conf.
	// For compatibility reasons this field is written to with the fully resolved paths
	// of each module as this is what podman expects today.
	Modules []string
}

// Item is a single config file that is being read once at a time and returned by the iterator from [Read].
type Item struct {
	// Reader is the reader from the file content. The Reader is only valid during
	Reader io.Reader
	// Name is the full filepath to the filename being read.
	Name string
}

func getConfName(name, extension string, noExtension bool) string {
	if noExtension {
		return name
	}
	return name + "." + extension
}

// Read parses all config files with the specified options and returns an iterator which returns all files as Item in the right order.
// If an error is returned by the iterator then this must be treated as fatal error and must fail the config file parsing.
// Expected ENOENT errors are already ignored in this function and must not be handled again by callers.
// The given File options must not be nil and populated with valid options.
func Read(conf *File) iter.Seq2[*Item, error] {
	configFileName := getConfName(conf.Name, conf.Extension, conf.DoNotUseExtensionForConfigName)

	// Note this can be empty which is a valid case and should be simply ignored then.
	defaultConfig := systemConfigPath
	if defaultConfig != "" {
		defaultConfig = filepath.Join(defaultConfig, configFileName)
		if conf.RootForImplicitAbsolutePaths != "" {
			defaultConfig = filepath.Join(conf.RootForImplicitAbsolutePaths, defaultConfig)
		}
	}

	// Same here this can be empty.
	overrideConfig := adminOverrideConfigPath
	if overrideConfig != "" {
		overrideConfig = filepath.Join(overrideConfig, configFileName)
		if conf.RootForImplicitAbsolutePaths != "" {
			overrideConfig = filepath.Join(conf.RootForImplicitAbsolutePaths, overrideConfig)
		}
	}

	return func(yield func(*Item, error) bool) {
		shouldLoadMainFile := !conf.DoNotLoadMainFiles
		shouldLoadDropIns := !conf.DoNotLoadDropInFiles

		yieldAndClose := func(f *os.File) bool {
			ok := yield(&Item{
				Reader: f,
				Name:   f.Name(),
			}, nil)
			// Once yield returns always close the file as the consumer should be done with it.
			if err := f.Close(); err != nil {
				if ok {
					// don't yield again if the previous yield returned false
					yield(nil, err)
				}
				return false
			}
			return ok
		}

		if conf.EnvironmentName != "" {
			if path := os.Getenv(conf.EnvironmentName); path != "" {
				f, err := os.Open(path)
				// Do not ignore ErrNotExist here, we want to hard error if users set a wrong path here.
				if err != nil {
					yield(nil, err)
					return
				}
				if !yieldAndClose(f) {
					return
				}
				// Also when the env is set skip the loading of the main and drop in files, modules and _OVERRIDE env are still read though.
				shouldLoadMainFile = false
				shouldLoadDropIns = false
			}
		}

		// userConfig can be empty as well
		userConfig, err := UserConfigPath()
		if err != nil {
			// return error via iterator
			yield(nil, err)
			return
		}
		if userConfig != "" {
			userConfig = filepath.Join(userConfig, configFileName)
		}

		if shouldLoadMainFile {
			for _, path := range []string{userConfig, overrideConfig, defaultConfig} {
				if path == "" {
					continue
				}
				f, err := os.Open(path)
				// only ignore ErrNotExist, all other errors get return to the caller via yield
				if err != nil {
					if errors.Is(err, fs.ErrNotExist) {
						continue
					}
					yield(nil, err)
					return
				}

				if !yieldAndClose(f) {
					return
				}
				// we only read the first file
				break
			}
		}

		if shouldLoadDropIns {
			files, err := readDropIns(defaultConfig, overrideConfig, userConfig, conf.Extension, conf.UserId)
			if err != nil {
				// return error via iterator
				yield(nil, err)
				return
			}
			for _, file := range files {
				f, err := os.Open(file)
				// only ignore ErrNotExist, all other errors get return to the caller via yield
				if err != nil {
					if errors.Is(err, fs.ErrNotExist) {
						continue
					}
					yield(nil, err)
					return
				}

				if !yieldAndClose(f) {
					return
				}
			}
		}

		if len(conf.Modules) > 0 {
			dirs := moduleDirectories(defaultConfig, overrideConfig, userConfig)
			resolvedModules := make([]string, 0, len(conf.Modules))
			for _, module := range conf.Modules {
				f, err := resolveModule(module, dirs)
				if err != nil {
					yield(nil, err)
					return
				}
				resolvedModules = append(resolvedModules, f.Name())
				if !yieldAndClose(f) {
					return
				}
			}
			conf.Modules = resolvedModules
		}

		if conf.EnvironmentName != "" {
			// The _OVERRIDE env must be appended after loading all files, even modules.
			if path := os.Getenv(conf.EnvironmentName + "_OVERRIDE"); path != "" {
				f, err := os.Open(path)
				// Do not ignore ErrNotExist here, we want to hard error if users set a wrong path here.
				if err != nil {
					yield(nil, err)
					return
				}
				if !yieldAndClose(f) {
					return
				}
			}
		}
	}
}

const dropInSuffix = ".d"

func readDropIns(defaultConfig, overrideConfig, userConfig, extension string, uid int) ([]string, error) {
	dropInMap := make(map[string]string)
	paths := make([]string, 0, 7)

	suffix := "." + extension

	if defaultConfig != "" {
		paths = append(paths, getDropInPaths(defaultConfig, suffix, uid)...)
	}
	if overrideConfig != "" {
		paths = append(paths, getDropInPaths(overrideConfig, suffix, uid)...)
	}
	if userConfig != "" {
		// the $HOME config only has one .d path not the rootful/rootless ones.
		paths = append(paths, userConfig+dropInSuffix)
	}

	for _, path := range paths {
		entries, err := os.ReadDir(path)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return nil, err
		}
		for _, entry := range entries {
			if entry.Type().IsRegular() && strings.HasSuffix(entry.Name(), suffix) {
				dropInMap[entry.Name()] = filepath.Join(path, entry.Name())
			}
		}
	}

	sortedNames := slices.Sorted(maps.Keys(dropInMap))
	files := make([]string, 0, len(sortedNames))
	for _, file := range sortedNames {
		files = append(files, dropInMap[file])
	}
	return files, nil
}

func getDropInPaths(mainPath, suffix string, uid int) []string {
	paths := make([]string, 0, 3)
	paths = append(paths, mainPath+dropInSuffix)

	rootless := uid > 0
	var specialName string
	if rootless {
		specialName = "rootless"
	} else {
		specialName = "rootful"
	}
	// insert the name after the main config name but before the extension if it has one.
	mainPath, cut := strings.CutSuffix(mainPath, suffix)
	specialPath := mainPath + "." + specialName
	if cut {
		specialPath += suffix
	}
	specialPath += dropInSuffix
	paths = append(paths, specialPath)
	if rootless {
		paths = append(paths, filepath.Join(specialPath, strconv.Itoa(uid)))
	}
	return paths
}

func moduleDirectories(defaultConfig, overrideConfig, userConfig string) []string {
	const moduleSuffix = ".modules"
	modules := make([]string, 0, 3)
	if userConfig != "" {
		modules = append(modules, userConfig+moduleSuffix)
	}
	if overrideConfig != "" {
		modules = append(modules, overrideConfig+moduleSuffix)
	}
	if defaultConfig != "" {
		modules = append(modules, defaultConfig+moduleSuffix)
	}
	return modules
}

// Resolve the specified path to a module.
func resolveModule(path string, dirs []string) (*os.File, error) {
	if filepath.IsAbs(path) {
		return os.Open(path)
	}

	// Collect all errors to avoid suppressing important errors (e.g.,
	// permission errors).
	var multiErr error
	for _, d := range dirs {
		candidate := filepath.Join(d, path)

		f, err := os.Open(candidate)
		if err == nil {
			return f, nil
		}
		multiErr = errors.Join(multiErr, err)
	}
	return nil, multiErr
}

// ParseTOML parses the given config according to the rules in by [Read].
// Note the given configStruct must be a pointer to a struct that describes
// the toml config fields and is modified in place.
// If an error is returned the struct should not be used.
func ParseTOML(configStruct any, conf *File) error {
	for item, err := range Read(conf) {
		if err != nil {
			return err
		}
		meta, err := toml.NewDecoder(item.Reader).Decode(configStruct)
		if err != nil {
			return err
		}
		keys := meta.Undecoded()
		if len(keys) > 0 {
			logrus.Debugf("Failed to decode the keys %q from %q", keys, item.Name)
		}
	}
	return nil
}
