package apispec_test

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/nanostack-dev/echopoint-kit/apispec"
)

const lintHeader = "openapi: 3.0.3\ninfo:\n  title: Pets\n  version: 1.0.0\n"

const lintOKResponse = "responses:\n  \"200\":\n    description: OK\n"

func lintIndent(text string, spaces int) string {
	prefix := strings.Repeat(" ", spaces)
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	for index, line := range lines {
		lines[index] = prefix + line
	}
	return strings.Join(lines, "\n") + "\n"
}

func lintSpec(count int, operation func(index int) string, components string) string {
	var spec strings.Builder
	spec.WriteString(lintHeader + "paths:\n")
	for index := range count {
		fmt.Fprintf(&spec, "  /r%d:\n    get:\n%s", index, lintIndent(operation(index), 6))
	}
	if count == 0 {
		spec.WriteString("  {}\n")
	}
	if components != "" {
		spec.WriteString("components:\n" + lintIndent(components, 2))
	}
	return spec.String()
}

func lintProperties(names ...string) string {
	var properties strings.Builder
	properties.WriteString("properties:\n")
	for _, name := range names {
		fmt.Fprintf(&properties, "  %s:\n    type: string\n    description: documented\n", name)
	}
	return properties.String()
}

func lintSchema(name string, propertyNames ...string) string {
	return fmt.Sprintf("schemas:\n  %s:\n    type: object\n%s", name, lintIndent(lintProperties(propertyNames...), 4))
}

func lintFindings(t *testing.T, document *apispec.Document, rule apispec.LintRule) []apispec.Finding {
	t.Helper()
	var matching []apispec.Finding
	for _, finding := range document.Lint() {
		if finding.Rule == rule {
			matching = append(matching, finding)
		}
	}
	return matching
}

func lintPointers(findings []apispec.Finding) []string {
	pointers := make([]string, 0, len(findings))
	for _, finding := range findings {
		pointers = append(pointers, finding.Pointer)
	}
	return pointers
}

func wantPointers(t *testing.T, findings []apispec.Finding, want ...string) {
	t.Helper()
	if got := lintPointers(findings); !slices.Equal(got, want) {
		t.Errorf("finding pointers = %v, want %v\n%+v", got, want, findings)
	}
}

func TestLintPropertyCasing(t *testing.T) {
	t.Run("a property off the majority casing is reported", func(t *testing.T) {
		document := mustParse(t, lintSpec(0, nil,
			lintSchema("Pet", "petName", "petAge", "ownerId", "createdAt", "updatedAt", "birth_date")))
		findings := lintFindings(t, document, apispec.RulePropertyCasing)
		wantPointers(t, findings, "/components/schemas/Pet/properties/birth_date")
		finding := findings[0]
		if want := "Property name is snake_case, but this spec uses camelCase."; finding.Message != want {
			t.Errorf("Message = %q, want %q", finding.Message, want)
		}
		if want := "83% of 6 multi-word property names in this spec are camelCase"; finding.Convention != want {
			t.Errorf("Convention = %q, want %q", finding.Convention, want)
		}
		if want := "`birth_date`"; finding.Evidence != want {
			t.Errorf("Evidence = %q, want %q", finding.Evidence, want)
		}
	})

	t.Run("a nested property is reported at its own pointer", func(t *testing.T) {
		pet := lintSchema("Pet", "petName", "petAge", "ownerId", "createdAt", "updatedAt")
		document := mustParse(t, lintSpec(0, nil, pet+
			"  Owner:\n    type: object\n    properties:\n      homes:\n        type: array\n        description: d\n"+
			"        items:\n          type: object\n          properties:\n            street_name:\n"+
			"              type: string\n              description: d\n"))
		wantPointers(t, lintFindings(t, document, apispec.RulePropertyCasing),
			"/components/schemas/Owner/properties/homes/items/properties/street_name")
	})

	t.Run("a parameter name is held to the property casing", func(t *testing.T) {
		document := mustParse(t, lintSpec(1, func(int) string {
			return "parameters:\n  - name: page_size\n    in: query\n    schema:\n      type: integer\n" + lintOKResponse
		}, lintSchema("Pet", "petName", "petAge", "ownerId", "createdAt", "updatedAt")))
		wantPointers(t, lintFindings(t, document, apispec.RulePropertyCasing), "/paths/~1r0/get/parameters/0")
	})

	t.Run("path, header, and cookie parameter names follow their own conventions", func(t *testing.T) {
		document := mustParse(t, lintSpec(1, func(int) string {
			return "parameters:\n" +
				"  - name: Idempotency-Key\n    in: header\n    schema:\n      type: string\n" +
				"  - name: session_id\n    in: cookie\n    schema:\n      type: string\n" + lintOKResponse
		}, lintSchema("Pet", "petName", "petAge", "ownerId", "createdAt", "updatedAt")))
		wantPointers(t, lintFindings(t, document, apispec.RulePropertyCasing))
	})

	t.Run("no finding without a casing held by 80% of the properties", func(t *testing.T) {
		document := mustParse(t, lintSpec(0, nil,
			lintSchema("Pet", "petName", "petAge", "ownerId", "created_at", "updated_at", "birth_date")))
		wantPointers(t, lintFindings(t, document, apispec.RulePropertyCasing))
	})

	t.Run("single-word names follow every casing", func(t *testing.T) {
		document := mustParse(t, lintSpec(0, nil,
			lintSchema("Pet", "petName", "petAge", "ownerId", "createdAt", "updatedAt", "name", "id")))
		wantPointers(t, lintFindings(t, document, apispec.RulePropertyCasing))
	})
}

