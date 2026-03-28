package app

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/mhiro2/envdesk/internal/config"
	"github.com/mhiro2/envdesk/internal/envfile"
	"github.com/mhiro2/envdesk/internal/schema"
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
	Key              string              `json:"key"`
	Present          bool                `json:"present"`
	LastValueChange  *AuditFact          `json:"last_value_change,omitempty"`
	Schema           *AuditSchemaState   `json:"schema,omitempty"`
	LastSchemaChange *AuditFact          `json:"last_schema_change,omitempty"`
	SchemaChanges    []AuditSchemaChange `json:"schema_changes,omitempty"`
	Drift            AuditDriftState     `json:"drift"`
	Events           []AuditEvent        `json:"events,omitempty"`
}

type AuditFact struct {
	Author    string    `json:"author"`
	Date      time.Time `json:"date"`
	CommitSHA string    `json:"commit"`
	Summary   string    `json:"summary"`
}

type AuditSchemaState struct {
	Required bool   `json:"required"`
	Secret   bool   `json:"secret"`
	Type     string `json:"type"`
}

type AuditSchemaChange struct {
	Field  string    `json:"field"`
	From   string    `json:"from"`
	To     string    `json:"to"`
	Change AuditFact `json:"change"`
}

type AuditDriftState struct {
	State    string        `json:"state"`
	Kind     SyncIssueKind `json:"kind,omitempty"`
	Severity Severity      `json:"severity,omitempty"`
	Missing  []string      `json:"missing,omitempty"`
	Present  []string      `json:"present,omitempty"`
	Since    *AuditFact    `json:"since,omitempty"`
}

type AuditEventCategory string

const (
	AuditEventCategoryValue  AuditEventCategory = "value"
	AuditEventCategorySchema AuditEventCategory = "schema"
	AuditEventCategoryDrift  AuditEventCategory = "drift"
)

type AuditEvent struct {
	Category AuditEventCategory `json:"category"`
	Action   string             `json:"action"`
	Field    string             `json:"field,omitempty"`
	From     string             `json:"from,omitempty"`
	To       string             `json:"to,omitempty"`
	Kind     SyncIssueKind      `json:"kind,omitempty"`
	Severity Severity           `json:"severity,omitempty"`
	Missing  []string           `json:"missing,omitempty"`
	Present  []string           `json:"present,omitempty"`
	Change   AuditFact          `json:"change"`
}

type auditRunner interface {
	Blame(ctx context.Context, dir, filePath string) ([]byte, error)
	Log(ctx context.Context, dir string, paths []string) ([]gitCommitRecord, error)
	Show(ctx context.Context, dir, rev, filePath string) ([]byte, bool, error)
}

type gitCommitRecord struct {
	Fact  AuditFact
	Paths []string
}

type gitAuditRunner struct{}

func (*gitAuditRunner) Blame(ctx context.Context, dir, filePath string) ([]byte, error) {
	// #nosec G204 -- filePath is derived from project config, not user input.
	cmd := exec.CommandContext(ctx, "git", "blame", "--porcelain", filePath)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("run git blame on %q: %w", filePath, err)
	}

	return out, nil
}

func (*gitAuditRunner) Log(ctx context.Context, dir string, paths []string) ([]gitCommitRecord, error) {
	if len(paths) == 0 {
		return nil, nil
	}

	args := make([]string, 0, 5+len(paths))
	args = append(args,
		"log",
		"--reverse",
		"--name-only",
		"--format=commit%x1f%H%x1f%an%x1f%at%x1f%s",
		"--",
	)
	args = append(args, paths...)

	// #nosec G204 -- paths are derived from project config, not user input.
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("run git log: %w", err)
	}

	records, err := parseGitLogRecords(out)
	if err != nil {
		return nil, fmt.Errorf("parse git log: %w", err)
	}

	return records, nil
}

func (*gitAuditRunner) Show(ctx context.Context, dir, rev, filePath string) ([]byte, bool, error) {
	spec := rev + ":" + filePath

	// #nosec G204 -- spec is derived from project config and git history, not user input.
	cmd := exec.CommandContext(ctx, "git", "show", spec)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		if isGitMissingObject(string(out)) {
			return nil, false, nil
		}

		return nil, false, fmt.Errorf("show git object %q: %w", spec, err)
	}

	return out, true, nil
}

