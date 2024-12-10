package main

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/containers/podman/v5/pkg/systemd/parser"
	"github.com/containers/podman/v5/pkg/systemd/quadlet"
	"github.com/containers/podman/v5/version/rawversion"
)

// This commandline app is the systemd generator (system and user,
// decided by the name of the binary).

// Generators run at very early startup, so must work in a very
// limited environment (e.g. no /var, /home, or syslog).  See:
// https://www.freedesktop.org/software/systemd/man/systemd.generator.html#Notes%20about%20writing%20generators
// for more details.

var (
	verboseFlag bool // True if -v passed
	noKmsgFlag  bool
	isUserFlag  bool // True if run as quadlet-user-generator executable
	dryRunFlag  bool // True if -dryrun is used
	versionFlag bool // True if -version is used
)

var (
	// data saved between logToKmsg calls
	noKmsg   = false
	kmsgFile *os.File
)

var (
	void struct{}
	// Key: Extension
	// Value: Processing order for resource naming dependencies
	supportedExtensions = map[string]int{
		".container": 4,
		".volume":    2,
		".kube":      4,
		".network":   2,
		".image":     1,
		".build":     3,
		".pod":       5,
	}
)

// We log directly to /dev/kmsg, because that is the only way to get information out
// of the generator into the system logs.
func logToKmsg(s string) bool {
	if noKmsg {
		return false
	}

	if kmsgFile == nil {
		f, err := os.OpenFile("/dev/kmsg", os.O_WRONLY, 0644)
		if err != nil {
			noKmsg = true
			return false
		}
		kmsgFile = f
	}

	if _, err := kmsgFile.WriteString(s); err != nil {
		kmsgFile.Close()
		kmsgFile = nil
		return false
	}

	return true
}

func Logf(format string, a ...interface{}) {
	s := fmt.Sprintf(format, a...)
	line := fmt.Sprintf("quadlet-generator[%d]: %s", os.Getpid(), s)

	if !logToKmsg(line) || dryRunFlag {
		fmt.Fprintf(os.Stderr, "%s\n", line)
		os.Stderr.Sync()
	}
}

var debugEnabled = false

func enableDebug() {
	debugEnabled = true
}

func Debugf(format string, a ...interface{}) {
	if debugEnabled {
		Logf(format, a...)
	}
}

type searchPaths struct {
	sorted []string
	// map to store paths so we can quickly check if we saw them already and not loop in case of symlinks
	visitedDirs map[string]struct{}
}

func newSearchPaths() *searchPaths {
	return &searchPaths{
		sorted:      make([]string, 0),
		visitedDirs: make(map[string]struct{}, 0),
	}
}

func (s *searchPaths) Add(path string) {
	s.sorted = append(s.sorted, path)
	s.visitedDirs[path] = struct{}{}
}

func (s *searchPaths) Visited(path string) bool {
	_, visited := s.visitedDirs[path]
	return visited
}

// This returns the directories where we read quadlet .container and .volumes from
// For system generators these are in /usr/share/containers/systemd (for distro files)
// and /etc/containers/systemd (for sysadmin files).
// For user generators these can live in $XDG_RUNTIME_DIR/containers/systemd, /etc/containers/systemd/users, /etc/containers/systemd/users/$UID, and $XDG_CONFIG_HOME/containers/systemd
func getUnitDirs(rootless bool) []string {
	paths := newSearchPaths()

	// Allow overriding source dir, this is mainly for the CI tests
	if getDirsFromEnv(paths) {
		return paths.sorted
	}

	resolvedUnitDirAdminUser := resolveUnitDirAdminUser()
	userLevelFilter := getUserLevelFilter(resolvedUnitDirAdminUser)

	if rootless {
		systemUserDirLevel := len(strings.Split(resolvedUnitDirAdminUser, string(os.PathSeparator)))
		nonNumericFilter := getNonNumericFilter(resolvedUnitDirAdminUser, systemUserDirLevel)
		getRootlessDirs(paths, nonNumericFilter, userLevelFilter)
	} else {
		getRootDirs(paths, userLevelFilter)
	}
	return paths.sorted
}

