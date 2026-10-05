package apispec

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

const maxRefDepth = 8

var versionToken = regexp.MustCompile(`^v\d+$`)

func httpMethods() []string {
	return []string{"get", "put", "post", "delete", "options", "head", "patch", "trace"}
}

const untaggedOperation = "Untagged"

type lintNode struct {
	node    *yaml.Node
	pointer string
}

type operationRecord struct {
	method   string
	path     string
	pointer  string
	tag      string
	node     *yaml.Node
	pathItem *yaml.Node
}

func (o operationRecord) label() string {
	return strings.ToUpper(o.method) + " " + o.path
}

type propertyRecord struct {
	name          string
	pointer       string
	described     bool
	referenceOnly bool
}

func stringValue(node *yaml.Node) (string, bool) {
	node = resolveAlias(node)
	if node == nil || node.Kind != yaml.ScalarNode || node.ShortTag() != "!!str" {
		return "", false
	}
	return node.Value, true
}

func isNonBlankString(node *yaml.Node) bool {
	value, isString := stringValue(node)
	return isString && strings.TrimSpace(value) != ""
}

func mappingOf(node *yaml.Node) []mappingEntry {
	node = resolveAlias(node)
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	return mappingEntries(node)
}

func sequenceOf(node *yaml.Node) []*yaml.Node {
	node = resolveAlias(node)
	if node == nil || node.Kind != yaml.SequenceNode {
		return nil
	}
	return node.Content
}

func childPointer(pointer string, segments ...string) string {
	var builder strings.Builder
	builder.WriteString(pointer)
	for _, segment := range segments {
		builder.WriteByte('/')
		builder.WriteString(pointerToken(segment))
	}
	return builder.String()
}

func followPointer(root *yaml.Node, ref string) *yaml.Node {
	if !strings.HasPrefix(ref, "#") {
		return nil
	}
	rest := strings.TrimPrefix(ref, "#")
	current := root
	if rest == "" || rest == "/" {
		return current
	}
	if !strings.HasPrefix(rest, "/") {
		return nil
	}
	for token := range strings.SplitSeq(rest[1:], "/") {
		if decoded, err := url.PathUnescape(token); err == nil {
			token = decoded
		}
		token = strings.ReplaceAll(strings.ReplaceAll(token, "~1", "/"), "~0", "~")
		current = resolveAlias(current)
		if current == nil {
			return nil
		}
		switch current.Kind {
		case yaml.MappingNode:
			current = lookup(current, token)
		case yaml.SequenceNode:
			index, err := strconv.Atoi(token)
			if err != nil || index < 0 || index >= len(current.Content) {
				return nil
			}
			current = current.Content[index]
		case yaml.DocumentNode, yaml.ScalarNode, yaml.AliasNode:
			return nil
		}
		if current == nil {
			return nil
		}
	}
	return resolveAlias(current)
}

func resolveReferences(root, node *yaml.Node) *yaml.Node {
	node = resolveAlias(node)
	for range maxRefDepth {
		ref, isString := stringValue(lookup(node, refKey))
		if !isString {
			break
		}
		target := followPointer(root, ref)
		if target == nil {
			break
		}
		node = target
	}
	return node
}

func operationTag(path string, operation *yaml.Node) string {
	if tags := sequenceOf(lookup(operation, "tags")); len(tags) > 0 {
		if first, isString := stringValue(tags[0]); isString {
			return first
		}
	}
	segments := make([]string, 0)
	for segment := range strings.SplitSeq(path, "/") {
		if segment != "" {
			segments = append(segments, segment)
		}
	}
	if len(segments) > 0 && versionToken.MatchString(segments[0]) {
		segments = segments[1:]
	}
	for _, segment := range segments {
		if !strings.HasPrefix(segment, "{") {
			return segment
		}
	}
	return untaggedOperation
}

func collectOperations(root *yaml.Node) []operationRecord {
	var records []operationRecord
	for _, pathEntry := range mappingOf(lookup(root, "paths")) {
		path := pathEntry.key.Value
		if strings.HasPrefix(path, extensionPrefix) {
			continue
		}
		pathItem := resolveAlias(pathEntry.value)
		for _, method := range httpMethods() {
			operation := lookup(pathItem, method)
			if operation == nil || operation.Kind != yaml.MappingNode {
				continue
			}
			records = append(records, operationRecord{
				method:   method,
				path:     path,
				pointer:  childPointer("/paths", path, method),
				tag:      operationTag(path, operation),
				node:     operation,
				pathItem: pathItem,
			})
		}
	}
	return records
}