type auditIssueState struct {
	Kind     SyncIssueKind
	Severity Severity
	Missing  []string
	Present  []string
}

type auditServiceState struct {
	envDocs       map[string]*envfile.Document
	schemaDoc     *schema.Schema
	valueFacts    map[string]map[string]AuditFact
	schemaChanges map[string][]AuditSchemaChange
	valueEvents   map[string]map[string][]AuditEvent
	driftEvents   map[string]map[string][]AuditEvent
	issueStates   map[string]map[string]auditIssueState
}

func Audit(ctx context.Context, project *config.Project, opts AuditOptions) ([]AuditResult, error) {
	return auditWith(ctx, project, opts, &gitAuditRunner{})
}

func auditWith(ctx context.Context, project *config.Project, opts AuditOptions, runner auditRunner) ([]AuditResult, error) {
	if err := checkContext(ctx, "audit env files"); err != nil {
		return nil, err
	}

	services, err := selectServices(project, opts.Service)
	if err != nil {
		return nil, err
	}

	results := make([]AuditResult, 0)
	matchedEnvironment := false

	for _, service := range services {
		selectedEnvs := selectedAuditEnvironments(service.Environments(), opts.Environment)
		if len(selectedEnvs) == 0 {
			continue
		}
		if opts.Environment != "" {
			matchedEnvironment = true
		}

		state, err := buildAuditServiceState(ctx, project.BaseDir, service, runner)
		if err != nil {
			return nil, err
		}

		for _, envName := range selectedEnvs {
			path, err := service.FilePath(envName)
			if err != nil {
				return nil, fmt.Errorf("lookup env file for %s/%s: %w", service.Name, envName, err)
			}

			results = append(results, AuditResult{
				Service:     service.Name,
				Environment: envName,
				Path:        statusDisplayPath(project.BaseDir, path),
				Entries:     buildAuditEntries(envName, opts.Key, state),
			})
		}
	}

	if opts.Environment != "" && !matchedEnvironment {
		return nil, fmt.Errorf("select environment %q: not configured", opts.Environment)
	}

	return results, nil
}

func selectedAuditEnvironments(envNames []string, environment string) []string {
	if environment == "" {
		return slices.Clone(envNames)
	}

	if slices.Contains(envNames, environment) {
		return []string{environment}
	}

	return nil
}

