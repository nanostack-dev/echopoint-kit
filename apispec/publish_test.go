package apispec_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/nanostack-dev/echopoint-kit/apispec"
)

const petsSpec = `openapi: 3.0.3
info:
  title: Pets
  description: The pet store.
  version: 1.0.0
paths:
  /pets:
    get:
      operationId: listPets
      description: Lists pets.
      parameters:
        - name: status
          in: query
          schema:
            type: string
            enum: [available, sold]
      responses:
        "200":
          description: The pets.
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: "#/components/schemas/Pet"
    post:
      operationId: createPet
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: "#/components/schemas/NewPet"
      responses:
        "201":
          description: Created.
components:
  schemas:
    Pet:
      type: object
      required: [id, name]
      properties:
        id:
          type: string
        name:
          type: string
    NewPet:
      type: object
      required: [name]
      properties:
        name:
          type: string
          maxLength: 64
      example:
        name: Rex
`

const removedCreatePet = `    post:
      operationId: createPet
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: "#/components/schemas/NewPet"
      responses:
        "201":
          description: Created.
`

const addedGetPet = `  /pets/{id}:
    get:
      operationId: getPet
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
      responses:
        "200":
          description: The pet.
components:
`

func TestBreakingChangeBumpsMajor(t *testing.T) {
	assertPublishes(t, edit(petsSpec, removedCreatePet, ""), "2.0.0", apispec.BumpMajor)
}

func TestRiskyChangeBumpsMajor(t *testing.T) {
	revision := edit(petsSpec, `      parameters:
        - name: status
          in: query
          schema:
            type: string
            enum: [available, sold]
`, "")
	publication := assertPublishes(t, revision, "2.0.0", apispec.BumpMajor)
	assertHasSeverity(t, publication.Comparison, apispec.SeverityRisky)
}

func TestNewOperationBumpsMinor(t *testing.T) {
	assertPublishes(t, edit(petsSpec, "components:\n", addedGetPet), "1.1.0", apispec.BumpMinor)
}

func TestNewSchemaBumpsMinor(t *testing.T) {
	revision := edit(petsSpec, "  schemas:\n", "  schemas:\n    Owner:\n      type: object\n")
	publication := assertPublishes(t, revision, "1.1.0", apispec.BumpMinor)
	assertHasChange(t, publication.Comparison, "schema-added")
}

func TestNewOptionalFieldBumpsMinor(t *testing.T) {
	revision := edit(
		petsSpec,
		"          maxLength: 64\n",
		"          maxLength: 64\n        tag:\n          type: string\n",
	)
	assertPublishes(t, revision, "1.1.0", apispec.BumpMinor)
}

func TestNewEnumValueBumpsMinor(t *testing.T) {
	assertPublishes(t, edit(petsSpec, "[available, sold]", "[available, sold, pending]"), "1.1.0", apispec.BumpMinor)
}

func TestLooserLimitBumpsMinor(t *testing.T) {
	assertPublishes(t, edit(petsSpec, "maxLength: 64", "maxLength: 128"), "1.1.0", apispec.BumpMinor)
}

func TestDescriptionEditBumpsPatch(t *testing.T) {
	assertPublishes(t, edit(petsSpec, "Lists pets.", "Lists every pet."), "1.0.1", apispec.BumpPatch)
}

func TestExampleEditBumpsPatch(t *testing.T) {
	assertPublishes(t, edit(petsSpec, "name: Rex", "name: Max"), "1.0.1", apispec.BumpPatch)
}

func TestExtensionEditBumpsPatch(t *testing.T) {
	revision := edit(petsSpec, "      operationId: listPets\n", "      operationId: listPets\n      x-internal: true\n")
	assertPublishes(t, revision, "1.0.1", apispec.BumpPatch)
}

func TestReorderedFieldsBumpPatch(t *testing.T) {
	revision := edit(petsSpec, `        id:
          type: string
        name:
          type: string
`, `        name:
          type: string
        id:
          type: string
`)
	publication := assertPublishes(t, revision, "1.0.1", apispec.BumpPatch)
	assertHasChange(t, publication.Comparison, "document-edited")
}

func TestMixedChangesBumpToTheHighest(t *testing.T) {
	describedAndAdded := edit(edit(petsSpec, "Lists pets.", "Lists every pet."), "components:\n", addedGetPet)
	assertPublishes(t, describedAndAdded, "1.1.0", apispec.BumpMinor)

	describedAddedAndRemoved := edit(describedAndAdded, removedCreatePet, "")
	assertPublishes(t, describedAddedAndRemoved, "2.0.0", apispec.BumpMajor)
}

func TestIdenticalDocumentIsNothingToPublish(t *testing.T) {
	_, err := apispec.Publish(live(t), mustParse(t, petsSpec))
	if !errors.Is(err, apispec.ErrNothingToPublish) {
		t.Errorf("Publish() error = %v, want ErrNothingToPublish", err)
	}
}

func TestChangedInfoVersionAloneIsNothingToPublish(t *testing.T) {
	_, err := apispec.Publish(live(t), mustParse(t, edit(petsSpec, "version: 1.0.0", "version: 7.0.0")))
	if !errors.Is(err, apispec.ErrNothingToPublish) {
		t.Errorf("Publish() error = %v, want ErrNothingToPublish", err)
	}
}

