package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/containers/common/pkg/auth"
	"github.com/containers/common/pkg/completion"
	"github.com/containers/common/pkg/report"
	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/errorhandling"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type cliAutoUpdateOptions struct {
	entities.AutoUpdateOptions
	format string
}

var (
	autoUpdateOptions     = cliAutoUpdateOptions{}
	autoUpdateDescription = `Auto update containers according to their auto-update policy.

  Auto-update policies are specified with the "io.containers.autoupdate" label.
  Containers are expected to run in systemd units created with "podman-generate-systemd --new",
  or similar units that create new containers in order to run the updated images.
  Please refer to the podman-auto-update(1) man page for details.`
	autoUpdateCommand = &cobra.Command{
		Annotations:       map[string]string{registry.EngineMode: registry.ABIMode},
		Use:               "auto-update [options]",
		Short:             "Auto update containers according to their auto-update policy",
		Long:              autoUpdateDescription,
		RunE:              autoUpdate,
		ValidArgsFunction: completion.AutocompleteNone,
		Example: `podman auto-update
  podman auto-update --authfile ~/authfile.json`,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: autoUpdateCommand,
	})

	flags := autoUpdateCommand.Flags()

	authfileFlagName := "authfile"
	flags.StringVar(&autoUpdateOptions.Authfile, authfileFlagName, auth.GetDefaultAuthFile(), "Path to the authentication file. Use REGISTRY_AUTH_FILE environment variable to override")
	_ = autoUpdateCommand.RegisterFlagCompletionFunc(authfileFlagName, completion.AutocompleteDefault)

	flags.BoolVar(&autoUpdateOptions.DryRun, "dry-run", false, "Check for pending updates")
	flags.BoolVar(&autoUpdateOptions.Rollback, "rollback", true, "Rollback to previous image if update fails")

	flags.StringVar(&autoUpdateOptions.format, "format", "", "Change the output format to JSON or a Go template")
	_ = autoUpdateCommand.RegisterFlagCompletionFunc("format", common.AutocompleteFormat(&autoUpdateOutput{}))
}

func autoUpdate(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		// Backwards compat. System tests expect this error string.
		return errors.Errorf("`%s` takes no arguments", cmd.CommandPath())
	}

	allReports, failures := registry.ContainerEngine().AutoUpdate(registry.GetContext(), autoUpdateOptions.AutoUpdateOptions)
	if allReports == nil {
		return errorhandling.JoinErrors(failures)
	}

	if err := writeTemplate(allReports, autoUpdateOptions.format); err != nil {
		failures = append(failures, err)
	}

	return errorhandling.JoinErrors(failures)
}

type autoUpdateOutput struct {
	Unit          string
	Container     string
	ContainerName string
	ContainerID   string
	Image         string
	Policy        string
	Updated       string
}

func reportsToOutput(allReports []*entities.AutoUpdateReport) []autoUpdateOutput {
	output := make([]autoUpdateOutput, len(allReports))
	for i, r := range allReports {
		output[i] = autoUpdateOutput{
			Unit:          r.SystemdUnit,
			Container:     fmt.Sprintf("%s (%s)", r.ContainerID[:12], r.ContainerName),
			ContainerName: r.ContainerName,
			ContainerID:   r.ContainerID,
			Image:         r.ImageName,
			Policy:        r.Policy,
			Updated:       r.Updated,
		}
	}
	return output
}

func writeTemplate(allReports []*entities.AutoUpdateReport, inputFormat string) error {
	var format string
	var printHeader bool

	output := reportsToOutput(allReports)
	switch inputFormat {
	case "":
		rows := []string{"{{.Unit}}", "{{.Container}}", "{{.Image}}", "{{.Policy}}", "{{.Updated}}"}
		format = "{{range . }}" + strings.Join(rows, "\t") + "\n{{end -}}"
		printHeader = true
	case "json":
		prettyJSON, err := json.MarshalIndent(output, "", "    ")
		if err != nil {
			return err
		}
		fmt.Println(string(prettyJSON))
		return nil
	default:
		format = "{{range . }}" + inputFormat + "\n{{end -}}"
	}

	tmpl, err := report.NewTemplate("auto-update").Parse(format)
	if err != nil {
		return err
	}

	w, err := report.NewWriterDefault(os.Stdout)
	if err != nil {
		return err
	}
	defer w.Flush()

	if printHeader {
		headers := report.Headers(autoUpdateOutput{}, nil)
		if err := tmpl.Execute(w, headers); err != nil {
			return err
		}
	}

	return tmpl.Execute(w, output)
}
