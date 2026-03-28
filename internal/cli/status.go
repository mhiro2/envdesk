package cli

import (
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/mhiro2/envdesk/internal/app"
)

func newStatusCommand(newCryptoAdapter cryptoAdapterFactory) *cobra.Command {
	var service string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show lint, sync, and update status across environments",
		Example: `  envdesk status
  envdesk status --service api
  envdesk status --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			project, err := loadProject(cmd)
			if err != nil {
				return err
			}

			result, err := app.Status(cmd.Context(), project, newCryptoAdapter(project.BaseDir), app.StatusOptions{
				Service: service,
			})
			if err != nil {
				return fmt.Errorf("status env files: %w", err)
			}

			if jsonOutput {
				if err := writeJSON(cmd.OutOrStdout(), result); err != nil {
					return err
				}
			} else {
				if err := writeStatusTable(cmd, result); err != nil {
					return err
				}
			}

			if result.Healthy {
				return nil
			}

			return withExitCode(fmt.Errorf(
				"status env files: found %d lint errors, %d lint warnings, and %d drift issues",
				result.Summary.LintErrorCount,
				result.Summary.LintWarningCount,
				result.Summary.DriftIssueCount,
			))
		},
	}

	cmd.Flags().StringVar(&service, "service", "", "Limit checks to a single service")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable output")

	_ = cmd.RegisterFlagCompletionFunc("service", completeServiceFlag)

	return cmd
}

func writeStatusTable(cmd *cobra.Command, result *app.StatusResult) error {
	writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)

	if isVerbose(cmd) {
		if _, err := fmt.Fprintln(writer, "SERVICE\tENV\tLINT\tSYNC\tUPDATED\tPATH"); err != nil {
			return fmt.Errorf("write status output: %w", err)
		}
	} else {
		if _, err := fmt.Fprintln(writer, "SERVICE\tENV\tLINT\tSYNC\tUPDATED"); err != nil {
			return fmt.Errorf("write status output: %w", err)
		}
	}

	for _, row := range result.Rows {
		if isVerbose(cmd) {
			if _, err := fmt.Fprintf(
				writer,
				"%s\t%s\t%s\t%s\t%s\t%s\n",
				row.Service,
				row.Environment,
				formatStatusLint(row.Lint),
				formatStatusSync(row.Sync),
				formatStatusUpdated(row.UpdatedAt),
				row.Path,
			); err != nil {
				return fmt.Errorf("write status output: %w", err)
			}
		} else {
			if _, err := fmt.Fprintf(
				writer,
				"%s\t%s\t%s\t%s\t%s\n",
				row.Service,
				row.Environment,
				formatStatusLint(row.Lint),
				formatStatusSync(row.Sync),
				formatStatusUpdated(row.UpdatedAt),
			); err != nil {
				return fmt.Errorf("write status output: %w", err)
			}
		}
	}

	if err := writer.Flush(); err != nil {
		return fmt.Errorf("write status output: %w", err)
	}

	if result.Summary.LintErrorCount == 0 && result.Summary.LintWarningCount == 0 && result.Summary.DriftIssueCount == 0 {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n", colorSuccess("all environments look healthy")); err != nil {
			return fmt.Errorf("write status output: %w", err)
		}
		return nil
	}

	if _, err := fmt.Fprintf(
		cmd.OutOrStdout(),
		"\nsummary: %d services, %d environments, %d lint errors, %d lint warnings, %d drift issues across %d environments\n",
		result.Summary.ServiceCount,
		result.Summary.EnvironmentCount,
		result.Summary.LintErrorCount,
		result.Summary.LintWarningCount,
		result.Summary.DriftIssueCount,
		result.Summary.DriftEnvironmentCount,
	); err != nil {
		return fmt.Errorf("write status output: %w", err)
	}

	return nil
}

func formatStatusLint(lint app.StatusLint) string {
	switch lint.State {
	case "error":
		if lint.WarningCount > 0 {
			return fmt.Sprintf("error:%d warn:%d", lint.ErrorCount, lint.WarningCount)
		}
		return fmt.Sprintf("error:%d", lint.ErrorCount)
	case "warning":
		return fmt.Sprintf("warn:%d", lint.WarningCount)
	default:
		return "ok"
	}
}

func formatStatusSync(sync app.StatusSync) string {
	if sync.State == "in_sync" {
		return "ok"
	}

	parts := []string{fmt.Sprintf("issues:%d", sync.IssueCount)}
	if sync.ErrorCount > 0 {
		parts = append(parts, fmt.Sprintf("error:%d", sync.ErrorCount))
	}
	if sync.WarningCount > 0 {
		parts = append(parts, fmt.Sprintf("warn:%d", sync.WarningCount))
	}
	if sync.MissingCount > 0 {
		parts = append(parts, fmt.Sprintf("missing:%d", sync.MissingCount))
	}
	if sync.PresentCount > 0 {
		parts = append(parts, fmt.Sprintf("present:%d", sync.PresentCount))
	}

	return "drift " + strings.Join(parts, " ")
}

func formatStatusUpdated(updatedAt time.Time) string {
	if updatedAt.IsZero() {
		return "-"
	}

	return updatedAt.Local().Format("2006-01-02 15:04")
}
