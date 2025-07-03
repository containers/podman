//go:build !remote

package abi

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/containers/podman/v5/pkg/rootless"
	"github.com/containers/podman/v5/pkg/systemd"
	"github.com/containers/podman/v5/pkg/systemd/parser"
	systemdquadlet "github.com/containers/podman/v5/pkg/systemd/quadlet"
	"github.com/containers/podman/v5/pkg/util"
	"github.com/containers/storage/pkg/fileutils"
	"github.com/sirupsen/logrus"
)

// deleteAsset reads .<name>.asset, deletes listed files, then deletes the asset file
func deleteAsset(name string) error {
	assetFilename := fmt.Sprintf(".%s.asset", name)

	installDir := systemdquadlet.GetInstallUnitDirPath(rootless.IsRootless())
	assetFilePath := filepath.Join(installDir, assetFilename)
	result, err := getAssetListFromFile(assetFilePath)
	if err != nil {
		return fmt.Errorf("unable to get list of files to delete: %w", err)
	}
	for _, entry := range result {
		err = os.Remove(filepath.Join(installDir, entry))
		if err != nil {
			return fmt.Errorf("unable to delete %s: %w", filepath.Join(installDir, entry), err)
		}
	}
	err = os.Remove(assetFilePath)
	if err != nil {
		return fmt.Errorf("unable to delete %s: %w", assetFilePath, err)
	}
	return err
}

// readLinesFromFile reads lines from a file and calls the provided callback for each non-empty line.
// It handles file opening, scanning, trimming whitespace, and error checking.
func readLinesFromFile(filePath string, callback func(line string) error) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("could not open file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if err := callback(line); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading file: %w", err)
	}
	return nil
}

func getAssetListFromFile(path string) ([]string, error) {
	var result []string
	err := readLinesFromFile(path, func(line string) error {
		if strings.Contains(line, "/") {
			logrus.Warnf("Unexpected file line %q, expected name but got path components", line)
			return nil
		}
		result = append(result, line)
		return nil
	})
	if err != nil {
		return result, fmt.Errorf("error reading asset file: %w", err)
	}
	return result, nil
}

