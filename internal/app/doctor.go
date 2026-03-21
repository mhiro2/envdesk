package app

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/mhiro2/envdesk/internal/config"
)

type DoctorFinding struct {
	Severity Severity `json:"severity"`
	Check    string   `json:"check"`
	Target   string   `json:"target,omitempty"`
	Message  string   `json:"message"`
}

type DoctorResult struct {
	Healthy        bool            `json:"healthy"`
	RepositoryMode string          `json:"repository_mode"`
	Findings       []DoctorFinding `json:"findings"`
}

type DoctorOptions struct {
	ConfigPath string
}

func Doctor(ctx context.Context, opts DoctorOptions) (*DoctorResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	configPath := filepath.Clean(opts.ConfigPath)
	baseDir := filepath.Dir(configPath)

	result := &DoctorResult{Healthy: true}

	project, err := loadDoctorProject(configPath)
	if err != nil {
		addDoctorFinding(result, SeverityError, "config", doctorDisplayPath(baseDir, configPath), err.Error())
	}

	result.RepositoryMode = detectDoctorMode(baseDir, project)
	if result.RepositoryMode == "mixed" {
		addDoctorFinding(result, SeverityWarning, "mode", "", "detected repository mode: mixed")
	}
	if err == nil && result.RepositoryMode != "plaintext" {
		checkDoctorPermissions(result, baseDir, project)
	}

	if result.RepositoryMode != "plaintext" {
		checkToolAvailability(ctx, result, "sops")
		checkAgeAvailability(ctx, result)
		checkDoctorSOPSConfig(result, baseDir, project)
	}

	trackedFiles, err := gitTrackedFiles(ctx, baseDir)
	if err != nil {
		severity := SeverityError
		if strings.Contains(err.Error(), "not a git repository") {
			severity = SeverityWarning
		}
		addDoctorFinding(result, severity, "git", ".git", err.Error())
	} else if result.RepositoryMode != "plaintext" {
		checkDoctorTrackedPlaintext(result, baseDir, trackedFiles, project)
	}

	if result.RepositoryMode != "plaintext" {
		checkDoctorGitignore(result, baseDir)
	}

	result.Healthy = !hasDoctorErrors(result.Findings)

	return result, nil
}

func addDoctorFinding(result *DoctorResult, severity Severity, check, target, message string) {
	result.Findings = append(result.Findings, DoctorFinding{
		Severity: severity,
		Check:    check,
		Target:   target,
		Message:  message,
	})
}

func hasDoctorErrors(findings []DoctorFinding) bool {
	for _, finding := range findings {
		if finding.Severity == SeverityError {
			return true
		}
	}

	return false
}

func checkToolAvailability(ctx context.Context, result *DoctorResult, tool string) {
	if _, err := exec.LookPath(tool); err != nil {
		addDoctorFinding(result, SeverityError, tool, tool, fmt.Sprintf("locate %s: %v", tool, err))
		return
	}

	_ = ctx
}

func checkAgeAvailability(ctx context.Context, result *DoctorResult) {
	if _, err := exec.LookPath("age"); err == nil {
		return
	}

	problems := make([]string, 0)
	for _, keyPath := range ageKeyPaths() {
		usable, problem := validateAgeKeyFile(keyPath)
		if usable {
			return
		}
		if problem != "" {
			problems = append(problems, problem)
		}
	}

	message := "locate age binary or usable local age key: not found"
	if len(problems) > 0 {
		message = fmt.Sprintf("locate age binary or usable local age key: %s", strings.Join(problems, "; "))
	}

	addDoctorFinding(result, SeverityError, "age", "age", message)
	_ = ctx
}

func validateAgeKeyFile(path string) (bool, string) {
	// #nosec G304 -- the path is resolved from explicit age key file locations.
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, ""
		}

		return false, fmt.Sprintf("read local age key %q: %v", path, err)
	}

	if strings.TrimSpace(string(data)) == "" {
		return false, fmt.Sprintf("read local age key %q: empty file", path)
	}

	return true, ""
}

