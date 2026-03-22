package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mhiro2/envdesk/internal/app"
)

func newLintCommand(newCryptoAdapter cryptoAdapterFactory) *cobra.Command {
	var service string
	var environment string
	var strict bool
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "lint",
		Short: "Validate env files and schemas",
		Example: `  envdesk lint
  envdesk lint --service api --env dev
  envdesk lint --json --strict`,
		RunE: func(cmd *cobra.Command, args []string) error {
			project, err := loadProject(cmd)
			if err != nil {
				return err
			}

			result, err := app.Lint(cmd.Context(), project, newCryptoAdapter(), app.LintOptions{
				Service:     service,
				Environment: environment,
			})
			if err != nil {
				return fmt.Errorf("validate env files: %w", err)
			}

			if jsonOutput {
				if err := writeJSON(cmd.OutOrStdout(), result); err != nil {
					return err
				}
			} else {
				for _, problem := range result.Problems {
					line := fmt.Sprintf(
						"%s %s/%s",
						colorSeverity(string(problem.Severity)),
						problem.Service,
						problem.Environment,
					)
					if problem.Key != "" {
						line += " " + problem.Key
					}
					line += ": " + problem.Message

					if _, err := fmt.Fprintln(cmd.OutOrStdout(), line); err != nil {
						return fmt.Errorf("write lint output: %w", err)
					}
				}

				if result.ErrorCount > 0 || result.WarningCount > 0 {
					if _, err := fmt.Fprintf(
						cmd.OutOrStdout(),
						"%d errors, %d warnings\n",
						result.ErrorCount,
						result.WarningCount,
					); err != nil {
						return fmt.Errorf("write lint output: %w", err)
					}
				} else {
					if _, err := fmt.Fprintln(cmd.OutOrStdout(), colorSuccess("all checks passed")); err != nil {
						return fmt.Errorf("write lint output: %w", err)
					}
				}
			}

			if result.ErrorCount > 0 || (strict && result.WarningCount > 0) {
				return withExitCode(fmt.Errorf(
					"validate env files: found %d errors and %d warnings",
					result.ErrorCount,
					result.WarningCount,
				))
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&service, "service", "", "Limit validation to a single service")
	cmd.Flags().StringVar(&environment, "env", "", "Limit validation to a single environment")
	cmd.Flags().BoolVar(&strict, "strict", false, "Treat warnings as errors")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable output")

	_ = cmd.RegisterFlagCompletionFunc("service", completeServiceFlag)

	return cmd
}
