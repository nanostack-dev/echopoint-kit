//go:build !js

package main_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
      operationId: create_pet
      responses:
        "201":
          description: Created.
components:
  schemas:
    Pet:
      type: object
      properties:
        petName:
          type: string
        ownerId:
          type: string
`

func TestWebAssemblyBuildAnswersLikeTheNativeEngine(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the WebAssembly binary")
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed")
	}
	wasm := filepath.Join(t.TempDir(), "apispec.wasm")
	build := exec.Command("go", "build", "-o", wasm, ".")
	build.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm")
	if output, buildErr := build.CombinedOutput(); buildErr != nil {
		t.Fatalf("build: %v\n%s", buildErr, output)
	}

	requests := []bridge.Request{
		{Operation: bridge.OperationLint, Document: pets},
		{Operation: bridge.OperationLintChanges, Base: pets, Document: pets},
		{Operation: bridge.OperationCompare, Base: pets, Document: pets},
		{Operation: bridge.OperationCanonical, Document: pets},
		{Operation: bridge.OperationEdit, Document: pets, Commands: []apispec.Command{
			{
				Kind:    apispec.CommandAddProperty,
				Pointer: apispec.SchemaPointer("Pet"),
				Name:    "birth_date",
				Type:    "string",
			},
			{Kind: apispec.CommandAddOperation, Path: "/pets/{petId}", Method: "get", Status: "200"},
		}},
		{Operation: bridge.OperationEdit, Document: pets, Commands: []apispec.Command{
			{Kind: apispec.CommandAddOperation, Path: "/pets", Method: "get"},
		}},
		{Operation: bridge.OperationLint, Document: "swagger: \"2.0\"\n"},
	}
	encoded, err := json.Marshal(requests)
	if err != nil {
		t.Fatal(err)
	}
	goroot, err := exec.Command("go", "env", "GOROOT").Output()
	if err != nil {
		t.Fatal(err)
	}
	run := exec.Command(node, "testdata/run.mjs",
		filepath.Join(strings.TrimSpace(string(goroot)), "lib", "wasm", "wasm_exec.js"), wasm)
	run.Stdin = bytes.NewReader(encoded)
	var stderr bytes.Buffer
	run.Stderr = &stderr
	output, err := run.Output()
	if err != nil {
		t.Fatalf("node: %v\n%s", err, stderr.String())
	}
	var fromWasm []json.RawMessage
	if decodeErr := json.Unmarshal(output, &fromWasm); decodeErr != nil {
		t.Fatalf("decode %s: %v", output, decodeErr)
	}

	for index, request := range requests {
		requestJSON, _ := json.Marshal(request)
		native := bridge.Handle(requestJSON)
		if !jsonEqual(t, native, fromWasm[index]) {
			t.Errorf("request %d (%s):\nnative %s\nwasm   %s", index, request.Operation, native, fromWasm[index])
		}
	}
}

func jsonEqual(t *testing.T, left, right []byte) bool {
	t.Helper()
	var a, b any
	if err := json.Unmarshal(left, &a); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(right, &b); err != nil {
		t.Fatal(err)
	}
	leftCanonical, _ := json.Marshal(a)
	rightCanonical, _ := json.Marshal(b)
	return bytes.Equal(leftCanonical, rightCanonical)
}
