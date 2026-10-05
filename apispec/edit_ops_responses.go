package apispec

import (
	"net/http"
	"strconv"

	"go.yaml.in/yaml/v3"
)

const jsonMediaType = "application/json"

func defaultResponseDescription(status string) string {
	switch status {
	case statusDefault:
		return "Default response"
	case "1XX":
		return "Informational"
	case "2XX":
		return "Successful response"
	case "3XX":
		return "Redirection"
	case "4XX":
		return "Client error"
	case "5XX":
		return "Server error"
	}
	if code, err := strconv.Atoi(status); err == nil {
		if text := http.StatusText(code); text != "" {
			return text
		}
	}
	return "Response"
}

func (e *editor) newResponse(description string) *yaml.Node {
	response := newMapNode()
	e.putString(response, kindResponse, fieldDescription, description)
	return response
}

// statusKey is the key node of a response status. A numeric status is quoted
// like the numeric statuses already in the responses.
func (e *editor) statusKey(responses *yaml.Node, status string) *yaml.Node {
	key := strNode(status)
	if _, err := strconv.Atoi(status); err != nil {
		return key
	}
	key.Style = yaml.DoubleQuotedStyle
	for i := 0; i+1 < len(responses.Content); i += nodesPerEntry {
		existing := responses.Content[i]
		if _, err := strconv.Atoi(existing.Value); err == nil && existing.Style != 0 {
			key.Style = existing.Style
			break
		}
	}
	return key
}

func (e *editor) setResponseBody(response *yaml.Node) error {
	c := e.command
	schema := newMapNode()
	if c.Schema != "" {
		if !e.schemaExists(c.Schema) {
			return e.refusef("no schema %s in components/schemas", c.Schema)
		}
		schema = e.refNode(c.Schema)
	} else {
		e.putString(schema, kindSchema, "type", c.Type)
	}
	content, err := e.ensureMap(response, kindResponse, "content")
	if err != nil {
		return err
	}
	media, err := e.ensureMap(content, kindMediaType, jsonMediaType)
	if err != nil {
		return err
	}
	e.put(media, kindMediaType, "schema", schema)
	return nil
}

func (e *editor) addResponse() error {
	c := e.command
	operation, err := e.operation(c.Pointer)
	if err != nil {
		return err
	}
	responses, err := e.ensureMap(operation.node, kindOperation, "responses")
	if err != nil {
		return err
	}
	if keyPosition(responses, c.Status) >= 0 {
		return e.refusef("response %s already exists", c.Status)
	}
	description := defaultResponseDescription(c.Status)
	if c.Description != nil && *c.Description != "" {
		description = *c.Description
	}
	response := e.newResponse(description)
	if c.Schema != "" || c.Type != "" {
		if err = e.setResponseBody(response); err != nil {
			return err
		}
	}
	appendPair(responses, e.statusKey(responses, c.Status), response)
	return nil
}

func (e *editor) setResponse() error {
	c := e.command
	response, err := e.response(c.Pointer)
	if err != nil {
		return err
	}
	if fieldNode(response.node, refKey) != nil {
		return e.refusef("the response at %s is a $ref: edit the response it points to",
			pointerFromTokens(response.tokens))
	}
	if c.Description != nil {
		e.putString(response.node, kindResponse, fieldDescription, *c.Description)
	}
	if c.Schema != "" || c.Type != "" {
		return e.setResponseBody(response.node)
	}
	return nil
}

func (e *editor) removeResponse() error {
	response, err := e.response(e.command.Pointer)
	if err != nil {
		return err
	}
	response.remove()
	return nil
}