func getDirsFromEnv(paths *searchPaths) bool {
	unitDirsEnv := os.Getenv("QUADLET_UNIT_DIRS")
	if len(unitDirsEnv) == 0 {
		return false
	}

	for _, eachUnitDir := range strings.Split(unitDirsEnv, ":") {
		if !filepath.IsAbs(eachUnitDir) {
			Logf("%s not a valid file path", eachUnitDir)
			break
		}
		appendSubPaths(paths, eachUnitDir, false, nil)
	}
	return true
}

func getRootlessDirs(paths *searchPaths, nonNumericFilter, userLevelFilter func(string, bool) bool) {
	runtimeDir, found := os.LookupEnv("XDG_RUNTIME_DIR")
	if found {
		appendSubPaths(paths, path.Join(runtimeDir, "containers/systemd"), false, nil)
	}

	configDir, err := os.UserConfigDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v", err)
		return
	}
	appendSubPaths(paths, path.Join(configDir, "containers/systemd"), false, nil)

	u, err := user.Current()
	if err == nil {
		appendSubPaths(paths, filepath.Join(quadlet.UnitDirAdmin, "users"), true, nonNumericFilter)
		appendSubPaths(paths, filepath.Join(quadlet.UnitDirAdmin, "users", u.Uid), true, userLevelFilter)
	} else {
		fmt.Fprintf(os.Stderr, "Warning: %v", err)
		// Add the base directory even if the UID was not found
		paths.Add(filepath.Join(quadlet.UnitDirAdmin, "users"))
	}
}

func getRootDirs(paths *searchPaths, userLevelFilter func(string, bool) bool) {
	appendSubPaths(paths, quadlet.UnitDirTemp, false, userLevelFilter)
	appendSubPaths(paths, quadlet.UnitDirAdmin, false, userLevelFilter)
	appendSubPaths(paths, quadlet.UnitDirDistro, false, nil)
}

func resolveUnitDirAdminUser() string {
	unitDirAdminUser := filepath.Join(quadlet.UnitDirAdmin, "users")
	var err error
	var resolvedUnitDirAdminUser string
	if resolvedUnitDirAdminUser, err = filepath.EvalSymlinks(unitDirAdminUser); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			Debugf("Error occurred resolving path %q: %s", unitDirAdminUser, err)
		}
		resolvedUnitDirAdminUser = unitDirAdminUser
	}
	return resolvedUnitDirAdminUser
}

func appendSubPaths(paths *searchPaths, path string, isUserFlag bool, filterPtr func(string, bool) bool) {
	resolvedPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			Debugf("Error occurred resolving path %q: %s", path, err)
		}
		// Despite the failure add the path to the list for logging purposes
		// This is the equivalent of adding the path when info==nil below
		paths.Add(path)
		return
	}

	if skipPath(paths, resolvedPath, isUserFlag, filterPtr) {
		return
	}

	// Add the current directory
	paths.Add(resolvedPath)

	// Read the contents of the directory
	entries, err := os.ReadDir(resolvedPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			Debugf("Error occurred walking sub directories %q: %s", path, err)
		}
		return
	}

	// Recursively run through the contents of the directory
	for _, entry := range entries {
		fullPath := filepath.Join(resolvedPath, entry.Name())
		appendSubPaths(paths, fullPath, isUserFlag, filterPtr)
	}
}

func skipPath(paths *searchPaths, path string, isUserFlag bool, filterPtr func(string, bool) bool) bool {
	// If the path is already in the map no need to read it again
	if paths.Visited(path) {
		return true
	}

	// Don't traverse drop-in directories
	if strings.HasSuffix(path, ".d") {
		return true
	}

	// Check if the directory should be filtered out
	if filterPtr != nil && !filterPtr(path, isUserFlag) {
		return true
	}

	stat, err := os.Stat(path)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			Debugf("Error occurred resolving path %q: %s", path, err)
		}
		return true
	}

	// Not a directory nothing to add
	return !stat.IsDir()
}