func buildAuditServiceState(ctx context.Context, baseDir string, service config.Service, runner auditRunner) (*auditServiceState, error) {
	if err := checkContext(ctx, "audit env files"); err != nil {
		return nil, err
	}

	envNames := service.Environments()
	envDocs := make(map[string]*envfile.Document, len(envNames))
	valueFacts := make(map[string]map[string]AuditFact, len(envNames))
	pathToEnv := make(map[string]string, len(envNames))
	logPaths := make([]string, 0, len(envNames)+1)

	for _, envName := range envNames {
		path, err := service.FilePath(envName)
		if err != nil {
			return nil, fmt.Errorf("lookup env file for %s/%s: %w", service.Name, envName, err)
		}

		doc, err := envfile.Load(path)
		if err != nil {
			return nil, fmt.Errorf("load env file for %s/%s: %w", service.Name, envName, err)
		}

		relPath, err := filepath.Rel(baseDir, path)
		if err != nil {
			return nil, fmt.Errorf("resolve relative path for %q: %w", path, err)
		}

		facts, err := loadAuditValueFacts(ctx, baseDir, relPath, runner)
		if err != nil {
			return nil, fmt.Errorf("audit %s/%s: %w", service.Name, envName, err)
		}

		envDocs[envName] = doc
		valueFacts[envName] = facts
		pathToEnv[filepath.ToSlash(relPath)] = envName
		logPaths = append(logPaths, filepath.ToSlash(relPath))
	}

	loadedSchema, err := loadServiceSchema(service)
	if err != nil {
		return nil, err
	}

	schemaRelPath := ""
	if service.SchemaPath != "" {
		relPath, err := filepath.Rel(baseDir, service.SchemaPath)
		if err != nil {
			return nil, fmt.Errorf("resolve relative path for %q: %w", service.SchemaPath, err)
		}
		schemaRelPath = filepath.ToSlash(relPath)
		logPaths = append(logPaths, schemaRelPath)
	}

	records, err := runner.Log(ctx, baseDir, logPaths)
	if err != nil {
		return nil, fmt.Errorf("load audit history for service %q: %w", service.Name, err)
	}

	state := &auditServiceState{
		envDocs:       envDocs,
		schemaDoc:     loadedSchema,
		valueFacts:    valueFacts,
		schemaChanges: make(map[string][]AuditSchemaChange),
		valueEvents:   make(map[string]map[string][]AuditEvent),
		driftEvents:   make(map[string]map[string][]AuditEvent),
	}

	historyDocs := make(map[string]*envfile.Document, len(envNames))
	var historySchema *schema.Schema
	prevIssueStates := make(map[string]map[string]auditIssueState)

	for _, record := range records {
		if err := checkContext(ctx, "audit env files"); err != nil {
			return nil, err
		}

		for _, changedPath := range record.Paths {
			if envName, ok := pathToEnv[changedPath]; ok {
				nextDoc, ok, err := loadAuditEnvSnapshot(ctx, baseDir, runner, record.Fact.CommitSHA, changedPath)
				if err != nil {
					return nil, fmt.Errorf("load audit env snapshot for %s/%s at %s: %w", service.Name, envName, record.Fact.CommitSHA[:8], err)
				}
				if !ok {
					nextDoc = nil
				}

				appendAuditValueEvents(state.valueEvents, envName, historyDocs[envName], nextDoc, record.Fact)
				historyDocs[envName] = nextDoc

				continue
			}

			if schemaRelPath != "" && changedPath == schemaRelPath {
				nextSchema, ok, err := loadAuditSchemaSnapshot(ctx, baseDir, runner, record.Fact.CommitSHA, changedPath)
				if err != nil {
					return nil, fmt.Errorf("load audit schema snapshot for service %q at %s: %w", service.Name, record.Fact.CommitSHA[:8], err)
				}
				if !ok {
					nextSchema = nil
				}

				appendAuditSchemaChanges(state.schemaChanges, historySchema, nextSchema, record.Fact)
				historySchema = nextSchema
			}
		}

		issues := syncIssuesForService(service.Name, envNames, historyDocs, historySchema, false)
		currentIssueStates := issueStatesByEnvironment(issues)
		appendAuditDriftEvents(state.driftEvents, prevIssueStates, currentIssueStates, record.Fact)
		prevIssueStates = currentIssueStates
	}

	state.issueStates = issueStatesByEnvironment(syncIssuesForService(service.Name, envNames, envDocs, loadedSchema, false))

	return state, nil
}

func buildAuditEntries(environment, keyFilter string, state *auditServiceState) []AuditEntry {
	keys := auditEntryKeys(environment, keyFilter, state)
	entries := make([]AuditEntry, 0, len(keys))
	currentDoc := state.envDocs[environment]

	for _, key := range keys {
		_, present := lookupAuditDocument(currentDoc, key)
		schemaState := currentAuditSchemaState(state.schemaDoc, key)
		schemaChanges := slices.Clone(state.schemaChanges[key])
		valueEvents := auditEventsForKey(state.valueEvents, environment, key)
		driftEvents := auditEventsForKey(state.driftEvents, environment, key)

		drift := AuditDriftState{State: "in_sync"}
		if issue, ok := auditIssueStateForKey(state.issueStates, environment, key); ok {
			drift = AuditDriftState{
				State:    "drift",
				Kind:     issue.Kind,
				Severity: issue.Severity,
				Missing:  slices.Clone(issue.Missing),
				Present:  slices.Clone(issue.Present),
				Since:    auditCurrentDriftSince(driftEvents),
			}
		}

		entries = append(entries, AuditEntry{
			Key:              key,
			Present:          present,
			LastValueChange:  auditLastValueChange(present, auditValueFactForKey(state.valueFacts, environment, key), valueEvents),
			Schema:           schemaState,
			LastSchemaChange: auditLastSchemaChange(schemaChanges),
			SchemaChanges:    schemaChanges,
			Drift:            drift,
			Events:           mergeAuditEvents(valueEvents, auditSchemaEvents(schemaChanges), driftEvents),
		})
	}

	return entries
}

