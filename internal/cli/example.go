package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mhiro2/envdesk/internal/app"
)

func newExampleCommand(newCryptoAdapter cryptoAdapterFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "example",
		Short: "Generate or update .env.example files",
		Example: `  envdesk example generate --service api
  envdesk example generate --service api --out docs/api.env.example --force`,
	}

	cmd.AddCommand(newExampleGenerateCommand(newCryptoAdapter))

	return cmd
}

func newExampleGenerateCommand(newCryptoAdapter cryptoAdapterFactory) *cobra.Command {
	var service string
	var out string
	var force bool

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate or update .env.example files",
		Long: `Generate or update .env.example files.

This command creates a non-secret example env file from schema and
existing env structure so teams can document required configuration
without exposing real values.`,
		Example: `  envdesk example generate --service api
  envdesk example generate --service api --out docs/api.env.example --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			project, err := loadProject(cmd)
			if err != nil {
				return err
			}

			result, err := app.ExampleGenerate(cmd.Context(), project, newCryptoAdapter(project.BaseDir), app.ExampleGenerateOptions{
				Service: service,
				Out:     out,
				Force:   force,
			})
			if err != nil {
				return fmt.Errorf("generate example file: %w", err)
			}

			for _, file := range result.Files {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", file.Action, formatProjectPath(project.BaseDir, file.Path)); err != nil {
					return fmt.Errorf("write example output: %w", err)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&service, "service", "", "Limit generation to a single service")
	cmd.Flags().StringVar(&out, "out", "", "Output path for the generated example file")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing example files")

	_ = cmd.RegisterFlagCompletionFunc("service", completeServiceFlag)

	return cmd
}