func TestLintOperationIDCasing(t *testing.T) {
	operation := func(id func(int) string) func(int) string {
		return func(index int) string {
			return "operationId: " + id(index) + "\n" + lintOKResponse
		}
	}

	t.Run("an operation ID off the majority casing is reported", func(t *testing.T) {
		document := mustParse(t, lintSpec(6, operation(func(index int) string {
			if index == 5 {
				return "list_pets_five"
			}
			return fmt.Sprintf("listPets%d", index)
		}), ""))
		findings := lintFindings(t, document, apispec.RuleOperationIDCasing)
		wantPointers(t, findings, "/paths/~1r5/get")
		if want := "Operation ID is snake_case, but this spec uses camelCase."; findings[0].Message != want {
			t.Errorf("Message = %q, want %q", findings[0].Message, want)
		}
		if want := "83% of 6 multi-word operation IDs in this spec are camelCase"; findings[0].Convention != want {
			t.Errorf("Convention = %q, want %q", findings[0].Convention, want)
		}
	})

	t.Run("no finding when the casings are mixed", func(t *testing.T) {
		document := mustParse(t, lintSpec(6, operation(func(index int) string {
			if index%2 == 0 {
				return fmt.Sprintf("list_pets_%d", index)
			}
			return fmt.Sprintf("listPets%d", index)
		}), ""))
		wantPointers(t, lintFindings(t, document, apispec.RuleOperationIDCasing))
	})
}

