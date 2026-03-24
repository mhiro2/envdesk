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
		Short: "Show git blame based audit view of env file keys",
		Long: `Show who added or last modified each key in env files,
based on git blame. Useful for auditing when a key was
introduced or changed, and by whom.`,
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
			if _, err := fmt.Fprintln(writer, "KEY\tAUTHOR\tDATE\tCOMMIT\tSUMMARY"); err != nil {
				return fmt.Errorf("write audit output: %w", err)
			}
		} else {
			if _, err := fmt.Fprintln(writer, "KEY\tAUTHOR\tDATE\tSUMMARY"); err != nil {
				return fmt.Errorf("write audit output: %w", err)
			}
		}

		for _, entry := range result.Entries {
			if isVerbose(cmd) {
				if _, err := fmt.Fprintf(
					writer,
					"%s\t%s\t%s\t%s\t%s\n",
					entry.Key,
					entry.Author,
					entry.Date.Local().Format("2006-01-02"),
					entry.CommitSHA[:8],
					entry.Summary,
				); err != nil {
					return fmt.Errorf("write audit output: %w", err)
				}
			} else {
				if _, err := fmt.Fprintf(
					writer,
					"%s\t%s\t%s\t%s\n",
					entry.Key,
					entry.Author,
					entry.Date.Local().Format("2006-01-02"),
					entry.Summary,
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
