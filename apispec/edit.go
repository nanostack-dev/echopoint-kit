package apispec

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"
)

// CommandKind names one edit. Kinds are snake_case.
type CommandKind string

const (
	CommandSetText              CommandKind = "set_text"
	CommandRenamePath           CommandKind = "rename_path"
	CommandSetMethod            CommandKind = "set_method"
	CommandSetParameterField    CommandKind = "set_parameter_field"
	CommandSetParameterRequired CommandKind = "set_parameter_required"
	CommandSetParameterType     CommandKind = "set_parameter_type"
	CommandAddParameter         CommandKind = "add_parameter"
	CommandRemoveParameter      CommandKind = "remove_parameter"
	CommandRenameProperty       CommandKind = "rename_property"
	CommandSetType              CommandKind = "set_type"
	CommandSetRef               CommandKind = "set_ref"
	CommandSetRequired          CommandKind = "set_required"
	CommandSetEnum              CommandKind = "set_enum"
	CommandAddProperty          CommandKind = "add_property"
	CommandRemoveProperty       CommandKind = "remove_property"
	CommandAddOperation         CommandKind = "add_operation"
	CommandRemoveOperation      CommandKind = "remove_operation"
	CommandSetTags              CommandKind = "set_tags"
	CommandAddSchema            CommandKind = "add_schema"
	CommandRemoveSchema         CommandKind = "remove_schema"
	CommandAddResponse          CommandKind = "add_response"
	CommandSetResponse          CommandKind = "set_response"
	CommandRemoveResponse       CommandKind = "remove_response"
)