func auditEntryKeys(environment, keyFilter string, state *auditServiceState) []string {
	if keyFilter != "" {
		if auditEntryKeyExists(environment, keyFilter, state) {
			return []string{keyFilter}
		}

		return nil
	}

	keys := make([]string, 0)

	if doc := state.envDocs[environment]; doc != nil {
		keys = append(keys, doc.Keys()...)
	}

	if state.schemaDoc != nil {
		keys = append(keys, state.schemaDoc.SortedKeys()...)
	}

	if issues, ok := state.issueStates[environment]; ok {
		for key := range issues {
			keys = append(keys, key)
		}
	}

	return unionKeys(keys)
}

func auditEntryKeyExists(environment, key string, state *auditServiceState) bool {
	if _, ok := lookupAuditDocument(state.envDocs[environment], key); ok {
		return true
	}

	if state.schemaDoc != nil {
		if _, ok := state.schemaDoc.Keys[key]; ok {
			return true
		}
	}

	if issues, ok := state.issueStates[environment]; ok {
		if _, ok := issues[key]; ok {
			return true
		}
	}

	if facts, ok := state.valueFacts[environment]; ok {
		if _, ok := facts[key]; ok {
			return true
		}
	}

	if len(auditEventsForKey(state.valueEvents, environment, key)) > 0 {
		return true
	}

	if len(state.schemaChanges[key]) > 0 {
		return true
	}

	return len(auditEventsForKey(state.driftEvents, environment, key)) > 0
}

func loadAuditValueFacts(ctx context.Context, baseDir, relPath string, runner auditRunner) (map[string]AuditFact, error) {
	if err := checkContext(ctx, "audit env file"); err != nil {
		return nil, err
	}

	out, err := runner.Blame(ctx, baseDir, relPath)
	if err != nil {
		return nil, fmt.Errorf("run blame: %w", err)
	}

	lines, commits, err := parsePorcelainBlame(out)
	if err != nil {
		return nil, fmt.Errorf("parse blame: %w", err)
	}

	facts := make(map[string]AuditFact)
	for _, line := range lines {
		key := extractEnvKey(line.content)
		if key == "" {
			continue
		}

		commit, ok := commits[line.commitSHA]
		if !ok {
			continue
		}

		facts[key] = commit
	}

	return facts, nil
}

func loadAuditEnvSnapshot(ctx context.Context, baseDir string, runner auditRunner, rev, relPath string) (*envfile.Document, bool, error) {
	if err := checkContext(ctx, "audit env file"); err != nil {
		return nil, false, err
	}

	data, ok, err := runner.Show(ctx, baseDir, rev, relPath)
	if err != nil {
		return nil, false, fmt.Errorf("show env file snapshot %q at %s: %w", relPath, rev, err)
	}
	if !ok {
		return nil, false, nil
	}

	doc, err := envfile.Parse(data)
	if err != nil {
		return nil, false, fmt.Errorf("parse env file snapshot: %w", err)
	}

	return doc, true, nil
}

func loadAuditSchemaSnapshot(ctx context.Context, baseDir string, runner auditRunner, rev, relPath string) (*schema.Schema, bool, error) {
	if err := checkContext(ctx, "audit env files"); err != nil {
		return nil, false, err
	}

	data, ok, err := runner.Show(ctx, baseDir, rev, relPath)
	if err != nil {
		return nil, false, fmt.Errorf("show schema snapshot %q at %s: %w", relPath, rev, err)
	}
	if !ok {
		return nil, false, nil
	}

	loaded, err := parseAuditSchemaSnapshot(data)
	if err != nil {
		return nil, false, fmt.Errorf("parse schema snapshot: %w", err)
	}

	return loaded, true, nil
}

func parseAuditSchemaSnapshot(data []byte) (*schema.Schema, error) {
	loaded, err := schema.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("parse schema data: %w", err)
	}

	return loaded, nil
}