func detectDoctorMode(baseDir string, project *config.Project) string {
	if _, err := os.Stat(filepath.Join(baseDir, ".sops.yaml")); err == nil {
		if mode := detectDoctorEnvMode(project); mode != "plaintext" {
			return mode
		}
		return "encrypted"
	}

	return detectDoctorEnvMode(project)
}

func detectDoctorEnvMode(project *config.Project) string {
	if project == nil {
		return "plaintext"
	}

	encrypted := 0
	plaintext := 0
	for _, service := range project.Services {
		for _, path := range service.Files {
			// #nosec G304 -- the path comes from project configuration.
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			if isDoctorEncryptedEnv(data) {
				encrypted++
			} else {
				plaintext++
			}
		}
	}

	switch {
	case encrypted > 0 && plaintext > 0:
		return "mixed"
	case encrypted > 0:
		return "encrypted"
	default:
		return "plaintext"
	}
}

func loadDoctorProject(configPath string) (*config.Project, error) {
	if _, err := os.Stat(configPath); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("missing config file")
		}

		return nil, fmt.Errorf("check config file: %w", err)
	}

	project, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	return project, nil
}

func checkDoctorSOPSConfig(result *DoctorResult, baseDir string, project *config.Project) {
	sopsPath := filepath.Join(baseDir, ".sops.yaml")
	// #nosec G304 -- the path is derived from the repository root.
	data, err := os.ReadFile(sopsPath)
	if err != nil {
		if os.IsNotExist(err) {
			addDoctorFinding(result, SeverityError, "sops_config", ".sops.yaml", "missing .sops.yaml")
			return
		}

		addDoctorFinding(result, SeverityError, "sops_config", ".sops.yaml", fmt.Sprintf("read .sops.yaml: %v", err))
		return
	}

	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		addDoctorFinding(result, SeverityError, "sops_config", ".sops.yaml", fmt.Sprintf("parse .sops.yaml: %v", err))
		return
	}

	document, err := doctorSOPSDocument(&root)
	if err != nil {
		addDoctorFinding(result, SeverityError, "sops_config", ".sops.yaml", fmt.Sprintf("validate .sops.yaml: %v", err))
		return
	}

	creationRules, ok := mappingValue(document, "creation_rules")
	if !ok {
		addDoctorFinding(result, SeverityError, "sops_config", ".sops.yaml", "validate .sops.yaml: missing creation_rules")
		return
	}
	if creationRules.Kind != yaml.SequenceNode {
		addDoctorFinding(result, SeverityError, "sops_config", ".sops.yaml", "validate .sops.yaml: creation_rules must be a sequence")
		return
	}
	if len(creationRules.Content) == 0 {
		addDoctorFinding(result, SeverityError, "sops_config", ".sops.yaml", "validate .sops.yaml: no creation rules configured")
		return
	}

	targets := doctorProjectEnvPaths(baseDir, project)
	matchedRules := 0
	hasRuleErrors := false

	for idx, rule := range creationRules.Content {
		re, ruleValid := validateDoctorSOPSRule(result, rule, idx+1)
		if !ruleValid {
			hasRuleErrors = true
			continue
		}

		if len(targets) > 0 && slices.ContainsFunc(targets, re.MatchString) {
			matchedRules++
		}
	}

	if !hasRuleErrors && len(targets) > 0 && matchedRules == 0 {
		addDoctorFinding(result, SeverityWarning, "sops_config", ".sops.yaml", "validate .sops.yaml: no creation rule matches configured env files")
	}
}

func gitTrackedFiles(ctx context.Context, baseDir string) ([]string, error) {
	// #nosec G204 -- the command shape is fixed and only the repository root varies.
	command := exec.CommandContext(ctx, "git", "-C", baseDir, "ls-files", "--cached", "--full-name", "-z")
	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("list tracked files: %w", err)
	}

	files := bytes.Split(output, []byte{0})
	tracked := make([]string, 0, len(files))
	for _, file := range files {
		if len(file) == 0 {
			continue
		}

		tracked = append(tracked, string(file))
	}

	return tracked, nil
}

