package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/mhiro2/envdesk/internal/atomicwrite"
	"github.com/mhiro2/envdesk/internal/config"
	"github.com/mhiro2/envdesk/internal/crypto"
	"github.com/mhiro2/envdesk/internal/envfile"
	"github.com/mhiro2/envdesk/internal/schema"
)

type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

type Problem struct {
	Severity    Severity `json:"severity"`
	Service     string   `json:"service"`
	Environment string   `json:"environment"`
	Key         string   `json:"key,omitempty"`
	Message     string   `json:"message"`
}

type LintResult struct {
	Healthy      bool      `json:"healthy"`
	Problems     []Problem `json:"problems"`
	ErrorCount   int       `json:"error_count"`
	WarningCount int       `json:"warning_count"`
}

type LintOptions struct {
	Service     string
	Environment string
}

type DiffValueMode string

const (
	DiffValueModeNone   DiffValueMode = ""
	DiffValueModeHidden DiffValueMode = "hidden"
	DiffValueModeHash   DiffValueMode = "hash"
	DiffValueModePublic DiffValueMode = "public"
	DiffValueModeAll    DiffValueMode = "all"
)

type DiffOptions struct {
	ValueMode    DiffValueMode
	ShowMetadata bool
}

type DiffResult struct {
	Service          string                `json:"service"`
	From             string                `json:"from"`
	To               string                `json:"to"`
	Changes          []DiffChange          `json:"changes"`
	RenameCandidates []DiffRenameCandidate `json:"rename_candidates,omitempty"`
	Findings         []DiffFinding         `json:"findings,omitempty"`
	Summary          DiffSummary           `json:"summary"`
}

type DiffSummary struct {
	Added      int `json:"added"`
	Removed    int `json:"removed"`
	Modified   int `json:"modified"`
	Renamed    int `json:"renamed"`
	Violations int `json:"violations"`
	Total      int `json:"total"`
}

type DiffChange struct {
	Type     string      `json:"type"`
	Key      string      `json:"key"`
	From     string      `json:"from,omitempty"`
	To       string      `json:"to,omitempty"`
	Metadata *schema.Key `json:"metadata,omitempty"`
}

type DiffRenameCandidate struct {
	From     string      `json:"from"`
	To       string      `json:"to"`
	Value    string      `json:"value,omitempty"`
	Metadata *schema.Key `json:"metadata,omitempty"`
}

type DiffFinding struct {
	Severity    Severity `json:"severity"`
	Environment string   `json:"environment"`
	Key         string   `json:"key"`
	Message     string   `json:"message"`
}

type SyncIssue struct {
	Service  string        `json:"service"`
	Key      string        `json:"key"`
	Kind     SyncIssueKind `json:"kind"`
	Severity Severity      `json:"severity"`
	Missing  []string      `json:"missing"`
	Present  []string      `json:"present"`
}

type SyncIssueKind string

const (
	SyncIssueKindRequired   SyncIssueKind = "required"
	SyncIssueKindOptional   SyncIssueKind = "optional"
	SyncIssueKindUndeclared SyncIssueKind = "undeclared"
	SyncIssueKindUntracked  SyncIssueKind = "untracked"
)

type CheckSyncOptions struct {
	Service            string
	StrictRequiredOnly bool
}

type SyncKeysOptions struct {
	Service            string
	SourceEnvironment  string
	TargetEnvironments []string
	DryRun             bool
	Placeholders       bool
}

type SyncKeysResult struct {
	Service            string             `json:"service"`
	SourceEnvironment  string             `json:"source_environment"`
	TargetEnvironments []TargetSyncResult `json:"target_environments"`
}

type EnvTarget struct {
	Service     string
	Environment string
	Path        string
}

type TargetSyncResult struct {
	Environment string   `json:"environment"`
	Added       []string `json:"added"`
	Removed     []string `json:"removed"`
	Changed     bool     `json:"changed"`
}

