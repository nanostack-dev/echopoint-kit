package apispec_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/nanostack-dev/echopoint-kit/apispec"
)

func TestParseAcceptsOpenAPI30(t *testing.T) {
	document := mustParse(t, petsSpec)
	if got := document.OpenAPIVersion(); got != "3.0.3" {
		t.Errorf("OpenAPIVersion() = %q, want 3.0.3", got)
	}
	if err := document.Validate(); err != nil {
		t.Errorf("Validate() = %v", err)
	}
}

func TestParseAcceptsOpenAPI31(t *testing.T) {
	document := mustParse(t, `openapi: 3.1.0
info:
  title: Pets
  version: 1.0.0
paths:
  /pets:
    get:
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema:
                type: [string, "null"]
`)
	if err := document.Validate(); err != nil {
		t.Errorf("Validate() = %v", err)
	}
}

func TestParseRefusesSwagger2(t *testing.T) {
	_, err := apispec.Parse([]byte("swagger: \"2.0\"\ninfo:\n  title: Pets\n  version: 1.0.0\npaths: {}\n"))
	if !errors.Is(err, apispec.ErrSwagger2) {
		t.Errorf("Parse() error = %v, want ErrSwagger2", err)
	}
}

func TestParseRefusesOtherOpenAPIVersions(t *testing.T) {
	for _, version := range []string{"3.2.0", "4.0.0", "2.0"} {
		_, err := apispec.Parse([]byte("openapi: " + version + "\ninfo:\n  title: Pets\n  version: 1.0.0\npaths: {}\n"))
		if !errors.Is(err, apispec.ErrUnsupportedVersion) {
			t.Errorf("Parse(openapi %s) error = %v, want ErrUnsupportedVersion", version, err)
		}
	}
}

func TestParseRefusesDocumentWithoutOpenAPIField(t *testing.T) {
	_, err := apispec.Parse([]byte("info:\n  title: Pets\n  version: 1.0.0\npaths: {}\n"))
	if !errors.Is(err, apispec.ErrUnsupportedVersion) {
		t.Errorf("Parse() error = %v, want ErrUnsupportedVersion", err)
	}
}

func TestParseRefusesExternalRefs(t *testing.T) {
	_, err := apispec.Parse([]byte(`openapi: 3.0.3
info:
  title: Pets
  version: 1.0.0
paths:
  /pets:
    get:
      responses:
        "200":
          $ref: responses.yaml#/Ok
components:
  schemas:
    Pet:
      $ref: https://example.com/schemas/pet.json
`))
	var refError *apispec.ExternalRefError
	if !errors.As(err, &refError) || !errors.Is(err, apispec.ErrExternalRef) {
		t.Fatalf("Parse() error = %v, want an ExternalRefError", err)
	}
	want := []apispec.ExternalRef{
		{Pointer: "/paths/~1pets/get/responses/200/$ref", Ref: "responses.yaml#/Ok"},
		{Pointer: "/components/schemas/Pet/$ref", Ref: "https://example.com/schemas/pet.json"},
	}
	if len(refError.Refs) != len(want) {
		t.Fatalf("Refs = %v, want %v", refError.Refs, want)
	}
	for i := range want {
		if refError.Refs[i] != want[i] {
			t.Errorf("Refs[%d] = %v, want %v", i, refError.Refs[i], want[i])
		}
	}
}

func TestParseIgnoresRefKeysInsideExampleValues(t *testing.T) {
	mustParse(t, `openapi: 3.0.3
info:
  title: Pets
  version: 1.0.0
paths:
  /pets:
    get:
      responses:
        "200":
          description: OK
          content:
            application/json:
              example:
                $ref: https://example.com/not-a-reference
`)
}

func TestParseRefusesUnreadableDocument(t *testing.T) {
	_, err := apispec.Parse([]byte("openapi: [3.0.3\n"))
	if !errors.Is(err, apispec.ErrUnreadable) {
		t.Errorf("Parse() error = %v, want ErrUnreadable", err)
	}
}

