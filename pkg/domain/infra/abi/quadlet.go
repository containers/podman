//go:build !remote && (linux || freebsd)

package abi

import (
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
	"strings"

	"github.com/containers/podman/v6/libpod/define"
	"github.com/containers/podman/v6/pkg/domain/entities"
	"github.com/containers/podman/v6/pkg/rootless"
	"github.com/containers/podman/v6/pkg/systemd"
	systemdquadlet "github.com/containers/podman/v6/pkg/systemd/quadlet"
	"github.com/sirupsen/logrus"
	"go.podman.io/common/pkg/config"
	"go.podman.io/storage/pkg/fileutils"
)

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

	if !quadletStat.Mode().IsRegular() || quadletStat.Mode()&0o100 == 0 {
		return nil, fmt.Errorf("no valid Quadlet binary installed to %q, unable to use Quadlet", quadletPath)
	}

	installDir := systemdquadlet.GetInstallUnitDirPath(rootless.IsRootless())

	if len(options.Application) > 0 {
		installDir = filepath.Join(installDir, options.Application)
	}

	logrus.Debugf("Going to install Quadlet to directory %s", installDir)

	if err := os.MkdirAll(installDir, 0o755); err != nil {
		return nil, fmt.Errorf("unable to create Quadlet install path %s: %w", installDir, err)
	}

	installReport := entities.QuadletInstallReport{
		InstalledQuadlets: make(map[string]string),
		QuadletErrors:     make(map[string]error),
	}

	paths := pathsOrURLs
	if len(pathsOrURLs) > 0 && !strings.HasPrefix(pathsOrURLs[0], "http://") && !strings.HasPrefix(pathsOrURLs[0], "https://") {
		// Check if first path is dir, this is an APP
		info, err := os.Stat(pathsOrURLs[0])
		if err != nil {
			return nil, fmt.Errorf("unable to stat Quadlet path %s: %w", pathsOrURLs[0], err)
		}
		if info.IsDir() {
			if len(options.Application) == 0 {
				return nil, fmt.Errorf("application name cannot be empty when installing from directory")
			}

			// If it's a directory, then read all files and add it to paths
			entries, err := os.ReadDir(pathsOrURLs[0]) // TODO: make recursive
			if err != nil {
				return nil, fmt.Errorf("unable to read Quadlet dir %s: %w", pathsOrURLs[0], err)
			}
			redoPaths := make([]string, 0, len(entries))
			for _, entry := range entries {
				redoPaths = append(redoPaths, filepath.Join(pathsOrURLs[0], entry.Name()))
			}
			redoPaths = append(redoPaths, pathsOrURLs[1:]...)
			paths = redoPaths
		}
	}

	// Loop over all given URLs
	for _, toInstall := range paths {
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
			installedPath, err := ic.installQuadlet(ctx, tmpFile.Name(), quadletFileName, installDir, options.Replace)
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

			// Check if this file has a supported extension or is a .quadlets file
			isQuadletsFile := filepath.Ext(toInstall) == ".quadlets"

			if isQuadletsFile {
				// Parse the multi-quadlet file
				quadlets, err := parseMultiQuadletFile(toInstall)
				if err != nil {
					installReport.QuadletErrors[toInstall] = err
					continue
				}

				// Install each quadlet section as a separate file
				for _, quadlet := range quadlets {
					// Create a temporary file for this quadlet section
					tmpFile, err := os.CreateTemp("", quadlet.name+"*"+quadlet.extension)
					if err != nil {
						installReport.QuadletErrors[toInstall] = fmt.Errorf("unable to create temporary file for quadlet section %s: %w", quadlet.name, err)
						continue
					}
					defer os.Remove(tmpFile.Name())
					// Write the quadlet content to the temporary file
					_, err = tmpFile.WriteString(quadlet.content)
					tmpFile.Close()
					if err != nil {
						installReport.QuadletErrors[toInstall] = fmt.Errorf("unable to write quadlet section %s to temporary file: %w", quadlet.name, err)
						continue
					}

					// Install the quadlet from the temporary file
					destName := quadlet.name + quadlet.extension
					installedPath, err := ic.installQuadlet(ctx, tmpFile.Name(), destName, installDir, options.Replace)
					if err != nil {
						installReport.QuadletErrors[toInstall] = fmt.Errorf("unable to install quadlet section %s: %w", destName, err)
						continue
					}

					// Record the installation (use a unique key for each section)
					sectionKey := fmt.Sprintf("%s#%s", toInstall, destName)
					installReport.InstalledQuadlets[sectionKey] = installedPath
				}
			} else {
				// If toInstall is a single file with a supported extension, execute the original logic
				installedPath, err := ic.installQuadlet(ctx, toInstall, filepath.Base(toInstall), installDir, options.Replace)
				if err != nil {
					installReport.QuadletErrors[toInstall] = err
					continue
				}
				installReport.InstalledQuadlets[toInstall] = installedPath
			}
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
		if _, after, ok := strings.Cut(cd, prefix); ok {
			filename := after
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
func (ic *ContainerEngine) installQuadlet(ctx context.Context, path, destName, installDir string, replace bool) (string, error) {
	select {
	case <-ctx.Done():
		return "", fmt.Errorf("context cancelled: %w", ctx.Err())
	default:
	}

	// First, validate that the source path exists and is a file
	stat, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("quadlet to install %q does not exist or cannot be read: %w", path, err)
	}
	if stat.IsDir() {
		dirs, err := os.ReadDir(path)
		if err != nil {
			return "", err
		}

		for _, d := range dirs {
			nInstallDir := filepath.Join(installDir, destName)
			err := os.MkdirAll(nInstallDir, 0o755)
			if err != nil {
				return "", err
			}

			_, err = ic.installQuadlet(
				ctx,
				filepath.Join(path, d.Name()), // path
				d.Name(),                      // destName
				nInstallDir,                   // installDir
				replace)
			if err != nil {
				return "", err
			}
		}
		return path, nil
	}

	finalPath := filepath.Join(installDir, filepath.Base(filepath.Clean(path)))
	if destName != "" {
		finalPath = filepath.Join(installDir, destName)
	}

	var destFile *os.File
	var tempPath string

	if !replace {
		var err error
		// O_EXCL ensures we fail if the file already exists (avoids TOCTOU race)
		destFile, err = os.OpenFile(finalPath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o644)
		if err != nil {
			if errors.Is(err, fs.ErrExist) {
				return "", fmt.Errorf("a Quadlet with name %s already exists, refusing to overwrite", filepath.Base(finalPath))
			}
			return "", fmt.Errorf("unable to open file %s: %w", finalPath, err)
		}
	} else {
		var err error
		destFile, err = os.CreateTemp(filepath.Dir(finalPath), ".quadlet-install-*")
		if err != nil {
			return "", fmt.Errorf("unable to create temp file: %w", err)
		}
		tempPath = destFile.Name()
	}

	defer func() {
		if destFile != nil {
			destFile.Close()
		}
		if tempPath != "" {
			os.Remove(tempPath)
		}
	}()

	srcFile, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("unable to open file: %w", err)
	}
	defer srcFile.Close()

	err = fileutils.ReflinkOrCopy(srcFile, destFile)
	if err != nil {
		return "", fmt.Errorf("unable to copy file from %s to %s: %w", path, finalPath, err)
	}

	// Close before rename to flush writes; nil out to prevent double-close in defer
	if err := destFile.Close(); err != nil {
		return "", fmt.Errorf("unable to close file: %w", err)
	}
	destFile = nil

	if tempPath != "" {
		if err := os.Chmod(tempPath, 0o644); err != nil {
			return "", fmt.Errorf("unable to set permissions on temp file: %w", err)
		}

		if err := os.Rename(tempPath, finalPath); err != nil {
			return "", fmt.Errorf("unable to rename temp file to %s: %w", finalPath, err)
		}
		tempPath = ""
	}
	return finalPath, nil
}