// Install one or more Quadlet files
func (ic *ContainerEngine) QuadletInstall(ctx context.Context, pathsOrURLs []string, options entities.QuadletInstallOptions) (*entities.QuadletInstallReport, error) {
	// Is systemd available to the current user?
	// We cannot proceed if not.
	conn, err := systemd.ConnectToDBUS()
	if err != nil {
		return nil, fmt.Errorf("connecting to systemd dbus: %w", err)
	}
	defer conn.Close()
	cfg, err := config.Default()
	if err != nil {
		return nil, fmt.Errorf("unable to load default config: %w", err)
	}

	// Is Quadlet installed? No point if not.
	quadletPath, err := cfg.FindHelperBinary("quadlet", true)
	if err != nil {
		return nil, fmt.Errorf("cannot stat Quadlet generator, Quadlet may not be installed: %w", err)
	}
	if quadletPath == "" {
		return nil, fmt.Errorf("unable to find `quadlet` binary, Quadlet may not be installed")
	}
	quadletStat, err := os.Stat(quadletPath)
	if err != nil {
		return nil, fmt.Errorf("cannot stat Quadlet generator, Quadlet may not be installed: %w", err)
	}

	if !quadletStat.Mode().IsRegular() || quadletStat.Mode()&0100 == 0 {
		return nil, fmt.Errorf("no valid Quadlet binary installed to %q, unable to use Quadlet", quadletPath)
	}

	installDir := systemdquadlet.GetInstallUnitDirPath(rootless.IsRootless())
	logrus.Debugf("Going to install Quadlet to directory %s", installDir)

	if err := os.MkdirAll(installDir, 0755); err != nil {
		return nil, fmt.Errorf("unable to create Quadlet install path %s: %w", installDir, err)
	}

	installReport := entities.QuadletInstallReport{
		InstalledQuadlets: make(map[string]string),
		QuadletErrors:     make(map[string]error),
	}

	assetFile := ""
	paths := pathsOrURLs
	if len(pathsOrURLs) > 0 && !strings.HasPrefix(pathsOrURLs[0], "http://") && !strings.HasPrefix(pathsOrURLs[0], "https://") {
		// Check if first path is dir, this is an APP
		info, err := os.Stat(pathsOrURLs[0])
		if err != nil {
			return nil, fmt.Errorf("unable to stat Quadlet path %s: %w", pathsOrURLs[0], err)
		}
		if info.IsDir() {
			// If it's a directory, then read all files and add it to paths
			entries, err := os.ReadDir(pathsOrURLs[0])
			if err != nil {
				return nil, fmt.Errorf("unable to read Quadlet dir %s: %w", pathsOrURLs[0], err)
			}
			redoPaths := make([]string, 0, len(entries))
			for _, entry := range entries {
				redoPaths = append(redoPaths, filepath.Join(pathsOrURLs[0], entry.Name()))
			}
			redoPaths = append(redoPaths, pathsOrURLs[1:]...)
			paths = redoPaths
			// treat all file in this session as part of one app.
			assetFile = "." + filepath.Base(pathsOrURLs[0]) + ".app"
		}
	}

	// Loop over all given URLs
	for _, toInstall := range paths {
		validateQuadletFile := false
		if assetFile == "" {
			assetFile = "." + filepath.Base(toInstall) + ".asset"
			validateQuadletFile = true
		}
		switch {
		case strings.HasPrefix(toInstall, "http://") || strings.HasPrefix(toInstall, "https://"):
			r, err := http.Get(toInstall)
			if err != nil {
				installReport.QuadletErrors[toInstall] = fmt.Errorf("unable to download URL %s: %w", toInstall, err)
				continue
			}
			defer r.Body.Close()
			quadletFileName, err := getFileName(r, toInstall)
			if err != nil {
				installReport.QuadletErrors[toInstall] = fmt.Errorf("unable to get file name from url %s: %w", toInstall, err)
				continue
			}
			// It's a URL. Pull to temporary file.
			tmpFile, err := os.CreateTemp("", quadletFileName)
			if err != nil {
				installReport.QuadletErrors[toInstall] = fmt.Errorf("unable to create temporary file to download URL %s: %w", toInstall, err)
				continue
			}
			defer func() {
				tmpFile.Close()
				if err := os.Remove(tmpFile.Name()); err != nil {
					logrus.Errorf("unable to remove temporary file %q: %v", tmpFile.Name(), err)
				}
			}()
			_, err = io.Copy(tmpFile, r.Body)
			if err != nil {
				installReport.QuadletErrors[toInstall] = fmt.Errorf("populating temporary file: %w", err)
				continue
			}
			installedPath, err := ic.installQuadlet(ctx, tmpFile.Name(), quadletFileName, installDir, assetFile, validateQuadletFile)
			if err != nil {
				installReport.QuadletErrors[toInstall] = err
				continue
			}
			installReport.InstalledQuadlets[toInstall] = installedPath
		default:
			err := fileutils.Exists(toInstall)
			if err != nil {
				installReport.QuadletErrors[toInstall] = err
				continue
			}
			// If toInstall is a single file, execute the original logic
			installedPath, err := ic.installQuadlet(ctx, toInstall, "", installDir, assetFile, validateQuadletFile)
			if err != nil {
				installReport.QuadletErrors[toInstall] = err
				continue
			}
			installReport.InstalledQuadlets[toInstall] = installedPath
		}
	}

	// TODO: Should we still do this if the above validation errored?
	if options.ReloadSystemd {
		if err := conn.ReloadContext(ctx); err != nil {
			return &installReport, fmt.Errorf("reloading systemd: %w", err)
		}
	}

	return &installReport, nil
}

// Extracts file name from Content-Disposition or URL
func getFileName(resp *http.Response, fileURL string) (string, error) {
	// Try to get filename from Content-Disposition header
	cd := resp.Header.Get("Content-Disposition")
	if cd != "" {
		const prefix = "filename="
		if idx := strings.Index(cd, prefix); idx != -1 {
			filename := cd[idx+len(prefix):]
			filename = strings.Trim(filename, "\"'")
			return filename, nil
		}
	}

	// Fallback: get filename from URL path
	u, err := url.Parse(fileURL)
	if err != nil {
		return "", err
	}
	return path.Base(u.Path), nil
}

