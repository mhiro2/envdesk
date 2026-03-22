package sopsconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/mhiro2/envdesk/internal/atomicwrite"

	"gopkg.in/yaml.v3"
)

// Document represents a parsed .sops.yaml file with Node-level access
// for comment- and format-preserving updates.
type Document struct {
	path          string
	root          *yaml.Node
	creationRules *yaml.Node
}

type UpdateRecipientsResult struct {
	MatchedRules int
	ChangedRules int
}

// Load reads and validates a .sops.yaml file at the given path.
func Load(path string) (*Document, error) {
	cleaned := filepath.Clean(path)
	// #nosec G304 -- the path is resolved from project configuration.
	data, err := os.ReadFile(cleaned)
	if err != nil {
		return nil, fmt.Errorf("read sops config %q: %w", cleaned, err)
	}

	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("parse sops config %q: %w", cleaned, err)
	}

	document, err := documentRoot(&root)
	if err != nil {
		return nil, fmt.Errorf("validate sops config %q: %w", cleaned, err)
	}

	creationRules, ok := mappingValue(document, "creation_rules")
	if !ok {
		return nil, fmt.Errorf("validate sops config %q: missing creation_rules", cleaned)
	}
	if creationRules.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("validate sops config %q: creation_rules must be a sequence", cleaned)
	}

	return &Document{
		path:          cleaned,
		root:          &root,
		creationRules: creationRules,
	}, nil
}

// Write marshals the modified YAML back to the file.
func (d *Document) Write() error {
	data, err := yaml.Marshal(d.root)
	if err != nil {
		return fmt.Errorf("marshal sops config %q: %w", d.path, err)
	}

	if err := atomicwrite.File(d.path, data, 0o644); err != nil {
		return fmt.Errorf("write sops config %q: %w", d.path, err)
	}

	return nil
}

// UpdateRecipients adds or removes a recipient from matching creation rules.
func (d *Document) UpdateRecipients(paths []string, recipient string, remove bool) (*UpdateRecipientsResult, error) {
	normalizedRecipient := strings.TrimSpace(recipient)
	if normalizedRecipient == "" {
		return nil, fmt.Errorf("validate recipient: empty value")
	}

	rules, err := d.matchingRules(paths)
	if err != nil {
		return nil, err
	}
	if len(rules) == 0 {
		return nil, fmt.Errorf("select sops creation rules: no matching rules")
	}

	result := &UpdateRecipientsResult{
		MatchedRules: len(rules),
	}
	for _, rule := range rules {
		ruleChanged, err := updateRuleRecipients(rule, normalizedRecipient, remove)
		if err != nil {
			return nil, err
		}
		if ruleChanged {
			result.ChangedRules++
		}
	}

	if result.ChangedRules == 0 {
		if remove {
			return nil, fmt.Errorf("remove recipient %q: not configured", normalizedRecipient)
		}

		return nil, fmt.Errorf("add recipient %q: already configured", normalizedRecipient)
	}

	return result, nil
}

func (d *Document) matchingRules(paths []string) ([]*yaml.Node, error) {
	if len(paths) == 0 {
		return d.creationRules.Content, nil
	}

	rules := make([]*yaml.Node, 0, len(d.creationRules.Content))
	for _, rule := range d.creationRules.Content {
		matches, err := ruleMatchesPaths(rule, paths)
		if err != nil {
			return nil, err
		}
		if matches {
			rules = append(rules, rule)
		}
	}

	return rules, nil
}

func ruleMatchesPaths(rule *yaml.Node, paths []string) (bool, error) {
	pathRegexNode, ok := mappingValue(rule, "path_regex")
	if !ok {
		return false, fmt.Errorf("validate sops creation rule: missing path_regex")
	}
	if pathRegexNode.Kind != yaml.ScalarNode {
		return false, fmt.Errorf("validate sops creation rule: path_regex must be a scalar")
	}

	re, err := regexp.Compile(pathRegexNode.Value)
	if err != nil {
		return false, fmt.Errorf("compile sops path regex %q: %w", pathRegexNode.Value, err)
	}

	return slices.ContainsFunc(paths, func(path string) bool {
		return re.MatchString(path)
	}), nil
}

func updateRuleRecipients(rule *yaml.Node, recipient string, remove bool) (bool, error) {
	ageNode, ok := mappingValue(rule, "age")
	if !ok {
		if remove {
			return false, nil
		}

		ageNode = &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		rule.Content = append(rule.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "age"}, ageNode)
	}
	if ageNode.Kind != yaml.SequenceNode {
		return false, fmt.Errorf("validate sops creation rule: age must be a sequence")
	}

	if entry := findRecipientNode(ageNode.Content, recipient); entry != nil {
		if remove {
			ageNode.Content = removeSequenceValue(ageNode.Content, recipient)
			return true, nil
		}

		if entry.Value != recipient {
			entry.Value = recipient
			return true, nil
		}

		return false, nil
	}

	if remove {
		return false, nil
	}

	ageNode.Content = append(ageNode.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: recipient})
	return true, nil
}

func findRecipientNode(nodes []*yaml.Node, recipient string) *yaml.Node {
	for _, node := range nodes {
		if strings.TrimSpace(node.Value) == recipient {
			return node
		}
	}

	return nil
}

func removeSequenceValue(nodes []*yaml.Node, value string) []*yaml.Node {
	filtered := make([]*yaml.Node, 0, len(nodes))
	for _, node := range nodes {
		if strings.TrimSpace(node.Value) == value {
			continue
		}

		filtered = append(filtered, node)
	}

	return filtered
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

func documentRoot(root *yaml.Node) (*yaml.Node, error) {
	if root == nil || len(root.Content) == 0 {
		return nil, fmt.Errorf("empty document")
	}

	document := root.Content[0]
	if document.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("document must be a mapping")
	}

	return document, nil
}
