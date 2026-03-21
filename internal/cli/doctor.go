package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mhiro2/envdesk/internal/app"
)

func newDoctorCommand() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check repository and crypto readiness",
		Example: `  envdesk doctor
  envdesk doctor --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			configPath, err := readConfigPath(cmd)
			if err != nil {
				return err
			}

			result, err := app.Doctor(cmd.Context(), app.DoctorOptions{
				ConfigPath: configPath,
			})
			if err != nil {
				return fmt.Errorf("run doctor checks: %w", err)
			}

			if jsonOutput {
				if err := writeJSON(cmd.OutOrStdout(), result); err != nil {
					return err
				}
			} else {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "mode: %s\n", result.RepositoryMode); err != nil {
					return fmt.Errorf("write doctor output: %w", err)
				}
				for _, finding := range result.Findings {
					line := fmt.Sprintf("%s %s", colorSeverity(string(finding.Severity)), finding.Check)
					if finding.Target != "" {
						line += " " + finding.Target
					}
					line += ": " + finding.Message

					if _, err := fmt.Fprintln(cmd.OutOrStdout(), line); err != nil {
						return fmt.Errorf("write doctor output: %w", err)
					}
				}

				if result.Healthy {
					if _, err := fmt.Fprintln(cmd.OutOrStdout(), colorSuccess("all checks passed")); err != nil {
						return fmt.Errorf("write doctor output: %w", err)
					}
				}
			}

			if result.Healthy {
				return nil
			}

			errorCount := 0
			warningCount := 0
			for _, finding := range result.Findings {
				if finding.Severity == app.SeverityError {
					errorCount++
					continue
				}

				warningCount++
			}

			if errorCount > 0 {
				return withExitCode(fmt.Errorf("run doctor checks: found %d errors and %d warnings", errorCount, warningCount))
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable output")

	return cmd
}