func checkDoctorTrackedPlaintext(result *DoctorResult, baseDir string, trackedFiles []string, project *config.Project) {
	candidates := make(map[string]struct{})

	for _, path := range trackedFiles {
		if isDoctorEnvCandidate(path) {
			candidates[path] = struct{}{}
		}
	}

	if project != nil {
		for _, service := range project.Services {
			for _, path := range service.Files {
				relative := doctorDisplayPath(baseDir, path)
				if isDoctorEnvCandidate(relative) {
					candidates[relative] = struct{}{}
				}
			}
		}
	}

	paths := make([]string, 0, len(candidates))
	for path := range candidates {
		paths = append(paths, path)
	}
	slices.Sort(paths)

	for _, relative := range paths {
		absPath := filepath.Join(baseDir, filepath.FromSlash(relative))
		// #nosec G304 -- the path comes from tracked git metadata or project config.
		data, err := os.ReadFile(absPath)
		if err != nil {
			continue
		}

		if isDoctorEncryptedEnv(data) {
			continue
		}

		addDoctorFinding(result, SeverityError, "tracked_plaintext", relative, "tracked plaintext env file")
	}
}

func checkDoctorPermissions(result *DoctorResult, baseDir string, project *config.Project) {
	if runtime.GOOS == "windows" {
		return
	}

	paths, err := doctorPermissionPaths(baseDir, project)
	if err != nil {
		addDoctorFinding(result, SeverityWarning, "permissions", "", fmt.Sprintf("walk plaintext env files: %v", err))
		return
	}

	for _, relative := range paths {
		absPath := filepath.Join(baseDir, filepath.FromSlash(relative))
		info, err := os.Stat(absPath)
		if err != nil {
			continue
		}

		if info.Mode().Perm()&0o077 == 0 {
			continue
		}

		// #nosec G304 -- the path comes from project config.
		data, err := os.ReadFile(absPath)
		if err != nil || isDoctorEncryptedEnv(data) {
			continue
		}

		addDoctorFinding(result, SeverityWarning, "permissions", relative, fmt.Sprintf("plaintext env file has permissive mode %03o", info.Mode().Perm()))
	}
}

func doctorPermissionPaths(baseDir string, project *config.Project) ([]string, error) {
	candidates := make(map[string]struct{})
	if project != nil {
		for _, service := range project.Services {
			for _, path := range service.Files {
				candidates[doctorDisplayPath(baseDir, path)] = struct{}{}
			}
		}
	}

	if err := filepath.WalkDir(baseDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}

			return nil
		}

		relative, err := filepath.Rel(baseDir, path)
		if err != nil {
			return fmt.Errorf("resolve permission path %q: %w", path, err)
		}

		displayPath := filepath.ToSlash(relative)
		if !isDoctorEnvCandidate(displayPath) {
			return nil
		}

		candidates[displayPath] = struct{}{}

		return nil
	}); err != nil {
		return nil, fmt.Errorf("walk permission paths: %w", err)
	}

	paths := make([]string, 0, len(candidates))
	for path := range candidates {
		paths = append(paths, path)
	}
	slices.Sort(paths)

	return paths, nil
}

func checkDoctorGitignore(result *DoctorResult, baseDir string) {
	gitignorePath := filepath.Join(baseDir, ".gitignore")
	// #nosec G304 -- the path is derived from the repository root.
	data, err := os.ReadFile(gitignorePath)
	if err != nil {
		if os.IsNotExist(err) {
			addDoctorFinding(result, SeverityWarning, "gitignore", ".gitignore", "missing .gitignore protections for plaintext env files")
			return
		}

		addDoctorFinding(result, SeverityWarning, "gitignore", ".gitignore", fmt.Sprintf("read .gitignore: %v", err))
		return
	}

	if !gitignoreCoversPlaintextEnv(string(data)) {
		addDoctorFinding(result, SeverityWarning, "gitignore", ".gitignore", "missing ignore rules for plaintext env outputs")
	}
}