func TestLintMissingStandardResponses(t *testing.T) {
	withNotFound := "responses:\n  \"200\":\n    description: OK\n  \"404\":\n    description: Not found\n"

	t.Run("an operation without the error response most operations declare is reported", func(t *testing.T) {
		document := mustParse(t, lintSpec(5, func(index int) string {
			if index == 4 {
				return lintOKResponse
			}
			return withNotFound
		}, ""))
		findings := lintFindings(t, document, apispec.RuleMissingStandardResponses)
		wantPointers(t, findings, "/paths/~1r4/get")
		if want := "Operation does not declare the `404` response its neighbours declare."; findings[0].Message != want {
			t.Errorf("Message = %q, want %q", findings[0].Message, want)
		}
		if want := "80% of 5 operations declare `404`"; findings[0].Convention != want {
			t.Errorf("Convention = %q, want %q", findings[0].Convention, want)
		}
		if want := "missing `404`"; findings[0].Evidence != want {
			t.Errorf("Evidence = %q, want %q", findings[0].Evidence, want)
		}
	})

	t.Run("a tag baseline names the tag", func(t *testing.T) {
		document := mustParse(t, lintSpec(8, func(index int) string {
			switch {
			case index < 4:
				return "tags: [pets]\n" + withNotFound
			case index == 4:
				return "tags: [pets]\n" + lintOKResponse
			default:
				return "tags: [owners]\n" + lintOKResponse
			}
		}, ""))
		findings := lintFindings(t, document, apispec.RuleMissingStandardResponses)
		wantPointers(t, findings, "/paths/~1r4/get")
		if want := "80% of 5 pets operations declare `404`"; findings[0].Convention != want {
			t.Errorf("Convention = %q, want %q", findings[0].Convention, want)
		}
	})

	t.Run("a tag with fewer than three operations falls back to the whole spec", func(t *testing.T) {
		document := mustParse(t, lintSpec(5, func(index int) string {
			switch index {
			case 3:
				return "tags: [owners]\n" + lintOKResponse
			case 4:
				return "tags: [owners]\n" + withNotFound
			default:
				return "tags: [pets]\n" + withNotFound
			}
		}, ""))
		findings := lintFindings(t, document, apispec.RuleMissingStandardResponses)
		wantPointers(t, findings, "/paths/~1r3/get")
		if want := "80% of 5 operations declare `404`"; findings[0].Convention != want {
			t.Errorf("Convention = %q, want %q", findings[0].Convention, want)
		}
	})

	t.Run("no finding when fewer than 80% declare the response", func(t *testing.T) {
		document := mustParse(t, lintSpec(5, func(index int) string {
			if index < 3 {
				return withNotFound
			}
			return lintOKResponse
		}, ""))
		wantPointers(t, lintFindings(t, document, apispec.RuleMissingStandardResponses))
	})
}

func TestLintErrorShape(t *testing.T) {
	shared := "responses:\n  \"400\":\n    $ref: '#/components/responses/Error'\n"
	inline := "responses:\n  \"400\":\n    description: Bad\n    content:\n      application/json:\n" +
		"        schema:\n          type: object\n"
	components := "responses:\n  Error:\n    description: Error\n"

	t.Run("an inline error response is reported when most reuse a shared shape", func(t *testing.T) {
		document := mustParse(t, lintSpec(6, func(index int) string {
			if index == 5 {
				return inline
			}
			return shared
		}, components))
		findings := lintFindings(t, document, apispec.RuleErrorShape)
		wantPointers(t, findings, "/paths/~1r5/get/responses/400")
		if want := "83% of 6 error responses in this spec reference a shared response or error schema"; findings[0].Convention != want {
			t.Errorf("Convention = %q, want %q", findings[0].Convention, want)
		}
		if want := "`400` declares an inline schema"; findings[0].Evidence != want {
			t.Errorf("Evidence = %q, want %q", findings[0].Evidence, want)
		}
	})

	t.Run("an error response whose body references a schema counts as shared", func(t *testing.T) {
		referencing := "responses:\n  \"400\":\n    description: Bad\n    content:\n      application/json:\n" +
			"        schema:\n          $ref: '#/components/schemas/Error'\n"
		document := mustParse(t, lintSpec(6, func(index int) string {
			if index == 5 {
				return referencing
			}
			return shared
		}, components+"schemas:\n  Error:\n    type: object\n"))
		wantPointers(t, lintFindings(t, document, apispec.RuleErrorShape))
	})

	t.Run("no finding when fewer than 80% reuse a shared shape", func(t *testing.T) {
		document := mustParse(t, lintSpec(6, func(index int) string {
			if index < 3 {
				return inline
			}
			return shared
		}, components))
		wantPointers(t, lintFindings(t, document, apispec.RuleErrorShape))
	})
}

