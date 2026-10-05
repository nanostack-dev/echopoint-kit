package apispec

import (
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

func textKey(field string) string {
	if field == fieldOperationID {
		return "operationId"
	}
	return field
}

func kindAt(tokens []string) objectKind {
	switch {
	case isOperationTokens(tokens):
		return kindOperation
	case isPathItemTokens(tokens):
		return kindPathItem
	case len(tokens) >= 2 && tokens[len(tokens)-2] == "parameters":
		return kindParameter
	case len(tokens) >= 2 && tokens[len(tokens)-2] == "responses":
		return kindResponse
	}
	return kindSchema
}

func (e *editor) setText() error {
	c := e.command
	tokens, err := parsePointerTokens(c.Pointer)
	if err != nil {
		return e.refusef("invalid pointer %q: %s", c.Pointer, err)
	}
	var found target
	switch {
	case isOperationTokens(tokens):
		found, err = e.operation(c.Pointer)
	case c.Field != fieldDescription:
		return e.refusef("%s can only be set on an operation", c.Field)
	default:
		found, err = e.at(c.Pointer, "node", nil)
	}
	if err != nil {
		return err
	}
	if c.Value == "" {
		removeField(found.node, textKey(c.Field))
		return nil
	}
	e.putString(found.node, kindAt(tokens), textKey(c.Field), c.Value)
	return nil
}

func (e *editor) renamePath() error {
	tokens, err := parsePointerTokens(e.command.Pointer)
	if err != nil {
		return e.refusef("invalid pointer %q: %s", e.command.Pointer, err)
	}
	if len(tokens) < 2 || tokens[0] != pathsKey {
		return e.refusef("no path item at %s", pointerFromTokens(tokens))
	}
	item, err := e.at(pointerFromTokens(tokens[:2]), "path item", nil)
	if err != nil {
		return err
	}
	if e.command.Path == item.key.Value {
		return nil
	}
	if keyPosition(item.owner, e.command.Path) >= 0 {
		return e.refusef("path %s already exists", e.command.Path)
	}
	item.key.Value = e.command.Path
	return nil
}

func (e *editor) setMethod() error {
	operation, err := e.operation(e.command.Pointer)
	if err != nil {
		return err
	}
	method := strings.ToLower(e.command.Method)
	if method == operation.key.Value {
		return nil
	}
	if keyPosition(operation.owner, method) >= 0 {
		return e.refusef("operation %s %s already exists", method, operation.tokens[1])
	}
	operation.key.Value = method
	return nil
}

func tagsNode(tags []string) *yaml.Node {
	list := newSeqNode()
	for _, tag := range tags {
		list.Content = append(list.Content, strNode(tag))
	}
	return list
}

func (e *editor) setTags() error {
	operation, err := e.operation(e.command.Pointer)
	if err != nil {
		return err
	}
	if len(e.command.Tags) == 0 {
		removeField(operation.node, "tags")
		return nil
	}
	tags := tagsNode(e.command.Tags)
	if previous := fieldNode(operation.node, "tags"); previous != nil && previous.Kind == yaml.SequenceNode &&
		isFlow(previous) {
		tags.Style = yaml.FlowStyle
	}
	e.put(operation.node, kindOperation, "tags", tags)
	return nil
}

// parameterIdentity is the name and location of a parameter, read through a
// local $ref when the parameter is one.
func (e *editor) parameterIdentity(parameter *yaml.Node) (string, string) {
	if ref := fieldNode(parameter, refKey); ref != nil {
		tokens, err := parsePointerTokens(ref.Value)
		if err != nil {
			return "", ""
		}
		if resolved, status := e.walk(tokens); status == lookupFound {
			parameter = resolved.node
		}
	}
	return scalarText(fieldNode(parameter, "name")), scalarText(fieldNode(parameter, "in"))
}

func (e *editor) declaresPathParameter(owner *yaml.Node, name string) bool {
	for _, parameter := range sequenceItems(owner, "parameters") {
		declaredName, in := e.parameterIdentity(parameter)
		if declaredName == name && in == locationPath {
			return true
		}
	}
	return false
}

func sequenceItems(mapping *yaml.Node, key string) []*yaml.Node {
	list := fieldNode(mapping, key)
	if list == nil || list.Kind != yaml.SequenceNode {
		return nil
	}
	return list.Content
}

func (e *editor) newPathParameter(name string) *yaml.Node {
	parameter := newMapNode()
	e.putString(parameter, kindParameter, "name", name)
	e.putString(parameter, kindParameter, "in", locationPath)
	e.putBool(parameter, kindParameter, "required", true)
	schema := newMapNode()
	e.putString(schema, kindSchema, "type", typeString)
	e.put(parameter, kindParameter, "schema", schema)
	return parameter
}

func (e *editor) undeclaredPathParameters(item *yaml.Node, path string) []*yaml.Node {
	var parameters []*yaml.Node
	for _, match := range templateParameter.FindAllStringSubmatch(path, -1) {
		name := match[1]
		alreadyAdded := slices.ContainsFunc(parameters, func(parameter *yaml.Node) bool {
			return scalarText(fieldNode(parameter, "name")) == name
		})
		if !alreadyAdded && !e.declaresPathParameter(item, name) {
			parameters = append(parameters, e.newPathParameter(name))
		}
	}
	return parameters
}

func (e *editor) addOperation() error {
	c := e.command
	method := strings.ToLower(c.Method)
	paths, err := e.ensureMap(e.root, kindRoot, pathsKey)
	if err != nil {
		return err
	}
	item := fieldNode(paths, c.Path)
	if item == nil {
		item = newMapNode()
		appendPair(paths, strNode(c.Path), item)
	}
	if item.Kind != yaml.MappingNode {
		return e.refusef("the path item %s is not an object", c.Path)
	}
	if keyPosition(item, method) >= 0 {
		return e.refusef("operation %s %s already exists", method, c.Path)
	}
	operation := newMapNode()
	e.describeOperation(operation)
	if parameters := e.undeclaredPathParameters(item, c.Path); len(parameters) > 0 {
		e.put(operation, kindOperation, "parameters", newSeqNode(parameters...))
	}
	status := c.Status
	if status == "" {
		status = "200"
	}
	responses := newMapNode()
	appendPair(responses, e.statusKey(responses, status), e.newResponse(defaultResponseDescription(status)))
	e.put(operation, kindOperation, "responses", responses)
	e.put(item, kindPathItem, method, operation)
	return nil
}

func (e *editor) describeOperation(operation *yaml.Node) {
	c := e.command
	if len(c.Tags) > 0 {
		e.put(operation, kindOperation, "tags", tagsNode(c.Tags))
	}
	if c.Summary != "" {
		e.putString(operation, kindOperation, fieldSummary, c.Summary)
	}
	if c.Description != nil && *c.Description != "" {
		e.putString(operation, kindOperation, fieldDescription, *c.Description)
	}
	if c.OperationID != "" {
		e.putString(operation, kindOperation, "operationId", c.OperationID)
	}
}

func (e *editor) removeOperation() error {
	operation, err := e.operation(e.command.Pointer)
	if err != nil {
		return err
	}
	item, err := e.above(operation, 1)
	if err != nil {
		return err
	}
	operation.remove()
	if !hasOperation(item.node) {
		item.remove()
	}
	return nil
}

func hasOperation(item *yaml.Node) bool {
	if fieldNode(item, refKey) != nil {
		return true
	}
	for i := 0; i+1 < len(item.Content); i += nodesPerEntry {
		if isHTTPMethod(item.Content[i].Value) {
			return true
		}
	}
	return false
}