func Lint(ctx context.Context, project *config.Project, adapter crypto.Adapter, opts LintOptions) (*LintResult, error) {
	if adapter == nil {
		return nil, fmt.Errorf("lint env files: missing crypto adapter")
	}
	if err := checkContext(ctx, "lint env files"); err != nil {
		return nil, err
	}

	services, err := selectServices(project, opts.Service)
	if err != nil {
		return nil, err
	}

	problems := make([]Problem, 0)
	matchedEnvironment := false

	for _, service := range services {
		if err := checkContext(ctx, "lint env files"); err != nil {
			return nil, err
		}

		var loadedSchema *schema.Schema
		if service.SchemaPath != "" {
			loadedSchema, err = schema.Load(service.SchemaPath)
			if err != nil {
				return nil, fmt.Errorf("load schema for service %q: %w", service.Name, err)
			}
		}

		for _, envName := range service.Environments() {
			if err := checkContext(ctx, "lint env files"); err != nil {
				return nil, err
			}

			if opts.Environment != "" && opts.Environment != envName {
				continue
			}

			matchedEnvironment = true

			doc, err := decryptAndParse(ctx, adapter, service, envName)
			if err != nil {
				return nil, err
			}

			problems = append(problems, lintDocument(service.Name, envName, doc, loadedSchema)...)
		}
	}

	if opts.Environment != "" && !matchedEnvironment {
		return nil, fmt.Errorf("select environment %q: not configured", opts.Environment)
	}

	result := &LintResult{
		Problems: problems,
	}
	for _, problem := range problems {
		if problem.Severity == SeverityError {
			result.ErrorCount++
			continue
		}

		result.WarningCount++
	}
	result.Healthy = result.ErrorCount == 0

	return result, nil
}

func Diff(ctx context.Context, project *config.Project, adapter crypto.Adapter, serviceName, fromEnv, toEnv string, opts DiffOptions) (*DiffResult, error) {
	if adapter == nil {
		return nil, fmt.Errorf("diff env files: missing crypto adapter")
	}
	if !isValidDiffValueMode(opts.ValueMode) {
		return nil, fmt.Errorf("diff env files: invalid value mode %q", opts.ValueMode)
	}

	service, err := project.Service(serviceName)
	if err != nil {
		return nil, fmt.Errorf("lookup service %q: %w", serviceName, err)
	}

	loadedSchema, err := loadServiceSchema(service)
	if err != nil {
		return nil, err
	}

	fromDoc, err := decryptAndParse(ctx, adapter, service, fromEnv)
	if err != nil {
		return nil, err
	}

	toDoc, err := decryptAndParse(ctx, adapter, service, toEnv)
	if err != nil {
		return nil, err
	}

	keys := unionKeys(fromDoc.Keys(), toDoc.Keys())
	changes := make([]DiffChange, 0)
	summary := DiffSummary{}

	for _, key := range keys {
		fromValue, inFrom := fromDoc.Lookup(key)
		toValue, inTo := toDoc.Lookup(key)

		switch {
		case !inFrom && inTo:
			summary.Added++
			change := DiffChange{Type: "add", Key: key}
			if opts.ValueMode != DiffValueModeNone {
				change.To = renderDiffValue(toValue, schemaKeyMeta(loadedSchema, key), opts.ValueMode)
			}
			if opts.ShowMetadata && loadedSchema != nil {
				if meta, ok := loadedSchema.Keys[key]; ok {
					change.Metadata = &meta
				}
			}
			changes = append(changes, change)
		case inFrom && !inTo:
			summary.Removed++
			change := DiffChange{Type: "remove", Key: key}
			if opts.ValueMode != DiffValueModeNone {
				change.From = renderDiffValue(fromValue, schemaKeyMeta(loadedSchema, key), opts.ValueMode)
			}
			if opts.ShowMetadata && loadedSchema != nil {
				if meta, ok := loadedSchema.Keys[key]; ok {
					change.Metadata = &meta
				}
			}
			changes = append(changes, change)
		case inFrom && inTo && fromValue != toValue:
			summary.Modified++
			change := DiffChange{Type: "modify", Key: key}
			if opts.ValueMode != DiffValueModeNone {
				meta := schemaKeyMeta(loadedSchema, key)
				change.From = renderDiffValue(fromValue, meta, opts.ValueMode)
				change.To = renderDiffValue(toValue, meta, opts.ValueMode)
			}
			if opts.ShowMetadata && loadedSchema != nil {
				if meta, ok := loadedSchema.Keys[key]; ok {
					change.Metadata = &meta
				}
			}
			changes = append(changes, change)
		}
	}

	renameCandidates := detectRenameCandidates(fromDoc, toDoc, loadedSchema, opts.ValueMode)
	var findings []DiffFinding
	if opts.ShowMetadata && loadedSchema != nil {
		findings = slices.Concat(collectDiffFindings(fromEnv, fromDoc, loadedSchema), collectDiffFindings(toEnv, toDoc, loadedSchema))
	}

	summary.Renamed = len(renameCandidates)
	summary.Violations = len(findings)
	summary.Total = summary.Added + summary.Removed + summary.Modified + summary.Violations

	return &DiffResult{
		Service:          serviceName,
		From:             fromEnv,
		To:               toEnv,
		Changes:          changes,
		RenameCandidates: renameCandidates,
		Findings:         findings,
		Summary:          summary,
	}, nil
}

