package apispec_test

import (
	"errors"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nanostack-dev/echopoint-kit/apispec"
)

func compare(t *testing.T, base, revision string) apispec.Comparison {
	t.Helper()
	comparison, err := apispec.Compare(mustParse(t, base), mustParse(t, revision))
	if err != nil {
		t.Fatalf("Compare() = %v", err)
	}
	return comparison
}

func changeByID(t *testing.T, comparison apispec.Comparison, id string) apispec.Change {
	t.Helper()
	for _, change := range comparison.Changes {
		if change.ID == id {
			return change
		}
	}
	t.Fatalf("no %s change in %+v", id, comparison.Changes)
	return apispec.Change{}
}

func TestNewOptionalResponseFieldIsAdditive(t *testing.T) {
	revision := edit(petsSpec, "        name:\n          type: string\n    NewPet:",
		"        name:\n          type: string\n        nickname:\n          type: string\n    NewPet:")
	comparison := compare(t, petsSpec, revision)
	if got := changeByID(t, comparison, "response-optional-property-added").Severity; got != apispec.SeverityAdditive {
		t.Errorf("severity = %s, want additive", got)
	}
	if comparison.Bump != apispec.BumpMinor {
		t.Errorf("bump = %s, want minor", comparison.Bump)
	}
}

func TestTighterResponseLimitIsAnEdit(t *testing.T) {
	revision := edit(petsSpec, "        name:\n          type: string\n    NewPet:",
		"        name:\n          type: string\n          maxLength: 64\n    NewPet:")
	comparison := compare(t, petsSpec, revision)
	if got := changeByID(t, comparison, "response-property-max-length-set").Severity; got != apispec.SeverityEdit {
		t.Errorf("severity = %s, want edit", got)
	}
	if comparison.Bump != apispec.BumpPatch {
		t.Errorf("bump = %s, want patch", comparison.Bump)
	}
}

func TestDeprecatedOperationIsAnEdit(t *testing.T) {
	revision := edit(petsSpec, "      operationId: listPets\n", "      operationId: listPets\n      deprecated: true\n")
	comparison := compare(t, petsSpec, revision)
	if got := changeByID(t, comparison, "endpoint-deprecated").Severity; got != apispec.SeverityEdit {
		t.Errorf("severity = %s, want edit", got)
	}
	if comparison.Bump != apispec.BumpPatch {
		t.Errorf("bump = %s, want patch", comparison.Bump)
	}
}

func TestRemovedOperationIsBreakingAndLocated(t *testing.T) {
	comparison := compare(t, petsSpec, edit(petsSpec, removedCreatePet, ""))
	breaking := changeByID(t, comparison, "api-removed-without-deprecation")
	if breaking.Severity != apispec.SeverityBreaking || breaking.Method != http.MethodPost || breaking.Path != "/pets" {
		t.Errorf("change = %+v", breaking)
	}
	if breaking.Pointer != "/paths/~1pets/post" || breaking.Text == "" {
		t.Errorf("pointer %q, text %q", breaking.Pointer, breaking.Text)
	}
}

func TestEditedOperationPointerEscapesThePath(t *testing.T) {
	base := edit(petsSpec, "components:\n", addedGetPet)
	revision := edit(base, "description: The pet.", "description: The pet, by id.")
	edited := changeByID(t, compare(t, base, revision), "operation-edited")
	if edited.Pointer != "/paths/~1pets~1{id}/get" || edited.Severity != apispec.SeverityEdit {
		t.Errorf("change = %+v", edited)
	}
}

func TestNewSchemaChangeCarriesItsPointer(t *testing.T) {
	revision := edit(petsSpec, "  schemas:\n", "  schemas:\n    Owner/Legacy:\n      type: object\n")
	added := changeByID(t, compare(t, petsSpec, revision), "schema-added")
	if added.Pointer != "/components/schemas/Owner~1Legacy" || added.Severity != apispec.SeverityAdditive {
		t.Errorf("change = %+v", added)
	}
}

func TestCompareOpenAPI31Webhooks(t *testing.T) {
	base := `openapi: 3.1.0
info:
  title: Pets
  version: 1.0.0
paths: {}
`
	revision := base + `webhooks:
  petAdopted:
    post:
      responses:
        "200":
          description: Received.
`
	comparison := compare(t, base, revision)
	if comparison.Bump != apispec.BumpMinor {
		t.Errorf("bump = %s, want minor; changes %+v", comparison.Bump, comparison.Changes)
	}
}

func TestCompareLeavesItsInputsUnchanged(t *testing.T) {
	base := mustParse(t, petsSpec)
	revision := mustParse(t, edit(edit(petsSpec, "version: 1.0.0", "version: 3.0.0"), "Lists pets.", "Lists all."))
	before, err := revision.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = apispec.Compare(base, revision); err != nil {
		t.Fatal(err)
	}
	after, err := revision.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	if base.Version() != "1.0.0" || revision.Version() != "3.0.0" || string(before) != string(after) {
		t.Error("Compare modified a document it compared")
	}
}

func TestCompareStripeSizedDocuments(t *testing.T) {
	if testing.Short() {
		t.Skip("parses a 6.6 MB document twice")
	}
	data := string(gunzip(t, filepath.Join("testdata", "stripe", "spec3.yaml.gz")))
	base := mustParse(t, data)

	const accountPath = "\n  /v1/account:\n"
	if !strings.Contains(data, accountPath) {
		t.Fatal("the Stripe fixture has no /v1/account path")
	}
	revision := mustParse(t, strings.Replace(data, accountPath, "\n  /v1/account-renamed:\n", 1))

	comparison, err := apispec.Compare(base, revision)
	if err != nil {
		t.Fatal(err)
	}
	if comparison.Bump != apispec.BumpMajor {
		t.Errorf("bump = %s, want major", comparison.Bump)
	}
	removed := changeByID(t, comparison, "api-path-removed-without-deprecation")
	if removed.Path != "/v1/account" {
		t.Errorf("removed path = %q", removed.Path)
	}
}

func TestComparisonErrorNamesTheReason(t *testing.T) {
	err := &apispec.ComparisonError{Reason: "unsupported construct"}
	if !errors.Is(err, apispec.ErrComparisonFailed) || !strings.Contains(err.Error(), "unsupported construct") {
		t.Errorf("error = %v", err)
	}
}