// Command is one edit addressed by JSON pointer. It is a flat struct: each
// kind reads the fields it documents and ignores the others.
type Command struct {
	Kind CommandKind `json:"kind"`
	// Pointer is an RFC 6901 pointer to the node the command acts on, with or
	// without a leading '#'.
	Pointer     string   `json:"pointer,omitempty"`
	Field       string   `json:"field,omitempty"`
	Value       string   `json:"value,omitempty"`
	Path        string   `json:"path,omitempty"`
	Method      string   `json:"method,omitempty"`
	Name        string   `json:"name,omitempty"`
	In          string   `json:"in,omitempty"`
	Type        string   `json:"type,omitempty"`
	Format      string   `json:"format,omitempty"`
	Nullable    bool     `json:"nullable,omitempty"`
	Required    *bool    `json:"required,omitempty"`
	Values      []string `json:"values,omitempty"`
	Schema      string   `json:"schema,omitempty"`
	Description *string  `json:"description,omitempty"`
	Status      string   `json:"status,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	OperationID string   `json:"operation_id,omitempty"`
	Summary     string   `json:"summary,omitempty"`
}

// ErrCommand is wrapped by every refusal of an edit command.
var ErrCommand = errors.New("the edit command was refused")

// CommandError says which command of an Edit call was refused and why.
type CommandError struct {
	Index   int
	Command Command
	Reason  string
}

func (e *CommandError) Error() string {
	return fmt.Sprintf("%s: command %d (%s): %s", ErrCommand, e.Index, e.Command.Kind, e.Reason)
}

func (e *CommandError) Unwrap() error {
	return ErrCommand
}

// Edit applies the commands in order to an OpenAPI document, YAML or JSON, and
// returns the edited bytes. It is all or nothing: the first command that fails
// returns an error and nothing is applied.
//
// A YAML document is edited node by node: the command changes a copy of the
// parsed tree, then only the lines of the nodes that differ are rewritten in
// the original bytes. Everything else stays byte-identical: comments, key
// order, blank lines, and quoting. A JSON document is written again as JSON
// with the indentation it came with.
func Edit(data []byte, commands ...Command) ([]byte, error) {
	if len(commands) == 0 {
		return bytes.Clone(data), nil
	}
	source := newEditSource(data)
	original, err := decodeEditable(source.text)
	if err != nil {
		return nil, err
	}
	work := cloneTree(original, map[*yaml.Node]*yaml.Node{})
	editor := newEditor(work)
	for index, command := range commands {
		if err = editor.run(index, command); err != nil {
			return nil, err
		}
	}
	edited, err := source.render(original, work)
	if err != nil {
		return nil, err
	}
	if _, err = Parse(edited); err != nil {
		return nil, fmt.Errorf("the edited document does not parse again: %w", err)
	}
	return edited, nil
}

func decodeEditable(text []byte) (*yaml.Node, error) {
	root, err := decodeRoot(text)
	if err != nil {
		return nil, err
	}
	if err = checkOpenAPIVersion(root); err != nil {
		return nil, err
	}
	var document yaml.Node
	if err = yaml.Unmarshal(text, &document); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrUnreadable, err.Error())
	}
	return &document, nil
}

// Validate checks that the command names a known kind and carries the fields
// its kind needs. Edit runs it before it applies a command.
func (c Command) Validate() error {
	if reason := c.invalid(); reason != "" {
		return &CommandError{Command: c, Reason: reason}
	}
	return nil
}

const (
	statusDefault = "default"
)

func (c Command) invalid() string {
	switch c.Kind {
	case CommandSetText:
		return firstReason(c.needPointer(), c.checkTextField())
	case CommandRenamePath:
		return firstReason(c.needPointer(), c.checkPath())
	case CommandSetMethod:
		return firstReason(c.needPointer(), c.checkMethod())
	case CommandSetParameterField:
		return firstReason(c.needPointer(), c.checkParameterField())
	case CommandSetParameterRequired, CommandSetRequired:
		return firstReason(c.needPointer(), c.needRequired())
	case CommandSetParameterType, CommandSetType:
		return firstReason(c.needPointer(), c.needType())
	case CommandAddParameter:
		return firstReason(c.needPointer(), c.needName(), c.checkLocation(), c.checkOptionalType())
	case CommandRenameProperty:
		return firstReason(c.needPointer(), c.needName())
	case CommandSetRef:
		return firstReason(c.needPointer(), c.needSchema())
	case CommandAddProperty:
		return firstReason(c.needPointer(), c.needName(), c.checkOptionalType())
	case CommandRemoveParameter, CommandSetEnum, CommandRemoveProperty, CommandRemoveOperation,
		CommandSetTags, CommandRemoveResponse:
		return c.needPointer()
	case CommandAddOperation:
		return firstReason(c.checkPath(), c.checkMethod())
	case CommandAddSchema:
		return firstReason(c.checkSchemaName(), c.checkOptionalType())
	case CommandRemoveSchema:
		return c.checkRemoveSchema()
	case CommandAddResponse:
		return firstReason(c.needPointer(), c.checkStatus(), c.checkBody())
	case CommandSetResponse:
		return firstReason(c.needPointer(), c.checkBody(), c.checkSetResponse())
	}
	return fmt.Sprintf("unknown command kind %q", c.Kind)
}

func firstReason(reasons ...string) string {
	for _, reason := range reasons {
		if reason != "" {
			return reason
		}
	}
	return ""
}

func (c Command) needPointer() string {
	if c.Pointer == "" {
		return "pointer is required"
	}
	return ""
}

func (c Command) checkTextField() string {
	switch c.Field {
	case fieldSummary, fieldDescription, fieldOperationID:
		return ""
	}
	return fmt.Sprintf("field must be summary, description or operation_id, not %q", c.Field)
}

func (c Command) checkPath() string {
	if !strings.HasPrefix(c.Path, "/") {
		return "path is required and starts with /"
	}
	return ""
}

func (c Command) checkMethod() string {
	if !isHTTPMethod(strings.ToLower(c.Method)) {
		return fmt.Sprintf("method must be a lowercase HTTP method, not %q", c.Method)
	}
	return ""
}

func (c Command) checkParameterField() string {
	switch c.Field {
	case "name":
		if c.Value == "" {
			return "value is required for the parameter name"
		}
	case "in":
		if !isParameterLocation(c.Value) {
			return fmt.Sprintf("value must be query, path, header or cookie, not %q", c.Value)
		}
	case fieldDescription:
	default:
		return fmt.Sprintf("field must be name, in or description, not %q", c.Field)
	}
	return ""
}

func (c Command) needRequired() string {
	if c.Required == nil {
		return "required is required"
	}
	return ""
}

func (c Command) needType() string {
	if !isSchemaType(c.Type) {
		return fmt.Sprintf("type must be string, integer, number, boolean, array or object, not %q", c.Type)
	}
	return ""
}

func (c Command) checkOptionalType() string {
	if c.Type != "" {
		return c.needType()
	}
	return ""
}

func (c Command) needName() string {
	if c.Name == "" {
		return "name is required"
	}
	return ""
}

func (c Command) checkLocation() string {
	if !isParameterLocation(c.In) {
		return fmt.Sprintf("in must be query, path, header or cookie, not %q", c.In)
	}
	return ""
}

func (c Command) needSchema() string {
	if c.Schema == "" {
		return "schema is required"
	}
	return ""
}

func (c Command) checkSchemaName() string {
	if !schemaNamePattern.MatchString(c.Name) {
		return fmt.Sprintf("name must match %s, not %q", schemaNamePattern, c.Name)
	}
	return ""
}

func (c Command) checkRemoveSchema() string {
	if c.Pointer == "" && c.Name == "" {
		return "pointer or name is required"
	}
	return ""
}

func (c Command) checkStatus() string {
	if !statusPattern.MatchString(c.Status) {
		return fmt.Sprintf("status must be a status code, a range such as 4XX, or default, not %q", c.Status)
	}
	return ""
}

func (c Command) checkBody() string {
	if c.Schema != "" && c.Type != "" {
		return "schema and type are exclusive"
	}
	return c.checkOptionalType()
}

func (c Command) checkSetResponse() string {
	if c.Description == nil && c.Schema == "" && c.Type == "" {
		return "description, schema or type is required"
	}
	if c.Description != nil && *c.Description == "" {
		return "a response description cannot be empty"
	}
	return ""
}