func CheckSync(ctx context.Context, project *config.Project, adapter crypto.Adapter, opts CheckSyncOptions) ([]SyncIssue, error) {
	if adapter == nil {
		return nil, fmt.Errorf("check sync: missing crypto adapter")
	}
	if err := checkContext(ctx, "check key sync"); err != nil {
		return nil, err
	}

	services, err := selectServices(project, opts.Service)
	if err != nil {
		return nil, err
	}

	issues := make([]SyncIssue, 0)

	for _, service := range services {
		if err := checkContext(ctx, "check key sync"); err != nil {
			return nil, err
		}

		loadedSchema, err := loadServiceSchema(service)
		if err != nil {
			return nil, err
		}

		envNames := service.Environments()
		envDocs := make(map[string]*envfile.Document, len(envNames))
		for _, envName := range envNames {
			if err := checkContext(ctx, "check key sync"); err != nil {
				return nil, err
			}

			doc, err := decryptAndParse(ctx, adapter, service, envName)
			if err != nil {
				return nil, err
			}

			envDocs[envName] = doc
		}

		issues = append(issues, syncIssuesForService(service.Name, envNames, envDocs, loadedSchema, opts.StrictRequiredOnly)...)
	}

	slices.SortFunc(issues, func(a, b SyncIssue) int {
		if a.Service == b.Service {
			return strings.Compare(a.Key, b.Key)
		}

		return strings.Compare(a.Service, b.Service)
	})

	return issues, nil
}

func syncIssuesForService(serviceName string, envNames []string, envDocs map[string]*envfile.Document, loadedSchema *schema.Schema, strictRequiredOnly bool) []SyncIssue {
	keys := make([]string, 0)
	for _, doc := range envDocs {
		if doc == nil {
			continue
		}

		keys = append(keys, doc.Keys()...)
	}
	keys = unionKeys(keys)

	issues := make([]SyncIssue, 0)

	for _, key := range keys {
		missing := make([]string, 0)
		present := make([]string, 0)

		for _, envName := range envNames {
			if doc := envDocs[envName]; doc != nil {
				if _, ok := doc.Lookup(key); ok {
					present = append(present, envName)
					continue
				}
			}

			missing = append(missing, envName)
		}

		if len(missing) == 0 || len(present) == 0 {
			continue
		}

		kind, severity := classifySyncIssue(loadedSchema, key)
		if strictRequiredOnly && kind != SyncIssueKindRequired {
			continue
		}

		issues = append(issues, SyncIssue{
			Service:  serviceName,
			Key:      key,
			Kind:     kind,
			Severity: severity,
			Missing:  missing,
			Present:  present,
		})
	}

	return issues
}