func getNonNumericFilter(resolvedUnitDirAdminUser string, systemUserDirLevel int) func(string, bool) bool {
	return func(path string, isUserFlag bool) bool {
		// when running in rootless, recursive walk directories that are non numeric
		// ignore sub dirs under the `users` directory which correspond to a user id
		if strings.HasPrefix(path, resolvedUnitDirAdminUser) {
			listDirUserPathLevels := strings.Split(path, string(os.PathSeparator))
			// Make sure to add the base directory
			if len(listDirUserPathLevels) == systemUserDirLevel {
				return true
			}
			if len(listDirUserPathLevels) > systemUserDirLevel {
				if !(regexp.MustCompile(`^[0-9]*$`).MatchString(listDirUserPathLevels[systemUserDirLevel])) {
					return true
				}
			}
		} else {
			return true
		}
		return false
	}
}

func getUserLevelFilter(resolvedUnitDirAdminUser string) func(string, bool) bool {
	return func(_path string, isUserFlag bool) bool {
		// if quadlet generator is run rootless, do not recurse other user sub dirs
		// if quadlet generator is run as root, ignore users sub dirs
		if strings.HasPrefix(_path, resolvedUnitDirAdminUser) {
			if isUserFlag {
				return true
			}
		} else {
			return true
		}
		return false
	}
}

func isExtSupported(filename string) bool {
	ext := filepath.Ext(filename)
	_, ok := supportedExtensions[ext]
	return ok
}

var seen = make(map[string]struct{})

func loadUnitsFromDir(sourcePath string) ([]*parser.UnitFile, error) {
	var prevError error
	files, err := os.ReadDir(sourcePath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		return []*parser.UnitFile{}, nil
	}

	var units []*parser.UnitFile

	for _, file := range files {
		name := file.Name()
		if _, ok := seen[name]; !ok && isExtSupported(name) {
			path := path.Join(sourcePath, name)

			Debugf("Loading source unit file %s", path)

			if f, err := parser.ParseUnitFile(path); err != nil {
				err = fmt.Errorf("error loading %q, %w", path, err)
				if prevError == nil {
					prevError = err
				} else {
					prevError = fmt.Errorf("%s\n%s", prevError, err)
				}
			} else {
				seen[name] = void
				units = append(units, f)
			}
		}
	}

	return units, prevError
}

func loadUnitDropins(unit *parser.UnitFile, sourcePaths []string) error {
	var prevError error
	reportError := func(err error) {
		if prevError != nil {
			err = fmt.Errorf("%s\n%s", prevError, err)
		}
		prevError = err
	}

	dropinDirs := []string{}
	unitDropinPaths := unit.GetUnitDropinPaths()

	for _, sourcePath := range sourcePaths {
		for _, dropinPath := range unitDropinPaths {
			dropinDirs = append(dropinDirs, path.Join(sourcePath, dropinPath))
		}
	}

	var dropinPaths = make(map[string]string)
	for _, dropinDir := range dropinDirs {
		dropinFiles, err := os.ReadDir(dropinDir)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				reportError(fmt.Errorf("error reading directory %q, %w", dropinDir, err))
			}

			continue
		}

		for _, dropinFile := range dropinFiles {
			dropinName := dropinFile.Name()
			if filepath.Ext(dropinName) != ".conf" {
				continue // Only *.conf supported
			}

			if _, ok := dropinPaths[dropinName]; ok {
				continue // We already saw this name
			}

			dropinPaths[dropinName] = path.Join(dropinDir, dropinName)
		}
	}

	dropinFiles := make([]string, len(dropinPaths))
	i := 0
	for k := range dropinPaths {
		dropinFiles[i] = k
		i++
	}

	// Merge in alpha-numerical order
	sort.Strings(dropinFiles)

	for _, dropinFile := range dropinFiles {
		dropinPath := dropinPaths[dropinFile]

		Debugf("Loading source drop-in file %s", dropinPath)

		if f, err := parser.ParseUnitFile(dropinPath); err != nil {
			reportError(fmt.Errorf("error loading %q, %w", dropinPath, err))
		} else {
			unit.Merge(f)
		}
	}

	return prevError
}

