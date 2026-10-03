package apispec

import (
	"strings"

	"go.yaml.in/yaml/v3"
)

const extensionPrefix = "x-"

const nodesPerEntry = 2

type objectLayout struct {
	rank   map[string]int
	shapes map[string]valueShape
}

type layouts map[objectKind]objectLayout

func newLayouts() layouts {
	result := make(layouts, int(kindOAuthFlow)+1)
	for kind := kindAny; kind <= kindOAuthFlow; kind++ {
		fields := objectFields(kind)
		layout := objectLayout{
			rank:   make(map[string]int, len(fields)),
			shapes: make(map[string]valueShape, len(fields)),
		}
		for position, f := range fields {
			layout.rank[f.name] = position
			layout.shapes[f.name] = f.shape
		}
		result[kind] = layout
	}
	return result
}

func (l layouts) entryShape(parent valueShape, key string) valueShape {
	if parent.kind == kindAny {
		return object(kindAny)
	}
	switch parent.container {
	case containerMap:
		return object(parent.kind)
	case containerExtensibleMap:
		if strings.HasPrefix(key, extensionPrefix) {
			return object(kindAny)
		}
		return object(parent.kind)
	case containerList:
		return object(kindAny)
	case containerSingle:
		if shape, known := l[parent.kind].shapes[key]; known {
			return shape
		}
	}
	return object(kindAny)
}

func itemShape(parent valueShape) valueShape {
	if parent.container == containerList {
		return object(parent.kind)
	}
	return object(kindAny)
}

func resolveAlias(node *yaml.Node) *yaml.Node {
	for node != nil && node.Kind == yaml.AliasNode {
		node = node.Alias
	}
	return node
}

type mappingEntry struct {
	key   *yaml.Node
	value *yaml.Node
}

// mappingEntries returns a mapping's key/value pairs with YAML merge keys
// ("<<") applied: merged entries come after the mapping's own and never
// override them.
func mappingEntries(node *yaml.Node) []mappingEntry {
	own := make([]mappingEntry, 0, len(node.Content)/nodesPerEntry)
	var merged []*yaml.Node
	for i := 0; i+1 < len(node.Content); i += 2 {
		key, value := node.Content[i], node.Content[i+1]
		if key.Kind == yaml.ScalarNode && key.Value == "<<" && key.ShortTag() == "!!merge" {
			merged = append(merged, mergeSources(value)...)
			continue
		}
		own = append(own, mappingEntry{key: key, value: value})
	}

	seen := make(map[string]bool, len(own))
	for _, entry := range own {
		seen[entry.key.Value] = true
	}
	for _, source := range merged {
		for _, entry := range mappingEntries(source) {
			if !seen[entry.key.Value] {
				seen[entry.key.Value] = true
				own = append(own, entry)
			}
		}
	}
	return own
}

func mergeSources(value *yaml.Node) []*yaml.Node {
	value = resolveAlias(value)
	switch value.Kind {
	case yaml.MappingNode:
		return []*yaml.Node{value}
	case yaml.SequenceNode:
		sources := make([]*yaml.Node, 0, len(value.Content))
		for _, item := range value.Content {
			if item = resolveAlias(item); item.Kind == yaml.MappingNode {
				sources = append(sources, item)
			}
		}
		return sources
	case yaml.DocumentNode, yaml.ScalarNode, yaml.AliasNode:
		return nil
	}
	return nil
}

func lookup(node *yaml.Node, key string) *yaml.Node {
	node = resolveAlias(node)
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for _, entry := range mappingEntries(node) {
		if entry.key.Value == key {
			return resolveAlias(entry.value)
		}
	}
	return nil
}

func pointerToken(key string) string {
	return strings.ReplaceAll(strings.ReplaceAll(key, "~", "~0"), "/", "~1")
}
