package apispec

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// editor applies commands to the working copy of a document tree.
type editor struct {
	root    *yaml.Node
	is31    bool
	layouts layouts
	index   int
	command Command
}

func newEditor(work *yaml.Node) *editor {
	root := work.Content[0]
	return &editor{
		root:    root,
		is31:    strings.HasPrefix(scalarText(fieldNode(root, "openapi")), "3.1"),
		layouts: newLayouts(),
	}
}

func (e *editor) refusef(format string, args ...any) error {
	return &CommandError{Index: e.index, Command: e.command, Reason: fmt.Sprintf(format, args...)}
}

func (e *editor) run(index int, command Command) error {
	e.index, e.command = index, command
	if reason := command.invalid(); reason != "" {
		return e.refusef("%s", reason)
	}
	return e.apply()
}

func (e *editor) apply() error {
	switch e.command.Kind {
	case CommandSetText:
		return e.setText()
	case CommandRenamePath:
		return e.renamePath()
	case CommandSetMethod:
		return e.setMethod()
	case CommandSetParameterField:
		return e.setParameterField()
	case CommandSetParameterRequired:
		return e.setParameterRequired()
	case CommandSetParameterType:
		return e.setParameterType()
	case CommandAddParameter:
		return e.addParameter()
	case CommandRemoveParameter:
		return e.removeParameter()
	case CommandRenameProperty:
		return e.renameProperty()
	case CommandSetType:
		return e.setType()
	case CommandSetRef:
		return e.setRef()
	case CommandSetRequired:
		return e.setRequired()
	case CommandSetEnum:
		return e.setEnum()
	case CommandAddProperty:
		return e.addProperty()
	case CommandRemoveProperty:
		return e.removeProperty()
	case CommandAddOperation:
		return e.addOperation()
	case CommandRemoveOperation:
		return e.removeOperation()
	case CommandSetTags:
		return e.setTags()
	case CommandAddSchema:
		return e.addSchema()
	case CommandRemoveSchema:
		return e.removeSchema()
	case CommandAddResponse:
		return e.addResponse()
	case CommandSetResponse:
		return e.setResponse()
	case CommandRemoveResponse:
		return e.removeResponse()
	}
	return e.refusef("unknown command kind %q", e.command.Kind)
}

// target is a node of the working tree with the collection that holds it.
type target struct {
	tokens []string
	node   *yaml.Node
	owner  *yaml.Node
	key    *yaml.Node
	pos    int
}

func (t target) remove() {
	if t.key != nil {
		t.owner.Content = slices.Delete(t.owner.Content, t.pos-1, t.pos+1)
		return
	}
	t.owner.Content = slices.Delete(t.owner.Content, t.pos, t.pos+1)
}

type lookupStatus int

const (
	lookupFound lookupStatus = iota
	lookupMissing
	lookupViaAlias
)

func (e *editor) walk(tokens []string) (target, lookupStatus) {
	current := target{tokens: tokens, node: e.root}
	for _, token := range tokens {
		node := current.node
		switch node.Kind {
		case yaml.MappingNode:
			position := keyPosition(node, token)
			if position < 0 {
				return target{}, lookupMissing
			}
			current = target{
				tokens: tokens, node: node.Content[position+1],
				owner: node, key: node.Content[position], pos: position + 1,
			}
		case yaml.SequenceNode:
			index, err := strconv.Atoi(token)
			if err != nil || index < 0 || index >= len(node.Content) || strconv.Itoa(index) != token {
				return target{}, lookupMissing
			}
			current = target{tokens: tokens, node: node.Content[index], owner: node, pos: index}
		case yaml.AliasNode:
			return target{}, lookupViaAlias
		case yaml.DocumentNode, yaml.ScalarNode:
			return target{}, lookupMissing
		}
	}
	if current.node.Kind == yaml.AliasNode {
		return target{}, lookupViaAlias
	}
	return current, lookupFound
}

type pointerShape func(tokens []string) bool

func (e *editor) at(pointer, noun string, shape pointerShape) (target, error) {
	tokens, err := parsePointerTokens(pointer)
	if err != nil {
		return target{}, e.refusef("invalid pointer %q: %s", pointer, err)
	}
	normalized := pointerFromTokens(tokens)
	if shape != nil && !shape(tokens) {
		return target{}, e.refusef("no %s at %s", noun, normalized)
	}
	found, status := e.walk(tokens)
	switch status {
	case lookupViaAlias:
		return target{}, e.refusef("%s goes through an alias: edit the anchored node instead", normalized)
	case lookupMissing:
		return target{}, e.refusef("no %s at %s", noun, normalized)
	case lookupFound:
	}
	if found.node.Kind != yaml.MappingNode {
		return target{}, e.refusef("no %s at %s", noun, normalized)
	}
	return found, nil
}