func TestFirstVersionTakesSemanticInfoVersion(t *testing.T) {
	publication, err := apispec.Publish(nil, mustParse(t, edit(petsSpec, "version: 1.0.0", "version: 2.3.0")))
	if err != nil {
		t.Fatal(err)
	}
	if publication.Version != "2.3.0" || publication.Comparison.Bump != apispec.BumpInitial {
		t.Errorf("Publish() = %s (%s), want 2.3.0 (initial)", publication.Version, publication.Comparison.Bump)
	}
}

func TestFirstVersionIsOneWhenInfoVersionIsNotSemantic(t *testing.T) {
	publication, err := apispec.Publish(nil, mustParse(t, edit(petsSpec, "version: 1.0.0", "version: v1")))
	if err != nil {
		t.Fatal(err)
	}
	if publication.Version != "1.0.0" {
		t.Errorf("Publish() version = %s, want 1.0.0", publication.Version)
	}
	if got := mustParse(t, string(publication.YAML)).Version(); got != "1.0.0" {
		t.Errorf("published info.version = %s, want 1.0.0", got)
	}
}

func TestPublishedVersionIsWrittenWithoutAnotherBump(t *testing.T) {
	publication := assertPublishes(t, edit(petsSpec, "components:\n", addedGetPet), "1.1.0", apispec.BumpMinor)

	published := mustParse(t, string(publication.YAML))
	if published.Version() != "1.1.0" {
		t.Errorf("published info.version = %s, want 1.1.0", published.Version())
	}
	_, err := apispec.Publish(&apispec.Live{Document: published, Version: "1.1.0"}, published)
	if !errors.Is(err, apispec.ErrNothingToPublish) {
		t.Errorf("republishing the published document: error = %v, want ErrNothingToPublish", err)
	}
}

func TestPublishWritesTheCanonicalLayout(t *testing.T) {
	publication := assertPublishes(t, edit(petsSpec, "Lists pets.", "Lists every pet."), "1.0.1", apispec.BumpPatch)
	canonical, err := mustParse(t, string(publication.YAML)).Canonical()
	if err != nil {
		t.Fatal(err)
	}
	if string(canonical) != string(publication.YAML) {
		t.Error("the published YAML is not in the canonical layout")
	}
	if publication.LayoutVersion != apispec.LayoutVersion {
		t.Errorf("LayoutVersion = %d, want %d", publication.LayoutVersion, apispec.LayoutVersion)
	}
}

func TestPublishRefusesInvalidDocument(t *testing.T) {
	_, err := apispec.Publish(live(t), mustParse(t, edit(petsSpec, "  title: Pets\n", "")))
	if !errors.Is(err, apispec.ErrInvalid) {
		t.Errorf("Publish() error = %v, want ErrInvalid", err)
	}
}

func TestNextVersion(t *testing.T) {
	tests := []struct {
		previous string
		bump     apispec.Bump
		want     string
	}{
		{"1.4.2", apispec.BumpMajor, "2.0.0"},
		{"1.4.2", apispec.BumpMinor, "1.5.0"},
		{"1.4.2", apispec.BumpPatch, "1.4.3"},
		{"1.4.2-beta.1+build.7", apispec.BumpMinor, "1.5.0"},
		{"0.9.9", apispec.BumpMajor, "1.0.0"},
	}
	for _, test := range tests {
		got, err := apispec.NextVersion(test.previous, test.bump)
		if err != nil || got != test.want {
			t.Errorf("NextVersion(%s, %s) = %s, %v; want %s", test.previous, test.bump, got, err, test.want)
		}
	}
}

func TestNextVersionRefusesNonSemanticVersion(t *testing.T) {
	if _, err := apispec.NextVersion("v1", apispec.BumpMinor); err == nil {
		t.Error("NextVersion(v1) succeeded, want an error")
	}
}

func live(t *testing.T) *apispec.Live {
	t.Helper()
	return &apispec.Live{Document: mustParse(t, petsSpec), Version: "1.0.0"}
}

func edit(spec, old, replacement string) string {
	if !strings.Contains(spec, old) {
		panic("fixture edit not found: " + old)
	}
	return strings.Replace(spec, old, replacement, 1)
}

func assertPublishes(t *testing.T, revision, wantVersion string, wantBump apispec.Bump) apispec.Publication {
	t.Helper()
	publication, err := apispec.Publish(live(t), mustParse(t, revision))
	if err != nil {
		t.Fatalf("Publish() = %v", err)
	}
	if publication.Version != wantVersion || publication.Comparison.Bump != wantBump {
		t.Errorf("Publish() = %s (%s), want %s (%s); changes: %+v",
			publication.Version, publication.Comparison.Bump, wantVersion, wantBump, publication.Comparison.Changes)
	}
	return publication
}

func assertHasChange(t *testing.T, comparison apispec.Comparison, id string) {
	t.Helper()
	for _, change := range comparison.Changes {
		if change.ID == id {
			return
		}
	}
	t.Errorf("no %s change in %+v", id, comparison.Changes)
}

func assertHasSeverity(t *testing.T, comparison apispec.Comparison, severity apispec.Severity) {
	t.Helper()
	for _, change := range comparison.Changes {
		if change.Severity == severity {
			return
		}
	}
	t.Errorf("no %s change in %+v", severity, comparison.Changes)
}