func TestLintMissingDescription(t *testing.T) {
	t.Run("an operation without a description is reported when most have one", func(t *testing.T) {
		document := mustParse(t, lintSpec(6, func(index int) string {
			if index == 5 {
				return lintOKResponse
			}
			return "description: Does a thing.\n" + lintOKResponse
		}, ""))
		findings := lintFindings(t, document, apispec.RuleMissingDescription)
		wantPointers(t, findings, "/paths/~1r5/get")
		if want := "Operation has no description."; findings[0].Message != want {
			t.Errorf("Message = %q, want %q", findings[0].Message, want)
		}
		if want := "83% of 6 operations in this spec have a description"; findings[0].Convention != want {
			t.Errorf("Convention = %q, want %q", findings[0].Convention, want)
		}
		if want := "`GET /r5`"; findings[0].Evidence != want {
			t.Errorf("Evidence = %q, want %q", findings[0].Evidence, want)
		}
	})

	t.Run("no operation finding when fewer than 80% have a description", func(t *testing.T) {
		document := mustParse(t, lintSpec(6, func(index int) string {
			if index < 3 {
				return "description: Does a thing.\n" + lintOKResponse
			}
			return lintOKResponse
		}, ""))
		wantPointers(t, lintFindings(t, document, apispec.RuleMissingDescription))
	})

	t.Run("a property without a description is reported when 70% have one", func(t *testing.T) {
		document := mustParse(t, lintSpec(0, nil, lintSchema("Pet", "petName", "petAge", "ownerId")+
			"  Owner:\n    type: object\n    properties:\n      ownerName:\n        type: string\n"+
			"      pet:\n        $ref: '#/components/schemas/Pet'\n"))
		findings := lintFindings(t, document, apispec.RuleMissingDescription)
		wantPointers(t, findings, "/components/schemas/Owner/properties/ownerName")
		if want := "Property has no description."; findings[0].Message != want {
			t.Errorf("Message = %q, want %q", findings[0].Message, want)
		}
		if want := "75% of 4 properties in this spec have a description"; findings[0].Convention != want {
			t.Errorf("Convention = %q, want %q", findings[0].Convention, want)
		}
	})

	t.Run("no property finding when fewer than 70% have a description", func(t *testing.T) {
		document := mustParse(t, lintSpec(0, nil, lintSchema("Pet", "petName", "petAge")+
			"  Owner:\n    type: object\n    properties:\n      ownerName:\n        type: string\n"+
			"      ownerAge:\n        type: integer\n"))
		wantPointers(t, lintFindings(t, document, apispec.RuleMissingDescription))
	})
}

func TestLintMissingExtension(t *testing.T) {
	t.Run("an operation without the extension most operations set is reported", func(t *testing.T) {
		document := mustParse(t, lintSpec(5, func(index int) string {
			if index == 4 {
				return lintOKResponse
			}
			return "x-audience: public\n" + lintOKResponse
		}, ""))
		findings := lintFindings(t, document, apispec.RuleMissingExtension)
		wantPointers(t, findings, "/paths/~1r4/get")
		if want := "Operation lacks the `x-audience` extension its neighbours carry."; findings[0].Message != want {
			t.Errorf("Message = %q, want %q", findings[0].Message, want)
		}
		if want := "80% of 5 operations set `x-audience`"; findings[0].Convention != want {
			t.Errorf("Convention = %q, want %q", findings[0].Convention, want)
		}
	})

	t.Run("no finding when fewer than 80% set the extension", func(t *testing.T) {
		document := mustParse(t, lintSpec(5, func(index int) string {
			if index < 3 {
				return "x-audience: public\n" + lintOKResponse
			}
			return lintOKResponse
		}, ""))
		wantPointers(t, lintFindings(t, document, apispec.RuleMissingExtension))
	})
}

func TestLintMissingSecurity(t *testing.T) {
	secured := "security:\n  - bearer: []\n" + lintOKResponse

	t.Run("an operation without security is reported when most declare it", func(t *testing.T) {
		document := mustParse(t, lintSpec(5, func(index int) string {
			if index == 4 {
				return lintOKResponse
			}
			return secured
		}, ""))
		findings := lintFindings(t, document, apispec.RuleMissingSecurity)
		wantPointers(t, findings, "/paths/~1r4/get")
		if want := "80% of 5 operations declare security"; findings[0].Convention != want {
			t.Errorf("Convention = %q, want %q", findings[0].Convention, want)
		}
	})

	t.Run("an explicit empty security list is a declaration", func(t *testing.T) {
		document := mustParse(t, lintSpec(5, func(index int) string {
			if index == 4 {
				return "security: []\n" + lintOKResponse
			}
			return secured
		}, ""))
		wantPointers(t, lintFindings(t, document, apispec.RuleMissingSecurity))
	})

	t.Run("no finding when fewer than 80% declare security", func(t *testing.T) {
		document := mustParse(t, lintSpec(5, func(index int) string {
			if index < 3 {
				return secured
			}
			return lintOKResponse
		}, ""))
		wantPointers(t, lintFindings(t, document, apispec.RuleMissingSecurity))
	})
}