// Install a single Quadlet from a path on local disk to the given install directory.
// Perform some minimal validation, but not much.
// We can't know about a lot of problems without running the Quadlet binary, which we
// only want to do once.
func (ic *ContainerEngine) installQuadlet(_ context.Context, path, destName, installDir, assetFile string, isQuadletFile bool) (string, error) {
	// First, validate that the source path exists and is a file
	stat, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("quadlet to install %q does not exist or cannot be read: %w", path, err)
	}
	if stat.IsDir() {
		return "", fmt.Errorf("quadlet to install %q is not a file", path)
	}

	finalPath := filepath.Join(installDir, filepath.Base(filepath.Clean(path)))
	if destName != "" {
		finalPath = filepath.Join(installDir, destName)
	}

	// Validate extension is valid
	if isQuadletFile && !systemdquadlet.IsExtSupported(finalPath) {
		return "", fmt.Errorf("%q is not a supported Quadlet file type", filepath.Ext(finalPath))
	}

	file, err := os.OpenFile(finalPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		if errors.Is(err, fs.ErrExist) {
			return "", fmt.Errorf("a Quadlet with name %s already exists, refusing to overwrite", filepath.Base(finalPath))
		}
		return "", fmt.Errorf("unable to open file %s: %w", filepath.Base(finalPath), err)
	}
	defer file.Close()

	// Move the file in
	srcFile, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("unable to open file: %w", err)
	}
	defer srcFile.Close()

	err = fileutils.ReflinkOrCopy(srcFile, file)
	if err != nil {
		return "", fmt.Errorf("unable to copy file from %s to %s: %w", path, finalPath, err)
	}

	// When we install files using this function, caller of this function can turn off `validateQuadletFile`
	// when they are installing `non-quadlet` files.
	if !isQuadletFile {
		err := appendStringToFile(filepath.Join(installDir, assetFile), filepath.Base(filepath.Clean(path)))
		if err != nil {
			return "", fmt.Errorf("error while writing non-quadlet filename: %w", err)
		}
	}
	return finalPath, nil
}

// appendStringToFile appends the given text to the specified file.
// If the file does not exist, it will be created with 0644 permissions.
func appendStringToFile(filePath, text string) error {
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(text + "\n")
	return err
}

// buildAppMap scans the given directory for files that start with '.'
// and end with '.app', reads their contents (one filename per line), and
// returns a map where each filename maps to the .app file that contains it.
// Also returns a map where each `.app` points to a slice of strings containing
// all the files in that `.app`.
func buildAppMap(dir string) (map[string]string, map[string][]string, error) {
	reverseMap := make(map[string]string)
	appMap := make(map[string][]string)

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				logrus.Warnf("Error descending into path %s: %v", path, err)
			}
			return filepath.SkipDir
		}
		info, err := d.Info()
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				logrus.Warnf("Error descending into path %s: %v", path, err)
			}
			return filepath.SkipDir
		}
		if !info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") && strings.HasSuffix(name, ".app") {
				err := readLinesFromFile(path, func(line string) error {
					reverseMap[line] = name
					appMap[name] = append(appMap[name], line)
					return nil
				})
				if err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return reverseMap, appMap, nil
}

// Get the paths of all quadlets available to the current user
func getAllQuadletPaths() []string {
	var quadletPaths []string
	quadletDirs := systemdquadlet.GetUnitDirs(rootless.IsRootless())
	for _, dir := range quadletDirs {
		dents, err := os.ReadDir(dir)
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				// This is perfectly normal, some quadlet directories aren't created by the package
				logrus.Warnf("Cannot list Quadlet directory %s: %v", dir, err)
			}
			continue
		}
		logrus.Debugf("Checking for quadlets in %q", dir)
		for _, dent := range dents {
			if systemdquadlet.IsExtSupported(dent.Name()) && !dent.IsDir() {
				logrus.Debugf("Found quadlet %q", dent.Name())
				quadletPaths = append(quadletPaths, filepath.Join(dir, dent.Name()))
			}
		}
	}
	return quadletPaths
}

// Generate systemd service name for a Quadlet from full path to the Quadlet file
func getQuadletServiceName(quadletPath string) (string, error) {
	unit, err := parser.ParseUnitFile(quadletPath)
	if err != nil {
		return "", fmt.Errorf("parsing Quadlet file %s: %w", quadletPath, err)
	}

	serviceName, err := systemdquadlet.GetUnitServiceName(unit)
	if err != nil {
		return "", fmt.Errorf("generating service name for Quadlet %s: %w", filepath.Base(quadletPath), err)
	}
	return serviceName + ".service", nil
}

