package cli

import (
	"fmt"
	"slices"

	"github.com/spf13/cobra"

	"github.com/mhiro2/envdesk/internal/config"
)

func completeServiceNames(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	project, err := loadProjectForCompletion(cmd)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	names := make([]string, 0, len(project.Services))
	for name := range project.Services {
		names = append(names, name)
	}

	return names, cobra.ShellCompDirectiveNoFileComp
}

func completeEnvironmentNames(cmd *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	if len(args) == 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	project, err := loadProjectForCompletion(cmd)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	service, err := project.Service(args[0])
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	return service.Environments(), cobra.ShellCompDirectiveNoFileComp
}

func completeServiceEnvArgs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	switch len(args) {
	case 0:
		return completeServiceNames(cmd, args, toComplete)
	case 1:
		return completeEnvironmentNames(cmd, args, toComplete)
	default:
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
}

func completeDiffArgs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	switch len(args) {
	case 0:
		return completeServiceNames(cmd, args, toComplete)
	case 1, 2:
		return completeEnvironmentNames(cmd, args, toComplete)
	default:
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
}

func completeServiceFlag(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return completeServiceNames(cmd, args, toComplete)
}

func completeEnvironmentFlag(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	project, err := loadProjectForCompletion(cmd)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	serviceName, err := cmd.Flags().GetString("service")
	if err == nil && serviceName != "" {
		service, err := project.Service(serviceName)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		return service.Environments(), cobra.ShellCompDirectiveNoFileComp
	}

	seen := make(map[string]struct{})
	envs := make([]string, 0)
	for _, service := range project.Services {
		for _, envName := range service.Environments() {
			if _, ok := seen[envName]; ok {
				continue
			}

			seen[envName] = struct{}{}
			envs = append(envs, envName)
		}
	}
	slices.Sort(envs)

	return envs, cobra.ShellCompDirectiveNoFileComp
}

func completeSyncTargetEnvs(cmd *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	if len(args) == 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	project, err := loadProjectForCompletion(cmd)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	service, err := project.Service(args[0])
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	envs := service.Environments()
	if len(args) > 1 {
		envs = slices.DeleteFunc(envs, func(env string) bool {
			return env == args[1]
		})
	}

	return envs, cobra.ShellCompDirectiveNoFileComp
}

func loadProjectForCompletion(cmd *cobra.Command) (*config.Project, error) {
	configPath, err := readConfigPath(cmd)
	if err != nil {
		return nil, err
	}

	project, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("load project config: %w", err)
	}
	return project, nil
}