func TestLintPathParameters(t *testing.T) {
	lintPath := func(path, pathItemParameters, operationParameters, components string) string {
		spec := lintHeader + "paths:\n  " + path + ":\n"
		if pathItemParameters != "" {
			spec += "    parameters:\n" + lintIndent(pathItemParameters, 6)
		}
		spec += "    get:\n"
		if operationParameters != "" {
			spec += "      parameters:\n" + lintIndent(operationParameters, 8)
		}
		spec += "      responses:\n        \"200\":\n          description: OK\n"
		if components != "" {
			spec += "components:\n" + lintIndent(components, 2)
		}
		return spec
	}
	idParameter := "- name: id\n  in: path\n  required: true\n  schema:\n    type: string\n"

	t.Run("a templated name with no path parameter is reported", func(t *testing.T) {
		document := mustParse(t, lintPath("/pets/{id}", "", "", ""))
		findings := lintFindings(t, document, apispec.RulePathParameters)
		wantPointers(t, findings, "/paths/~1pets~1{id}/get")
		if want := "The path has `id` with no path parameter."; findings[0].Message != want {
			t.Errorf("Message = %q, want %q", findings[0].Message, want)
		}
		if want := "`GET /pets/{id}`"; findings[0].Evidence != want {
			t.Errorf("Evidence = %q, want %q", findings[0].Evidence, want)
		}
	})

	t.Run("a path parameter missing from the path is reported", func(t *testing.T) {
		document := mustParse(t, lintPath("/pets", "", idParameter, ""))
		findings := lintFindings(t, document, apispec.RulePathParameters)
		wantPointers(t, findings, "/paths/~1pets/get")
		if want := "`id` is declared as a path parameter but not in the path."; findings[0].Message != want {
			t.Errorf("Message = %q, want %q", findings[0].Message, want)
		}
	})

	t.Run("both problems share one message", func(t *testing.T) {
		document := mustParse(t, lintPath("/pets/{petId}", "", idParameter, ""))
		findings := lintFindings(t, document, apispec.RulePathParameters)
		want := "The path has `petId` with no path parameter. `id` is declared as a path parameter but not in the path."
		if len(findings) != 1 || findings[0].Message != want {
			t.Errorf("findings = %+v, want one with message %q", findings, want)
		}
	})

	t.Run("operation, path item, and referenced parameters all count", func(t *testing.T) {
		referenced := "- $ref: '#/components/parameters/OwnerID'\n"
		components := "parameters:\n  OwnerID:\n    name: ownerId\n    in: path\n    required: true\n    schema:\n      type: string\n"
		document := mustParse(t, lintPath("/owners/{ownerId}/pets/{id}", idParameter, referenced, components))
		wantPointers(t, lintFindings(t, document, apispec.RulePathParameters))
	})

	t.Run("no finding when every name matches", func(t *testing.T) {
		document := mustParse(t, lintPath("/pets/{id}", "", idParameter, ""))
		wantPointers(t, lintFindings(t, document, apispec.RulePathParameters))
	})
}

func TestLintFindingsAreSortedByPointerThenRule(t *testing.T) {
	document := mustParse(t, lintSpec(6, func(index int) string {
		if index == 0 {
			return "operationId: list_pets_zero\n" + lintOKResponse
		}
		return fmt.Sprintf("operationId: listPets%d\ndescription: d\n", index) + lintOKResponse
	}, lintSchema("Pet", "petName", "petAge", "ownerId", "createdAt", "updatedAt", "birth_date")))
	findings := document.Lint()
	if len(findings) < 2 {
		t.Fatalf("findings = %+v, want at least 2", findings)
	}
	sorted := slices.IsSortedFunc(findings, func(left, right apispec.Finding) int {
		if byPointer := strings.Compare(left.Pointer, right.Pointer); byPointer != 0 {
			return byPointer
		}
		return strings.Compare(string(left.Rule), string(right.Rule))
	})
	if !sorted {
		t.Errorf("findings are not sorted by pointer then rule: %+v", findings)
	}
}

