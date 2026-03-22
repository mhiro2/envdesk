package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mhiro2/envdesk/internal/config"
	"github.com/mhiro2/envdesk/internal/crypto"
)

type StatusOptions struct {
	Service string
}

type StatusResult struct {
	Healthy bool          `json:"healthy"`
	Summary StatusSummary `json:"summary"`
	Rows    []StatusRow   `json:"rows"`
}

type StatusSummary struct {
	ServiceCount          int `json:"service_count"`
	EnvironmentCount      int `json:"environment_count"`
	LintErrorCount        int `json:"lint_error_count"`
	LintWarningCount      int `json:"lint_warning_count"`
	DriftIssueCount       int `json:"drift_issue_count"`
	DriftEnvironmentCount int `json:"drift_environment_count"`
}

type StatusRow struct {
	Service     string     `json:"service"`
	Environment string     `json:"environment"`
	Path        string     `json:"path"`
	Lint        StatusLint `json:"lint"`
	Sync        StatusSync `json:"sync"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type StatusLint struct {
	State        string `json:"state"`
	ErrorCount   int    `json:"error_count"`
	WarningCount int    `json:"warning_count"`
}

type StatusSync struct {
	State        string `json:"state"`
	IssueCount   int    `json:"issue_count"`
	ErrorCount   int    `json:"error_count"`
	WarningCount int    `json:"warning_count"`
	MissingCount int    `json:"missing_count"`
	PresentCount int    `json:"present_count"`
}

func Status(ctx context.Context, project *config.Project, adapter crypto.Adapter, opts StatusOptions) (*StatusResult, error) {
	if adapter == nil {
		return nil, fmt.Errorf("status env files: missing crypto adapter")
	}
	if err := checkContext(ctx, "status env files"); err != nil {
		return nil, err
	}

	lintResult, err := Lint(ctx, project, adapter, LintOptions{
		Service: opts.Service,
	})
	if err != nil {
		return nil, err
	}

	issues, err := CheckSync(ctx, project, adapter, CheckSyncOptions{
		Service: opts.Service,
	})
	if err != nil {
		return nil, err
	}

	services, err := selectServices(project, opts.Service)
	if err != nil {
		return nil, err
	}

	rows := make([]StatusRow, 0)
	rowIndex := make(map[string]int)

	for _, service := range services {
		for _, envName := range service.Environments() {
			path, err := service.FilePath(envName)
			if err != nil {
				return nil, fmt.Errorf("lookup env file for %s/%s: %w", service.Name, envName, err)
			}

			info, err := os.Stat(path)
			if err != nil {
				return nil, fmt.Errorf("stat env file for %s/%s: %w", service.Name, envName, err)
			}

			rows = append(rows, StatusRow{
				Service:     service.Name,
				Environment: envName,
				Path:        statusDisplayPath(project.BaseDir, path),
				Lint: StatusLint{
					State: "ok",
				},
				Sync: StatusSync{
					State: "in_sync",
				},
				UpdatedAt: info.ModTime(),
			})

			rowIndex[statusRowKey(service.Name, envName)] = len(rows) - 1
		}
	}

	for _, problem := range lintResult.Problems {
		idx, ok := rowIndex[statusRowKey(problem.Service, problem.Environment)]
		if !ok {
			continue
		}

		row := &rows[idx]
		if problem.Severity == SeverityError {
			row.Lint.ErrorCount++
		} else {
			row.Lint.WarningCount++
		}
	}

	for idx := range rows {
		rows[idx].Lint.State = statusLintState(rows[idx].Lint)
	}

	for _, issue := range issues {
		for _, envName := range issue.Missing {
			idx, ok := rowIndex[statusRowKey(issue.Service, envName)]
			if !ok {
				continue
			}

			row := &rows[idx]
			row.Sync.IssueCount++
			row.Sync.MissingCount++
			if issue.Severity == SeverityError {
				row.Sync.ErrorCount++
			} else {
				row.Sync.WarningCount++
			}
		}

		for _, envName := range issue.Present {
			idx, ok := rowIndex[statusRowKey(issue.Service, envName)]
			if !ok {
				continue
			}

			row := &rows[idx]
			row.Sync.IssueCount++
			row.Sync.PresentCount++
			if issue.Severity == SeverityError {
				row.Sync.ErrorCount++
			} else {
				row.Sync.WarningCount++
			}
		}
	}

	driftEnvironmentCount := 0
	for idx := range rows {
		rows[idx].Sync.State = statusSyncState(rows[idx].Sync)
		if rows[idx].Sync.IssueCount > 0 {
			driftEnvironmentCount++
		}
	}

	return &StatusResult{
		Healthy: lintResult.ErrorCount == 0 && len(issues) == 0,
		Summary: StatusSummary{
			ServiceCount:          len(services),
			EnvironmentCount:      len(rows),
			LintErrorCount:        lintResult.ErrorCount,
			LintWarningCount:      lintResult.WarningCount,
			DriftIssueCount:       len(issues),
			DriftEnvironmentCount: driftEnvironmentCount,
		},
		Rows: rows,
	}, nil
}

func statusRowKey(service, environment string) string {
	return service + "\x00" + environment
}

func statusLintState(lint StatusLint) string {
	switch {
	case lint.ErrorCount > 0:
		return "error"
	case lint.WarningCount > 0:
		return "warning"
	default:
		return "ok"
	}
}

func statusSyncState(sync StatusSync) string {
	if sync.IssueCount > 0 {
		return "drift"
	}

	return "in_sync"
}

func statusDisplayPath(baseDir, path string) string {
	relative, err := filepath.Rel(baseDir, path)
	if err != nil {
		return filepath.ToSlash(filepath.Clean(path))
	}

	return filepath.ToSlash(strings.TrimPrefix(relative, "./"))
}