// quadletSection represents a single quadlet extracted from a multi-quadlet file
type quadletSection struct {
	content   string
	extension string
	name      string
}

// parseMultiQuadletFile parses a file that may contain multiple quadlets separated by "---"
// Returns a slice of quadletSection structs, each representing a separate quadlet
func parseMultiQuadletFile(filePath string) ([]quadletSection, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("unable to read file %s: %w", filePath, err)
	}

	// Split content by lines and reconstruct sections manually to handle "---" properly
	lines := strings.Split(string(content), "\n")
	var sections []string
	var currentSection strings.Builder

	for _, line := range lines {
		if strings.TrimSpace(line) == "---" {
			// Found separator, save current section and start new one
			if currentSection.Len() > 0 {
				sections = append(sections, currentSection.String())
				currentSection.Reset()
			}
		} else {
			currentSection.WriteString(line)
			currentSection.WriteString("\n")
		}
	}

	// Add the last section
	if currentSection.Len() > 0 {
		sections = append(sections, currentSection.String())
	}

	// Pre-allocate slice with capacity based on number of sections
	quadlets := make([]quadletSection, 0, len(sections))

	for i, section := range sections {
		// Trim whitespace from section
		section = strings.TrimSpace(section)
		if section == "" {
			continue // Skip empty sections
		}

		// Determine quadlet type from section content
		extension, err := detectQuadletType(section)
		if err != nil {
			return nil, fmt.Errorf("unable to detect quadlet type in section %d: %w", i+1, err)
		}

		fileName, err := extractFileNameFromSection(section)
		if err != nil {
			return nil, fmt.Errorf("section %d: %w", i+1, err)
		}
		name := fileName

		quadlets = append(quadlets, quadletSection{
			content:   section,
			extension: extension,
			name:      name,
		})
	}

	if len(quadlets) == 0 {
		return nil, fmt.Errorf("no valid quadlet sections found in file %s", filePath)
	}

	return quadlets, nil
}