func TestLintChanges(t *testing.T) {
	baseComponents := lintSchema("Pet", "petName", "petAge", "ownerId", "createdAt", "updatedAt", "legacy_flag")
	operation := func(id string) func(int) string {
		return func(index int) string {
			return fmt.Sprintf("operationId: %s%d\ndescription: d\n", id, index) + lintOKResponse
		}
	}

	t.Run("an outlier unchanged from base is not reported", func(t *testing.T) {
		base := mustParse(t, lintSpec(0, nil, baseComponents))
		next := mustParse(t, lintSpec(0, nil, baseComponents))
		if findings := apispec.LintChanges(base, next); len(findings) != 0 {
			t.Errorf("LintChanges() = %+v, want none", findings)
		}
		if findings := base.Lint(); len(findings) != 1 {
			t.Fatalf("base.Lint() = %+v, want the legacy outlier", findings)
		}
	})

	t.Run("a new outlier is reported against the conventions of base", func(t *testing.T) {
		base := mustParse(t, lintSpec(0, nil, baseComponents))
		owner := lintSchema("Owner", "owner_name", "other_name", "third_name", "fourth_name")
		next := mustParse(t, lintSpec(0, nil, baseComponents+strings.TrimPrefix(owner, "schemas:\n")))
		findings := apispec.LintChanges(base, next)
		wantPointers(t, findings,
			"/components/schemas/Owner/properties/fourth_name",
			"/components/schemas/Owner/properties/other_name",
			"/components/schemas/Owner/properties/owner_name",
			"/components/schemas/Owner/properties/third_name",
		)
		if want := "83% of 6 multi-word property names in this spec are camelCase"; findings[0].Convention != want {
			t.Errorf("Convention = %q, want the base convention %q", findings[0].Convention, want)
		}
	})

	t.Run("a new operation is judged by the operation conventions of base", func(t *testing.T) {
		base := mustParse(t, lintSpec(6, operation("listPets"), ""))
		next := mustParse(t, lintSpec(7, func(index int) string {
			if index == 6 {
				return "operationId: list_pets_six\ndescription: d\n" + lintOKResponse
			}
			return operation("listPets")(index)
		}, ""))
		wantPointers(t, apispec.LintChanges(base, next), "/paths/~1r6/get")
	})

	t.Run("a changed operation is reported when it newly departs", func(t *testing.T) {
		withNotFound := "responses:\n  \"404\":\n    description: Not found\n"
		base := mustParse(t, lintSpec(5, func(int) string { return withNotFound }, ""))
		next := mustParse(t, lintSpec(5, func(index int) string {
			if index == 2 {
				return lintOKResponse
			}
			return withNotFound
		}, ""))
		wantPointers(t, apispec.LintChanges(base, next), "/paths/~1r2/get")
	})

	t.Run("a nil base lints next against itself", func(t *testing.T) {
		next := mustParse(t, lintSpec(0, nil, baseComponents))
		got, want := apispec.LintChanges(nil, next), next.Lint()
		if len(want) == 0 || !slices.Equal(got, want) {
			t.Errorf("LintChanges(nil, next) = %+v, want %+v", got, want)
		}
	})
}

func TestLintStripeSizedDocument(t *testing.T) {
	if testing.Short() {
		t.Skip("parses a 6.6 MB document")
	}
	document := mustParse(t, string(gunzip(t, filepath.Join("testdata", "stripe", "spec3.yaml.gz"))))
	first := document.Lint()
	second := document.Lint()
	if !slices.Equal(first, second) {
		t.Error("two Lint calls on the same document differ")
	}
	sorted := slices.IsSortedFunc(first, func(left, right apispec.Finding) int {
		if byPointer := strings.Compare(left.Pointer, right.Pointer); byPointer != 0 {
			return byPointer
		}
		return strings.Compare(string(left.Rule), string(right.Rule))
	})
	if !sorted {
		t.Error("findings are not sorted by pointer then rule")
	}
	if changes := apispec.LintChanges(document, document); len(changes) != 0 {
		t.Errorf("LintChanges of a document against itself = %d findings, want 0", len(changes))
	}
}