func tokensNameOwner(owner string) pointerShape {
	return func(tokens []string) bool {
		return len(tokens) >= 2 && tokens[len(tokens)-2] == owner
	}
}

func isOperationTokens(tokens []string) bool {
	return len(tokens) == 3 && tokens[0] == pathsKey && isHTTPMethod(tokens[2])
}

func isPathItemTokens(tokens []string) bool {
	return len(tokens) == 2 && tokens[0] == pathsKey
}

func (e *editor) operation(pointer string) (target, error) {
	return e.at(pointer, "operation", isOperationTokens)
}

func (e *editor) property(pointer string) (target, error) {
	return e.at(pointer, "property", tokensNameOwner("properties"))
}

func (e *editor) response(pointer string) (target, error) {
	return e.at(pointer, "response", tokensNameOwner("responses"))
}

func (e *editor) parameter(pointer string) (target, error) {
	found, err := e.at(pointer, "parameter", tokensNameOwner("parameters"))
	if err != nil {
		return target{}, err
	}
	if fieldNode(found.node, refKey) != nil {
		return target{}, e.refusef("the parameter at %s is a $ref: edit the parameter it points to",
			pointerFromTokens(found.tokens))
	}
	return found, nil
}

// parent is the node above the last count tokens of a target.
func (e *editor) above(found target, count int) (target, error) {
	tokens := found.tokens[:len(found.tokens)-count]
	parent, status := e.walk(tokens)
	if status != lookupFound || parent.node.Kind != yaml.MappingNode {
		return target{}, e.refusef("no object at %s", pointerFromTokens(tokens))
	}
	return parent, nil
}

// put sets a field. A new field goes where the canonical layout puts it
// among the fields the object already has.
func (e *editor) put(mapping *yaml.Node, kind objectKind, key string, value *yaml.Node) {
	if position := keyPosition(mapping, key); position >= 0 {
		previous := mapping.Content[position+1]
		if previous.Kind == yaml.ScalarNode && value.LineComment == "" {
			value.LineComment = previous.LineComment
		}
		mapping.Content[position+1] = value
		return
	}
	rank := e.layouts[kind].rank
	wanted := fieldRank(rank, key)
	at := len(mapping.Content)
	for i := 0; i+1 < len(mapping.Content); i += nodesPerEntry {
		if fieldRank(rank, mapping.Content[i].Value) > wanted {
			at = i
			break
		}
	}
	if len(mapping.Content) == 0 {
		mapping.Style &^= yaml.FlowStyle
	}
	mapping.Content = slices.Insert(mapping.Content, at, strNode(key), value)
}

func (e *editor) putString(mapping *yaml.Node, kind objectKind, key, value string) {
	if position := keyPosition(mapping, key); position >= 0 {
		if previous := mapping.Content[position+1]; previous.Kind == yaml.ScalarNode {
			setScalarString(previous, value)
			return
		}
	}
	e.put(mapping, kind, key, strNode(value))
}

func (e *editor) putBool(mapping *yaml.Node, kind objectKind, key string, value bool) {
	if position := keyPosition(mapping, key); position >= 0 {
		if previous := mapping.Content[position+1]; previous.Kind == yaml.ScalarNode {
			previous.Tag, previous.Value, previous.Style = tagBool, strconv.FormatBool(value), 0
			return
		}
	}
	e.put(mapping, kind, key, boolNode(value))
}

func setScalarString(node *yaml.Node, value string) {
	node.Tag = tagStr
	node.Value = value
	multiline := strings.Contains(value, "\n")
	block := node.Style&(yaml.LiteralStyle|yaml.FoldedStyle) != 0
	switch {
	case multiline && !block:
		node.Style = yaml.LiteralStyle
	case !multiline && block:
		node.Style = 0
	}
}

// ensureMap returns the object under a key, creating it when it is missing.
func (e *editor) ensureMap(parent *yaml.Node, kind objectKind, key string) (*yaml.Node, error) {
	if existing := fieldNode(parent, key); existing != nil {
		if isNull(existing) {
			*existing = *newMapNode()
		}
		if existing.Kind != yaml.MappingNode {
			return nil, e.refusef("%s is not an object", key)
		}
		return existing, nil
	}
	created := newMapNode()
	e.put(parent, kind, key, created)
	return created, nil
}

func (e *editor) ensureSeq(parent *yaml.Node, kind objectKind, key string) (*yaml.Node, error) {
	if existing := fieldNode(parent, key); existing != nil {
		if isNull(existing) {
			*existing = *newSeqNode()
		}
		if existing.Kind != yaml.SequenceNode {
			return nil, e.refusef("%s is not a list", key)
		}
		return existing, nil
	}
	created := newSeqNode()
	e.put(parent, kind, key, created)
	return created, nil
}

func (e *editor) schemaExists(name string) bool {
	components := fieldNode(e.root, componentsKey)
	return keyPosition(fieldNode(components, schemasKey), name) >= 0
}
