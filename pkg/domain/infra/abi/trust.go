//go:build !remote

package abi

import (
	"context"
	"fmt"
	"os"

	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/containers/podman/v5/pkg/trust"
)

func (ir *ImageEngine) ShowTrust(ctx context.Context, args []string, options entities.ShowTrustOptions) (*entities.ShowTrustReport, error) {
	var (
		err    error
		report entities.ShowTrustReport
	)
	policyPath := trust.DefaultPolicyPath(ir.Libpod.SystemContext())
	if len(options.PolicyPath) > 0 {
		policyPath = options.PolicyPath
	}
	report.Raw, err = os.ReadFile(policyPath)
	if err != nil {
		return nil, err
	}
	if options.Raw {
		return &report, nil
	}
	report.SystemRegistriesDirPath = trust.RegistriesDirPath(ir.Libpod.SystemContext())
	if len(options.RegistryPath) > 0 {
		report.SystemRegistriesDirPath = options.RegistryPath
	}
	report.Policies, err = trust.PolicyDescription(policyPath, report.SystemRegistriesDirPath)
	if err != nil {
		return nil, fmt.Errorf("could not show trust policies: %w", err)
	}
	return &report, nil
}

func (ir *ImageEngine) SetTrust(ctx context.Context, args []string, options entities.SetTrustOptions) error {
	if len(args) != 1 {
		return fmt.Errorf("SetTrust called with unexpected %d args", len(args))
	}
	scope := args[0]

	policyPath := trust.DefaultPolicyPath(ir.Libpod.SystemContext())
	if len(options.PolicyPath) > 0 {
		policyPath = options.PolicyPath
	}

	return trust.AddPolicyEntries(policyPath, trust.AddPolicyEntriesInput{
		Scope:       scope,
		Type:        options.Type,
		PubKeyFiles: options.PubKeysFile,
	})
}