// extractFileNameFromSection extracts the FileName from a comment in the quadlet section
// The comment must be in the format: # FileName=my-name
func extractFileNameFromSection(content string) (string, error) {
	lines := strings.SplitSeq(content, "\n")
	for line := range lines {
		line = strings.TrimSpace(line)
		// Look for comment lines starting with #
		if strings.HasPrefix(line, "#") {
			// Remove the # and trim whitespace
			commentContent := strings.TrimSpace(line[1:])
			// Check if it's a FileName directive
			if strings.HasPrefix(commentContent, "FileName=") {
				fileName := strings.TrimSpace(commentContent[9:]) // Remove "FileName="
				if fileName == "" {
					return "", fmt.Errorf("FileName comment found but no filename specified")
				}
				// Validate filename (basic validation - no path separators)
				if strings.ContainsAny(fileName, "/\\") {
					return "", fmt.Errorf("FileName '%s' cannot contain path separators", fileName)
				}
				return fileName, nil
			}
		}
	}
	return "", fmt.Errorf("missing required '# FileName=<name>' comment at the beginning of quadlet section")
}

// detectQuadletType analyzes the content of a quadlet section to determine its type
// Returns the appropriate file extension (.container, .volume, .network, etc.)
func detectQuadletType(content string) (string, error) {
	// Look for section headers like [Container], [Volume], [Network], etc.
	lines := strings.SplitSeq(content, "\n")
	for line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			sectionName := strings.ToLower(strings.Trim(line, "[]"))
			expected := "." + sectionName
			if systemdquadlet.IsExtSupported("a" + expected) {
				return expected, nil
			}
		}
	}
	return "", fmt.Errorf("no recognized quadlet section found (expected [Container], [Volume], [Network], [Kube], [Image], [Build], or [Pod])")
}

func (ic *ContainerEngine) QuadletList(ctx context.Context, options entities.QuadletListOptions) ([]*entities.ListQuadlet, error) {
	// Is systemd available to the current user?
	// We cannot proceed if not.
	conn, err := systemd.ConnectToDBUS()
	if err != nil {
		return nil, fmt.Errorf("connecting to systemd dbus: %w", err)
	}
	defer conn.Close()

	// Create filter functions
	filterFunc, err := generateQuadletFilters(options.Filters)
	if err != nil {
		return nil, fmt.Errorf("cannot use filters: %w", err)
	}

	reports, err := getAllQuadlets(ctx, conn)
	if err != nil {
		return nil, fmt.Errorf("cannot get quadlets: %w", err)
	}

	finalReports := make([]*entities.ListQuadlet, 0, len(reports))
	for _, report := range reports {
		include := filterFunc(report)
		if include {
			finalReports = append(finalReports, report)
		}
	}

	return finalReports, nil
}

