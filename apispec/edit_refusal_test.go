package apispec_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/nanostack-dev/echopoint-kit/apispec"
)

const refusalDocument = `openapi: 3.0.3
info:
  title: Refusals
  version: 1.0.0
paths:
  /pets:
    get:
      responses:
        "200":
          description: OK
  /pets/{petId}:
    parameters:
      - name: petId
        in: path
        required: true
        schema:
          type: string
    get:
      parameters:
        - name: limit
          in: query
          schema:
            type: integer
        - $ref: '#/components/parameters/Offset'
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Pet'
    put:
      responses:
        "200":
          description: OK
components:
  parameters:
    Offset:
      name: offset
      in: query
      schema:
        type: integer
  schemas:
    Pet:
      type: object
      properties:
        name:
          type: string
        petName:
          type: string
        owner:
          $ref: '#/components/schemas/Owner'
    Owner:
      type: object
    Free:
      type: object
`

func TestEditRefusals(t *testing.T) {
	yes, no := true, false
	refusals := []struct {
		name    string
		command apispec.Command
		reason  string
	}{
		{
			"no operation at /paths/~1pets~1{petId}/post",
			apispec.Command{
				Kind: apispec.CommandSetText, Pointer: "/paths/~1pets~1{petId}/post", Field: "summary", Value: "x",
			},
			"no operation at /paths/~1pets~1{petId}/post",
		},
		{
			"property petName already exists when renaming",
			apispec.Command{
				Kind: apispec.CommandRenameProperty, Pointer: apispec.PropertyPointer("Pet", "name"), Name: "petName",
			},
			"property petName already exists",
		},
		{
			"property petName already exists when adding",
			apispec.Command{Kind: apispec.CommandAddProperty, Pointer: apispec.SchemaPointer("Pet"), Name: "petName"},
			"property petName already exists",
		},
		{
			"a path parameter must stay required when unsetting it",
			apispec.Command{
				Kind:     apispec.CommandSetParameterRequired,
				Pointer:  "/paths/~1pets~1{petId}/parameters/0",
				Required: &no,
			},
			"a path parameter must stay required",
		},
		{
			"a path parameter must stay required when adding it optional",
			apispec.Command{
				Kind: apispec.CommandAddParameter, Pointer: apispec.OperationPointer("put", "/pets/{petId}"),
				Name: "other", In: "path", Required: &no,
			},
			"a path parameter must stay required",
		},
		{
			"operation put /pets/{petId} already exists when adding",
			apispec.Command{Kind: apispec.CommandAddOperation, Path: "/pets/{petId}", Method: "put"},
			"operation put /pets/{petId} already exists",
		},
		{
			"operation put /pets/{petId} already exists when changing the method",
			apispec.Command{
				Kind:    apispec.CommandSetMethod,
				Pointer: apispec.OperationPointer("get", "/pets/{petId}"),
				Method:  "put",
			},
			"operation put /pets/{petId} already exists",
		},
		{
			"path /pets already exists",
			apispec.Command{Kind: apispec.CommandRenamePath, Pointer: "/paths/~1pets~1{petId}", Path: "/pets"},
			"path /pets already exists",
		},
		{
			"operation get /pets already exists",
			apispec.Command{Kind: apispec.CommandAddOperation, Path: "/pets", Method: "get"},
			"operation get /pets already exists",
		},
		{
			"parameter limit in query already exists",
			apispec.Command{
				Kind: apispec.CommandAddParameter, Pointer: apispec.OperationPointer("get", "/pets/{petId}"),
				Name: "limit", In: "query",
			},
			"parameter limit in query already exists",
		},
		{
			"the parameter at a $ref cannot be edited",
			apispec.Command{
				Kind:     apispec.CommandSetParameterRequired,
				Pointer:  "/paths/~1pets~1{petId}/get/parameters/1",
				Required: &yes,
			},
			"is a $ref",
		},
		{
			"schema Owner is still referenced",
			apispec.Command{Kind: apispec.CommandRemoveSchema, Name: "Owner"},
			"schema Owner is still referenced by /components/schemas/Pet/properties/owner",
		},
		{
			"schema Pet is still referenced by a response",
			apispec.Command{Kind: apispec.CommandRemoveSchema, Pointer: apispec.SchemaPointer("Pet")},
			"/paths/~1pets~1{petId}/get/responses/200/content/application~1json/schema",
		},
		{
			"schema Pet already exists",
			apispec.Command{Kind: apispec.CommandAddSchema, Name: "Pet"},
			"schema Pet already exists",
		},
		{
			"no schema Missing in components/schemas when setting a ref",
			apispec.Command{
				Kind: apispec.CommandSetRef, Pointer: apispec.PropertyPointer("Pet", "name"), Schema: "Missing",
			},
			"no schema Missing in components/schemas",
		},
		{
			"response 200 already exists",
			apispec.Command{
				Kind:    apispec.CommandAddResponse,
				Pointer: apispec.OperationPointer("get", "/pets/{petId}"),
				Status:  "200",
			},
			"response 200 already exists",
		},
		{
			"no response at 404",
			apispec.Command{
				Kind: apispec.CommandRemoveResponse, Pointer: apispec.ResponsePointer("get", "/pets/{petId}", "404"),
			},
			"no response at /paths/~1pets~1{petId}/get/responses/404",
		},
		{
			"no property at a missing property",
			apispec.Command{Kind: apispec.CommandRemoveProperty, Pointer: apispec.PropertyPointer("Pet", "ghost")},
			"no property at /components/schemas/Pet/properties/ghost",
		},
		{
			"an unknown kind",
			apispec.Command{Kind: "explode"},
			"unknown command kind",
		},
		{
			"set_text on a summary of a schema",
			apispec.Command{
				Kind: apispec.CommandSetText, Pointer: apispec.SchemaPointer("Pet"), Field: "summary", Value: "x",
			},
			"summary can only be set on an operation",
		},
		{
			"a pointer that does not start with a slash",
			apispec.Command{Kind: apispec.CommandSetTags, Pointer: "paths"},
			"invalid pointer",
		},
	}
	for _, refusal := range refusals {
		t.Run(refusal.name, func(t *testing.T) {
			_, err := apispec.Edit([]byte(refusalDocument), refusal.command)
			if !errors.Is(err, apispec.ErrCommand) {
				t.Fatalf("got %v, want a refusal", err)
			}
			var refused *apispec.CommandError
			if !errors.As(err, &refused) || refused.Index != 0 || refused.Command.Kind != refusal.command.Kind {
				t.Fatalf("got %#v, want a CommandError of command 0", err)
			}
			if !strings.Contains(refused.Reason, refusal.reason) {
				t.Errorf("reason %q does not contain %q", refused.Reason, refusal.reason)
			}
		})
	}
}