func generateServiceFile(service *parser.UnitFile) error {
	Debugf("writing %q", service.Path)

	service.PrependComment("",
		fmt.Sprintf("Automatically generated by %s", os.Args[0]),
		"")

	f, err := os.Create(service.Path)
	if err != nil {
		return err
	}

	defer f.Close()

	err = service.Write(f)
	if err != nil {
		return err
	}

	err = f.Sync()
	if err != nil {
		return err
	}

	return nil
}

// This parses the `Install` group of the unit file and creates the required
// symlinks to get systemd to start the newly generated file as needed.
// In a traditional setup this is done by "systemctl enable", but that doesn't
// work for auto-generated files like these.
func enableServiceFile(outputPath string, service *parser.UnitFile) {
	symlinks := make([]string, 0)

	aliases := service.LookupAllStrv(quadlet.InstallGroup, "Alias")
	for _, alias := range aliases {
		symlinks = append(symlinks, filepath.Clean(alias))
	}

	serviceFilename := service.Filename
	templateBase, templateInstance, isTemplate := service.GetTemplateParts()

	// For non-instantiated template service we only support installs if a
	// DefaultInstance is given. Otherwise we ignore the Install group, but
	// it is still useful when instantiating the unit via a symlink.
	if isTemplate && templateInstance == "" {
		if defaultInstance, ok := service.Lookup(quadlet.InstallGroup, "DefaultInstance"); ok {
			serviceFilename = templateBase + "@" + defaultInstance + filepath.Ext(serviceFilename)
		} else {
			serviceFilename = ""
		}
	}

	if serviceFilename != "" {
		wantedBy := service.LookupAllStrv(quadlet.InstallGroup, "WantedBy")
		for _, wantedByUnit := range wantedBy {
			// Only allow filenames, not paths
			if !strings.Contains(wantedByUnit, "/") {
				symlinks = append(symlinks, fmt.Sprintf("%s.wants/%s", wantedByUnit, serviceFilename))
			}
		}

		requiredBy := service.LookupAllStrv(quadlet.InstallGroup, "RequiredBy")
		for _, requiredByUnit := range requiredBy {
			// Only allow filenames, not paths
			if !strings.Contains(requiredByUnit, "/") {
				symlinks = append(symlinks, fmt.Sprintf("%s.requires/%s", requiredByUnit, serviceFilename))
			}
		}
	}

	for _, symlinkRel := range symlinks {
		target, err := filepath.Rel(path.Dir(symlinkRel), service.Filename)
		if err != nil {
			Logf("Can't create symlink %s: %s", symlinkRel, err)
			continue
		}
		symlinkPath := path.Join(outputPath, symlinkRel)

		symlinkDir := path.Dir(symlinkPath)
		err = os.MkdirAll(symlinkDir, os.ModePerm)
		if err != nil {
			Logf("Can't create dir %s: %s", symlinkDir, err)
			continue
		}

		Debugf("Creating symlink %s -> %s", symlinkPath, target)
		_ = os.Remove(symlinkPath) // overwrite existing symlinks
		err = os.Symlink(target, symlinkPath)
		if err != nil {
			Logf("Failed creating symlink %s: %s", symlinkPath, err)
		}
	}
}

func isImageID(imageName string) bool {
	// All sha25:... names are assumed by podman to be fully specified
	if strings.HasPrefix(imageName, "sha256:") {
		return true
	}

	// However, podman also accepts image ids as pure hex strings,
	// but only those of length 64 are unambiguous image ids
	if len(imageName) != 64 {
		return false
	}

	for _, c := range imageName {
		if !unicode.Is(unicode.Hex_Digit, c) {
			return false
		}
	}

	return true
}

func isUnambiguousName(imageName string) bool {
	// Fully specified image ids are unambiguous
	if isImageID(imageName) {
		return true
	}

	// Otherwise we require a fully qualified name
	firstSlash := strings.Index(imageName, "/")
	if firstSlash == -1 {
		// No domain or path, not fully qualified
		return false
	}

	// What is before the first slash can be a domain or a path
	domain := imageName[:firstSlash]

	// If its a domain (has dot or port or is "localhost") it is considered fq
	if strings.ContainsAny(domain, ".:") || domain == "localhost" {
		return true
	}

	return false
}