func appendAuditValueEvents(events map[string]map[string][]AuditEvent, environment string, previous, current *envfile.Document, change AuditFact) {
	keys := unionKeys(auditDocumentKeys(previous), auditDocumentKeys(current))

	for _, key := range keys {
		previousValue, previousOK := lookupAuditDocument(previous, key)
		currentValue, currentOK := lookupAuditDocument(current, key)

		action := ""
		switch {
		case !previousOK && currentOK:
			action = "added"
		case previousOK && !currentOK:
			action = "removed"
		case previousOK && currentOK && previousValue != currentValue:
			action = "changed"
		default:
			continue
		}

		appendAuditEvent(events, environment, key, AuditEvent{
			Category: AuditEventCategoryValue,
			Action:   action,
			Change:   change,
		})
	}
}

func appendAuditSchemaChanges(changes map[string][]AuditSchemaChange, previous, current *schema.Schema, change AuditFact) {
	keys := unionKeys(auditSchemaKeys(previous), auditSchemaKeys(current))

	for _, key := range keys {
		previousMeta, previousOK := auditSchemaMeta(previous, key)
		currentMeta, currentOK := auditSchemaMeta(current, key)

		appendAuditSchemaChange(changes, key, "required", auditSchemaBoolValue(previousOK, previousMeta.Required), auditSchemaBoolValue(currentOK, currentMeta.Required), change)
		appendAuditSchemaChange(changes, key, "secret", auditSchemaBoolValue(previousOK, previousMeta.Secret), auditSchemaBoolValue(currentOK, currentMeta.Secret), change)
		appendAuditSchemaChange(changes, key, "type", auditSchemaTypeValue(previousOK, previousMeta.Type), auditSchemaTypeValue(currentOK, currentMeta.Type), change)
	}
}

func appendAuditSchemaChange(changes map[string][]AuditSchemaChange, key, field, from, to string, change AuditFact) {
	if from == to {
		return
	}

	changes[key] = append(changes[key], AuditSchemaChange{
		Field:  field,
		From:   from,
		To:     to,
		Change: change,
	})
}

func appendAuditDriftEvents(events map[string]map[string][]AuditEvent, previous, current map[string]map[string]auditIssueState, change AuditFact) {
	environments := unionKeys(auditIssueEnvironments(previous), auditIssueEnvironments(current))

	for _, environment := range environments {
		keys := unionKeys(auditIssueKeys(previous, environment), auditIssueKeys(current, environment))

		for _, key := range keys {
			previousState, previousOK := auditIssueStateForKey(previous, environment, key)
			currentState, currentOK := auditIssueStateForKey(current, environment, key)

			switch {
			case !previousOK && currentOK:
				appendAuditEvent(events, environment, key, auditDriftEvent("started", currentState, change))
			case previousOK && !currentOK:
				appendAuditEvent(events, environment, key, auditDriftEvent("resolved", previousState, change))
			case previousOK && currentOK && !auditIssueStatesEqual(previousState, currentState):
				appendAuditEvent(events, environment, key, auditDriftEvent("changed", currentState, change))
			}
		}
	}
}

func appendAuditEvent(events map[string]map[string][]AuditEvent, environment, key string, event AuditEvent) {
	if _, ok := events[environment]; !ok {
		events[environment] = make(map[string][]AuditEvent)
	}

	events[environment][key] = append(events[environment][key], event)
}

func auditDriftEvent(action string, state auditIssueState, change AuditFact) AuditEvent {
	return AuditEvent{
		Category: AuditEventCategoryDrift,
		Action:   action,
		Kind:     state.Kind,
		Severity: state.Severity,
		Missing:  slices.Clone(state.Missing),
		Present:  slices.Clone(state.Present),
		Change:   change,
	}
}

func issueStatesByEnvironment(issues []SyncIssue) map[string]map[string]auditIssueState {
	states := make(map[string]map[string]auditIssueState)

	for _, issue := range issues {
		for _, environment := range issue.Missing {
			setAuditIssueState(states, environment, issue.Key, issue)
		}
		for _, environment := range issue.Present {
			setAuditIssueState(states, environment, issue.Key, issue)
		}
	}

	return states
}

