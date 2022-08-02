package abi

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/domain/utils"
)

func (ic *ContainerEngine) SecretCreate(ctx context.Context, name string, reader io.Reader, options entities.SecretCreateOptions) (*entities.SecretCreateReport, error) {
	data, _ := ioutil.ReadAll(reader)
	secretsPath := ic.Libpod.GetSecretsStorageDir()
	manager, err := ic.Libpod.SecretsManager()
	if err != nil {
		return nil, err
	}

	// set defaults from config for the case they are not set by an upper layer
	// (-> i.e. tests that talk directly to the api)
	cfg, err := ic.Libpod.GetConfigNoCopy()
	if err != nil {
		return nil, err
	}
	if options.Driver == "" {
		options.Driver = cfg.Secrets.Driver
	}
	if len(options.DriverOpts) == 0 {
		options.DriverOpts = cfg.Secrets.Opts
	}
	if options.DriverOpts == nil {
		options.DriverOpts = make(map[string]string)
	}

	if options.Driver == "file" {
		if _, ok := options.DriverOpts["path"]; !ok {
			options.DriverOpts["path"] = filepath.Join(secretsPath, "filedriver")
		}
	}

	secretID, err := manager.Store(name, data, options.Driver, options.DriverOpts, nil)
	if err != nil {
		return nil, err
	}
	return &entities.SecretCreateReport{
		ID: secretID,
	}, nil
}

func (ic *ContainerEngine) SecretInspect(ctx context.Context, nameOrIDs []string) ([]*entities.SecretInfoReport, []error, error) {
	manager, err := ic.Libpod.SecretsManager()
	if err != nil {
		return nil, nil, err
	}
	errs := make([]error, 0, len(nameOrIDs))
	reports := make([]*entities.SecretInfoReport, 0, len(nameOrIDs))
	for _, nameOrID := range nameOrIDs {
		secret, err := manager.Lookup(nameOrID)
		if err != nil {
			if strings.Contains(err.Error(), "no such secret") {
				errs = append(errs, err)
				continue
			} else {
				return nil, nil, fmt.Errorf("error inspecting secret %s: %w", nameOrID, err)
			}
		}
		report := &entities.SecretInfoReport{
			ID:        secret.ID,
			CreatedAt: secret.CreatedAt,
			UpdatedAt: secret.CreatedAt,
			Spec: entities.SecretSpec{
				Name: secret.Name,
				Driver: entities.SecretDriverSpec{
					Name:    secret.Driver,
					Options: secret.DriverOptions,
				},
			},
		}
		reports = append(reports, report)
	}

	return reports, errs, nil
}

func (ic *ContainerEngine) SecretList(ctx context.Context, opts entities.SecretListRequest) ([]*entities.SecretInfoReport, error) {
	manager, err := ic.Libpod.SecretsManager()
	if err != nil {
		return nil, err
	}
	secretList, err := manager.List()
	if err != nil {
		return nil, err
	}
	report := make([]*entities.SecretInfoReport, 0, len(secretList))
	for _, secret := range secretList {
		result, err := utils.IfPassesSecretsFilter(secret, opts.Filters)
		if err != nil {
			return nil, err
		}
		if result {
			reportItem := entities.SecretInfoReport{
				ID:        secret.ID,
				CreatedAt: secret.CreatedAt,
				UpdatedAt: secret.CreatedAt,
				Spec: entities.SecretSpec{
					Name: secret.Name,
					Driver: entities.SecretDriverSpec{
						Name:    secret.Driver,
						Options: secret.DriverOptions,
					},
				},
			}
			report = append(report, &reportItem)
		}
	}
	return report, nil
}

func (ic *ContainerEngine) SecretRm(ctx context.Context, nameOrIDs []string, options entities.SecretRmOptions) ([]*entities.SecretRmReport, error) {
	var (
		err      error
		toRemove []string
		reports  = []*entities.SecretRmReport{}
	)
	manager, err := ic.Libpod.SecretsManager()
	if err != nil {
		return nil, err
	}
	toRemove = nameOrIDs
	if options.All {
		allSecrs, err := manager.List()
		if err != nil {
			return nil, err
		}
		for _, secr := range allSecrs {
			toRemove = append(toRemove, secr.ID)
		}
	}
	for _, nameOrID := range toRemove {
		deletedID, err := manager.Delete(nameOrID)
		if err == nil || strings.Contains(err.Error(), "no such secret") {
			reports = append(reports, &entities.SecretRmReport{
				Err: err,
				ID:  deletedID,
			})
			continue
		} else {
			return nil, err
		}
	}

	return reports, nil
}