// warns if input is an ambiguous name, i.e. a partial image id or a short
// name (i.e. is missing a registry)
//
// Examples:
//   - short names: "image:tag", "library/fedora"
//   - fully qualified names: "quay.io/image", "localhost/image:tag",
//     "server.org:5000/lib/image", "sha256:..."
//
// We implement a simple version of this from scratch here to avoid
// a huge dependency in the generator just for a warning.
func warnIfAmbiguousName(unit *parser.UnitFile, group string) {
	imageName, ok := unit.Lookup(group, quadlet.KeyImage)
	if !ok {
		return
	}
	if strings.HasSuffix(imageName, ".build") || strings.HasSuffix(imageName, ".image") {
		return
	}
	if !isUnambiguousName(imageName) {
		Logf("Warning: %s specifies the image \"%s\" which not a fully qualified image name. This is not ideal for performance and security reasons. See the podman-pull manpage discussion of short-name-aliases.conf for details.", unit.Filename, imageName)
	}
}

func generateUnitsInfoMap(units []*parser.UnitFile) map[string]*quadlet.UnitInfo {
	unitsInfoMap := make(map[string]*quadlet.UnitInfo)
	for _, unit := range units {
		var serviceName string
		var containers []string
		var resourceName string

		switch {
		case strings.HasSuffix(unit.Filename, ".container"):
			serviceName = quadlet.GetContainerServiceName(unit)
			// Prefill resouceNames for .container files. This solves network reusing.
			resourceName = quadlet.GetContainerResourceName(unit)
		case strings.HasSuffix(unit.Filename, ".volume"):
			serviceName = quadlet.GetVolumeServiceName(unit)
		case strings.HasSuffix(unit.Filename, ".kube"):
			serviceName = quadlet.GetKubeServiceName(unit)
		case strings.HasSuffix(unit.Filename, ".network"):
			serviceName = quadlet.GetNetworkServiceName(unit)
		case strings.HasSuffix(unit.Filename, ".image"):
			serviceName = quadlet.GetImageServiceName(unit)
		case strings.HasSuffix(unit.Filename, ".build"):
			serviceName = quadlet.GetBuildServiceName(unit)
			// Prefill resouceNames for .build files. This is significantly less complex than
			// pre-computing all resourceNames for all Quadlet types (which is rather complex for a few
			// types), but still breaks the dependency cycle between .volume and .build ([Volume] can
			// have Image=some.build, and [Build] can have Volume=some.volume:/some-volume)
			resourceName = quadlet.GetBuiltImageName(unit)
		case strings.HasSuffix(unit.Filename, ".pod"):
			serviceName = quadlet.GetPodServiceName(unit)
			containers = make([]string, 0)
		default:
			Logf("Unsupported file type %q", unit.Filename)
			continue
		}

		unitsInfoMap[unit.Filename] = &quadlet.UnitInfo{
			ServiceName:       serviceName,
			ContainersToStart: containers,
			ResourceName:      resourceName,
		}
	}

	return unitsInfoMap
}

func main() {
	if err := process(); err != nil {
		Logf("%s", err.Error())
		os.Exit(1)
	}
	os.Exit(0)
}

