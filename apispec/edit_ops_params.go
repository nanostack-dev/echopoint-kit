package apispec

import (
	"go.yaml.in/yaml/v3"
)

func (e *editor) checkParameterFree(list, self *yaml.Node, name, in string) error {
	for _, other := range list.Content {
		if other == self {
			continue
		}
		if otherName, otherIn := e.parameterIdentity(other); otherName == name && otherIn == in {
			return e.refusef("parameter %s in %s already exists", name, in)
		}
	}
	return nil
}

func (e *editor) setParameterField() error {
	c := e.command
	parameter, err := e.parameter(c.Pointer)
	if err != nil {
		return err
	}
	name, in := e.parameterIdentity(parameter.node)
	switch c.Field {
	case "name":
		if err = e.checkParameterFree(parameter.owner, parameter.node, c.Value, in); err != nil {
			return err
		}
		e.putString(parameter.node, kindParameter, "name", c.Value)
	case "in":
		if err = e.checkParameterFree(parameter.owner, parameter.node, name, c.Value); err != nil {
			return err
		}
		e.putString(parameter.node, kindParameter, "in", c.Value)
		if c.Value == locationPath {
			e.putBool(parameter.node, kindParameter, "required", true)
		}
	default:
		if c.Value == "" {
			removeField(parameter.node, fieldDescription)
			return nil
		}
		e.putString(parameter.node, kindParameter, fieldDescription, c.Value)
	}
	return nil
}

func (e *editor) setParameterRequired() error {
	parameter, err := e.parameter(e.command.Pointer)
	if err != nil {
		return err
	}
	required := *e.command.Required
	if !required && scalarText(fieldNode(parameter.node, "in")) == locationPath {
		return e.refusef("a path parameter must stay required")
	}
	if required || fieldNode(parameter.node, "required") != nil {
		e.putBool(parameter.node, kindParameter, "required", required)
	}
	return nil
}

func (e *editor) setParameterType() error {
	parameter, err := e.parameter(e.command.Pointer)
	if err != nil {
		return err
	}
	schema, err := e.ensureMap(parameter.node, kindParameter, "schema")
	if err != nil {
		return err
	}
	e.applyType(schema, e.command.Type, e.command.Nullable, e.command.Format)
	return nil
}

func (e *editor) addParameter() error {
	c := e.command
	owner, err := e.at(c.Pointer, "operation or path item", func(tokens []string) bool {
		return isOperationTokens(tokens) || isPathItemTokens(tokens)
	})
	if err != nil {
		return err
	}
	if c.In == locationPath && c.Required != nil && !*c.Required {
		return e.refusef("a path parameter must stay required")
	}
	kind := kindOperation
	if isPathItemTokens(owner.tokens) {
		kind = kindPathItem
	}
	list, err := e.ensureSeq(owner.node, kind, "parameters")
	if err != nil {
		return err
	}
	if err = e.checkParameterFree(list, nil, c.Name, c.In); err != nil {
		return err
	}
	parameter := newMapNode()
	e.putString(parameter, kindParameter, "name", c.Name)
	e.putString(parameter, kindParameter, "in", c.In)
	if c.Description != nil && *c.Description != "" {
		e.putString(parameter, kindParameter, fieldDescription, *c.Description)
	}
	if c.In == locationPath || (c.Required != nil && *c.Required) {
		e.putBool(parameter, kindParameter, "required", true)
	}
	schema := newMapNode()
	e.applyType(schema, orDefault(c.Type, typeString), c.Nullable, c.Format)
	e.put(parameter, kindParameter, "schema", schema)
	appendItem(list, parameter)
	return nil
}

func (e *editor) removeParameter() error {
	parameter, err := e.at(e.command.Pointer, "parameter", tokensNameOwner("parameters"))
	if err != nil {
		return err
	}
	owner, err := e.above(parameter, namedEntryDepth)
	if err != nil {
		return err
	}
	list := parameter.owner
	parameter.remove()
	if len(list.Content) == 0 {
		removeField(owner.node, "parameters")
	}
	return nil
}
