package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/mhiro2/envdesk/internal/app"
)

func newInitCommand(newCryptoAdapter cryptoAdapterFactory) *cobra.Command {
	var services []string
	var environments []string
	var force bool
	var scaffoldSOPS bool
	var encrypt bool
	var ageRecipients []string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize envdesk in the current repository",
		Long: `Initialize envdesk in the current repository.

This command creates the initial envdesk configuration, prepares the
expected directory layout, and scaffolds a minimal schema and env files.`,
		Example: `  envdesk init --services api,web --envs dev,stg,prod --sops
  envdesk init --services api --envs dev --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			configPath, err := readConfigPath(cmd)
			if err != nil {
				return err
			}

			if !force {
				if _, statErr := os.Stat(configPath); statErr == nil {
					if !isQuiet(cmd) {
						fprintWarning(cmd.ErrOrStderr(), fmt.Sprintf("existing project detected at %s; use --force to overwrite", configPath))
					}
					return fmt.Errorf("initialize project: %s already exists (use --force to overwrite)", configPath)
				}
			}

			result, err := app.Init(cmd.Context(), newCryptoAdapter(), app.InitOptions{
				ConfigPath:    configPath,
				Services:      services,
				Environments:  environments,
				Force:         force,
				ScaffoldSOPS:  scaffoldSOPS,
				Encrypt:       encrypt,
				AgeRecipients: ageRecipients,
			})
			if err != nil {
				return fmt.Errorf("initialize project: %w", err)
			}

			for _, file := range result.Files {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", file.Action, file.Path); err != nil {
					return fmt.Errorf("write init output: %w", err)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringSliceVar(&services, "services", nil, "Comma-separated service names to scaffold")
	cmd.Flags().StringSliceVar(&environments, "envs", []string{"dev", "stg", "prod"}, "Comma-separated environment names")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing scaffold files if present")
	cmd.Flags().BoolVar(&scaffoldSOPS, "sops", false, "Scaffold a minimal .sops.yaml file")
	cmd.Flags().BoolVar(&encrypt, "encrypt", false, "Write scaffolded env files in encrypted form")
	cmd.Flags().StringSliceVar(&ageRecipients, "age", nil, "Age recipients for scaffolded .sops.yaml and encrypted env files")

	return cmd
}
