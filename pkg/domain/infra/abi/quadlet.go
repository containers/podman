package abi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/containers/podman/v5/pkg/rootless"
	"github.com/containers/podman/v5/pkg/systemd"
	"github.com/containers/podman/v5/pkg/systemd/parser"
	systemdquadlet "github.com/containers/podman/v5/pkg/systemd/quadlet"
	"github.com/containers/podman/v5/pkg/util"
	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

// Install one or more Quadlet files
func (ic *ContainerEngine) QuadletInstall(ctx context.Context, pathsOrURLs []string, options entities.QuadletInstallOptions) (*entities.QuadletInstallReport, error) {
	// Is systemd available to the current user?
	// We cannot proceed if not.
	conn, err := systemd.ConnectToDBUS()
	if err != nil {
		return nil, fmt.Errorf("connecting to systemd dbus: %w", err)
	}

	// Is Quadlet installed? No point if not.
	quadletPath := "/usr/lib/systemd/system-generators/podman-system-generator"
	quadletStat, err := os.Stat(quadletPath)
	if err != nil {
		return nil, fmt.Errorf("cannot stat Quadlet generator, Quadlet may not be installed: %w", err)
	}
	if !quadletStat.Mode().IsRegular() || !(quadletStat.Mode()&0100 != 0) {
		return nil, fmt.Errorf("no valid Quadlet binary installed to %q, unable to use Quadlet", quadletPath)
	}

	installDir := systemdquadlet.UnitDirAdmin
	if rootless.IsRootless() {
		// Install just for the user in question.
		quadletRootlessDirs := systemdquadlet.GetUnitDirs(true)

		foundAdminDir := false
		for _, dir := range quadletRootlessDirs {
			// Prefer /etc/containers/systemd/users(/$UID)
			if strings.HasPrefix(dir, systemdquadlet.UnitDirAdmin) {
				// Does it exist and can we write to it? If it doesn't, we cannot use it.
				stat, err := os.Stat(dir)
				if err != nil || !stat.IsDir() {
					continue
				}
				if unix.Access(dir, unix.W_OK) == nil {
					installDir = dir
					foundAdminDir = true
				}
			}

			// If we can't use the /etc/ directory, use what is available.
			// The permanent directory should always be after the temporary one
			// if both exist, so iterate through all directories.
			if !foundAdminDir {
				installDir = dir
			}
		}
	}

	logrus.Debugf("Going to install Quadlet to directory %s", installDir)

	stat, err := os.Stat(installDir)
	if rootless.IsRootless() {
		// Make the directory if it doesn't exist
		if err != nil && os.IsNotExist(err) {
			if err := os.MkdirAll(installDir, 0755); err != nil {
				return nil, fmt.Errorf("unable to create Quadlet install path %s: %w", installDir, err)
			}
		} else if err != nil {
			return nil, fmt.Errorf("unable to stat Quadlet install path %s: %w", installDir, err)
		}
	} else {
		// Package manager should have created the dir for root Podman.
		// So just check that it exists.
		if err != nil {
			return nil, fmt.Errorf("unable to stat Quadlet install path %s: %w", installDir, err)
		}
		if !stat.IsDir() {
			return nil, fmt.Errorf("install path %s for Quadlets is not a directory", installDir)
		}
	}

	installReport := new(entities.QuadletInstallReport)

	as, err := ic.Libpod.ArtifactStore()
	if err != nil {
		return nil, err
	}

	// Loop over all given URLs
	for _, toInstall := range pathsOrURLs {
		switch {
		case strings.HasPrefix(toInstall, "oci-artifact://"):
			artName := strings.TrimPrefix(toInstall, "oci-artifact://")
			layers, err := as.GetLayers(ctx, artName)
			if err != nil {
				return nil, fmt.Errorf("retrieving OCI artifact %s: %w", artName, err)
			}

			// Handle install from OCI artifact.
			// Assume everything in the artifact is a valid Quadlet file.
			// TODO: support selectors.
			for filename, pathOnDisk := range layers {
				finalPath, err := ic.installQuadlet(ctx, pathOnDisk, filename, installDir)
				if err != nil {
					installReport.QuadletErrors[pathOnDisk] = err
					continue
				}
				installReport.InstalledQuadlets[pathOnDisk] = finalPath

			}
		case strings.HasPrefix(toInstall, "http://") || strings.HasPrefix(toInstall, "https://"):
			// It's a URL. Pull to temporary file.
			tmpFile, err := os.CreateTemp("", "quadlet-dl")
			if err != nil {
				installReport.QuadletErrors[toInstall] = fmt.Errorf("unable to create temporay file to download URL %s: %w", toInstall, err)
				continue
			}
			defer func() {
				tmpFile.Close()
				if err := os.Remove(tmpFile.Name()); err != nil {
					logrus.Errorf("Unable to remove temporary file %q: %w", tmpFile.Name(), err)
				}
			}()
			r, err := http.Get(toInstall)
			if err != nil {
				installReport.QuadletErrors[toInstall] = fmt.Errorf("unable to download URL %s: %w", toInstall, err)
				continue
			}
			defer r.Body.Close()
			_, err = io.Copy(tmpFile, r.Body)
			if err != nil {
				installReport.QuadletErrors[toInstall] = fmt.Errorf("populating temporary file: %w", err)
				continue
			}
			installedPath, err := ic.installQuadlet(ctx, tmpFile.Name(), "", installDir)
			if err != nil {
				installReport.QuadletErrors[toInstall] = err
				continue
			}
			installReport.InstalledQuadlets[toInstall] = installedPath
		default:
			installedPath, err := ic.installQuadlet(ctx, toInstall, "", installDir)
			if err != nil {
				installReport.QuadletErrors[toInstall] = err
			} else {
				installReport.InstalledQuadlets[toInstall] = installedPath
			}

		}
	}

	// Perform a Quadlet dry-run to validate the syntax of our quadlets
	// TODO: This can definitely be improved.
	// We can't easily validate single quadlets (you can depend on other quadlets, so validation
	// really has to parse everything), but we should be able to output in a machine-readable format
	// (JSON, maybe?) so we can easily associated error to quadlet here, and return better
	// results.
	var validateErr error
	quadletArgs := []string{"--dryrun"}
	if rootless.IsRootless() {
		quadletArgs = append(quadletArgs, "--user")
	}
	quadletCmd := exec.Command(quadletPath, quadletArgs...)
	out, err := quadletCmd.CombinedOutput()
	if err != nil {
		logrus.Errorf("Error validating Quadlet syntax")
		fmt.Fprintf(os.Stderr, string(out))
		validateErr = errors.New("validating Quadlet syntax failed")
	}

	// TODO: Should we still do this if the above validation errored?
	if options.ReloadSystemd {
		if err := conn.ReloadContext(ctx); err != nil {
			return installReport, fmt.Errorf("reloading systemd: %w", err)
		}
	}

	return installReport, validateErr
}