func TestParseRefusesUnresolvableInternalRef(t *testing.T) {
	_, err := apispec.Parse([]byte(`openapi: 3.0.3
info:
  title: Pets
  version: 1.0.0
paths:
  /pets:
    get:
      responses:
        "200":
          $ref: "#/components/responses/Missing"
`))
	if !errors.Is(err, apispec.ErrInvalid) {
		t.Errorf("Parse() error = %v, want ErrInvalid", err)
	}
}

func TestValidateReportsInvalidDocument(t *testing.T) {
	document := mustParse(t, `openapi: 3.0.3
info:
  version: 1.0.0
paths:
  /pets:
    get:
      responses:
        "200":
          content: {}
`)
	err := document.Validate()
	var validationError *apispec.ValidationError
	if !errors.As(err, &validationError) || len(validationError.Problems) == 0 {
		t.Fatalf("Validate() = %v, want a ValidationError with problems", err)
	}
}

func TestWithVersionLeavesTheReceiverUnchanged(t *testing.T) {
	document := mustParse(t, petsSpec)
	versioned := document.WithVersion("9.0.0")
	if document.Version() != "1.0.0" || versioned.Version() != "9.0.0" {
		t.Errorf("Version() = %q and %q, want 1.0.0 and 9.0.0", document.Version(), versioned.Version())
	}
}

func mustParse(t *testing.T, yaml string) *apispec.Document {
	t.Helper()
	document, err := apispec.Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("Parse() = %v", err)
	}
	return document
}

func TestParseRefusesNonObjectRoot(t *testing.T) {
	_, err := apispec.Parse([]byte("- openapi\n- 3.0.3\n"))
	if !errors.Is(err, apispec.ErrInvalid) {
		t.Errorf("Parse() error = %v, want ErrInvalid", err)
	}
}

func TestParseRefusesEmptyInput(t *testing.T) {
	_, err := apispec.Parse(nil)
	if !errors.Is(err, apispec.ErrUnreadable) {
		t.Errorf("Parse() error = %v, want ErrUnreadable", err)
	}
}

func TestParseReportsTheYAMLProblemOnly(t *testing.T) {
	_, err := apispec.Parse([]byte(edit(petsSpec, "version: 1.0.0", "version: 1.0")))
	var validationError *apispec.ValidationError
	if !errors.As(err, &validationError) {
		t.Fatalf("Parse() error = %v, want a ValidationError", err)
	}
	if problem := validationError.Problems[0]; strings.Contains(problem, "json error") || problem == "" {
		t.Errorf("problem = %q", problem)
	}
}

func TestErrorMessagesNameWhatIsWrong(t *testing.T) {
	refError := &apispec.ExternalRefError{Refs: []apispec.ExternalRef{{Pointer: "/paths/~1a/$ref", Ref: "a.yaml"}}}
	if !strings.Contains(refError.Error(), "/paths/~1a/$ref -> a.yaml") {
		t.Errorf("ExternalRefError = %q", refError.Error())
	}
	validationError := &apispec.ValidationError{Problems: []string{"info.title is required", "paths is required"}}
	if !strings.Contains(validationError.Error(), "info.title is required; paths is required") {
		t.Errorf("ValidationError = %q", validationError.Error())
	}
}

func TestTitleAndVersionReadInfo(t *testing.T) {
	document := mustParse(t, petsSpec)
	if document.Title() != "Pets" || document.Version() != "1.0.0" {
		t.Errorf("Title() = %q, Version() = %q", document.Title(), document.Version())
	}
}

func TestWithVersionWritesAMissingInfoVersion(t *testing.T) {
	document := mustParse(t, edit(petsSpec, "  version: 1.0.0\n", ""))
	canonical, err := document.WithVersion("1.0.0").Canonical()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(canonical), "  version: 1.0.0\n") {
		t.Errorf("canonical YAML has no info.version:\n%s", canonical)
	}
}
