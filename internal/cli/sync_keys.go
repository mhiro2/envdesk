package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mhiro2/envdesk/internal/app"
)

func newSyncKeysCommand() *cobra.Command {
	var targets []string
	var dryRun bool
	var placeholders bool
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:               "sync-keys <service> <source-env>",
		Short:             "Align keysets across environments",
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: completeServiceEnvArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			project, err := loadProject(cmd)
			if err != nil {
				return err
			}

			result, err := app.SyncKeys(cmd.Context(), project, newCryptoAdapter(), app.SyncKeysOptions{
				Service:            args[0],
				SourceEnvironment:  args[1],
				TargetEnvironments: targets,
				DryRun:             dryRun,
				Placeholders:       placeholders,
			})
			if err != nil {
				return fmt.Errorf("sync keys: %w", err)
			}

			if jsonOutput {
				return writeJSON(cmd.OutOrStdout(), result)
			}

			for _, target := range result.TargetEnvironments {
				if !target.Changed {
					if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s: no changes\n", target.Environment); err != nil {
						return fmt.Errorf("write sync-keys output: %w", err)
					}

					continue
				}

				parts := make([]string, 0, 2)
				if len(target.Added) > 0 {
					parts = append(parts, "+"+strings.Join(target.Added, ","))
				}
				if len(target.Removed) > 0 {
					parts = append(parts, "-"+strings.Join(target.Removed, ","))
				}

				line := fmt.Sprintf("%s: %s", target.Environment, strings.Join(parts, " "))
				if dryRun {
					line += " (dry-run)"
				}

				if _, err := fmt.Fprintln(cmd.OutOrStdout(), line); err != nil {
					return fmt.Errorf("write sync-keys output: %w", err)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringSliceVar(&targets, "to", nil, "Target environments to update")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview changes without writing files")
	cmd.Flags().BoolVar(&placeholders, "placeholders", false, "Insert schema-aware placeholder values for missing keys")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable output")

	_ = cmd.RegisterFlagCompletionFunc("to", completeSyncTargetEnvs)

	return cmd
}
