package secrets

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v5/cmd/podman/common"
	"github.com/containers/podman/v5/cmd/podman/parse"
	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	createCmd = &cobra.Command{
		Use:   "create [options] NAME FILE|-",
		Short: "Create a new secret",
		Long:  "Create a secret. Input can be a path to a file or \"-\" (read from stdin). Secret drivers \"file\" (default), \"pass\", and \"shell\" are available.",
		RunE:  create,
		Args:  cobra.ExactArgs(2),
		Example: `podman secret create mysecret /path/to/secret
		printf "secretdata" | podman secret create mysecret -`,
		ValidArgsFunction: common.AutocompleteSecretCreate,
	}
)

var (
	createOpts = entities.SecretCreateOptions{}
	env        = false
	labels     []string
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: createCmd,
		Parent:  secretCmd,
	})
	cfg := registry.PodmanConfig()

	flags := createCmd.Flags()

	driverFlagName := "driver"
	flags.StringVarP(&createOpts.Driver, driverFlagName, "d", cfg.ContainersConfDefaultsRO.Secrets.Driver, "Specify secret driver")
	_ = createCmd.RegisterFlagCompletionFunc(driverFlagName, completion.AutocompleteNone)

	optsFlagName := "driver-opts"
	flags.StringToStringVar(&createOpts.DriverOpts, optsFlagName, cfg.ContainersConfDefaultsRO.Secrets.Opts, "Specify driver specific options")
	_ = createCmd.RegisterFlagCompletionFunc(optsFlagName, completion.AutocompleteNone)

	envFlagName := "env"
	flags.BoolVar(&env, envFlagName, false, "Read secret data from environment variable")

	flags.BoolVar(&createOpts.Replace, "replace", false, "If a secret with the same name exists, replace it")

	labelFlagName := "label"
	flags.StringArrayVarP(&labels, labelFlagName, "l", nil, "Specify labels on the secret")
	_ = createCmd.RegisterFlagCompletionFunc(labelFlagName, completion.AutocompleteNone)
}

func create(cmd *cobra.Command, args []string) error {
	name := args[0]

	var err error
	path := args[1]

	var reader io.Reader
	switch {
	case env:
		envValue := os.Getenv(path)
		if envValue == "" {
			return fmt.Errorf("cannot create store secret data: environment variable %s is not set", path)
		}
		reader = strings.NewReader(envValue)
	case path == "-" || path == "/dev/stdin":
		stat, err := os.Stdin.Stat()
		if err != nil {
			return err
		}
		if (stat.Mode() & os.ModeNamedPipe) == 0 {
			return errors.New("if `-` is used, data must be passed into stdin")
		}
		reader = os.Stdin
	default:
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		reader = file
	}

	createOpts.Labels, err = parse.GetAllLabels([]string{}, labels)
	if err != nil {
		return fmt.Errorf("unable to process labels: %w", err)
	}

	report, err := registry.ContainerEngine().SecretCreate(context.Background(), name, reader, createOpts)
	if err != nil {
		return err
	}
	fmt.Println(report.ID)
	return nil
}