func process() error {
	var prevError error

	prgname := path.Base(os.Args[0])
	isUserFlag = strings.Contains(prgname, "user")

	flag.Parse()

	if versionFlag {
		fmt.Printf("%s\n", rawversion.RawVersion)
		return prevError
	}

	if verboseFlag || dryRunFlag {
		enableDebug()
	}

	if noKmsgFlag || dryRunFlag {
		noKmsg = true
	}

	reportError := func(err error) {
		if prevError != nil {
			err = fmt.Errorf("%s\n%s", prevError, err)
		}
		prevError = err
	}

	if !dryRunFlag && flag.NArg() < 1 {
		reportError(errors.New("missing output directory argument"))
		return prevError
	}

	var outputPath string

	if !dryRunFlag {
		outputPath = flag.Arg(0)

		Debugf("Starting quadlet-generator, output to: %s", outputPath)
	}

	sourcePathsMap := getUnitDirs(isUserFlag)

	var units []*parser.UnitFile
	for _, d := range sourcePathsMap {
		if result, err := loadUnitsFromDir(d); err != nil {
			reportError(err)
		} else {
			units = append(units, result...)
		}
	}

	if len(units) == 0 {
		// containers/podman/issues/17374: exit cleanly but log that we
		// had nothing to do
		Debugf("No files parsed from %s", sourcePathsMap)
		return prevError
	}

	for _, unit := range units {
		if err := loadUnitDropins(unit, sourcePathsMap); err != nil {
			reportError(err)
		}
	}

	if !dryRunFlag {
		err := os.MkdirAll(outputPath, os.ModePerm)
		if err != nil {
			reportError(err)
			return prevError
		}
	}

	// Sort unit files according to potential inter-dependencies, with Volume and Network units
	// taking precedence over all others.
	sort.Slice(units, func(i, j int) bool {
		getOrder := func(i int) int {
			ext := filepath.Ext(units[i].Filename)
			order, ok := supportedExtensions[ext]
			if !ok {
				return 0
			}
			return order
		}
		return getOrder(i) < getOrder(j)
	})

	// Generate the PodsInfoMap to allow containers to link to their pods and add themselves to the pod's containers list
	unitsInfoMap := generateUnitsInfoMap(units)

	for _, unit := range units {
		var service *parser.UnitFile
		var err error

		switch {
		case strings.HasSuffix(unit.Filename, ".container"):
			warnIfAmbiguousName(unit, quadlet.ContainerGroup)
			service, err = quadlet.ConvertContainer(unit, isUserFlag, unitsInfoMap)
		case strings.HasSuffix(unit.Filename, ".volume"):
			warnIfAmbiguousName(unit, quadlet.VolumeGroup)
			service, err = quadlet.ConvertVolume(unit, unit.Filename, unitsInfoMap, isUserFlag)
		case strings.HasSuffix(unit.Filename, ".kube"):
			service, err = quadlet.ConvertKube(unit, unitsInfoMap, isUserFlag)
		case strings.HasSuffix(unit.Filename, ".network"):
			service, err = quadlet.ConvertNetwork(unit, unit.Filename, unitsInfoMap, isUserFlag)
		case strings.HasSuffix(unit.Filename, ".image"):
			warnIfAmbiguousName(unit, quadlet.ImageGroup)
			service, err = quadlet.ConvertImage(unit, unitsInfoMap, isUserFlag)
		case strings.HasSuffix(unit.Filename, ".build"):
			service, err = quadlet.ConvertBuild(unit, unitsInfoMap, isUserFlag)
		case strings.HasSuffix(unit.Filename, ".pod"):
			service, err = quadlet.ConvertPod(unit, unit.Filename, unitsInfoMap, isUserFlag)
		default:
			Logf("Unsupported file type %q", unit.Filename)
			continue
		}

		if err != nil {
			reportError(fmt.Errorf("converting %q: %w", unit.Filename, err))
			continue
		}

		service.Path = path.Join(outputPath, service.Filename)

		if dryRunFlag {
			data, err := service.ToString()
			if err != nil {
				reportError(fmt.Errorf("parsing %s: %w", service.Path, err))
				continue
			}
			fmt.Printf("---%s---\n%s\n", service.Path, data)
			continue
		}
		if err := generateServiceFile(service); err != nil {
			reportError(fmt.Errorf("generating service file %s: %w", service.Path, err))
		}
		enableServiceFile(outputPath, service)
	}
	return prevError
}

func init() {
	flag.BoolVar(&verboseFlag, "v", false, "Print debug information")
	flag.BoolVar(&noKmsgFlag, "no-kmsg-log", false, "Don't log to kmsg")
	flag.BoolVar(&isUserFlag, "user", false, "Run as systemd user")
	flag.BoolVar(&dryRunFlag, "dryrun", false, "Run in dryrun mode printing debug information")
	flag.BoolVar(&versionFlag, "version", false, "Print version information and exit")
}
