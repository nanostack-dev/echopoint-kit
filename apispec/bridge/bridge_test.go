package bridge_test

import (
	"encoding/json"
	"testing"

	"github.com/nanostack-dev/echopoint-kit/apispec"
	"github.com/nanostack-dev/echopoint-kit/apispec/bridge"
)

const pets = `openapi: 3.0.3
info:
  title: Pets
  version: 1.0.0
paths:
  /pets:
    get:
      operationId: listPets
      responses:
        "200":
          description: The pets.
    post:
      operationId: createPet
      responses:
        "201":
          description: Created.
  /pets/{petId}:
    parameters:
      - name: petId
        in: path
        required: true
        schema:
          type: string
    get:
      operationId: getPet
      responses:
        "200":
          description: The pet.
    put:
      operationId: updatePet
      responses:
        "200":
          description: The pet.
    delete:
      operationId: deletePet
      responses:
        "204":
          description: Deleted.
`

const snakeOwners = `  /owners:
    get:
      operationId: list_owners
      responses:
        "200":
          description: The owners.
`

type response[T any] struct {
	OK     bool          `json:"ok"`
	Result T             `json:"result"`
	Error  *bridge.Error `json:"error"`
}

func handle[T any](t *testing.T, request bridge.Request) response[T] {
	t.Helper()
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	var decoded response[T]
	if decodeErr := json.Unmarshal(bridge.Handle(encoded), &decoded); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	return decoded
}

func TestHandleLint(t *testing.T) {
	got := handle[bridge.Findings](t, bridge.Request{Operation: bridge.OperationLint, Document: pets + snakeOwners})

	if !got.OK || len(got.Result.Findings) != 1 || got.Result.Findings[0].Pointer != "/paths/~1owners/get" {
		t.Fatalf("got %+v", got)
	}
}

func TestHandleLintOfAConventionalDocumentIsAnEmptyList(t *testing.T) {
	encoded, _ := json.Marshal(bridge.Request{Operation: bridge.OperationLint, Document: pets})

	if got := string(bridge.Handle(encoded)); got != `{"ok":true,"result":{"findings":[]}}` {
		t.Fatalf("got %s", got)
	}
}

func TestHandleLintChanges(t *testing.T) {
	got := handle[bridge.Findings](t, bridge.Request{
		Operation: bridge.OperationLintChanges, Base: pets + snakeOwners, Document: pets + snakeOwners,
	})

	if !got.OK || len(got.Result.Findings) != 0 {
		t.Fatalf("got %+v", got)
	}
}

func TestHandleCompare(t *testing.T) {
	got := handle[bridge.Comparison](t, bridge.Request{
		Operation: bridge.OperationCompare, Base: pets, Document: pets + snakeOwners,
	})

	if !got.OK || got.Result.Bump != apispec.BumpMinor || len(got.Result.Changes) == 0 {
		t.Fatalf("got %+v", got)
	}
}

func TestHandleEdit(t *testing.T) {
	got := handle[bridge.Document](t, bridge.Request{
		Operation: bridge.OperationEdit,
		Document:  pets,
		Commands:  []apispec.Command{{Kind: apispec.CommandAddSchema, Name: "Pet"}},
	})

	want, err := apispec.Edit([]byte(pets), apispec.Command{Kind: apispec.CommandAddSchema, Name: "Pet"})
	if err != nil {
		t.Fatal(err)
	}
	if !got.OK || got.Result.Document != string(want) {
		t.Fatalf("got %+v", got)
	}
}

func TestHandleCanonical(t *testing.T) {
	got := handle[bridge.Document](t, bridge.Request{Operation: bridge.OperationCanonical, Document: pets})

	if !got.OK || got.Result.Document == "" {
		t.Fatalf("got %+v", got)
	}
}

func TestHandleRefusedCommandNamesItsIndex(t *testing.T) {
	got := handle[bridge.Document](t, bridge.Request{
		Operation: bridge.OperationEdit,
		Document:  pets,
		Commands: []apispec.Command{
			{Kind: apispec.CommandAddSchema, Name: "Pet"},
			{Kind: apispec.CommandAddOperation, Path: "/pets", Method: "get"},
		},
	})

	if got.OK || got.Error.Code != "SPEC_COMMAND_REFUSED" || got.Error.Index == nil || *got.Error.Index != 1 ||
		got.Error.Reason == "" {
		t.Fatalf("got %+v", got.Error)
	}
}

func TestHandleInvalidDocument(t *testing.T) {
	got := handle[bridge.Findings](t, bridge.Request{
		Operation: bridge.OperationLint, Document: "openapi: 3.0.3\ninfo: [x]\npaths: {}\n",
	})

	if got.OK || got.Error.Code != "SPEC_DOCUMENT_INVALID" {
		t.Fatalf("got %+v", got.Error)
	}
}

func TestHandleUnreadableDocument(t *testing.T) {
	got := handle[bridge.Findings](t, bridge.Request{Operation: bridge.OperationLint, Document: "openapi: [\n"})

	if got.OK || got.Error.Code != "SPEC_DOCUMENT_UNREADABLE" {
		t.Fatalf("got %+v", got.Error)
	}
}

func TestHandleSwagger2(t *testing.T) {
	got := handle[bridge.Findings](t, bridge.Request{
		Operation: bridge.OperationLint, Document: "swagger: \"2.0\"\ninfo: {title: X, version: 1.0.0}\npaths: {}\n",
	})

	if got.OK || got.Error.Code != "SPEC_UNSUPPORTED_VERSION" {
		t.Fatalf("got %+v", got.Error)
	}
}

func TestHandleExternalRef(t *testing.T) {
	got := handle[bridge.Findings](t, bridge.Request{
		Operation: bridge.OperationLint,
		Document: "openapi: 3.0.3\ninfo: {title: X, version: 1.0.0}\npaths:\n  /a:\n    get:\n" +
			"      responses:\n        \"200\":\n          $ref: other.yaml#/r\n",
	})

	if got.OK || got.Error.Code != "SPEC_EXTERNAL_REF" || len(got.Error.Refs) != 1 {
		t.Fatalf("got %+v", got.Error)
	}
}

func TestHandleUnknownOperation(t *testing.T) {
	got := handle[bridge.Findings](t, bridge.Request{Operation: "format", Document: pets})

	if got.OK || got.Error.Code != "BAD_REQUEST" {
		t.Fatalf("got %+v", got.Error)
	}
}

func TestHandleMalformedRequest(t *testing.T) {
	var got response[bridge.Findings]
	if err := json.Unmarshal(bridge.Handle([]byte("{")), &got); err != nil {
		t.Fatal(err)
	}

	if got.OK || got.Error.Code != "BAD_REQUEST" {
		t.Fatalf("got %+v", got.Error)
	}
}