// Install a single Quadlet from a path on local disk to the given install directory.
// Perform some minimal validation, but not much.
// We can't know about a lot of problems without running the Quadlet binary, which we
// only want to do once.
func (ic *ContainerEngine) installQuadlet(ctx context.Context, path, destName, installDir string) (string, error) {
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

	// Second, validate that the dest path does NOT exist.
	// TODO: Overwrite flag?
	if _, err := os.Stat(finalPath); err == nil {
		return "", fmt.Errorf("a Quadlet with name %s already exists, refusing to overwrite", filepath.Base(finalPath))
	}

	// Validate extension is valid
	if !systemdquadlet.IsExtSupported(finalPath) {
		return "", fmt.Errorf("%q is not a supported Quadlet file type", filepath.Ext(finalPath))
	}

	// Move the file in
	contents, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading source file %q: %w", path, err)
	}
	if err := os.WriteFile(finalPath, contents, 0644); err != nil {
		return "", fmt.Errorf("writing Quadlet %q to %q: %w", path, finalPath, err)
	}

	// TODO: It would be nice to do single-file validation here, and remove the file if it fails.

	return finalPath, nil
}

// Get the paths of all quadlets available to the current user
func getAllQuadletPaths() ([]string, error) {
	var quadletPaths []string
	quadletDirs := systemdquadlet.GetUnitDirs(rootless.IsRootless())
	for _, dir := range quadletDirs {
		dents, err := os.ReadDir(dir)
		if err != nil {
			// This is perfectly normal, some quadlet directories aren't created by the package
			logrus.Infof("Cannot list Quadlet directory %s: %v", dir, err)
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
	return quadletPaths, nil
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
			return util.StringMatchRegexSlice(q.Name, filterValues)
		}, nil
	default:
		return nil, fmt.Errorf("%s is not a valid filter", filter)
	}
}