func gitignoreCoversPlaintextEnv(content string) bool {
	rules := parseGitignoreRules(content)
	for _, candidate := range doctorPlaintextIgnoreCandidates() {
		if gitignoreMatchesPath(candidate, rules) {
			return true
		}
	}

	return false
}

type gitignoreRule struct {
	pattern string
	negate  bool
}

func parseGitignoreRules(content string) []gitignoreRule {
	rules := make([]gitignoreRule, 0)

	for line := range strings.Lines(content) {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		rule := gitignoreRule{pattern: trimmed}
		if strings.HasPrefix(rule.pattern, "!") {
			rule.negate = true
			rule.pattern = strings.TrimSpace(strings.TrimPrefix(rule.pattern, "!"))
		}
		if rule.pattern == "" {
			continue
		}

		rules = append(rules, rule)
	}

	return rules
}

func doctorPlaintextIgnoreCandidates() []string {
	return []string{
		".env.local",
		".envrc.local",
		"env/api/.env.local",
		"env/api/dev.env.local",
		"env/api/dev.local",
		"env/api/dev.local.env",
	}
}

func gitignoreMatchesPath(path string, rules []gitignoreRule) bool {
	ignored := false

	for _, rule := range rules {
		if !gitignoreRuleMatches(rule.pattern, path) {
			continue
		}

		ignored = !rule.negate
	}

	return ignored
}

func gitignoreRuleMatches(pattern, path string) bool {
	directoryOnly := strings.HasSuffix(pattern, "/")
	pattern = strings.TrimSuffix(pattern, "/")
	if pattern == "" {
		return false
	}

	anchored := strings.HasPrefix(pattern, "/")
	pattern = strings.TrimPrefix(pattern, "/")
	hasSlash := strings.Contains(pattern, "/")

	prefix := "(^|.*/)"
	if anchored {
		prefix = "^"
	}

	suffix := "$"
	if directoryOnly {
		suffix = "(/.*)?$"
	}

	re, err := regexp.Compile(prefix + gitignorePatternRegexp(pattern, hasSlash) + suffix)
	if err != nil {
		return false
	}

	return re.MatchString(path)
}

func gitignorePatternRegexp(pattern string, hasSlash bool) string {
	if !hasSlash {
		return gitignoreSegmentRegexp(pattern)
	}

	var builder strings.Builder
	for idx := 0; idx < len(pattern); idx++ {
		if strings.HasPrefix(pattern[idx:], "**/") {
			builder.WriteString("(?:.*/)?")
			idx += 2
			continue
		}

		builder.WriteString(gitignoreSegmentRegexp(string(pattern[idx])))
	}

	return builder.String()
}

func gitignoreSegmentRegexp(pattern string) string {
	var builder strings.Builder

	for idx := 0; idx < len(pattern); idx++ {
		switch ch := pattern[idx]; ch {
		case '*':
			if idx+1 < len(pattern) && pattern[idx+1] == '*' {
				builder.WriteString(".*")
				idx++
				continue
			}

			builder.WriteString("[^/]*")
		case '?':
			builder.WriteString("[^/]")
		case '\\':
			if idx+1 < len(pattern) {
				idx++
				builder.WriteString(regexp.QuoteMeta(string(pattern[idx])))
				continue
			}

			builder.WriteString(regexp.QuoteMeta(string(ch)))
		default:
			builder.WriteString(regexp.QuoteMeta(string(ch)))
		}
	}

	return builder.String()
}

func isDoctorEnvCandidate(path string) bool {
	base := filepath.Base(path)
	if base == ".env" {
		return true
	}
	if strings.HasSuffix(base, ".example") || strings.HasSuffix(base, ".sample") || strings.HasSuffix(base, ".template") {
		return false
	}
	if strings.HasSuffix(base, ".env") {
		return true
	}
	if strings.HasPrefix(base, ".env.") {
		return true
	}
	if strings.HasSuffix(base, ".env.local") || strings.HasSuffix(base, ".local.env") {
		return true
	}

	return false
}

