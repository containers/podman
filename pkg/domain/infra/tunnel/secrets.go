package tunnel

import (
	"context"
	"io"

	"github.com/containers/podman/v4/pkg/bindings/secrets"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/errorhandling"
	"github.com/pkg/errors"
)

func (ic *ContainerEngine) SecretCreate(ctx context.Context, name string, reader io.Reader, options entities.SecretCreateOptions) (*entities.SecretCreateReport, error) {
	opts := new(secrets.CreateOptions).
		WithDriver(options.Driver).
		WithDriverOpts(options.DriverOpts).
		WithName(name)
	created, err := secrets.Create(ic.ClientCtx, reader, opts)
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (ic *ContainerEngine) SecretInspect(ctx context.Context, nameOrIDs []string) ([]*entities.SecretInfoReport, []error, error) {
	allInspect := make([]*entities.SecretInfoReport, 0, len(nameOrIDs))
	errs := make([]error, 0, len(nameOrIDs))
	for _, name := range nameOrIDs {
		inspected, err := secrets.Inspect(ic.ClientCtx, name, nil)
		if err != nil {
			errModel, ok := err.(*errorhandling.ErrorModel)
			if !ok {
				return nil, nil, err
			}
			if errModel.ResponseCode == 404 {
				errs = append(errs, errors.Errorf("no such secret %q", name))
				continue
			}
			return nil, nil, err
		}
		allInspect = append(allInspect, inspected)
	}
	return allInspect, errs, nil
}

func (ic *ContainerEngine) SecretList(ctx context.Context, opts entities.SecretListRequest) ([]*entities.SecretInfoReport, error) {
	options := new(secrets.ListOptions).WithFilters(opts.Filters)
	secrs, _ := secrets.List(ic.ClientCtx, options)
	return secrs, nil
}

func (ic *ContainerEngine) SecretRm(ctx context.Context, nameOrIDs []string, options entities.SecretRmOptions) ([]*entities.SecretRmReport, error) {
	allRm := make([]*entities.SecretRmReport, 0, len(nameOrIDs))
	if options.All {
		allSecrets, err := secrets.List(ic.ClientCtx, nil)
		if err != nil {
			return nil, err
		}
		for _, secret := range allSecrets {
			allRm = append(allRm, &entities.SecretRmReport{
				Err: secrets.Remove(ic.ClientCtx, secret.ID),
				ID:  secret.ID,
			})
		}
		return allRm, nil
	}
	for _, name := range nameOrIDs {
		secret, err := secrets.Inspect(ic.ClientCtx, name, nil)
		if err != nil {
			errModel, ok := err.(*errorhandling.ErrorModel)
			if !ok {
				return nil, err
			}
			if errModel.ResponseCode == 404 {
				allRm = append(allRm, &entities.SecretRmReport{
					Err: errors.Errorf("no secret with name or id %q: no such secret ", name),
					ID:  "",
				})
				continue
			}
		}
		allRm = append(allRm, &entities.SecretRmReport{
			Err: secrets.Remove(ic.ClientCtx, name),
			ID:  secret.ID,
		})
	}
	return allRm, nil
}
