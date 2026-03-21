package app

import (
	"fmt"

	"github.com/mhiro2/envdesk/internal/config"
)

func selectEnvTargets(project *config.Project, serviceFilter, environmentFilter string) ([]EnvTarget, error) {
	services, err := selectServices(project, serviceFilter)
	if err != nil {
		return nil, err
	}

	targets := make([]EnvTarget, 0)
	for _, service := range services {
		for _, envName := range service.Environments() {
			if environmentFilter != "" && environmentFilter != envName {
				continue
			}

			filePath, err := service.FilePath(envName)
			if err != nil {
				return nil, fmt.Errorf("lookup env file for %s/%s: %w", service.Name, envName, err)
			}

			targets = append(targets, EnvTarget{
				Service:     service.Name,
				Environment: envName,
				Path:        filePath,
			})
		}
	}

	if len(targets) == 0 {
		if environmentFilter != "" {
			return nil, fmt.Errorf("select env targets: no matching environment %q", environmentFilter)
		}

		if serviceFilter != "" {
			return nil, fmt.Errorf("select env targets for service %q: no matching files", serviceFilter)
		}

		return nil, fmt.Errorf("select env targets: no matching files")
	}

	return targets, nil
}
