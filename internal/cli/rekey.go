package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mhiro2/envdesk/internal/app"
)

func newRekeyCommand() *cobra.Command {
	var service string
	var envName string
	var dryRun bool
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "rekey",
		Short: "Re-encrypt files after recipient changes",
		Example: `  envdesk rekey
  envdesk rekey --service api --env stg
  envdesk rekey --service api --dry-run --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			project, err := loadProject(cmd)
			if err != nil {
				return err
			}

			result, err := app.Rekey(cmd.Context(), project, newCryptoAdapter(), app.RekeyOptions{
				Service: service,
				Env:     envName,
				DryRun:  dryRun,
			})

			if jsonOutput {
				if result == nil {
					if err != nil {
						return fmt.Errorf("rekey env files: %w", err)
					}

					return fmt.Errorf("rekey env files: empty result")
				}
				if writeErr := writeJSON(cmd.OutOrStdout(), result); writeErr != nil {
					return writeErr
				}
				if err != nil {
					return fmt.Errorf("rekey env files: %w", err)
				}

				return nil
			}

			if result != nil {
				for i, path := range result.Files {
					line := fmt.Sprintf("[%d/%d] rekeyed %s", i+1, len(result.Files), formatProjectPath(project.BaseDir, path))
					if dryRun {
						line += " (dry-run)"
					}

					if _, printErr := fmt.Fprintln(cmd.OutOrStdout(), line); printErr != nil {
						return fmt.Errorf("write rekey output: %w", printErr)
					}
				}
				if err := writeRekeyErrorSummary(cmd.ErrOrStderr(), project.BaseDir, result.Errors); err != nil {
					return err
				}
			}

			if err != nil {
				return fmt.Errorf("rekey env files: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&service, "service", "", "Limit rekeying to a single service")
	cmd.Flags().StringVar(&envName, "env", "", "Limit rekeying to a single environment")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview affected files without modifying them")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable output")

	_ = cmd.RegisterFlagCompletionFunc("service", completeServiceFlag)

	return cmd
}
