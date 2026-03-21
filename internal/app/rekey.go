package app

import (
	"context"
	"fmt"

	"github.com/mhiro2/envdesk/internal/config"
	"github.com/mhiro2/envdesk/internal/crypto"
)

type RekeyOptions struct {
	Service string
	Env     string
	DryRun  bool
}

type RekeyResult struct {
	Files  []string     `json:"files"`
	Errors []RekeyError `json:"errors,omitempty"`
}

type RekeyError struct {
	Path    string `json:"path"`
	Service string `json:"service"`
	Env     string `json:"env"`
	Message string `json:"message"`
}

func Rekey(ctx context.Context, project *config.Project, adapter crypto.Adapter, opts RekeyOptions) (*RekeyResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if project == nil {
		return nil, fmt.Errorf("resolve project: nil project")
	}
	if adapter == nil {
		return nil, fmt.Errorf("resolve crypto adapter: nil adapter")
	}

	targets, err := selectEnvTargets(project, opts.Service, opts.Env)
	if err != nil {
		return nil, err
	}

	files := make([]string, 0, len(targets))
	for _, target := range targets {
		files = append(files, target.Path)
	}

	rekeyedFiles := files
	var rekeyErrors []RekeyError
	if !opts.DryRun {
		rekeyedFiles, rekeyErrors, err = rekeyTargets(ctx, adapter, targets)
		if err != nil {
			return &RekeyResult{Files: rekeyedFiles, Errors: rekeyErrors}, err
		}
	}

	return &RekeyResult{Files: rekeyedFiles, Errors: rekeyErrors}, nil
}

func rekeyTargets(ctx context.Context, adapter crypto.Adapter, targets []EnvTarget) ([]string, []RekeyError, error) {
	rekeyed := make([]string, 0, len(targets))
	rekeyErrors := make([]RekeyError, 0)
	for _, target := range targets {
		if err := adapter.Rekey(ctx, target.Path); err != nil {
			rekeyErrors = append(rekeyErrors, RekeyError{
				Path:    target.Path,
				Service: target.Service,
				Env:     target.Environment,
				Message: err.Error(),
			})
			continue
		}

		rekeyed = append(rekeyed, target.Path)
	}

	if len(rekeyErrors) > 0 {
		return rekeyed, rekeyErrors, fmt.Errorf("rekey env files: %d of %d files failed", len(rekeyErrors), len(targets))
	}

	return rekeyed, nil, nil
}
