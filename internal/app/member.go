package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mhiro2/envdesk/internal/config"
	"github.com/mhiro2/envdesk/internal/crypto"
	"github.com/mhiro2/envdesk/internal/sopsconfig"
)

type MemberOptions struct {
	Recipient string
	Scope     string
	Rekey     bool
	DryRun    bool
}

type MemberResult struct {
	ConfigPath    string       `json:"config_path"`
	DryRun        bool         `json:"dry_run,omitempty"`
	Recipient     string       `json:"recipient"`
	AffectedFiles []string     `json:"affected_files,omitempty"`
	MatchedRules  int          `json:"matched_rules"`
	RekeyedFiles  []string     `json:"rekeyed_files"`
	Errors        []RekeyError `json:"errors,omitempty"`
}

func MemberAdd(ctx context.Context, project *config.Project, adapter crypto.Adapter, opts MemberOptions) (*MemberResult, error) {
	return updateMemberRecipients(ctx, project, adapter, opts, false)
}

func MemberRemove(ctx context.Context, project *config.Project, adapter crypto.Adapter, opts MemberOptions) (*MemberResult, error) {
	return updateMemberRecipients(ctx, project, adapter, opts, true)
}

func updateMemberRecipients(ctx context.Context, project *config.Project, adapter crypto.Adapter, opts MemberOptions, remove bool) (*MemberResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if project == nil {
		return nil, fmt.Errorf("resolve project: nil project")
	}
	if adapter == nil {
		return nil, fmt.Errorf("resolve crypto adapter: nil adapter")
	}

	recipient, err := resolveRecipient(project.BaseDir, opts.Recipient)
	if err != nil {
		return nil, err
	}

	sopsPath := filepath.Join(filepath.Dir(project.ConfigPath), ".sops.yaml")
	sopsCfg, err := sopsconfig.Load(sopsPath)
	if err != nil {
		return nil, fmt.Errorf("load sops config: %w", err)
	}

	targets, err := selectEnvTargets(project, opts.Scope, "")
	if err != nil {
		return nil, err
	}

	targetPaths := make([]string, 0, len(targets))
	for _, target := range targets {
		targetPaths = append(targetPaths, relPath(project.BaseDir, target.Path))
	}

	updateResult, err := sopsCfg.UpdateRecipients(targetPaths, recipient, remove)
	if err != nil {
		return nil, fmt.Errorf("update sops recipients: %w", err)
	}

	// Collect ALL files affected by the matched creation rules, not just scope-filtered
	// targets. A shared rule may cover files outside the scope, and those files are also
	// affected by the recipient change.
	allTargets, err := selectEnvTargets(project, "", "")
	if err != nil {
		return nil, fmt.Errorf("enumerate project files: %w", err)
	}

	affectedTargets := filterTargetsByPathRegexes(project.BaseDir, allTargets, updateResult.MatchedPathRegexes)
	affectedFiles := make([]string, 0, len(affectedTargets))
	for _, t := range affectedTargets {
		affectedFiles = append(affectedFiles, t.Path)
	}

	result := &MemberResult{
		ConfigPath:    sopsPath,
		DryRun:        opts.DryRun,
		Recipient:     recipient,
		AffectedFiles: affectedFiles,
		MatchedRules:  updateResult.MatchedRules,
	}

	if opts.DryRun {
		return result, nil
	}

	if err := sopsCfg.Write(); err != nil {
		return nil, fmt.Errorf("write sops config: %w", err)
	}

	if !opts.Rekey {
		return result, nil
	}

	// Rekey all affected files, not just scope targets.
	rekeyed, rekeyErrors, err := rekeyTargets(ctx, adapter, affectedTargets)
	result.RekeyedFiles = rekeyed
	result.Errors = rekeyErrors
	if err != nil {
		return result, err
	}

	return result, nil
}

func filterTargetsByPathRegexes(baseDir string, targets []EnvTarget, patterns []string) []EnvTarget {
	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		re, err := regexp.Compile(p)
		if err != nil {
			continue
		}
		compiled = append(compiled, re)
	}

	filtered := make([]EnvTarget, 0, len(targets))
	for _, t := range targets {
		rel := relPath(baseDir, t.Path)
		for _, re := range compiled {
			if re.MatchString(rel) {
				filtered = append(filtered, t)
				break
			}
		}
	}

	return filtered
}

func resolveRecipient(baseDir, raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("resolve recipient: empty recipient")
	}

	if recipient, ok, err := readRecipientFile(trimmed, raw); err != nil {
		return "", err
	} else if ok {
		return recipient, nil
	}

	if !filepath.IsAbs(trimmed) {
		resolved := filepath.Join(baseDir, trimmed)
		if recipient, ok, err := readRecipientFile(resolved, raw); err != nil {
			return "", err
		} else if ok {
			return recipient, nil
		}
	}

	return trimmed, nil
}

func readRecipientFile(path, original string) (string, bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}

		return "", false, fmt.Errorf("read recipient %q: %w", original, err)
	}
	if info.IsDir() {
		return "", false, fmt.Errorf("read recipient file %q: is a directory", original)
	}

	// #nosec G304 -- the recipient file is selected explicitly by the caller.
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false, fmt.Errorf("read recipient file %q: %w", original, err)
	}

	recipient := strings.TrimSpace(string(data))
	if recipient == "" {
		return "", false, fmt.Errorf("resolve recipient file %q: empty recipient", original)
	}

	return recipient, true, nil
}
