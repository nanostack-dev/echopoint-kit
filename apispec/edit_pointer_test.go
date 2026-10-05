package apispec_test

import (
	"errors"
	"testing"

	"github.com/nanostack-dev/echopoint-kit/apispec"
)

func TestPointerHelpers(t *testing.T) {
	cases := map[string]struct{ got, want string }{
		"operation":     {apispec.OperationPointer("GET", "/pets/{id}"), "/paths/~1pets~1{id}/get"},
		"schema":        {apispec.SchemaPointer("Pet"), "/components/schemas/Pet"},
		"schema with ~": {apispec.SchemaPointer("a~b"), "/components/schemas/a~0b"},
		"property":      {apispec.PropertyPointer("Pet", "name"), "/components/schemas/Pet/properties/name"},
		"nested property": {
			apispec.PropertyPointer("Pet", "owner", "id"),
			"/components/schemas/Pet/properties/owner/properties/id",
		},
		"response": {apispec.ResponsePointer("get", "/pets", "201"), "/paths/~1pets/get/responses/201"},
	}
	for name, c := range cases {
		if c.got != c.want {
			t.Errorf("%s: got %s, want %s", name, c.got, c.want)
		}
	}
}

func TestParameterPointerMatchesByNameAndLocationNeverByIndex(t *testing.T) {
	document := []byte(refusalDocument)
	cases := []struct {
		name, parameter, in, want string
	}{
		{"an operation parameter", "limit", "query", "/paths/~1pets~1{petId}/get/parameters/0"},
		{"a parameter through a $ref", "offset", "query", "/paths/~1pets~1{petId}/get/parameters/1"},
		{"a path item parameter", "petId", "path", "/paths/~1pets~1{petId}/parameters/0"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := apispec.ParameterPointer(document, "get", "/pets/{petId}", c.parameter, c.in)
			if err != nil || got != c.want {
				t.Errorf("got %q, %v, want %q", got, err, c.want)
			}
		})
	}
	t.Run("the same name in another location is another parameter", func(t *testing.T) {
		if got, err := apispec.ParameterPointer(document, "get", "/pets/{petId}", "limit", "header"); err == nil {
			t.Errorf("got %q, want an error", got)
		}
	})
	t.Run("the operation parameter wins over the path item parameter", func(t *testing.T) {
		shadowed := []byte(`openapi: 3.0.3
info: {title: T, version: "1"}
paths:
  /a:
    parameters:
      - {name: q, in: query}
    get:
      parameters:
        - {name: other, in: query}
        - {name: q, in: query}
      responses:
        "200": {description: OK}
`)
		got, err := apispec.ParameterPointer(shadowed, "get", "/a", "q", "query")
		if err != nil || got != "/paths/~1a/get/parameters/1" {
			t.Errorf("got %q, %v", got, err)
		}
	})
	t.Run("a missing operation", func(t *testing.T) {
		_, err := apispec.ParameterPointer(document, "post", "/pets/{petId}", "limit", "query")
		if !errors.Is(err, apispec.ErrCommand) {
			t.Errorf("got %v", err)
		}
	})
}

func TestPointerAcceptsALeadingHash(t *testing.T) {
	command := apispec.Command{
		Kind: apispec.CommandSetText, Pointer: "#/paths/~1pets~1%7BpetId%7D/put", Field: "summary", Value: "Replace",
	}
	if _, err := apispec.Edit([]byte(refusalDocument), command); err != nil {
		t.Fatal(err)
	}
}