func (ic *ContainerEngine) QuadletList(ctx context.Context, options entities.QuadletListOptions) ([]*entities.ListQuadlet, error) {
	// Is systemd available to the current user?
	// We cannot proceed if not.
	conn, err := dbus.NewWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("connecting to systemd dbus: %w", err)
	}

	quadletPaths, err := getAllQuadletPaths()
	if err != nil {
		return nil, fmt.Errorf("listing all quadlets: %w", err)
	}

	// Create filter functions
	filterFuncs := make([]func(q *entities.ListQuadlet) bool, 0, len(options.Filters))
	filterMap := make(map[string][]string)
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
		report := entities.ListQuadlet{
			Name: filepath.Base(path),
			Path: path,
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

		// Unit is not loaded
		if unitStatus.LoadState != "loaded" {
			report.Status = "Not loaded"
			reports = append(reports, &report)
			delete(partialReports, unitStatus.Name)
			continue
		}

		report.Status = fmt.Sprintf("%s/%s", unitStatus.ActiveState, unitStatus.SubState)

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
			include = include || filterFunc(report)
		}
		if include {
			finalReports = append(finalReports, report)
		}
	}

	return finalReports, nil
}

// Retrieve path to a Quadlet file given full name including extension
func getQuadletByName(name string) (string, error) {
	// Check if we were given a valid extension
	if !systemdquadlet.IsExtSupported(name) {
		return "", fmt.Errorf("%q is not a supported quadlet file type", filepath.Ext(name))
	}

	quadletDirs := systemdquadlet.GetUnitDirs(rootless.IsRootless())
	for _, dir := range quadletDirs {
		testPath := filepath.Join(dir, name)
		if _, err := os.Stat(testPath); err != nil {
			if !os.IsNotExist(err) {
				return "", fmt.Errorf("cannot stat quadlet at path %q: %w", testPath, err)
			}
			continue
		}
		return testPath, nil
	}
	return "", fmt.Errorf("could not locate quadlet %q in any supported quadlet directory", name)
}

func (ic *ContainerEngine) QuadletPrint(ctx context.Context, quadlet string) (string, error) {
	quadletPath, err := getQuadletByName(quadlet)
	if err != nil {
		return "", err
	}

	contents, err := os.ReadFile(quadletPath)
	if err != nil {
		return "", fmt.Errorf("reading quadlet %q contents: %w", quadletPath, err)
	}

	return string(contents), nil
}

func (ic *ContainerEngine) QuadletRemove(ctx context.Context, quadlets []string, options entities.QuadletRemoveOptions) (*entities.QuadletRemoveReport, error) {
	report := new(entities.QuadletRemoveReport)
	allQuadletPaths := make([]string, 0, len(quadlets))
	allServiceNames := make([]string, 0, len(quadlets))
	runningQuadlets := make([]string, 0, len(quadlets))
	serviceNameToQuadletName := make(map[string]string)
	needReload := false

	// Early escape: if 0 quadlets are requested, bail immediately without error.
	if len(quadlets) == 0 && !options.All {
		return nil, errors.New("must provide at least 1 quadlet to remove")
	}

	// Is systemd available to the current user?
	// We cannot proceed if not.
	conn, err := systemd.ConnectToDBUS()
	if err != nil {
		return nil, fmt.Errorf("connecting to systemd dbus: %w", err)
	}

	if options.All {
		allQuadlets, err := getAllQuadletPaths()
		if err != nil {
			return nil, err
		}
		quadlets = allQuadlets
	}

	for _, quadlet := range quadlets {
		quadletPath, err := getQuadletByName(quadlet)
		if err != nil {
			// All implies Ignore, because the only reason we'd see an error here with all
			// is if the quadlet was removed in a TOCTOU scenario.
			if options.Ignore || options.All {
				report.Removed = append(report.Removed, quadlet)
			} else {
				report.Errors[quadlet] = err
			}
			continue
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
			needReload = true
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
			quadletName := filepath.Base(path)
			if slices.Contains(runningQuadlets, quadletName) {
				continue
			}
			if err := os.Remove(path); err != nil {
				if !errors.Is(err, fs.ErrNotExist) {
					report.Errors[quadletName] = fmt.Errorf("removing quadlet %s: %w", quadletName, err)
					continue
				}
			}
			report.Removed = append(report.Removed, quadletName)
		}
	}

	// Reload systemd, if necessary/requested.
	if needReload {
		if err := conn.ReloadContext(ctx); err != nil {
			return report, fmt.Errorf("reloading systemd: %w", err)
		}
	}

	return report, nil
}
