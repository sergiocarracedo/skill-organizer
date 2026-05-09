package skills

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const managedMetadataKey = "skill-organizer"

type ManagedMetadata struct {
	OriginalName       string
	SourceRelativePath string
	Disabled           bool
	Source             string
	SourceType         string
	InstalledVersion   string
	InstalledAt        string
	RepoSkillPath      string
	LastUpdatedAt      string
}

type Document struct {
	frontmatter yaml.Node
	body        string
}

func LoadDocument(path string) (Document, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return Document{}, fmt.Errorf("read skill file: %w", err)
	}

	return ParseDocument(content)
}

func ParseDocument(content []byte) (Document, error) {
	frontmatterContent, body, err := splitFrontmatter(content)
	if err != nil {
		return Document{}, err
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(frontmatterContent, &doc); err != nil {
		sanitized := sanitizeFrontmatter(frontmatterContent)
		if retryErr := yaml.Unmarshal(sanitized, &doc); retryErr != nil {
			return Document{}, fmt.Errorf("parse skill frontmatter: %w", err)
		}
	}

	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return Document{}, fmt.Errorf("skill frontmatter must be a YAML mapping")
	}

	return Document{frontmatter: doc, body: body}, nil
}

func sanitizeFrontmatter(content []byte) []byte {
	lines := strings.Split(string(content), "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		colonIndex := strings.Index(line, ":")
		if colonIndex <= 0 || colonIndex+1 >= len(line) {
			continue
		}

		key := line[:colonIndex]
		value := strings.TrimSpace(line[colonIndex+1:])
		if value == "" || value == "|" || value == ">" || strings.HasPrefix(value, "[") || strings.HasPrefix(value, "{") || strings.HasPrefix(value, "\"") || strings.HasPrefix(value, "'") {
			continue
		}
		if !strings.Contains(value, ": ") {
			continue
		}

		lines[i] = key + ": " + strconv.Quote(value)
	}

	return []byte(strings.Join(lines, "\n"))
}

func (d *Document) Name() string {
	if node := mappingValue(d.mapping(), "name"); node != nil {
		return node.Value
	}
	return ""
}

func (d *Document) Description() string {
	if node := mappingValue(d.mapping(), "description"); node != nil {
		return node.Value
	}
	return ""
}

func (d *Document) ManagedMetadata() ManagedMetadata {
	metadata := ManagedMetadata{}
	root := d.mapping()
	metadataNode := ensureMapping(root, "metadata")
	organizerNode := ensureMapping(metadataNode, managedMetadataKey)

	if node := mappingValue(organizerNode, "original-name"); node != nil {
		metadata.OriginalName = node.Value
	}
	if node := mappingValue(organizerNode, "source-relative-path"); node != nil {
		metadata.SourceRelativePath = node.Value
	}
	if node := mappingValue(organizerNode, "disabled"); node != nil {
		metadata.Disabled = strings.EqualFold(node.Value, "true")
	}
	if node := mappingValue(organizerNode, "source"); node != nil {
		metadata.Source = node.Value
	}
	if node := mappingValue(organizerNode, "source-type"); node != nil {
		metadata.SourceType = node.Value
	}
	if node := mappingValue(organizerNode, "installed-version"); node != nil {
		metadata.InstalledVersion = node.Value
	}
	if node := mappingValue(organizerNode, "installed-at"); node != nil {
		metadata.InstalledAt = node.Value
	}
	if node := mappingValue(organizerNode, "repo-skill-path"); node != nil {
		metadata.RepoSkillPath = node.Value
	}
	if node := mappingValue(organizerNode, "last-updated-at"); node != nil {
		metadata.LastUpdatedAt = node.Value
	}

	return metadata
}

func (d *Document) SetManagedFields(flattenedName string, metadata ManagedMetadata, rename bool) {
	root := d.mapping()
	if rename {
		setScalar(root, "name", flattenedName)
	}

	metadataNode := ensureMapping(root, "metadata")
	organizerNode := ensureMapping(metadataNode, managedMetadataKey)

	setScalar(organizerNode, "original-name", metadata.OriginalName)
	setScalar(organizerNode, "source-relative-path", metadata.SourceRelativePath)
	setBool(organizerNode, "disabled", metadata.Disabled)
	if strings.TrimSpace(metadata.Source) != "" {
		setScalar(organizerNode, "source", metadata.Source)
	}
	if strings.TrimSpace(metadata.SourceType) != "" {
		setScalar(organizerNode, "source-type", metadata.SourceType)
	}
	if strings.TrimSpace(metadata.InstalledVersion) != "" {
		setScalar(organizerNode, "installed-version", metadata.InstalledVersion)
	}
	if strings.TrimSpace(metadata.InstalledAt) != "" {
		setScalar(organizerNode, "installed-at", metadata.InstalledAt)
	}
	if strings.TrimSpace(metadata.RepoSkillPath) != "" {
		setScalar(organizerNode, "repo-skill-path", metadata.RepoSkillPath)
	}
	if strings.TrimSpace(metadata.LastUpdatedAt) != "" {
		setScalar(organizerNode, "last-updated-at", metadata.LastUpdatedAt)
	}
}