type QuadletFilter func(q *entities.ListQuadlet) bool

func generateQuadletFilter(filter string, filterValues []string) (func(q *entities.ListQuadlet) bool, error) {
	switch filter {
	case "name":
		return func(q *entities.ListQuadlet) bool {
			res := util.StringMatchRegexSlice(q.Name, filterValues)
			return res
		}, nil
	default:
		return nil, fmt.Errorf("%s is not a valid filter", filter)
	}
}

func (ic *ContainerEngine) QuadletList(ctx context.Context, options entities.QuadletListOptions) ([]*entities.ListQuadlet, error) {
	reverseMap, _, err := buildAppMap(systemdquadlet.GetInstallUnitDirPath(rootless.IsRootless()))
	if err != nil {
		return nil, fmt.Errorf("unable to build app map: %w", err)
	}
	// Is systemd available to the current user?
	// We cannot proceed if not.
	conn, err := systemd.ConnectToDBUS()
	if err != nil {
		return nil, fmt.Errorf("connecting to systemd dbus: %w", err)
	}
	defer conn.Close()

	quadletPaths := getAllQuadletPaths()

	// Create filter functions
	filterFuncs := make([]func(q *entities.ListQuadlet) bool, 0, len(options.Filters))
	filterMap := make(map[string][]string)
	// TODO: Add filter for app names.
	for _, f := range options.Filters {
		fname, filter, hasFilter := strings.Cut(f, "=")
		if !hasFilter {
			return nil, fmt.Errorf("invalid filter %q", f)
		}
		filterMap[fname] = append(filterMap[fname], filter)
	}
	for fname, filter := range filterMap {
		filterFunc, err := generateQuadletFilter(fname, filter)
		if err != nil {
			return nil, err
		}
		filterFuncs = append(filterFuncs, filterFunc)
	}

	reports := make([]*entities.ListQuadlet, 0, len(quadletPaths))
	allServiceNames := make([]string, 0, len(quadletPaths))
	partialReports := make(map[string]entities.ListQuadlet)

	for _, path := range quadletPaths {
		appName := ""
		value, ok := reverseMap[filepath.Base(path)]
		if ok {
			appName = value
		}
		report := entities.ListQuadlet{
			Name: filepath.Base(path),
			Path: path,
			App:  appName,
		}

		serviceName, err := getQuadletServiceName(path)
		if err != nil {
			report.Status = err.Error()
			reports = append(reports, &report)
			continue
		}
		partialReports[serviceName] = report
		allServiceNames = append(allServiceNames, serviceName)
	}

	// Get status of all systemd units with given names.
	statuses, err := conn.ListUnitsByNamesContext(ctx, allServiceNames)
	if err != nil {
		return nil, fmt.Errorf("querying systemd for unit status: %w", err)
	}
	if len(statuses) != len(allServiceNames) {
		logrus.Warnf("Queried for %d services but received %d responses", len(allServiceNames), len(statuses))
	}
	for _, unitStatus := range statuses {
		report, ok := partialReports[unitStatus.Name]
		if !ok {
			logrus.Errorf("Unexpected unit returned by systemd - was not searching for %s", unitStatus.Name)
		}

		logrus.Debugf("Unit %s has status %s %s %s", unitStatus.Name, unitStatus.LoadState, unitStatus.ActiveState, unitStatus.SubState)
		report.UnitName = unitStatus.Name

		// Unit is not loaded
		if unitStatus.LoadState != "loaded" {
			report.Status = "Not loaded"
		} else {
			report.Status = fmt.Sprintf("%s/%s", unitStatus.ActiveState, unitStatus.SubState)
		}
		reports = append(reports, &report)
		delete(partialReports, unitStatus.Name)
	}

	// This should not happen.
	// Systemd will give us output for everything we sent to them, even if it's not a valid unit.
	// We can find them with LoadState, as we do above.
	// Handle it anyways because it's easy enough to do.
	for _, report := range partialReports {
		report.Status = "Not loaded"
		reports = append(reports, &report)
	}

	finalReports := make([]*entities.ListQuadlet, 0, len(reports))
	for _, report := range reports {
		include := true
		for _, filterFunc := range filterFuncs {
			include = filterFunc(report)
		}
		if include {
			finalReports = append(finalReports, report)
		}
	}

	return finalReports, nil
}