// QuadletExists checks whether a quadlet with the given name exists.
func (ic *ContainerEngine) QuadletExists(_ context.Context, name string) (*entities.BoolReport, error) {
	_, err := getQuadletPathByName(name)
	if err != nil && !errors.Is(err, define.ErrNoSuchQuadlet) {
		return nil, err
	}
	return &entities.BoolReport{Value: err == nil}, nil
}

// Retrieve path to a Quadlet file given full name including extension
func getQuadletPathByName(name string) (string, error) {
	// Check if we were given a valid extension
	if !systemdquadlet.IsExtSupported(name) {
		return "", fmt.Errorf("%q is not a supported quadlet file type", filepath.Ext(name))
	}

	quadletDirs := systemdquadlet.GetUnitDirs(rootless.IsRootless(), true)
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
	return "", fmt.Errorf("could not locate quadlet %q in any supported quadlet directory: %w", name, define.ErrNoSuchQuadlet)
}

func (ic *ContainerEngine) QuadletPrint(_ context.Context, quadlet string) (string, error) {
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

	// Is systemd available to the current user?
	// We cannot proceed if not.
	conn, err := systemd.ConnectToDBUS()
	if err != nil {
		return nil, fmt.Errorf("connecting to systemd dbus: %w", err)
	}
	defer conn.Close()

	// Get all units (aka Quadlets)
	units, err := getAllQuadlets(ctx, conn)
	if err != nil {
		return nil, fmt.Errorf("cannot get quadlets: %w", err)
	}

	if len(quadlets) == 0 && !options.All {
		return nil, errors.New("must provide at least 1 quadlet to remove")
	}

	// Group units by application
	// Map application -> quadlets
	applications := make(map[string][]*entities.ListQuadlet)
	for _, unit := range units {
		if len(unit.App) > 0 {
			applications[unit.App] = append(applications[unit.App], unit)
		}
	}

	if options.All {
		// Add all units not part of an Application
		for _, unit := range units {
			if len(unit.App) == 0 {
				quadlets = append(quadlets, unit.Name)
			}
		}
		// Add all application if recursive is true
		if options.Recursive {
			for application := range applications {
				quadlets = append(quadlets, application)
			}
		}
	}

	// Create a map filename -> quadlet
	files := make(map[string]*entities.ListQuadlet, len(units))
	for _, unit := range units {
		files[unit.Name] = unit
	}

	// Iterate over the list of quadlets to remove
	for _, quadlet := range quadlets {
		if systemdquadlet.IsExtSupported(quadlet) {
			// deleting a quadlet file
			if files[quadlet] == nil {
				if options.Ignore {
					report.Removed = append(report.Removed, quadlet)
				} else {
					report.Errors[quadlet] = fmt.Errorf("no such quadlet")
				}
				continue
			}

			err := removeQuadlet(ctx, conn, files[quadlet], options.Force)
			if err != nil {
				report.Errors[quadlet] = err
			} else {
				report.Removed = append(report.Removed, files[quadlet].Name)
			}
		} else {
			// delete an application
			if len(applications[quadlet]) == 0 {
				return nil, fmt.Errorf("no such application %q", quadlet)
			}

			if !options.Recursive {
				return nil, fmt.Errorf("refusing to remove application %q: recursive option is not set", quadlet)
			}

			for _, unit := range applications[quadlet] {
				err := removeQuadlet(ctx, conn, unit, options.Force)
				if err != nil {
					report.Errors[quadlet] = err
				} else {
					report.Removed = append(report.Removed, unit.Name)
				}
			}
		}
	}

	// Reload systemd, if necessary/requested.
	if options.ReloadSystemd {
		if err := conn.ReloadContext(ctx); err != nil {
			return &report, fmt.Errorf("reloading systemd: %w", err)
		}
	}

	return &report, nil
}