func SyncKeys(ctx context.Context, project *config.Project, adapter crypto.Adapter, opts SyncKeysOptions) (*SyncKeysResult, error) {
	if adapter == nil {
		return nil, fmt.Errorf("sync keys: missing crypto adapter")
	}
	if err := checkContext(ctx, "sync keys"); err != nil {
		return nil, err
	}

	service, err := project.Service(opts.Service)
	if err != nil {
		return nil, fmt.Errorf("lookup service %q: %w", opts.Service, err)
	}

	loadedSchema, err := loadServiceSchema(service)
	if err != nil {
		return nil, err
	}

	sourceDoc, err := decryptAndParse(ctx, adapter, service, opts.SourceEnvironment)
	if err != nil {
		return nil, err
	}

	targetEnvs, err := selectTargetEnvironments(service, opts.SourceEnvironment, opts.TargetEnvironments)
	if err != nil {
		return nil, err
	}

	results := make([]TargetSyncResult, 0, len(targetEnvs))
	sourceValues := sourceDoc.Values()

	for _, targetEnv := range targetEnvs {
		if err := checkContext(ctx, "sync keys"); err != nil {
			return nil, err
		}

		targetPath, err := service.FilePath(targetEnv)
		if err != nil {
			return nil, fmt.Errorf("lookup target env file for %s/%s: %w", service.Name, targetEnv, err)
		}

		targetDoc, err := decryptAndParse(ctx, adapter, service, targetEnv)
		if err != nil {
			return nil, err
		}

		targetValues := targetDoc.Values()
		missing := missingKeys(sourceDoc.Keys(), targetValues)
		extra := extraKeys(targetValues, sourceValues)

		blocked := make([]string, 0)
		if !opts.Placeholders {
			for _, key := range missing {
				if sourceValues[key] != "" || placeholderValueForKey(schemaKeyMeta(loadedSchema, key), targetEnv) != "" {
					blocked = append(blocked, key)
				}
			}
		}

		if len(blocked) > 0 {
			return nil, fmt.Errorf(
				"sync target %s/%s: missing keys require --placeholders: %s",
				service.Name,
				targetEnv,
				strings.Join(blocked, ", "),
			)
		}

		entries := make([]envfile.Entry, 0, len(sourceDoc.Entries))
		for _, sourceEntry := range sourceDoc.Entries {
			value, ok := targetValues[sourceEntry.Key]
			if !ok {
				value = placeholderValueForKey(schemaKeyMeta(loadedSchema, sourceEntry.Key), targetEnv)
				if !opts.Placeholders {
					value = ""
				}
			}

			entries = append(entries, envfile.Entry{
				Key:   sourceEntry.Key,
				Value: value,
			})
		}

		changed := len(missing) > 0 || len(extra) > 0 || !sameKeyOrder(sourceDoc.Keys(), targetDoc.Keys())
		if changed && !opts.DryRun {
			doc := &envfile.Document{Entries: entries}
			ciphertext, encErr := adapter.Encrypt(ctx, targetPath, doc.Bytes())
			if encErr != nil {
				return nil, fmt.Errorf("encrypt target env file for %s/%s: %w", service.Name, targetEnv, encErr)
			}
			if writeErr := atomicwrite.File(targetPath, ciphertext, 0o600); writeErr != nil {
				return nil, fmt.Errorf("write target env file for %s/%s: %w", service.Name, targetEnv, writeErr)
			}
		}

		results = append(results, TargetSyncResult{
			Environment: targetEnv,
			Added:       missing,
			Removed:     extra,
			Changed:     changed,
		})
	}

	return &SyncKeysResult{
		Service:            service.Name,
		SourceEnvironment:  opts.SourceEnvironment,
		TargetEnvironments: results,
	}, nil
}

func checkContext(ctx context.Context, action string) error {
	if ctx == nil {
		return nil
	}

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("%s: %w", action, err)
	}

	return nil
}

// SchemaViolation represents a single schema validation issue found in a document.
type SchemaViolation struct {
	Severity Severity
	Key      string
	Message  string
}

// validateDocumentAgainstSchema checks a document against a schema and returns violations.
func validateDocumentAgainstSchema(doc *envfile.Document, loadedSchema *schema.Schema) []SchemaViolation {
	if loadedSchema == nil {
		return nil
	}

	violations := make([]SchemaViolation, 0)
	values := doc.Values()

	for _, keyName := range loadedSchema.SortedKeys() {
		keySchema := loadedSchema.Keys[keyName]
		value, ok := values[keyName]
		if !ok {
			if keySchema.Required {
				violations = append(violations, SchemaViolation{
					Severity: SeverityError,
					Key:      keyName,
					Message:  "missing required key",
				})
			}

			continue
		}

		if err := keySchema.ValidateValue(value); err != nil {
			violations = append(violations, SchemaViolation{
				Severity: SeverityError,
				Key:      keyName,
				Message:  fmt.Sprintf("invalid value: %v", err),
			})
		}
	}

	for _, entry := range doc.Entries {
		if _, ok := loadedSchema.Keys[entry.Key]; ok {
			continue
		}

		violations = append(violations, SchemaViolation{
			Severity: SeverityWarning,
			Key:      entry.Key,
			Message:  "key not declared in schema",
		})
	}

	return violations
}

