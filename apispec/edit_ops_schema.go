package apispec

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

const (
	schemaRefPrefix = "#/components/schemas/"
	maxListedRefs   = 5
)

func (e *editor) applyType(schema *yaml.Node, typ string, nullable bool, format string) {
	removeField(schema, refKey)
	switch {
	case e.is31 && nullable:
		nullName := strNode(typeNull)
		nullName.Style = yaml.DoubleQuotedStyle
		union := newSeqNode(strNode(typ), nullName)
		union.Style = yaml.FlowStyle
		e.put(schema, kindSchema, "type", union)
		removeField(schema, "nullable")
	case nullable:
		e.putString(schema, kindSchema, "type", typ)
		e.putBool(schema, kindSchema, "nullable", true)
	default:
		e.putString(schema, kindSchema, "type", typ)
		removeField(schema, "nullable")
	}
	if format == "" {
		removeField(schema, "format")
		return
	}
	e.putString(schema, kindSchema, "format", format)
}

func (e *editor) setType() error {
	schema, err := e.at(e.command.Pointer, "schema", nil)
	if err != nil {
		return err
	}
	e.applyType(schema.node, e.command.Type, e.command.Nullable, e.command.Format)
	return nil
}

func (e *editor) refNode(name string) *yaml.Node {
	node := newMapNode()
	node.Content = []*yaml.Node{strNode(refKey), strNode(schemaRefPrefix + pointerToken(name))}
	return node
}

func (e *editor) setRef() error {
	schema, err := e.at(e.command.Pointer, "schema", nil)
	if err != nil {
		return err
	}
	if !e.schemaExists(e.command.Schema) {
		return e.refusef("no schema %s in components/schemas", e.command.Schema)
	}
	content := e.refNode(e.command.Schema).Content
	if position := keyPosition(schema.node, fieldDescription); position >= 0 {
		content = append(content, schema.node.Content[position], schema.node.Content[position+1])
	}
	if len(schema.node.Content) == 0 {
		schema.node.Style &^= yaml.FlowStyle
	}
	schema.node.Content = content
	return nil
}

func (e *editor) requiredList(schema *yaml.Node) (*yaml.Node, error) {
	return e.ensureSeq(schema, kindSchema, "required")
}

func (e *editor) addRequired(schema *yaml.Node, name string) error {
	list, err := e.requiredList(schema)
	if err != nil {
		return err
	}
	if !slices.Contains(sequenceTexts(list), name) {
		appendItem(list, strNode(name))
	}
	return nil
}

func dropRequired(schema *yaml.Node, name string) {
	list := fieldNode(schema, "required")
	if list == nil || list.Kind != yaml.SequenceNode {
		return
	}
	before := len(list.Content)
	list.Content = slices.DeleteFunc(list.Content, func(item *yaml.Node) bool {
		return scalarText(item) == name
	})
	if before != len(list.Content) && len(list.Content) == 0 {
		removeField(schema, "required")
	}
}

func renameRequired(schema *yaml.Node, from, to string) {
	list := fieldNode(schema, "required")
	if list == nil || list.Kind != yaml.SequenceNode {
		return
	}
	for _, item := range list.Content {
		if item.Kind == yaml.ScalarNode && item.Value == from {
			setScalarString(item, to)
		}
	}
}

func (e *editor) setRequired() error {
	property, err := e.property(e.command.Pointer)
	if err != nil {
		return err
	}
	schema, err := e.above(property, namedEntryDepth)
	if err != nil {
		return err
	}
	name := property.tokens[len(property.tokens)-1]
	if *e.command.Required {
		return e.addRequired(schema.node, name)
	}
	dropRequired(schema.node, name)
	return nil
}

func (e *editor) renameProperty() error {
	property, err := e.property(e.command.Pointer)
	if err != nil {
		return err
	}
	previous := property.key.Value
	name := e.command.Name
	if name == previous {
		return nil
	}
	if keyPosition(property.owner, name) >= 0 {
		return e.refusef("property %s already exists", name)
	}
	schema, err := e.above(property, namedEntryDepth)
	if err != nil {
		return err
	}
	property.key.Value = name
	renameRequired(schema.node, previous, name)
	return nil
}

func (e *editor) addProperty() error {
	c := e.command
	schema, err := e.at(c.Pointer, "schema", nil)
	if err != nil {
		return err
	}
	if fieldNode(schema.node, refKey) != nil {
		return e.refusef("the schema at %s is a $ref: add the property to the schema it points to",
			pointerFromTokens(schema.tokens))
	}
	properties, err := e.ensureMap(schema.node, kindSchema, "properties")
	if err != nil {
		return err
	}
	if keyPosition(properties, c.Name) >= 0 {
		return e.refusef("property %s already exists", c.Name)
	}
	property := newMapNode()
	e.applyType(property, orDefault(c.Type, typeString), c.Nullable, c.Format)
	if c.Description != nil && *c.Description != "" {
		e.putString(property, kindSchema, fieldDescription, *c.Description)
	}
	appendPair(properties, strNode(c.Name), property)
	if c.Required != nil && *c.Required {
		return e.addRequired(schema.node, c.Name)
	}
	return nil
}

func orDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func (e *editor) removeProperty() error {
	property, err := e.property(e.command.Pointer)
	if err != nil {
		return err
	}
	schema, err := e.above(property, namedEntryDepth)
	if err != nil {
		return err
	}
	property.remove()
	dropRequired(schema.node, property.tokens[len(property.tokens)-1])
	return nil
}

func schemaTypeName(schema *yaml.Node) string {
	typeNode := fieldNode(schema, "type")
	if typeNode == nil {
		return ""
	}
	if typeNode.Kind == yaml.ScalarNode {
		return typeNode.Value
	}
	for _, item := range typeNode.Content {
		if item.Value != typeNull {
			return item.Value
		}
	}
	return ""
}

func (e *editor) enumItem(typ, value string) (*yaml.Node, error) {
	switch typ {
	case typeInteger:
		if _, err := strconv.ParseInt(value, 10, 64); err != nil {
			return nil, e.refusef("enum value %q is not an integer", value)
		}
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: tagInt, Value: value}, nil
	case typeNumber:
		if _, err := strconv.ParseFloat(value, 64); err != nil {
			return nil, e.refusef("enum value %q is not a number", value)
		}
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: tagFloat, Value: value}, nil
	case typeBoolean:
		if value != "true" && value != "false" {
			return nil, e.refusef("enum value %q is not true or false", value)
		}
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: tagBool, Value: value}, nil
	}
	return strNode(value), nil
}

func (e *editor) setEnum() error {
	schema, err := e.at(e.command.Pointer, "schema", nil)
	if err != nil {
		return err
	}
	if len(e.command.Values) == 0 {
		removeField(schema.node, "enum")
		return nil
	}
	list := newSeqNode()
	typ := schemaTypeName(schema.node)
	for _, value := range e.command.Values {
		var item *yaml.Node
		if item, err = e.enumItem(typ, value); err != nil {
			return err
		}
		list.Content = append(list.Content, item)
	}
	if previous := fieldNode(schema.node, "enum"); previous != nil && previous.Kind == yaml.SequenceNode &&
		isFlow(previous) {
		list.Style = yaml.FlowStyle
	}
	e.put(schema.node, kindSchema, "enum", list)
	return nil
}

func (e *editor) addSchema() error {
	c := e.command
	components, err := e.ensureMap(e.root, kindRoot, componentsKey)
	if err != nil {
		return err
	}
	schemas, err := e.ensureMap(components, kindComponents, schemasKey)
	if err != nil {
		return err
	}
	if keyPosition(schemas, c.Name) >= 0 {
		return e.refusef("schema %s already exists", c.Name)
	}
	schema := newMapNode()
	e.putString(schema, kindSchema, "type", orDefault(c.Type, typeObject))
	if c.Description != nil && *c.Description != "" {
		e.putString(schema, kindSchema, fieldDescription, *c.Description)
	}
	appendPair(schemas, strNode(c.Name), schema)
	return nil
}

func (e *editor) removeSchema() error {
	c := e.command
	pointer := c.Pointer
	if pointer == "" {
		pointer = SchemaPointer(c.Name)
	}
	schema, err := e.at(pointer, "schema", func(tokens []string) bool {
		return len(tokens) == 3 && tokens[0] == componentsKey && tokens[1] == schemasKey
	})
	if err != nil {
		return err
	}
	name := schema.tokens[2]
	references := schemaReferences(e.root, []string{componentsKey, schemasKey, name}, schema.node)
	if references.total > 0 {
		return e.refusef("schema %s is still referenced by %s", name, references.describe())
	}
	schema.remove()
	return nil
}

type referenceList struct {
	pointers []string
	total    int
}

func (r *referenceList) describe() string {
	text := strings.Join(r.pointers, ", ")
	if more := r.total - len(r.pointers); more > 0 {
		text += fmt.Sprintf(" and %d more", more)
	}
	return text
}

func schemaReferences(root *yaml.Node, schemaTokens []string, schema *yaml.Node) *referenceList {
	references := &referenceList{}
	collectReferences(root, nil, schemaTokens, schema, references)
	return references
}

func collectReferences(node *yaml.Node, path, schemaTokens []string, skip *yaml.Node, found *referenceList) {
	if node == skip {
		return
	}
	switch node.Kind {
	case yaml.MappingNode:
		for i := 0; i+1 < len(node.Content); i += nodesPerEntry {
			key, value := node.Content[i], node.Content[i+1]
			if key.Value == refKey && value.Kind == yaml.ScalarNode && refTargets(value.Value, schemaTokens) {
				found.total++
				if len(found.pointers) < maxListedRefs {
					found.pointers = append(found.pointers, pointerFromTokens(path))
				}
				continue
			}
			collectReferences(value, appendToken(path, key.Value), schemaTokens, skip, found)
		}
	case yaml.SequenceNode:
		for index, item := range node.Content {
			collectReferences(item, appendToken(path, strconv.Itoa(index)), schemaTokens, skip, found)
		}
	case yaml.DocumentNode, yaml.ScalarNode, yaml.AliasNode:
	}
}

func appendToken(path []string, token string) []string {
	return append(slices.Clip(path), token)
}

func refTargets(ref string, tokens []string) bool {
	if !strings.HasPrefix(ref, "#") {
		return false
	}
	parsed, err := parsePointerTokens(ref)
	return err == nil && slices.Equal(parsed, tokens)
}