func isDoctorEncryptedEnv(data []byte) bool {
	trimmed := strings.TrimSpace(string(data))
	return strings.Contains(trimmed, "ENC[") || strings.Contains(trimmed, "\nsops:") || strings.HasPrefix(trimmed, "sops:")
}

func ageKeyPaths() []string {
	paths := make([]string, 0, 2)
	if path := os.Getenv("SOPS_AGE_KEY_FILE"); path != "" {
		paths = append(paths, path)
	}

	configDir, err := os.UserConfigDir()
	if err != nil || configDir == "" {
		return paths
	}

	defaultPath := filepath.Join(configDir, "sops", "age", "keys.txt")
	if len(paths) == 0 || paths[len(paths)-1] != defaultPath {
		paths = append(paths, defaultPath)
	}

	return paths
}

func doctorDisplayPath(baseDir, path string) string {
	if path == "" {
		return path
	}

	if filepath.IsAbs(path) {
		relative, err := filepath.Rel(baseDir, path)
		if err == nil {
			return filepath.ToSlash(relative)
		}
	}

	return filepath.ToSlash(filepath.Clean(path))
}

func doctorSOPSDocument(root *yaml.Node) (*yaml.Node, error) {
	if root == nil || len(root.Content) == 0 {
		return nil, fmt.Errorf("empty document")
	}

	document := root.Content[0]
	if document.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("document must be a mapping")
	}

	return document, nil
}

func validateDoctorSOPSRule(result *DoctorResult, rule *yaml.Node, index int) (*regexp.Regexp, bool) {
	if rule.Kind != yaml.MappingNode {
		addDoctorFinding(
			result,
			SeverityError,
			"sops_config",
			".sops.yaml",
			fmt.Sprintf("validate .sops.yaml: creation rule %d must be a mapping", index),
		)
		return nil, false
	}

	pathRegexNode, ok := mappingValue(rule, "path_regex")
	if !ok {
		addDoctorFinding(
			result,
			SeverityError,
			"sops_config",
			".sops.yaml",
			fmt.Sprintf("validate .sops.yaml: creation rule %d missing path_regex", index),
		)
		return nil, false
	}
	if pathRegexNode.Kind != yaml.ScalarNode || strings.TrimSpace(pathRegexNode.Value) == "" {
		addDoctorFinding(
			result,
			SeverityError,
			"sops_config",
			".sops.yaml",
			fmt.Sprintf("validate .sops.yaml: creation rule %d path_regex must be a non-empty scalar", index),
		)
		return nil, false
	}

	re, err := regexp.Compile(pathRegexNode.Value)
	if err != nil {
		addDoctorFinding(
			result,
			SeverityError,
			"sops_config",
			".sops.yaml",
			fmt.Sprintf("validate .sops.yaml: creation rule %d path_regex: %v", index, err),
		)
		return nil, false
	}

	if ageNode, ok := mappingValue(rule, "age"); ok && ageNode.Kind != yaml.SequenceNode {
		addDoctorFinding(
			result,
			SeverityError,
			"sops_config",
			".sops.yaml",
			fmt.Sprintf("validate .sops.yaml: creation rule %d age must be a sequence", index),
		)
		return nil, false
	}

	return re, true
}

func doctorProjectEnvPaths(baseDir string, project *config.Project) []string {
	if project == nil {
		return nil
	}

	paths := make([]string, 0)
	for _, service := range project.Services {
		for _, path := range service.Files {
			paths = append(paths, doctorDisplayPath(baseDir, path))
		}
	}

	slices.Sort(paths)

	return paths
}

func mappingValue(node *yaml.Node, key string) (*yaml.Node, bool) {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil, false
	}

	for idx := 0; idx+1 < len(node.Content); idx += 2 {
		if node.Content[idx].Value == key {
			return node.Content[idx+1], true
		}
	}

	return nil, false
}