func setAuditIssueState(states map[string]map[string]auditIssueState, environment, key string, issue SyncIssue) {
	if _, ok := states[environment]; !ok {
		states[environment] = make(map[string]auditIssueState)
	}

	states[environment][key] = auditIssueState{
		Kind:     issue.Kind,
		Severity: issue.Severity,
		Missing:  slices.Clone(issue.Missing),
		Present:  slices.Clone(issue.Present),
	}
}

func auditIssueStatesEqual(left, right auditIssueState) bool {
	return left.Kind == right.Kind &&
		left.Severity == right.Severity &&
		slices.Equal(left.Missing, right.Missing) &&
		slices.Equal(left.Present, right.Present)
}

func auditEventsForKey(events map[string]map[string][]AuditEvent, environment, key string) []AuditEvent {
	if byKey, ok := events[environment]; ok {
		return slices.Clone(byKey[key])
	}

	return nil
}

func auditValueFactForKey(facts map[string]map[string]AuditFact, environment, key string) *AuditFact {
	if byKey, ok := facts[environment]; ok {
		if fact, ok := byKey[key]; ok {
			return &fact
		}
	}

	return nil
}

func auditIssueStateForKey(states map[string]map[string]auditIssueState, environment, key string) (auditIssueState, bool) {
	byKey, ok := states[environment]
	if !ok {
		return auditIssueState{}, false
	}

	state, ok := byKey[key]
	return state, ok
}

func auditLastValueChange(present bool, fact *AuditFact, events []AuditEvent) *AuditFact {
	if present && fact != nil {
		return fact
	}

	for i := len(events) - 1; i >= 0; i-- {
		if events[i].Category == AuditEventCategoryValue {
			change := events[i].Change
			return &change
		}
	}

	return fact
}

func auditLastSchemaChange(changes []AuditSchemaChange) *AuditFact {
	if len(changes) == 0 {
		return nil
	}

	change := changes[len(changes)-1].Change
	return &change
}

func auditCurrentDriftSince(events []AuditEvent) *AuditFact {
	for i := len(events) - 1; i >= 0; i-- {
		switch events[i].Action {
		case "resolved":
			return nil
		case "started", "changed":
			change := events[i].Change
			return &change
		}
	}

	return nil
}

func currentAuditSchemaState(loadedSchema *schema.Schema, key string) *AuditSchemaState {
	if loadedSchema == nil {
		return nil
	}

	meta, ok := loadedSchema.Keys[key]
	if !ok {
		return nil
	}

	return &AuditSchemaState{
		Required: meta.Required,
		Secret:   meta.Secret,
		Type:     auditSchemaTypeLabel(meta.Type),
	}
}

func auditSchemaEvents(changes []AuditSchemaChange) []AuditEvent {
	events := make([]AuditEvent, 0, len(changes))

	for _, change := range changes {
		events = append(events, AuditEvent{
			Category: AuditEventCategorySchema,
			Action:   "changed",
			Field:    change.Field,
			From:     change.From,
			To:       change.To,
			Change:   change.Change,
		})
	}

	return events
}

func mergeAuditEvents(groups ...[]AuditEvent) []AuditEvent {
	merged := make([]AuditEvent, 0)
	for _, group := range groups {
		merged = append(merged, group...)
	}

	slices.SortFunc(merged, func(left, right AuditEvent) int {
		if left.Change.Date.Equal(right.Change.Date) {
			if left.Category == right.Category {
				if left.Action == right.Action {
					return strings.Compare(left.Change.CommitSHA, right.Change.CommitSHA)
				}

				return strings.Compare(left.Action, right.Action)
			}

			return strings.Compare(string(left.Category), string(right.Category))
		}

		if left.Change.Date.Before(right.Change.Date) {
			return -1
		}

		return 1
	})

	return merged
}

func auditDocumentKeys(doc *envfile.Document) []string {
	if doc == nil {
		return nil
	}

	return doc.Keys()
}

func lookupAuditDocument(doc *envfile.Document, key string) (string, bool) {
	if doc == nil {
		return "", false
	}

	return doc.Lookup(key)
}