// Retrieve path to a Quadlet file given full name including extension
func getQuadletPathByName(name string) (string, error) {
	// Check if we were given a valid extension
	if !systemdquadlet.IsExtSupported(name) {
		return "", fmt.Errorf("%q is not a supported quadlet file type", filepath.Ext(name))
	}

	quadletDirs := systemdquadlet.GetUnitDirs(rootless.IsRootless())
	for _, dir := range quadletDirs {
		testPath := filepath.Join(dir, name)
		if _, err := os.Stat(testPath); err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				return "", fmt.Errorf("cannot stat quadlet at path %q: %w", testPath, err)
			}
			continue
		}
		return testPath, nil
	}
	return "", fmt.Errorf("could not locate quadlet %q in any supported quadlet directory", name)
}

func (ic *ContainerEngine) QuadletPrint(ctx context.Context, quadlet string) (string, error) {
	quadletPath, err := getQuadletPathByName(quadlet)
	if err != nil {
		return "", err
	}

	contents, err := os.ReadFile(quadletPath)
	if err != nil {
		return "", fmt.Errorf("reading quadlet %q contents: %w", quadletPath, err)
	}

	return string(contents), nil
}

// QuadletRemove removes one or more Quadlet files or applications and reloads systemd daemon as needed. The function returns a `QuadletRemoveReport`
// containing the removal status for each quadlet file or application, or returns an error if the entire function fails.
func (ic *ContainerEngine) QuadletRemove(ctx context.Context, quadlets []string, options entities.QuadletRemoveOptions) (*entities.QuadletRemoveReport, error) {
	report := entities.QuadletRemoveReport{
		Errors:  make(map[string]error),
		Removed: []string{},
	}
	removeList := []string{}
	reverseMap, appMap, err := buildAppMap(systemdquadlet.GetInstallUnitDirPath(rootless.IsRootless()))
	if err != nil {
		return nil, fmt.Errorf("unable to build app map: %w", err)
	}
	expandQuadletList := []string{}
	// Process all `.app` files in arguments, if `.app` file
	// is found then expand it to its respective quadlet files
	// and remove it from the processing list.
	for _, quadlet := range quadlets {
		// Most likely this is an app
		if strings.HasPrefix(quadlet, ".") && strings.HasSuffix(quadlet, ".app") {
			files, ok := appMap[quadlet]
			// Add all files of this application in to-be removed list.
			if ok {
				for _, file := range files {
					if !systemdquadlet.IsExtSupported(file) {
						removeList = append(removeList, file)
					} else {
						expandQuadletList = append(expandQuadletList, file)
					}
				}
			}
			// also add .app file itself to the remove list so it can
			// be cleaned after removal of all components in the list
			if !slices.Contains(removeList, quadlet) {
				removeList = append(removeList, quadlet)
			}
		} else {
			expandQuadletList = append(expandQuadletList, quadlet)
		}
	}
	quadlets = expandQuadletList
	allQuadletPaths := make([]string, 0, len(quadlets))
	allServiceNames := make([]string, 0, len(quadlets))
	runningQuadlets := make([]string, 0, len(quadlets))
	serviceNameToQuadletName := make(map[string]string)
	needReload := options.ReloadSystemd

	if len(quadlets) == 0 && !options.All {
		return nil, errors.New("must provide at least 1 quadlet to remove")
	}

	// Is systemd available to the current user?
	// We cannot proceed if not.
	conn, err := systemd.ConnectToDBUS()
	if err != nil {
		return nil, fmt.Errorf("connecting to systemd dbus: %w", err)
	}
	defer conn.Close()

	if options.All {
		allQuadlets := getAllQuadletPaths()
		quadlets = allQuadlets
	}

	// We are using index wise iteration here instead of `range`
	// because we are modifying `quadlets` in this loop by appending
	// more elements to it if needed, we cannot do this with `range`.
	for i := 0; i < len(quadlets); i++ {
		var err error
		var quadletPath string
		quadlet := quadlets[i]
		if options.All {
			quadletPath = quadlet
		} else {
			quadletPath, err = getQuadletPathByName(quadlet)
		}
		if !options.All && err != nil {
			// All implies Ignore, because the only reason we'd see an error here with all
			// is if the quadlet was removed in a TOCTOU scenario.
			if options.Ignore {
				report.Removed = append(report.Removed, quadlet)
			} else {
				report.Errors[quadlet] = err
			}
			continue
		}
		value, ok := reverseMap[quadlet]
		if ok {
			// If this is part of app and we are cleaning entire .app
			// make sure to add .app file itself to the removal list
			// if it does not already exists.
			if !slices.Contains(removeList, value) {
				removeList = append(removeList, value)
			}
			appFilePath := filepath.Join(systemdquadlet.GetInstallUnitDirPath(rootless.IsRootless()), value)
			filesToRemove, err := getAssetListFromFile(appFilePath)
			if err != nil {
				return nil, fmt.Errorf("unable to get list of files to remove: %w", err)
			}
			for _, entry := range filesToRemove {
				if !systemdquadlet.IsExtSupported(entry) {
					removeList = append(removeList, entry)
					if !slices.Contains(removeList, value) {
						// In the last also clean .<quadlet>.app file
						removeList = append(removeList, value)
					}
					continue
				}
				if !slices.Contains(quadlets, entry) {
					quadlets = append(quadlets, entry)
				}
			}
		}

		allQuadletPaths = append(allQuadletPaths, quadletPath)

		serviceName, err := getQuadletServiceName(quadletPath)
		if err != nil {
			report.Errors[quadlet] = err
			continue
		}

		allServiceNames = append(allServiceNames, serviceName)
		serviceNameToQuadletName[serviceName] = quadlet
	}

	if len(allServiceNames) != 0 {
		// Check if units are loaded into systemd, and further if they are running.
		// If running and force is not set, error.
		// If force is set, try and stop the unit.
		statuses, err := conn.ListUnitsByNamesContext(ctx, allServiceNames)
		if err != nil {
			return nil, fmt.Errorf("querying systemd for unit status: %w", err)
		}
		for _, unitStatus := range statuses {
			quadletName := serviceNameToQuadletName[unitStatus.Name]

			if unitStatus.LoadState != "loaded" {
				// Nothing to do here if it doesn't exist in systemd
				continue
			}
			needReload = options.ReloadSystemd
			if unitStatus.ActiveState == "active" {
				if !options.Force {
					report.Errors[quadletName] = fmt.Errorf("quadlet %s is running and force is not set, refusing to remove", quadletName)
					runningQuadlets = append(runningQuadlets, quadletName)
					continue
				}
				logrus.Infof("Going to stop systemd unit %s (Quadlet %s)", unitStatus.Name, quadletName)
				ch := make(chan string)
				if _, err := conn.StopUnitContext(ctx, unitStatus.Name, "replace", ch); err != nil {
					report.Errors[quadletName] = fmt.Errorf("stopping quadlet %s: %w", quadletName, err)
					runningQuadlets = append(runningQuadlets, quadletName)
					continue
				}
				logrus.Debugf("Waiting for systemd unit %s to stop", unitStatus.Name)
				stopResult := <-ch
				if stopResult != "done" && stopResult != "skipped" {
					report.Errors[quadletName] = fmt.Errorf("unable to stop quadlet %s: %s", quadletName, stopResult)
					runningQuadlets = append(runningQuadlets, quadletName)
					continue
				}
			}
		}
	}

	// Remove the actual files behind the quadlets
	if len(allQuadletPaths) != 0 {
		for _, path := range allQuadletPaths {
			var errAsset error
			quadletName := filepath.Base(path)
			errAsset = deleteAsset(quadletName)
			if slices.Contains(runningQuadlets, quadletName) {
				continue
			}
			if err := os.Remove(path); err != nil {
				if !errors.Is(err, fs.ErrNotExist) {
					reportErr := fmt.Errorf("removing quadlet %s: %w", quadletName, err)
					if errAsset != nil {
						reportErr = errors.Join(reportErr, errAsset)
					}
					report.Errors[quadletName] = reportErr
					continue
				}
			}
			for _, entry := range removeList {
				os.Remove(filepath.Join(systemdquadlet.GetInstallUnitDirPath(rootless.IsRootless()), entry))
			}
			report.Removed = append(report.Removed, quadletName)
		}
	}

	// Reload systemd, if necessary/requested.
	if needReload {
		if err := conn.ReloadContext(ctx); err != nil {
			return &report, fmt.Errorf("reloading systemd: %w", err)
		}
	}

	return &report, nil
}
