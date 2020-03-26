// +build ABISupport

package abi

import (
	"context"
	"io/ioutil"
	"strings"

	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/adapter/shortcuts"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/containers/libpod/pkg/signal"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// TODO: Should return *entities.ContainerExistsReport, error
func (ic *ContainerEngine) ContainerExists(ctx context.Context, nameOrId string) (*entities.BoolReport, error) {
	_, err := ic.Libpod.LookupContainer(nameOrId)
	if err != nil && errors.Cause(err) != define.ErrNoSuchCtr {
		return nil, err
	}
	return &entities.BoolReport{Value: err == nil}, nil
}

func (ic *ContainerEngine) ContainerWait(ctx context.Context, namesOrIds []string, options entities.WaitOptions) ([]entities.WaitReport, error) {
	var (
		responses []entities.WaitReport
	)
	ctrs, err := shortcuts.GetContainersByContext(false, options.Latest, namesOrIds, ic.Libpod)
	if err != nil {
		return nil, err
	}
	for _, c := range ctrs {
		response := entities.WaitReport{Id: c.ID()}
		exitCode, err := c.WaitForConditionWithInterval(options.Interval, options.Condition)
		if err != nil {
			response.Error = err
		} else {
			response.ExitCode = exitCode
		}
		responses = append(responses, response)
	}
	return responses, nil
}

func (ic *ContainerEngine) ContainerPause(ctx context.Context, namesOrIds []string, options entities.PauseUnPauseOptions) ([]*entities.PauseUnpauseReport, error) {
	var (
		ctrs   []*libpod.Container
		err    error
		report []*entities.PauseUnpauseReport
	)
	if options.All {
		ctrs, err = ic.Libpod.GetAllContainers()
	} else {
		ctrs, err = shortcuts.GetContainersByContext(false, false, namesOrIds, ic.Libpod)
	}
	if err != nil {
		return nil, err
	}
	for _, c := range ctrs {
		err := c.Pause()
		report = append(report, &entities.PauseUnpauseReport{Id: c.ID(), Err: err})
	}
	return report, nil
}

func (ic *ContainerEngine) ContainerUnpause(ctx context.Context, namesOrIds []string, options entities.PauseUnPauseOptions) ([]*entities.PauseUnpauseReport, error) {
	var (
		ctrs   []*libpod.Container
		err    error
		report []*entities.PauseUnpauseReport
	)
	if options.All {
		ctrs, err = ic.Libpod.GetAllContainers()
	} else {
		ctrs, err = shortcuts.GetContainersByContext(false, false, namesOrIds, ic.Libpod)
	}
	if err != nil {
		return nil, err
	}
	for _, c := range ctrs {
		err := c.Unpause()
		report = append(report, &entities.PauseUnpauseReport{Id: c.ID(), Err: err})
	}
	return report, nil
}
func (ic *ContainerEngine) ContainerStop(ctx context.Context, namesOrIds []string, options entities.StopOptions) ([]*entities.StopReport, error) {
	var (
		reports []*entities.StopReport
	)
	names := namesOrIds
	for _, cidFile := range options.CIDFiles {
		content, err := ioutil.ReadFile(cidFile)
		if err != nil {
			return nil, errors.Wrap(err, "error reading CIDFile")
		}
		id := strings.Split(string(content), "\n")[0]
		names = append(names, id)
	}
	ctrs, err := shortcuts.GetContainersByContext(options.All, options.Latest, names, ic.Libpod)
	if err != nil && !(options.Ignore && errors.Cause(err) == define.ErrNoSuchCtr) {
		return nil, err
	}
	for _, con := range ctrs {
		report := entities.StopReport{Id: con.ID()}
		err = con.StopWithTimeout(options.Timeout)
		if err != nil {
			// These first two are considered non-fatal under the right conditions
			if errors.Cause(err) == define.ErrCtrStopped {
				logrus.Debugf("Container %s is already stopped", con.ID())
				reports = append(reports, &report)
				continue

			} else if options.All && errors.Cause(err) == define.ErrCtrStateInvalid {
				logrus.Debugf("Container %s is not running, could not stop", con.ID())
				reports = append(reports, &report)
				continue
			}
			report.Err = err
			reports = append(reports, &report)
			continue
		}
		reports = append(reports, &report)
	}
	return reports, nil
}

func (ic *ContainerEngine) ContainerKill(ctx context.Context, namesOrIds []string, options entities.KillOptions) ([]*entities.KillReport, error) {
	var (
		reports []*entities.KillReport
	)
	sig, err := signal.ParseSignalNameOrNumber(options.Signal)
	if err != nil {
		return nil, err
	}
	ctrs, err := shortcuts.GetContainersByContext(options.All, options.Latest, namesOrIds, ic.Libpod)
	if err != nil {
		return nil, err
	}
	for _, con := range ctrs {
		reports = append(reports, &entities.KillReport{
			Id:  con.ID(),
			Err: con.Kill(uint(sig)),
		})
	}
	return reports, nil
}
func (ic *ContainerEngine) ContainerRestart(ctx context.Context, namesOrIds []string, options entities.RestartOptions) ([]*entities.RestartReport, error) {
	var (
		reports []*entities.RestartReport
	)
	ctrs, err := shortcuts.GetContainersByContext(options.All, options.Latest, namesOrIds, ic.Libpod)
	if err != nil {
		return nil, err
	}
	for _, con := range ctrs {
		timeout := con.StopTimeout()
		if options.Timeout != nil {
			timeout = *options.Timeout
		}
		reports = append(reports, &entities.RestartReport{
			Id:  con.ID(),
			Err: con.RestartWithTimeout(ctx, timeout),
		})
	}
	return reports, nil
}

func (ic *ContainerEngine) ContainerRm(ctx context.Context, namesOrIds []string, options entities.RmOptions) ([]*entities.RmReport, error) {
	var (
		reports []*entities.RmReport
	)
	if options.Storage {
		for _, ctr := range namesOrIds {
			report := entities.RmReport{Id: ctr}
			if err := ic.Libpod.RemoveStorageContainer(ctr, options.Force); err != nil {
				report.Err = err
			}
			reports = append(reports, &report)
		}
		return reports, nil
	}

	names := namesOrIds
	for _, cidFile := range options.CIDFiles {
		content, err := ioutil.ReadFile(cidFile)
		if err != nil {
			return nil, errors.Wrap(err, "error reading CIDFile")
		}
		id := strings.Split(string(content), "\n")[0]
		names = append(names, id)
	}

	ctrs, err := shortcuts.GetContainersByContext(options.All, options.Latest, names, ic.Libpod)
	if err != nil && !(options.Ignore && errors.Cause(err) == define.ErrNoSuchCtr) {
		// Failed to get containers. If force is specified, get the containers ID
		// and evict them
		if !options.Force {
			return nil, err
		}

		for _, ctr := range namesOrIds {
			logrus.Debugf("Evicting container %q", ctr)
			report := entities.RmReport{Id: ctr}
			id, err := ic.Libpod.EvictContainer(ctx, ctr, options.Volumes)
			if err != nil {
				if options.Ignore && errors.Cause(err) == define.ErrNoSuchCtr {
					logrus.Debugf("Ignoring error (--allow-missing): %v", err)
					reports = append(reports, &report)
					continue
				}
				report.Err = errors.Wrapf(err, "Failed to evict container: %q", id)
				reports = append(reports, &report)
				continue
			}
			reports = append(reports, &report)
		}
		return reports, nil
	}

	for _, c := range ctrs {
		report := entities.RmReport{Id: c.ID()}
		err := ic.Libpod.RemoveContainer(ctx, c, options.Force, options.Volumes)
		if err != nil {
			if options.Ignore && errors.Cause(err) == define.ErrNoSuchCtr {
				logrus.Debugf("Ignoring error (--allow-missing): %v", err)
				reports = append(reports, &report)
				continue
			}
			logrus.Debugf("Failed to remove container %s: %s", c.ID(), err.Error())
			report.Err = err
			reports = append(reports, &report)
			continue
		}
		reports = append(reports, &report)
	}
	return reports, nil
}

func (ic *ContainerEngine) ContainerInspect(ctx context.Context, namesOrIds []string, options entities.ContainerInspectOptions) ([]*entities.ContainerInspectReport, error) {
	var reports []*entities.ContainerInspectReport
	ctrs, err := shortcuts.GetContainersByContext(false, options.Latest, namesOrIds, ic.Libpod)
	if err != nil {
		return nil, err
	}
	for _, c := range ctrs {
		data, err := c.Inspect(options.Size)
		if err != nil {
			return nil, err
		}
		reports = append(reports, &entities.ContainerInspectReport{InspectContainerData: data})
	}
	return reports, nil
}

func (ic *ContainerEngine) ContainerTop(ctx context.Context, options entities.TopOptions) (*entities.StringSliceReport, error) {
	var (
		container *libpod.Container
		err       error
	)

	// Look up the container.
	if options.Latest {
		container, err = ic.Libpod.GetLatestContainer()
	} else {
		container, err = ic.Libpod.LookupContainer(options.NameOrID)
	}
	if err != nil {
		return nil, errors.Wrap(err, "unable to lookup requested container")
	}

	// Run Top.
	report := &entities.StringSliceReport{}
	report.Value, err = container.Top(options.Descriptors)
	return report, err
}
