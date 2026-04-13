//go:build !remote && (linux || freebsd)

package abi

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/containers/podman/v6/libpod/define"
	"github.com/containers/podman/v6/pkg/domain/entities"
	"github.com/containers/podman/v6/pkg/rootless"
	"github.com/containers/podman/v6/pkg/systemd/parser"
	systemdquadlet "github.com/containers/podman/v6/pkg/systemd/quadlet"
	"github.com/containers/podman/v6/pkg/util"
	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/sirupsen/logrus"
)

type QuadletFilter func(q *entities.ListQuadlet) bool

func generateQuadletFilter(filter string, filterValues []string) (QuadletFilter, error) {
	switch filter {
	case "name":
		return func(q *entities.ListQuadlet) bool {
			res := util.StringMatchRegexSlice(q.Name, filterValues)
			return res
		}, nil
	case "status":
		return func(q *entities.ListQuadlet) bool {
			res := util.StringMatchRegexSlice(q.Status, filterValues)
			return res
		}, nil
	case "app":
		return func(q *entities.ListQuadlet) bool {
			res := util.StringMatchRegexSlice(q.App, filterValues)
			return res
		}, nil
	default:
		return nil, fmt.Errorf("%s is not a valid filter", filter)
	}
}

func generateQuadletFilters(filters []string) (QuadletFilter, error) {
	// Create filter functions
	filterFuncs := make([]QuadletFilter, 0, len(filters))
	filterMap := make(map[string][]string)
	// TODO: Add filter for app names.
	for _, f := range filters {
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

	return func(q *entities.ListQuadlet) bool {
		for _, filterFunc := range filterFuncs {
			if !filterFunc(q) {
				return false
			}
		}
		return true
	}, nil
}

func getQuadlets(dir string) ([]string, error) {
	reports := make([]string, 0)

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				logrus.Warnf("Error descending into path %s: %v", path, err)
			}
			return filepath.SkipDir
		}

		if d.IsDir() {
			return nil
		}

		if systemdquadlet.IsExtSupported(d.Name()) {
			reports = append(reports, path)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return reports, nil
}

func getAllQuadlets(ctx context.Context, conn *dbus.Conn) ([]*entities.ListQuadlet, error) {
	reports := make([]*entities.ListQuadlet, 0)

	// Service name -> quadlet
	partialReports := make(map[string]entities.ListQuadlet)

	// Get the root paths of all quadlets available to the current user
	quadletDirs := systemdquadlet.GetUnitDirs(rootless.IsRootless(), false)

	allServiceNames := make([]string, 0)

	// for every quadlet dir, let's get the quadlets
	for _, dir := range quadletDirs {
		quadlets, err := getQuadlets(dir)
		if err != nil {
			return nil, err
		}

		// for every quadlet we found, let's get the corresponding service name
		for _, quadlet := range quadlets {
			basename := filepath.Base(quadlet)
			app := ""

			// Let's compare how "nested" the quadlet is
			// if he is not at the root directory we use the first directory as app name
			rel, err := filepath.Rel(dir, quadlet)
			if err == nil && rel != basename {
				app = strings.Split(rel, string(filepath.Separator))[0]
			}

			report := entities.ListQuadlet{
				Name: basename,
				Path: quadlet,
				App:  app,
			}

			serviceName, err := getQuadletServiceName(quadlet)
			if err != nil {
				report.Status = err.Error()
				reports = append(reports, &report)
				continue
			}
			allServiceNames = append(allServiceNames, serviceName)
			partialReports[serviceName] = report
		}
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

	return reports, nil
}

func removeQuadlet(ctx context.Context, conn *dbus.Conn, quadlet *entities.ListQuadlet, force bool) error {
	switch quadlet.Status {
	case "Not loaded":
	case "inactive/dead":
		// Nothing to do here if it doesn't exist in systemd
		break
	case "active/running":
		if !force {
			return fmt.Errorf("quadlet %s is running and force is not set, refusing to remove: %w", quadlet.Name, define.ErrQuadletRunning)
		}
		logrus.Infof("Going to stop systemd unit %s (Quadlet %s)", quadlet.Name, quadlet.Path)

		ch := make(chan string)
		if _, err := conn.StopUnitContext(ctx, quadlet.UnitName, "replace", ch); err != nil {
			return fmt.Errorf("stopping quadlet %s: %w", quadlet.Name, err)
		}

		logrus.Debugf("Waiting for systemd unit %s to stop", quadlet.Name)
		stopResult := <-ch
		if stopResult != "done" && stopResult != "skipped" {
			return fmt.Errorf("unable to stop quadlet %s: %s", quadlet.Name, stopResult)
		}
	}

	return os.Remove(quadlet.Path)
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
