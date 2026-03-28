package cli

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/mhiro2/envdesk/internal/app"
)

func newAuditCommand() *cobra.Command {
	var (
		service     string
		environment string
		key         string
		jsonOutput  bool
	)

	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Show key, schema, and drift audit history for env files",
		Long: `Show an audit view for env file keys from tracked git history.
It combines current key blame, schema metadata changes, and
drift start dates across environments.`,
		Example: `  envdesk audit --service api --env dev
  envdesk audit --service api --env prod --key DATABASE_URL
  envdesk audit --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			project, err := loadProject(cmd)
			if err != nil {
				return err
			}

			results, err := app.Audit(cmd.Context(), project, app.AuditOptions{
				Service:     service,
				Environment: environment,
				Key:         key,
			})
			if err != nil {
				return fmt.Errorf("audit env files: %w", err)
			}

			if jsonOutput {
				return writeJSON(cmd.OutOrStdout(), results)
			}

			return writeAuditTable(cmd, results)
		},
	}

	cmd.Flags().StringVar(&service, "service", "", "Limit to a single service")
	cmd.Flags().StringVar(&environment, "env", "", "Limit to a single environment")
	cmd.Flags().StringVar(&key, "key", "", "Show only a specific key")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable output")

	_ = cmd.RegisterFlagCompletionFunc("service", completeServiceFlag)
	_ = cmd.RegisterFlagCompletionFunc("env", completeEnvironmentFlag)

	return cmd
}

func writeAuditTable(cmd *cobra.Command, results []app.AuditResult) error {
	if len(results) == 0 {
		if _, err := fmt.Fprintln(cmd.OutOrStdout(), "no env files found"); err != nil {
			return fmt.Errorf("write audit output: %w", err)
		}
		return nil
	}

	for i, result := range results {
		if i > 0 {
			if _, err := fmt.Fprintln(cmd.OutOrStdout()); err != nil {
				return fmt.Errorf("write audit output: %w", err)
			}
		}

		if _, err := fmt.Fprintf(
			cmd.OutOrStdout(),
			"%s  %s/%s\n",
			colorYellow.Sprint("##"),
			result.Service,
			result.Environment,
		); err != nil {
			return fmt.Errorf("write audit output: %w", err)
		}

		if len(result.Entries) == 0 {
			if _, err := fmt.Fprintln(cmd.OutOrStdout(), "  (no keys)"); err != nil {
				return fmt.Errorf("write audit output: %w", err)
			}
			continue
		}

		writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)

		if isVerbose(cmd) {
			if _, err := fmt.Fprintln(writer, "KEY\tPRESENT\tREQUIRED\tSECRET\tTYPE\tVALUE_CHANGED\tSCHEMA_CHANGED\tDRIFT\tVALUE_COMMIT\tSCHEMA_COMMIT\tDRIFT_COMMIT"); err != nil {
				return fmt.Errorf("write audit output: %w", err)
			}
		} else {
			if _, err := fmt.Fprintln(writer, "KEY\tPRESENT\tREQUIRED\tSECRET\tTYPE\tVALUE_CHANGED\tSCHEMA_CHANGED\tDRIFT"); err != nil {
				return fmt.Errorf("write audit output: %w", err)
			}
		}

		for _, entry := range result.Entries {
			if isVerbose(cmd) {
				if _, err := fmt.Fprintf(
					writer,
					"%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
					entry.Key,
					formatAuditPresent(entry.Present),
					formatAuditRequired(entry.Schema),
					formatAuditSecret(entry.Schema),
					formatAuditType(entry.Schema),
					formatAuditDate(entry.LastValueChange),
					formatAuditDate(entry.LastSchemaChange),
					formatAuditDrift(entry.Drift),
					formatAuditCommit(entry.LastValueChange),
					formatAuditCommit(entry.LastSchemaChange),
					formatAuditCommit(entry.Drift.Since),
				); err != nil {
					return fmt.Errorf("write audit output: %w", err)
				}
			} else {
				if _, err := fmt.Fprintf(
					writer,
					"%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
					entry.Key,
					formatAuditPresent(entry.Present),
					formatAuditRequired(entry.Schema),
					formatAuditSecret(entry.Schema),
					formatAuditType(entry.Schema),
					formatAuditDate(entry.LastValueChange),
					formatAuditDate(entry.LastSchemaChange),
					formatAuditDrift(entry.Drift),
				); err != nil {
					return fmt.Errorf("write audit output: %w", err)
				}
			}
		}

		if err := writer.Flush(); err != nil {
			return fmt.Errorf("write audit output: %w", err)
		}
	}

	return nil
}

func formatAuditPresent(present bool) string {
	if present {
		return "yes"
	}

	return "no"
}

func formatAuditRequired(state *app.AuditSchemaState) string {
	if state == nil {
		return "-"
	}
	if state.Required {
		return "yes"
	}

	return "no"
}

func formatAuditSecret(state *app.AuditSchemaState) string {
	if state == nil {
		return "-"
	}
	if state.Secret {
		return "yes"
	}

	return "no"
}

func formatAuditType(state *app.AuditSchemaState) string {
	if state == nil {
		return "-"
	}

	return state.Type
}

func formatAuditDate(fact *app.AuditFact) string {
	if fact == nil || fact.Date.IsZero() {
		return "-"
	}

	return fact.Date.Local().Format("2006-01-02")
}

func formatAuditCommit(fact *app.AuditFact) string {
	if fact == nil || fact.CommitSHA == "" {
		return "-"
	}

	return fact.CommitSHA[:8]
}

func formatAuditDrift(drift app.AuditDriftState) string {
	if drift.State != "drift" {
		return "ok"
	}
	if drift.Since == nil {
		return string(drift.Kind)
	}

	return fmt.Sprintf("%s since %s", drift.Kind, drift.Since.Date.Local().Format("2006-01-02"))
}
