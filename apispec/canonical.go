package apispec

import (
	"bytes"
	"fmt"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

// LayoutVersion identifies the canonical layout Canonical writes. It changes
// whenever the canonical output of any document changes, so a stored Live
// version can say which layout its bytes follow.
const LayoutVersion = 1

const canonicalIndent = 2

const (
	tagNull = "!!null"
	tagBool = "!!bool"
	tagStr  = "!!str"
	tagMap  = "!!map"
	tagSeq  = "!!seq"

	nullValue = "null"
)

// Canonical writes the document in the canonical YAML layout (ADR-0017):
// OpenAPI fields in a fixed order, extensions and unknown fields after them
// sorted by name, user-named maps (paths, schemas, properties, responses) in
// document order, two-space indentation, block style, no comments, no anchors.
func (d *Document) Canonical() ([]byte, error) {
	canonicalRoot := newLayouts().canonicalNode(d.root, object(kindRoot))
	if d.version != nil {
		setInfoVersion(canonicalRoot, *d.version)
	}

	var out bytes.Buffer
	encoder := yaml.NewEncoder(&out)
	encoder.SetIndent(canonicalIndent)
	if err := encoder.Encode(canonicalRoot); err != nil {
		return nil, fmt.Errorf("encode canonical yaml: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return nil, fmt.Errorf("encode canonical yaml: %w", err)
	}
	return out.Bytes(), nil
}

func (l layouts) canonicalNode(node *yaml.Node, shape valueShape) *yaml.Node {
	node = resolveAlias(node)
	switch node.Kind {
	case yaml.MappingNode:
		return l.canonicalMapping(node, shape)
	case yaml.SequenceNode:
		items := make([]*yaml.Node, 0, len(node.Content))
		for _, item := range node.Content {
			items = append(items, l.canonicalNode(item, itemShape(shape)))
		}
		return &yaml.Node{Kind: yaml.SequenceNode, Tag: tagSeq, Content: items}
	case yaml.ScalarNode:
		return canonicalScalar(node)
	case yaml.DocumentNode:
		if len(node.Content) == 0 {
			return &yaml.Node{Kind: yaml.ScalarNode, Tag: tagNull, Value: nullValue}
		}
		return l.canonicalNode(node.Content[0], shape)
	case yaml.AliasNode:
		return l.canonicalNode(node.Alias, shape)
	}
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: tagNull, Value: nullValue}
}

func (l layouts) canonicalMapping(node *yaml.Node, shape valueShape) *yaml.Node {
	entries := mappingEntries(node)
	if shape.kind != kindAny && shape.container == containerSingle {
		rank := l[shape.kind].rank
		slices.SortStableFunc(entries, func(a, b mappingEntry) int {
			return compareFieldOrder(rank, a.key.Value, b.key.Value)
		})
	}

	content := make([]*yaml.Node, 0, len(entries)*nodesPerEntry)
	for _, entry := range entries {
		key := resolveAlias(entry.key)
		content = append(content,
			canonicalString(key.Value),
			l.canonicalNode(entry.value, l.entryShape(shape, key.Value)),
		)
	}
	return &yaml.Node{Kind: yaml.MappingNode, Tag: tagMap, Content: content}
}

const refKey = "$ref"

func compareFieldOrder(rank map[string]int, a, b string) int {
	return compareRanked(fieldRank(rank, a), fieldRank(rank, b), a, b)
}

func fieldRank(rank map[string]int, key string) int {
	if key == refKey {
		return -1
	}
	if position, known := rank[key]; known {
		return position
	}
	return len(rank)
}

func compareRanked(rankA, rankB int, a, b string) int {
	if rankA != rankB {
		return rankA - rankB
	}
	return strings.Compare(a, b)
}

func canonicalScalar(node *yaml.Node) *yaml.Node {
	tag := node.ShortTag()
	value := node.Value
	switch tag {
	case tagNull:
		value = nullValue
	case tagBool:
		value = strings.ToLower(value)
	case tagStr:
		return canonicalString(value)
	}
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: tag, Value: value}
}

// canonicalString quotes the strings YAML 1.1 reads as booleans ("yes",
// "on", "n", ...), so tools that still read YAML 1.1 see a string too.
func canonicalString(value string) *yaml.Node {
	node := &yaml.Node{Kind: yaml.ScalarNode, Tag: tagStr, Value: value}
	if isYAML11Bool(value) {
		node.Style = yaml.DoubleQuotedStyle
	}
	return node
}

func isYAML11Bool(value string) bool {
	switch strings.ToLower(value) {
	case "y", "yes", "n", "no", "on", "off", "true", "false":
		return true
	}
	return false
}

func setInfoVersion(canonicalRoot *yaml.Node, version string) {
	versionNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: tagStr, Value: version}
	info := lookup(canonicalRoot, "info")
	if info == nil || info.Kind != yaml.MappingNode {
		info = &yaml.Node{Kind: yaml.MappingNode, Tag: tagMap}
		setEntry(canonicalRoot, "info", info)
	}
	setEntry(info, "version", versionNode)
}

// setEntry replaces a key's value, or appends the key. It only runs on trees
// canonicalNode built, which no one else holds.
func setEntry(mapping *yaml.Node, key string, value *yaml.Node) {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			mapping.Content[i+1] = value
			return
		}
	}
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: tagStr, Value: key}
	mapping.Content = append(mapping.Content, keyNode, value)
}