func (d Document) WriteTo(path string) error {
	content, err := d.Marshal()
	if err != nil {
		return err
	}

	if err := os.WriteFile(path, content, 0o644); err != nil {
		return fmt.Errorf("write skill file: %w", err)
	}

	return nil
}

func (d Document) Marshal() ([]byte, error) {
	frontmatterContent, err := yaml.Marshal(&d.frontmatter)
	if err != nil {
		return nil, fmt.Errorf("marshal skill frontmatter: %w", err)
	}

	var out bytes.Buffer
	out.WriteString("---\n")
	out.Write(frontmatterContent)
	out.WriteString("---\n")
	if d.body != "" && !strings.HasPrefix(d.body, "\n") {
		out.WriteString("\n")
	}
	out.WriteString(d.body)

	return out.Bytes(), nil
}

func splitFrontmatter(content []byte) ([]byte, string, error) {
	text := string(content)
	if !strings.HasPrefix(text, "---\n") {
		return nil, "", fmt.Errorf("skill file is missing YAML frontmatter")
	}

	rest := strings.TrimPrefix(text, "---\n")
	end := strings.Index(rest, "\n---\n")
	if end == -1 {
		return nil, "", fmt.Errorf("skill file frontmatter is not terminated")
	}

	frontmatterContent := rest[:end]
	body := rest[end+len("\n---\n"):]
	return []byte(frontmatterContent), body, nil
}

func mappingValue(mapping *yaml.Node, key string) *yaml.Node {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil
	}

	for i := 0; i < len(mapping.Content)-1; i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}

	return nil
}

func ensureMapping(mapping *yaml.Node, key string) *yaml.Node {
	if node := mappingValue(mapping, key); node != nil && node.Kind == yaml.MappingNode {
		return node
	}

	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}
	valueNode := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	mapping.Content = append(mapping.Content, keyNode, valueNode)
	return valueNode
}

func setScalar(mapping *yaml.Node, key, value string) {
	if node := mappingValue(mapping, key); node != nil {
		node.Kind = yaml.ScalarNode
		node.Tag = "!!str"
		node.Value = value
		return
	}

	mapping.Content = append(mapping.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value},
	)
}

func setBool(mapping *yaml.Node, key string, value bool) {
	scalar := "false"
	if value {
		scalar = "true"
	}

	if node := mappingValue(mapping, key); node != nil {
		node.Kind = yaml.ScalarNode
		node.Tag = "!!bool"
		node.Value = scalar
		return
	}

	mapping.Content = append(mapping.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: scalar},
	)
}

func (d *Document) mapping() *yaml.Node {
	return d.frontmatter.Content[0]
}

func RewriteManagedFields(skill Skill, rename bool, disabled bool) error {
	return RewriteManagedFieldsWithMetadata(skill, rename, disabled, ManagedMetadata{})
}

func RewriteManagedFieldsWithMetadata(skill Skill, rename bool, disabled bool, updates ManagedMetadata) error {
	doc, err := LoadDocument(skill.SkillFile)
	if err != nil {
		return err
	}

	metadata := doc.ManagedMetadata()
	if metadata.OriginalName == "" {
		metadata.OriginalName = doc.Name()
	}
	metadata.SourceRelativePath = skill.RelativePath
	metadata.Disabled = disabled
	mergeManagedMetadata(&metadata, updates)

	doc.SetManagedFields(skill.FlattenedName, metadata, rename)
	return doc.WriteTo(skill.SkillFile)
}

func mergeManagedMetadata(target *ManagedMetadata, updates ManagedMetadata) {
	if strings.TrimSpace(updates.OriginalName) != "" {
		target.OriginalName = updates.OriginalName
	}
	if strings.TrimSpace(updates.SourceRelativePath) != "" {
		target.SourceRelativePath = updates.SourceRelativePath
	}
	if strings.TrimSpace(updates.Source) != "" {
		target.Source = updates.Source
	}
	if strings.TrimSpace(updates.SourceType) != "" {
		target.SourceType = updates.SourceType
	}
	if strings.TrimSpace(updates.InstalledVersion) != "" {
		target.InstalledVersion = updates.InstalledVersion
	}
	if strings.TrimSpace(updates.InstalledAt) != "" {
		target.InstalledAt = updates.InstalledAt
	}
	if strings.TrimSpace(updates.RepoSkillPath) != "" {
		target.RepoSkillPath = updates.RepoSkillPath
	}
	if strings.TrimSpace(updates.LastUpdatedAt) != "" {
		target.LastUpdatedAt = updates.LastUpdatedAt
	}
}
