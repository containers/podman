package quadlet

import (
	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/cmd/podman/validate"
	"github.com/containers/podman/v5/pkg/logiface"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// logrusLogger implements the logiface.Logger interface using logrus
type logrusLogger struct{}

func (l logrusLogger) Errorf(format string, args ...interface{}) {
	logrus.Errorf(format, args...)
}

func (l logrusLogger) Debugf(format string, args ...interface{}) {
	logrus.Debugf(format, args...)
}

var (
	// Pull in configured json library
	json = registry.JSONLibrary()

	// Command: podman _quadlet_
	quadletCmd = &cobra.Command{
		Use:   "quadlet",
		Short: "Allows users to manage Quadlets",
		Long:  "Allows users to manage Quadlets",
		RunE:  validate.SubCommandExists,
	}
)

func init() {
	// Initialize logiface with logrus logger
	logiface.SetLogger(logrusLogger{})

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: quadletCmd,
	})
}