func collectProperties(schema *yaml.Node, pointer string, into *[]propertyRecord) {
	schema = resolveAlias(schema)
	if schema == nil || schema.Kind != yaml.MappingNode {
		return
	}
	for _, entry := range mappingOf(lookup(schema, "properties")) {
		property := resolveAlias(entry.value)
		propertyPointer := childPointer(pointer, "properties", entry.key.Value)
		_, referenceOnly := stringValue(lookup(property, refKey))
		*into = append(*into, propertyRecord{
			name:          entry.key.Value,
			pointer:       propertyPointer,
			described:     isNonBlankString(lookup(property, "description")),
			referenceOnly: referenceOnly,
		})
		collectProperties(property, propertyPointer, into)
	}
	collectProperties(lookup(schema, "items"), childPointer(pointer, "items"), into)
	collectProperties(lookup(schema, "additionalProperties"), childPointer(pointer, "additionalProperties"), into)
	for _, keyword := range []string{"allOf", "anyOf", "oneOf"} {
		for index, member := range sequenceOf(lookup(schema, keyword)) {
			collectProperties(member, childPointer(pointer, keyword, strconv.Itoa(index)), into)
		}
	}
}

func componentSchemaProperties(root *yaml.Node) []propertyRecord {
	var records []propertyRecord
	for _, entry := range mappingOf(lookup(lookup(root, "components"), "schemas")) {
		collectProperties(entry.value, childPointer("/components/schemas", entry.key.Value), &records)
	}
	return records
}

func holderSchemas(holder *yaml.Node, pointer string, into *[]lintNode) {
	if schema := lookup(holder, "schema"); schema != nil {
		*into = append(*into, lintNode{node: schema, pointer: childPointer(pointer, "schema")})
	}
	for _, media := range mappingOf(lookup(holder, "content")) {
		if schema := lookup(media.value, "schema"); schema != nil {
			*into = append(*into, lintNode{
				node:    schema,
				pointer: childPointer(pointer, "content", media.key.Value, "schema"),
			})
		}
	}
}

func responseSchemas(response *yaml.Node, pointer string, into *[]lintNode) {
	holderSchemas(response, pointer, into)
	for _, header := range mappingOf(lookup(response, "headers")) {
		holderSchemas(header.value, childPointer(pointer, "headers", header.key.Value), into)
	}
}

func operationSchemas(operation operationRecord, into *[]lintNode) {
	pathPointer := childPointer("/paths", operation.path)
	for index, parameter := range sequenceOf(lookup(operation.pathItem, "parameters")) {
		holderSchemas(parameter, childPointer(pathPointer, "parameters", strconv.Itoa(index)), into)
	}
	for index, parameter := range sequenceOf(lookup(operation.node, "parameters")) {
		holderSchemas(parameter, childPointer(operation.pointer, "parameters", strconv.Itoa(index)), into)
	}
	holderSchemas(lookup(operation.node, "requestBody"), childPointer(operation.pointer, "requestBody"), into)
	for _, response := range mappingOf(lookup(operation.node, "responses")) {
		responseSchemas(response.value, childPointer(operation.pointer, "responses", response.key.Value), into)
	}
}

func componentSchemas(root *yaml.Node, into *[]lintNode) {
	components := lookup(root, "components")
	for _, entry := range mappingOf(lookup(components, "schemas")) {
		pointer := childPointer("/components/schemas", entry.key.Value)
		*into = append(*into, lintNode{node: entry.value, pointer: pointer})
	}
	for _, section := range []string{"parameters", "requestBodies", "headers"} {
		for _, entry := range mappingOf(lookup(components, section)) {
			holderSchemas(entry.value, childPointer("/components", section, entry.key.Value), into)
		}
	}
	for _, entry := range mappingOf(lookup(components, "responses")) {
		responseSchemas(entry.value, childPointer("/components/responses", entry.key.Value), into)
	}
}

func lintedSchemaProperties(root *yaml.Node, operations []operationRecord) []propertyRecord {
	var roots []lintNode
	componentSchemas(root, &roots)
	for _, operation := range operations {
		operationSchemas(operation, &roots)
	}
	var records []propertyRecord
	seen := make(map[string]bool, len(roots))
	for _, schema := range roots {
		if !seen[schema.pointer] {
			seen[schema.pointer] = true
			collectProperties(schema.node, schema.pointer, &records)
		}
	}
	return records
}

func lintedParameters(root *yaml.Node, operations []operationRecord) []lintNode {
	var parameters []lintNode
	add := func(node *yaml.Node, pointer string) {
		if _, isReference := stringValue(lookup(node, refKey)); !isReference {
			parameters = append(parameters, lintNode{node: node, pointer: pointer})
		}
	}
	for _, entry := range mappingOf(lookup(lookup(root, "components"), "parameters")) {
		add(entry.value, childPointer("/components/parameters", entry.key.Value))
	}
	seenPathItems := make(map[string]bool)
	for _, operation := range operations {
		pathPointer := childPointer("/paths", operation.path)
		if !seenPathItems[pathPointer] {
			seenPathItems[pathPointer] = true
			for index, parameter := range sequenceOf(lookup(operation.pathItem, "parameters")) {
				add(parameter, childPointer(pathPointer, "parameters", strconv.Itoa(index)))
			}
		}
		for index, parameter := range sequenceOf(lookup(operation.node, "parameters")) {
			add(parameter, childPointer(operation.pointer, "parameters", strconv.Itoa(index)))
		}
	}
	return parameters
}
