package apispec

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

const (
	pathsKey      = "paths"
	componentsKey = "components"
	schemasKey    = "schemas"

	namedEntryDepth     = 2
	minSeparatedEntries = 2

	tagInt   = "!!int"
	tagFloat = "!!float"

	fieldSummary     = "summary"
	fieldDescription = "description"
	fieldOperationID = "operation_id"

	locationPath = "path"
	typeNull     = "null"
	typeObject   = "object"
	typeString   = "string"
	typeArray    = "array"
	typeInteger  = "integer"
	typeNumber   = "number"
	typeBoolean  = "boolean"
)

var (
	schemaNamePattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
	statusPattern     = regexp.MustCompile(`^(default|[1-5][0-9]{2}|[1-5]XX)$`)
	templateParameter = regexp.MustCompile(`\{([^{}/]+)\}`)
)

func isHTTPMethod(method string) bool {
	switch method {
	case "get", "put", "post", "delete", "options", "head", "patch", "trace":
		return true
	}
	return false
}

func isParameterLocation(in string) bool {
	switch in {
	case "query", locationPath, "header", "cookie":
		return true
	}
	return false
}

func isSchemaType(name string) bool {
	switch name {
	case typeString, typeInteger, typeNumber, typeBoolean, typeArray, typeObject:
		return true
	}
	return false
}

func strNode(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: tagStr, Value: value}
}

func boolNode(value bool) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: tagBool, Value: strconv.FormatBool(value)}
}

func newMapNode() *yaml.Node {
	return &yaml.Node{Kind: yaml.MappingNode, Tag: tagMap}
}

func newSeqNode(items ...*yaml.Node) *yaml.Node {
	return &yaml.Node{Kind: yaml.SequenceNode, Tag: tagSeq, Content: items}
}

func isFlow(node *yaml.Node) bool {
	return node.Style&yaml.FlowStyle != 0
}

func isBlockCollection(node *yaml.Node, kind yaml.Kind) bool {
	return node.Kind == kind && !isFlow(node) && len(node.Content) > 0
}

func isNull(node *yaml.Node) bool {
	return node.Kind == yaml.ScalarNode && node.ShortTag() == "!!null"
}

func scalarText(node *yaml.Node) string {
	if node == nil || node.Kind != yaml.ScalarNode {
		return ""
	}
	return node.Value
}

func keyPosition(mapping *yaml.Node, key string) int {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return -1
	}
	for i := 0; i+1 < len(mapping.Content); i += nodesPerEntry {
		if mapping.Content[i].Value == key {
			return i
		}
	}
	return -1
}

func fieldNode(mapping *yaml.Node, key string) *yaml.Node {
	if position := keyPosition(mapping, key); position >= 0 {
		return mapping.Content[position+1]
	}
	return nil
}

func removeField(mapping *yaml.Node, key string) {
	if position := keyPosition(mapping, key); position >= 0 {
		mapping.Content = slices.Delete(mapping.Content, position, position+nodesPerEntry)
	}
}

func appendPair(mapping, key, value *yaml.Node) {
	if len(mapping.Content) == 0 {
		mapping.Style &^= yaml.FlowStyle
	}
	mapping.Content = append(mapping.Content, key, value)
}

func appendItem(sequence, item *yaml.Node) {
	if len(sequence.Content) == 0 {
		sequence.Style &^= yaml.FlowStyle
	}
	sequence.Content = append(sequence.Content, item)
}

func sequenceTexts(sequence *yaml.Node) []string {
	if sequence == nil || sequence.Kind != yaml.SequenceNode {
		return nil
	}
	texts := make([]string, 0, len(sequence.Content))
	for _, item := range sequence.Content {
		texts = append(texts, scalarText(item))
	}
	return texts
}

func cloneTree(node *yaml.Node, copies map[*yaml.Node]*yaml.Node) *yaml.Node {
	if node == nil {
		return nil
	}
	if existing, done := copies[node]; done {
		return existing
	}
	clone := new(yaml.Node)
	*clone = *node
	copies[node] = clone
	if len(node.Content) > 0 {
		clone.Content = make([]*yaml.Node, len(node.Content))
		for i, child := range node.Content {
			clone.Content[i] = cloneTree(child, copies)
		}
	}
	clone.Alias = cloneTree(node.Alias, copies)
	return clone
}

