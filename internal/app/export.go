package app

import (
	"context"
	"fmt"

	"github.com/mhiro2/envdesk/internal/config"
	"github.com/mhiro2/envdesk/internal/crypto"
)

func Export(ctx context.Context, project *config.Project, adapter crypto.Adapter, serviceName, environment string) ([]byte, error) {
	if adapter == nil {
		return nil, fmt.Errorf("export env file: missing crypto adapter")
	}

	if project == nil {
		return nil, fmt.Errorf("resolve project: nil project")
	}

	service, err := project.Service(serviceName)
	if err != nil {
		return nil, fmt.Errorf("export env file: %w", err)
	}

	filePath, err := service.FilePath(environment)
	if err != nil {
		return nil, fmt.Errorf("lookup env file for %s/%s: %w", service.Name, environment, err)
	}

	plaintext, err := adapter.Decrypt(ctx, filePath)
	if err != nil {
		return nil, fmt.Errorf("decrypt env file for %s/%s: %w", service.Name, environment, err)
	}

	return plaintext, nil
}
