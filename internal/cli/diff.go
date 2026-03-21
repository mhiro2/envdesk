package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mhiro2/envdesk/internal/app"
)

func newDiffCommand() *cobra.Command {
	var valueMode string
	var showMetadata bool
	var jsonOutput bool
	var ciSummary bool

	cmd := &cobra.Command{
		Use:               "diff <service> <from> <to>",
		Short:             "Compare env structure across environments",
		Args:              cobra.ExactArgs(3),
		ValidArgsFunction: completeDiffArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			project, err := loadProject(cmd)
			if err != nil {
				return err
			}

			result, err := app.Diff(cmd.Context(), project, newCryptoAdapter(), args[0], args[1], args[2], app.DiffOptions{
				ValueMode:    app.DiffValueMode(valueMode),
				ShowMetadata: showMetadata,
			})
			if err != nil {
				return fmt.Errorf("diff env files: %w", err)
			}

			if jsonOutput {
				if err := writeJSON(cmd.OutOrStdout(), result); err != nil {
					return err
				}

				if ciSummary && result.Summary.Total > 0 {
					return fmt.Errorf("diff env files: found %d changes", result.Summary.Total)
				}

				return nil
			}

			if ciSummary {
				if summaryPath := os.Getenv("GITHUB_STEP_SUMMARY"); summaryPath != "" {
					_ = writeDiffGitHubSummary(summaryPath, result)
				}

				line := formatDiffSummary(result)

				if _, err := fmt.Fprintln(cmd.OutOrStdout(), line); err != nil {
					return fmt.Errorf("write diff output: %w", err)
				}

				if result.Summary.Total > 0 {
					return fmt.Errorf("diff env files: found %d changes", result.Summary.Total)
				}

				return nil
			}

			for _, change := range result.Changes {
				line := colorDiffPrefix(change.Type) + " " + change.Key + formatMetadata(change.Metadata)
				if detail := formatDiffDetail(change.Type, change.From, change.To, app.DiffValueMode(valueMode)); detail != "" {
					line += detail
				}

				if _, err := fmt.Fprintln(cmd.OutOrStdout(), line); err != nil {
					return fmt.Errorf("write diff output: %w", err)
				}
			}

			if showMetadata {
				for _, candidate := range result.RenameCandidates {
					line := "~ " + candidate.From + " -> " + candidate.To
					if candidate.Metadata != nil {
						line += formatMetadata(candidate.Metadata)
					}
					if valueMode != "" {
						line += " = " + strconv.Quote(candidate.Value)
					}
					line += " [rename candidate]"

					if _, err := fmt.Fprintln(cmd.OutOrStdout(), line); err != nil {
						return fmt.Errorf("write diff output: %w", err)
					}
				}

				for _, finding := range result.Findings {
					line := fmt.Sprintf("%s %s/%s: %s", finding.Severity, finding.Environment, finding.Key, finding.Message)
					if _, err := fmt.Fprintln(cmd.OutOrStdout(), line); err != nil {
						return fmt.Errorf("write diff output: %w", err)
					}
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&valueMode, "value-mode", "", "Show diff details using hidden, hash, public, or all values")
	cmd.Flags().BoolVar(&showMetadata, "show-metadata", false, "Show schema metadata")
	cmd.Flags().BoolVar(&ciSummary, "ci-summary", false, "Print a concise summary and fail when changes are present")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable diff output")

	_ = cmd.RegisterFlagCompletionFunc("value-mode", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return []string{"hidden", "hash", "public", "all"}, cobra.ShellCompDirectiveNoFileComp
	})

	return cmd
}

func formatDiffSummary(result *app.DiffResult) string {
	if result.Summary.Total == 0 {
		return fmt.Sprintf("%s %s..%s: no changes", result.Service, result.From, result.To)
	}

	parts := make([]string, 0, 5)
	if result.Summary.Added > 0 {
		parts = append(parts, fmt.Sprintf("%d added", result.Summary.Added))
	}
	if result.Summary.Removed > 0 {
		parts = append(parts, fmt.Sprintf("%d removed", result.Summary.Removed))
	}
	if result.Summary.Modified > 0 {
		parts = append(parts, fmt.Sprintf("%d modified", result.Summary.Modified))
	}
	if result.Summary.Renamed > 0 {
		parts = append(parts, fmt.Sprintf("%d rename candidates", result.Summary.Renamed))
	}
	if result.Summary.Violations > 0 {
		parts = append(parts, fmt.Sprintf("%d schema findings", result.Summary.Violations))
	}

	return fmt.Sprintf(
		"%s %s..%s: %d changes (%s)",
		result.Service,
		result.From,
		result.To,
		result.Summary.Total,
		strings.Join(parts, ", "),
	)
}

func writeDiffGitHubSummary(path string, result *app.DiffResult) error {
	var buf strings.Builder
	buf.WriteString("### envdesk diff: " + result.Service + " " + result.From + ".." + result.To + "\n\n")
	if result.Summary.Total == 0 {
		buf.WriteString("No changes detected.\n")
	} else {
		fmt.Fprintf(&buf, "**%d changes** found\n\n", result.Summary.Total)
		buf.WriteString("| Type | Key |\n|---|---|\n")
		for _, change := range result.Changes {
			fmt.Fprintf(&buf, "| %s | `%s` |\n", change.Type, change.Key)
		}
	}
	if len(result.RenameCandidates) > 0 {
		buf.WriteString("\n#### Rename Candidates\n\n| From | To |\n|---|---|\n")
		for _, candidate := range result.RenameCandidates {
			fmt.Fprintf(&buf, "| `%s` | `%s` |\n", candidate.From, candidate.To)
		}
	}
	if len(result.Findings) > 0 {
		buf.WriteString("\n#### Schema Findings\n\n| Severity | Environment | Key | Message |\n|---|---|---|---|\n")
		for _, finding := range result.Findings {
			fmt.Fprintf(&buf, "| %s | `%s` | `%s` | %s |\n", finding.Severity, finding.Environment, finding.Key, finding.Message)
		}
	}
	// #nosec G304 -- path is from a trusted environment variable set by GitHub Actions.
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) //nolint:gosec // trusted env var from GitHub Actions
	if err != nil {
		return fmt.Errorf("open summary file: %w", err)
	}
	defer func() { _ = f.Close() }()
	if _, err := f.WriteString(buf.String()); err != nil {
		return fmt.Errorf("write summary: %w", err)
	}
	return nil
}

func formatDiffDetail(changeType, fromValue, toValue string, mode app.DiffValueMode) string {
	if mode == app.DiffValueModeNone {
		return ""
	}

	switch changeType {
	case "add":
		return "=" + strconv.Quote(toValue)
	case "remove":
		return "=" + strconv.Quote(fromValue)
	case "modify":
		return ": " + strconv.Quote(fromValue) + " -> " + strconv.Quote(toValue)
	default:
		return ""
	}
}