func TestEditAtomicity(t *testing.T) {
	input := []byte(refusalDocument)
	commands := []apispec.Command{
		{
			Kind:    apispec.CommandSetText,
			Pointer: apispec.OperationPointer("get", "/pets/{petId}"),
			Field:   "summary",
			Value:   "ok",
		},
		{Kind: apispec.CommandAddSchema, Name: "Fresh"},
		{Kind: apispec.CommandRemoveSchema, Name: "Owner"},
		{Kind: apispec.CommandAddSchema, Name: "Never"},
	}
	edited, err := apispec.Edit(input, commands...)
	var refused *apispec.CommandError
	if !errors.As(err, &refused) || refused.Index != 2 {
		t.Fatalf("got %v, want the third command refused", err)
	}
	if edited != nil {
		t.Errorf("got %d bytes with an error, want none", len(edited))
	}
	if string(input) != refusalDocument {
		t.Error("the input was modified")
	}
}

func TestEditWithoutCommandsReturnsACopy(t *testing.T) {
	input := []byte(refusalDocument)
	out, err := apispec.Edit(input)
	if err != nil || string(out) != refusalDocument {
		t.Fatalf("got %v, %q", err, out)
	}
	out[0] = 'X'
	if input[0] == 'X' {
		t.Error("the result shares memory with the input")
	}
}

func TestEditRefusesADocumentThatIsNotOpenAPI(t *testing.T) {
	_, err := apispec.Edit([]byte("swagger: '2.0'\n"), apispec.Command{Kind: apispec.CommandAddSchema, Name: "A"})
	if !errors.Is(err, apispec.ErrSwagger2) {
		t.Fatalf("got %v", err)
	}
}

func TestCommandValidate(t *testing.T) {
	invalid := []apispec.Command{
		{},
		{Kind: "nope"},
		{Kind: apispec.CommandSetText, Pointer: "/x", Field: "title"},
		{Kind: apispec.CommandAddOperation, Path: "pets", Method: "get"},
		{Kind: apispec.CommandAddOperation, Path: "/pets", Method: "fetch"},
		{Kind: apispec.CommandAddResponse, Pointer: "/x", Status: "ok"},
		{Kind: apispec.CommandAddResponse, Pointer: "/x", Status: "200", Schema: "A", Type: "string"},
		{Kind: apispec.CommandSetType, Pointer: "/x", Type: "date"},
		{Kind: apispec.CommandRemoveSchema},
	}
	for _, command := range invalid {
		err := command.Validate()
		var refused *apispec.CommandError
		if !errors.As(err, &refused) || !errors.Is(err, apispec.ErrCommand) {
			t.Errorf("%+v: got %v, want a CommandError", command, err)
		}
	}
	valid := apispec.Command{Kind: apispec.CommandRemoveSchema, Name: "Pet"}
	if err := valid.Validate(); err != nil {
		t.Errorf("got %v", err)
	}
}