func auditSchemaKeys(loadedSchema *schema.Schema) []string {
	if loadedSchema == nil {
		return nil
	}

	return loadedSchema.SortedKeys()
}

func auditSchemaMeta(loadedSchema *schema.Schema, key string) (schema.Key, bool) {
	if loadedSchema == nil {
		return schema.Key{}, false
	}

	meta, ok := loadedSchema.Keys[key]
	return meta, ok
}

func auditSchemaBoolValue(exists, value bool) string {
	if !exists {
		return "absent"
	}

	if value {
		return "true"
	}

	return "false"
}

func auditSchemaTypeValue(exists bool, typ string) string {
	if !exists {
		return "absent"
	}

	return auditSchemaTypeLabel(typ)
}

func auditSchemaTypeLabel(typ string) string {
	if typ == "" {
		return "string"
	}

	return typ
}

func auditIssueEnvironments(states map[string]map[string]auditIssueState) []string {
	environments := make([]string, 0, len(states))
	for environment := range states {
		environments = append(environments, environment)
	}

	return environments
}

func auditIssueKeys(states map[string]map[string]auditIssueState, environment string) []string {
	byKey, ok := states[environment]
	if !ok {
		return nil
	}

	keys := make([]string, 0, len(byKey))
	for key := range byKey {
		keys = append(keys, key)
	}

	return keys
}

func parseGitLogRecords(data []byte) ([]gitCommitRecord, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	records := make([]gitCommitRecord, 0)
	var current *gitCommitRecord

	for scanner.Scan() {
		text := scanner.Text()
		if strings.HasPrefix(text, "commit\x1f") {
			if current != nil {
				records = append(records, *current)
			}

			record, err := parseGitLogRecordHeader(text)
			if err != nil {
				return nil, err
			}

			current = &record
			continue
		}

		if text == "" || current == nil {
			continue
		}

		current.Paths = append(current.Paths, filepath.ToSlash(text))
	}

	if current != nil {
		records = append(records, *current)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan git log: %w", err)
	}

	return records, nil
}

func parseGitLogRecordHeader(line string) (gitCommitRecord, error) {
	parts := strings.Split(line, "\x1f")
	if len(parts) != 5 {
		return gitCommitRecord{}, fmt.Errorf("parse git log header %q: invalid field count", line)
	}

	commitTime, err := parseUnixTimestamp(parts[3])
	if err != nil {
		return gitCommitRecord{}, err
	}

	return gitCommitRecord{
		Fact: AuditFact{
			CommitSHA: parts[1],
			Author:    parts[2],
			Date:      commitTime,
			Summary:   parts[4],
		},
	}, nil
}

func isGitMissingObject(output string) bool {
	return strings.Contains(output, "does not exist in") || strings.Contains(output, "exists on disk, but not in")
}

// blameLine holds parsed porcelain blame data for a single line.
type blameLine struct {
	commitSHA string
	content   string
}

func parsePorcelainBlame(data []byte) ([]blameLine, map[string]AuditFact, error) {
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	lines := make([]blameLine, 0)
	commits := make(map[string]AuditFact)

	var currentSHA string
	currentCommitData := make(map[string]string)

	for scanner.Scan() {
		text := scanner.Text()

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

		if strings.HasPrefix(text, "\t") {
			lines = append(lines, blameLine{
				commitSHA: currentSHA,
				content:   text[1:],
			})
			continue
		}

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

			commitTime, _ := parseUnixTimestamp(currentCommitData["author-time"])
			commits[currentSHA] = AuditFact{
				Author:    currentCommitData["author"],
				Date:      commitTime,
				CommitSHA: currentSHA,
				Summary:   value,
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, fmt.Errorf("parse git blame output: %w", err)
	}

	return lines, commits, nil
}

var envKeyPattern = regexp.MustCompile(`^(?:export\s+)?([A-Za-z_][A-Za-z0-9_]*)=`)

func extractEnvKey(line string) string {
	matches := envKeyPattern.FindStringSubmatch(line)
	if matches == nil {
		return ""
	}

	return matches[1]
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