func lintDocument(serviceName, envName string, doc *envfile.Document, loadedSchema *schema.Schema) []Problem {
	violations := validateDocumentAgainstSchema(doc, loadedSchema)
	problems := make([]Problem, 0, len(violations))
	for _, v := range violations {
		problems = append(problems, Problem{
			Severity:    v.Severity,
			Service:     serviceName,
			Environment: envName,
			Key:         v.Key,
			Message:     v.Message,
		})
	}

	return problems
}

func selectServices(project *config.Project, serviceFilter string) ([]config.Service, error) {
	if serviceFilter != "" {
		service, err := project.Service(serviceFilter)
		if err != nil {
			return nil, fmt.Errorf("lookup service %q: %w", serviceFilter, err)
		}

		return []config.Service{service}, nil
	}

	names := make([]string, 0, len(project.Services))
	for name := range project.Services {
		names = append(names, name)
	}
	slices.Sort(names)

	services := make([]config.Service, 0, len(names))
	for _, name := range names {
		services = append(services, project.Services[name])
	}

	return services, nil
}

func loadServiceSchema(service config.Service) (*schema.Schema, error) {
	if service.SchemaPath == "" {
		return nil, nil //nolint:nilnil // Services may omit schema configuration.
	}

	loadedSchema, err := schema.Load(service.SchemaPath)
	if err != nil {
		return nil, fmt.Errorf("load schema for service %q: %w", service.Name, err)
	}

	return loadedSchema, nil
}

func decryptAndParse(ctx context.Context, adapter crypto.Adapter, service config.Service, envName string) (*envfile.Document, error) {
	filePath, err := service.FilePath(envName)
	if err != nil {
		return nil, fmt.Errorf("lookup env file for %s/%s: %w", service.Name, envName, err)
	}

	plaintext, err := adapter.Decrypt(ctx, filePath)
	if err != nil {
		return nil, fmt.Errorf("decrypt env file for %s/%s: %w", service.Name, envName, err)
	}

	doc, err := envfile.Parse(plaintext)
	if err != nil {
		return nil, fmt.Errorf("parse env file for %s/%s: %w", service.Name, envName, err)
	}

	return doc, nil
}

func unionKeys(groups ...[]string) []string {
	set := make(map[string]struct{})
	for _, group := range groups {
		for _, key := range group {
			set[key] = struct{}{}
		}
	}

	keys := slices.Sorted(maps.Keys(set))

	return keys
}

func missingKeys(sourceOrder []string, targetValues map[string]string) []string {
	keys := make([]string, 0)
	for _, key := range sourceOrder {
		if _, ok := targetValues[key]; ok {
			continue
		}

		keys = append(keys, key)
	}

	return keys
}

func extraKeys(targetValues, sourceValues map[string]string) []string {
	keys := make([]string, 0)
	for key := range targetValues {
		if _, ok := sourceValues[key]; ok {
			continue
		}

		keys = append(keys, key)
	}

	slices.Sort(keys)

	return keys
}

func selectTargetEnvironments(service config.Service, sourceEnv string, targets []string) ([]string, error) {
	if len(targets) == 0 {
		envs := service.Environments()
		selected := make([]string, 0, len(envs)-1)
		for _, envName := range envs {
			if envName == sourceEnv {
				continue
			}

			selected = append(selected, envName)
		}

		if len(selected) == 0 {
			return nil, fmt.Errorf("select target environments for service %q: no targets available", service.Name)
		}

		return selected, nil
	}

	selected := make([]string, 0, len(targets))
	seen := make(map[string]struct{}, len(targets))

	for _, targetEnv := range targets {
		if targetEnv == sourceEnv {
			return nil, fmt.Errorf("validate target environment %q: matches source environment", targetEnv)
		}

		if _, exists := seen[targetEnv]; exists {
			return nil, fmt.Errorf("validate target environment %q: duplicate target", targetEnv)
		}

		if _, err := service.FilePath(targetEnv); err != nil {
			return nil, fmt.Errorf("lookup target environment %q for service %q: %w", targetEnv, service.Name, err)
		}

		seen[targetEnv] = struct{}{}
		selected = append(selected, targetEnv)
	}

	slices.Sort(selected)

	return selected, nil
}

func sameKeyOrder(left, right []string) bool {
	return slices.Equal(left, right)
}