// nodesEqual compares what the encoder would write: kind, style, resolved
// tag, value, anchor, comments, and children. Positions do not count.
func nodesEqual(a, b *yaml.Node) bool {
	if a.Kind != b.Kind || a.Style != b.Style || a.Anchor != b.Anchor ||
		a.HeadComment != b.HeadComment || a.LineComment != b.LineComment || a.FootComment != b.FootComment ||
		len(a.Content) != len(b.Content) {
		return false
	}
	if a.Kind != yaml.AliasNode && a.ShortTag() != b.ShortTag() {
		return false
	}
	if a.Value != b.Value {
		return false
	}
	for i := range a.Content {
		if !nodesEqual(a.Content[i], b.Content[i]) {
			return false
		}
	}
	return true
}

// sameData compares two trees as data: comments, style, and positions do not
// count.
func sameData(a, b *yaml.Node) bool {
	if a.Kind != b.Kind || len(a.Content) != len(b.Content) || a.Value != b.Value {
		return false
	}
	if a.Kind == yaml.ScalarNode && a.ShortTag() != b.ShortTag() {
		return false
	}
	for i := range a.Content {
		if !sameData(a.Content[i], b.Content[i]) {
			return false
		}
	}
	return true
}

var errPointerSyntax = errors.New("a pointer is empty or starts with /")

func parsePointerTokens(pointer string) ([]string, error) {
	rest := pointer
	if after, fragment := strings.CutPrefix(rest, "#"); fragment {
		rest = after
		if decoded, err := url.PathUnescape(rest); err == nil {
			rest = decoded
		}
	}
	if rest == "" {
		return nil, nil
	}
	if rest[0] != '/' {
		return nil, errPointerSyntax
	}
	tokens := strings.Split(rest[1:], "/")
	for i, token := range tokens {
		tokens[i] = strings.ReplaceAll(strings.ReplaceAll(token, "~1", "/"), "~0", "~")
	}
	return tokens, nil
}

func pointerFromTokens(tokens []string) string {
	var pointer strings.Builder
	for _, token := range tokens {
		pointer.WriteString("/")
		pointer.WriteString(pointerToken(token))
	}
	return pointer.String()
}

// OperationPointer is the pointer to the operation of a method on a path.
func OperationPointer(method, path string) string {
	return pointerFromTokens([]string{pathsKey, path, strings.ToLower(method)})
}

// SchemaPointer is the pointer to a schema of components/schemas.
func SchemaPointer(name string) string {
	return pointerFromTokens([]string{componentsKey, schemasKey, name})
}

// PropertyPointer is the pointer to a property of a schema, or to a nested
// property when several property names are given.
func PropertyPointer(schema string, property ...string) string {
	tokens := []string{componentsKey, schemasKey, schema}
	for _, name := range property {
		tokens = append(tokens, "properties", name)
	}
	return pointerFromTokens(tokens)
}

// ResponsePointer is the pointer to a response of an operation.
func ResponsePointer(method, path, status string) string {
	return pointerFromTokens([]string{pathsKey, path, strings.ToLower(method), "responses", status})
}

// ParameterPointer is the pointer to the parameter of an operation with a name
// and a location. The operation's own parameters are searched first, then the
// parameters of its path item. A parameter is matched by name and location,
// never by position, and a $ref parameter is read through to what it points to.
func ParameterPointer(data []byte, method, path, name, in string) (string, error) {
	root, err := decodeRoot(data)
	if err != nil {
		return "", err
	}
	method = strings.ToLower(method)
	operation := []string{pathsKey, path, method}
	item := []string{pathsKey, path}
	if fieldNode(fieldNode(fieldNode(root, pathsKey), path), method) == nil {
		return "", fmt.Errorf("%w: no operation at %s", ErrCommand, pointerFromTokens(operation))
	}
	for _, owner := range [][]string{operation, item} {
		if pointer, found := findParameter(root, owner, name, in); found {
			return pointer, nil
		}
	}
	return "", fmt.Errorf("%w: no %s parameter %q on %s", ErrCommand, in, name, pointerFromTokens(operation))
}

func findParameter(root *yaml.Node, owner []string, name, in string) (string, bool) {
	node := root
	for _, token := range owner {
		node = fieldNode(node, token)
	}
	for index, parameter := range sequenceItems(node, "parameters") {
		declaredName, declaredIn := parameterIdentityIn(root, parameter)
		if declaredName == name && declaredIn == in {
			return pointerFromTokens(append(slices.Clone(owner), "parameters", strconv.Itoa(index))), true
		}
	}
	return "", false
}

func parameterIdentityIn(root, parameter *yaml.Node) (string, string) {
	if ref := fieldNode(parameter, refKey); ref != nil {
		tokens, err := parsePointerTokens(ref.Value)
		if err != nil {
			return "", ""
		}
		node := root
		for _, token := range tokens {
			node = fieldNode(node, token)
		}
		if node != nil {
			parameter = node
		}
	}
	return scalarText(fieldNode(parameter, "name")), scalarText(fieldNode(parameter, "in"))
}
