package app

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/mhiro2/envdesk/internal/config"
)

type AuditOptions struct {
	Service     string
	Environment string
	Key         string
}

type AuditResult struct {
	Service     string       `json:"service"`
	Environment string       `json:"environment"`
	Path        string       `json:"path"`
	Entries     []AuditEntry `json:"entries"`
}

type AuditEntry struct {
	Key       string    `json:"key"`
	Author    string    `json:"author"`
	Date      time.Time `json:"date"`
	CommitSHA string    `json:"commit"`
	Summary   string    `json:"summary"`
}

// blameRunner abstracts git blame execution for testing.
type blameRunner interface {
	Run(ctx context.Context, dir, filePath string) ([]byte, error)
}

type gitBlameRunner struct{}

func (*gitBlameRunner) Run(ctx context.Context, dir, filePath string) ([]byte, error) {
	// #nosec G204 -- filePath is derived from project config, not user input.
	cmd := exec.CommandContext(ctx, "git", "blame", "--porcelain", filePath)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("run git blame on %q: %w", filePath, err)
	}
	return out, nil
}

func Audit(ctx context.Context, project *config.Project, opts AuditOptions) ([]AuditResult, error) {
	return auditWith(ctx, project, opts, &gitBlameRunner{})
}

func auditWith(ctx context.Context, project *config.Project, opts AuditOptions, runner blameRunner) ([]AuditResult, error) {
	if err := checkContext(ctx, "audit env files"); err != nil {
		return nil, err
	}

	services, err := selectServices(project, opts.Service)
	if err != nil {
		return nil, err
	}

	var results []AuditResult
	for _, service := range services {
		envs := service.Environments()
		for _, envName := range envs {
			if opts.Environment != "" && envName != opts.Environment {
				continue
			}

			path, err := service.FilePath(envName)
			if err != nil {
				return nil, fmt.Errorf("lookup env file for %s/%s: %w", service.Name, envName, err)
			}

			entries, err := auditFile(ctx, project.BaseDir, path, runner, opts.Key)
			if err != nil {
				return nil, fmt.Errorf("audit %s/%s: %w", service.Name, envName, err)
			}

			results = append(results, AuditResult{
				Service:     service.Name,
				Environment: envName,
				Path:        statusDisplayPath(project.BaseDir, path),
				Entries:     entries,
			})
		}
	}

	return results, nil
}

// blameLine holds parsed porcelain blame data for a single line.
type blameLine struct {
	commitSHA string
	content   string
}

// blameCommit holds metadata for a single commit.
type blameCommit struct {
	author  string
	date    time.Time
	summary string
}

func auditFile(ctx context.Context, baseDir, filePath string, runner blameRunner, keyFilter string) ([]AuditEntry, error) {
	if err := checkContext(ctx, "audit env file"); err != nil {
		return nil, err
	}

	relPath, err := filepath.Rel(baseDir, filePath)
	if err != nil {
		return nil, fmt.Errorf("resolve relative path for %q: %w", filePath, err)
	}

	out, err := runner.Run(ctx, baseDir, relPath)
	if err != nil {
		return nil, fmt.Errorf("run blame: %w", err)
	}

	lines, commits, err := parsePorcelainBlame(out)
	if err != nil {
		return nil, fmt.Errorf("parse blame: %w", err)
	}

	var entries []AuditEntry
	for _, line := range lines {
		key := extractEnvKey(line.content)
		if key == "" {
			continue
		}
		if keyFilter != "" && key != keyFilter {
			continue
		}

		commit, ok := commits[line.commitSHA]
		if !ok {
			continue
		}

		entries = append(entries, AuditEntry{
			Key:       key,
			Author:    commit.author,
			Date:      commit.date,
			CommitSHA: line.commitSHA,
			Summary:   commit.summary,
		})
	}

	return entries, nil
}

var envKeyPattern = regexp.MustCompile(`^(?:export\s+)?([A-Za-z_][A-Za-z0-9_]*)=`)

func extractEnvKey(line string) string {
	m := envKeyPattern.FindStringSubmatch(line)
	if m == nil {
		return ""
	}
	return m[1]
}

func parsePorcelainBlame(data []byte) ([]blameLine, map[string]blameCommit, error) {
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var lines []blameLine
	commits := make(map[string]blameCommit)

	var currentSHA string
	currentCommitData := make(map[string]string)

	for scanner.Scan() {
		text := scanner.Text()

		// A line starting with a hex SHA (40 chars) begins a new blame entry.
		if isSHALine(text) {
			parts := strings.Fields(text)
			currentSHA = parts[0]
			if _, exists := commits[currentSHA]; !exists {
				currentCommitData = make(map[string]string)
			} else {
				currentCommitData = nil
			}
			continue
		}

		// Content line starts with a tab.
		if strings.HasPrefix(text, "\t") {
			lines = append(lines, blameLine{
				commitSHA: currentSHA,
				content:   text[1:],
			})
			continue
		}

		// Header lines for commit metadata.
		if currentCommitData == nil {
			continue
		}

		key, value, ok := strings.Cut(text, " ")
		if !ok {
			continue
		}

		switch key {
		case "author":
			currentCommitData["author"] = value
		case "author-time":
			currentCommitData["author-time"] = value
		case "summary":
			currentCommitData["summary"] = value
			// After summary, we have enough info to save the commit.
			commitTime, _ := parseUnixTimestamp(currentCommitData["author-time"])
			commits[currentSHA] = blameCommit{
				author:  currentCommitData["author"],
				date:    commitTime,
				summary: value,
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, fmt.Errorf("parse git blame output: %w", err)
	}

	return lines, commits, nil
}

var shaPattern = regexp.MustCompile(`^[0-9a-f]{40}\s`)

func isSHALine(line string) bool {
	return shaPattern.MatchString(line)
}

func parseUnixTimestamp(s string) (time.Time, error) {
	var ts int64
	if _, err := fmt.Sscanf(s, "%d", &ts); err != nil {
		return time.Time{}, fmt.Errorf("parse unix timestamp %q: %w", s, err)
	}
	return time.Unix(ts, 0), nil
}