func detectRenameCandidates(fromDoc, toDoc *envfile.Document, loadedSchema *schema.Schema, valueMode DiffValueMode) []DiffRenameCandidate {
	removed := make(map[string][]string)
	added := make(map[string][]string)

	for _, entry := range fromDoc.Entries {
		if entry.Value == "" {
			continue
		}
		if _, ok := toDoc.Lookup(entry.Key); ok {
			continue
		}

		removed[entry.Value] = append(removed[entry.Value], entry.Key)
	}

	for _, entry := range toDoc.Entries {
		if entry.Value == "" {
			continue
		}
		if _, ok := fromDoc.Lookup(entry.Key); ok {
			continue
		}

		added[entry.Value] = append(added[entry.Value], entry.Key)
	}

	values := make([]string, 0)
	for value := range removed {
		if _, ok := added[value]; ok {
			values = append(values, value)
		}
	}
	slices.Sort(values)

	candidates := make([]DiffRenameCandidate, 0)
	for _, value := range values {
		fromKeys := slices.Clone(removed[value])
		toKeys := slices.Clone(added[value])
		slices.Sort(fromKeys)
		slices.Sort(toKeys)

		limit := min(len(fromKeys), len(toKeys))

		for idx := range fromKeys[:limit] {
			fromKey := fromKeys[idx]
			toKey := toKeys[idx]
			if fromKey == toKey {
				continue
			}

			candidate := DiffRenameCandidate{
				From: fromKey,
				To:   toKey,
			}

			if loadedSchema != nil {
				if meta, ok := loadedSchema.Keys[toKey]; ok {
					candidate.Metadata = &meta
				} else if meta, ok := loadedSchema.Keys[fromKey]; ok {
					candidate.Metadata = &meta
				}
			}
			if valueMode != DiffValueModeNone {
				candidate.Value = renderDiffValue(value, candidate.Metadata, valueMode)
			}

			candidates = append(candidates, candidate)
		}
	}

	return candidates
}

func collectDiffFindings(envName string, doc *envfile.Document, loadedSchema *schema.Schema) []DiffFinding {
	violations := validateDocumentAgainstSchema(doc, loadedSchema)
	findings := make([]DiffFinding, 0, len(violations))
	for _, v := range violations {
		findings = append(findings, DiffFinding{
			Severity:    v.Severity,
			Environment: envName,
			Key:         v.Key,
			Message:     v.Message,
		})
	}

	return findings
}

func schemaKeyMeta(loadedSchema *schema.Schema, key string) *schema.Key {
	if loadedSchema == nil {
		return nil
	}

	meta, ok := loadedSchema.Keys[key]
	if !ok {
		return nil
	}

	return &meta
}

func renderDiffValue(value string, meta *schema.Key, mode DiffValueMode) string {
	switch mode {
	case DiffValueModeNone:
		return ""
	case DiffValueModeHidden:
		return "(value hidden)"
	case DiffValueModeHash:
		sum := sha256.Sum256([]byte(value))
		return "sha256:" + hex.EncodeToString(sum[:8])
	case DiffValueModePublic:
		if meta == nil || meta.Secret {
			return "(secret hidden)"
		}

		return value
	case DiffValueModeAll:
		return value
	default:
		return ""
	}
}

func isValidDiffValueMode(mode DiffValueMode) bool {
	switch mode {
	case DiffValueModeNone, DiffValueModeHidden, DiffValueModeHash, DiffValueModePublic, DiffValueModeAll:
		return true
	default:
		return false
	}
}

func classifySyncIssue(loadedSchema *schema.Schema, key string) (SyncIssueKind, Severity) {
	if loadedSchema == nil {
		return SyncIssueKindUntracked, SeverityWarning
	}

	meta, ok := loadedSchema.Keys[key]
	if !ok {
		return SyncIssueKindUndeclared, SeverityWarning
	}
	if meta.Required {
		return SyncIssueKindRequired, SeverityError
	}

	return SyncIssueKindOptional, SeverityWarning
}

func placeholderValueForKey(meta *schema.Key, environment string) string {
	if meta == nil || meta.Secret {
		return ""
	}

	switch meta.Type {
	case "bool":
		return "false"
	case "int":
		return "0"
	case "enum":
		if slices.Contains(meta.Values, environment) {
			return environment
		}
		if len(meta.Values) > 0 {
			return meta.Values[0]
		}
	}

	return ""
}
