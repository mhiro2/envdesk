package cli

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mhiro2/envdesk/internal/app"
)

func newCheckSyncCommand() *cobra.Command {
	var service string
	var jsonOutput bool
	var ciSummary bool
	var strictRequiredOnly bool

	cmd := &cobra.Command{
		Use:   "check-sync",
		Short: "Detect key drift across environments",
		RunE: func(cmd *cobra.Command, args []string) error {
			project, err := loadProject(cmd)
			if err != nil {
				return err
			}

			issues, err := app.CheckSync(cmd.Context(), project, newCryptoAdapter(), app.CheckSyncOptions{
				Service:            service,
				StrictRequiredOnly: strictRequiredOnly,
			})
			if err != nil {
				return fmt.Errorf("check key sync: %w", err)
			}

			switch {
			case jsonOutput:
				if err := writeJSON(cmd.OutOrStdout(), issues); err != nil {
					return err
				}
			case ciSummary:
				if summaryPath := os.Getenv("GITHUB_STEP_SUMMARY"); summaryPath != "" {
					_ = writeCheckSyncGitHubSummary(summaryPath, issues)
				}
				lines := checkSyncSummaryLines(issues)
				if len(lines) == 0 {
					if _, err := fmt.Fprintln(cmd.OutOrStdout(), "all environments are in sync"); err != nil {
						return fmt.Errorf("write check-sync output: %w", err)
					}
				} else {
					for _, line := range lines {
						if _, err := fmt.Fprintln(cmd.OutOrStdout(), line); err != nil {
							return fmt.Errorf("write check-sync output: %w", err)
						}
					}
				}
			default:
				for _, issue := range issues {
					line := fmt.Sprintf(
						"%s: %s missing in %s (present in %s) [%s, %s]",
						issue.Service,
						issue.Key,
						strings.Join(issue.Missing, ","),
						strings.Join(issue.Present, ","),
						issue.Kind,
						issue.Severity,
					)
					if _, err := fmt.Fprintln(cmd.OutOrStdout(), line); err != nil {
						return fmt.Errorf("write check-sync output: %w", err)
					}
				}

				if len(issues) == 0 {
					if _, err := fmt.Fprintln(cmd.OutOrStdout(), "all environments are in sync"); err != nil {
						return fmt.Errorf("write check-sync output: %w", err)
					}
				}
			}

			if len(issues) > 0 && (!jsonOutput || ciSummary) {
				return withExitCode(fmt.Errorf("check key sync: found %d drift issues", len(issues)))
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&service, "service", "", "Limit checks to a single service")
	cmd.Flags().BoolVar(&strictRequiredOnly, "strict-required-only", false, "Only report drift for required schema keys")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable output")
	cmd.Flags().BoolVar(&ciSummary, "ci-summary", false, "Print a concise summary and fail when drift is present")

	_ = cmd.RegisterFlagCompletionFunc("service", completeServiceFlag)

	return cmd
}

func checkSyncSummaryLines(issues []app.SyncIssue) []string {
	byService := make(map[string][]app.SyncIssue)
	for _, issue := range issues {
		byService[issue.Service] = append(byService[issue.Service], issue)
	}

	lines := make([]string, 0, len(byService))
	for _, service := range sortedSyncServices(byService) {
		serviceIssues := byService[service]
		envs := make(map[string]struct{})
		for _, issue := range serviceIssues {
			for _, envName := range issue.Missing {
				envs[envName] = struct{}{}
			}
			for _, envName := range issue.Present {
				envs[envName] = struct{}{}
			}
		}

		envNames := make([]string, 0, len(envs))
		for envName := range envs {
			envNames = append(envNames, envName)
		}
		slices.Sort(envNames)

		required := 0
		optional := 0
		undeclared := 0
		untracked := 0
		for _, issue := range serviceIssues {
			switch issue.Kind {
			case app.SyncIssueKindRequired:
				required++
			case app.SyncIssueKindOptional:
				optional++
			case app.SyncIssueKindUndeclared:
				undeclared++
			case app.SyncIssueKindUntracked:
				untracked++
			}
		}

		line := fmt.Sprintf("%s: %d drift issues", service, len(serviceIssues))
		if len(envNames) > 0 {
			line += " across " + strings.Join(envNames, ", ")
		}
		parts := make([]string, 0, 4)
		if required > 0 {
			parts = append(parts, fmt.Sprintf("%d required", required))
		}
		if optional > 0 {
			parts = append(parts, fmt.Sprintf("%d optional", optional))
		}
		if undeclared > 0 {
			parts = append(parts, fmt.Sprintf("%d undeclared", undeclared))
		}
		if untracked > 0 {
			parts = append(parts, fmt.Sprintf("%d untracked", untracked))
		}
		if len(parts) > 0 {
			line += " (" + strings.Join(parts, ", ") + ")"
		}

		lines = append(lines, line)
	}

	return lines
}

func sortedSyncServices(groups map[string][]app.SyncIssue) []string {
	services := make([]string, 0, len(groups))
	for service := range groups {
		services = append(services, service)
	}

	slices.Sort(services)

	return services
}

func writeCheckSyncGitHubSummary(path string, issues []app.SyncIssue) error {
	var buf strings.Builder
	buf.WriteString("### envdesk check-sync\n\n")
	if len(issues) == 0 {
		buf.WriteString("All environments are in sync.\n")
	} else {
		fmt.Fprintf(&buf, "**%d drift issues** found\n\n", len(issues))
		buf.WriteString("| Service | Key | Kind | Missing | Present |\n|---|---|---|---|---|\n")
		for _, issue := range issues {
			fmt.Fprintf(&buf, "| %s | `%s` | %s | `%s` | `%s` |\n", issue.Service, issue.Key, issue.Kind, strings.Join(issue.Missing, ","), strings.Join(issue.Present, ","))
		}
	}

	// #nosec G304 -- path is from a trusted environment variable set by GitHub Actions.
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
