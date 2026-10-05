package apispec_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/nanostack-dev/echopoint-kit/apispec"
)

const spliceBase = `# head
openapi: 3.0.3 # version
info: {title: Splice, version: 1.0.0}
paths:
  /pets:
    get:
      tags: [pets]
      parameters:
        - name: a
          in: query
          schema: {type: string}
        - name: b
          in: query
          schema: {type: string}
      responses:
        '200':
          description: OK # fine
  # about owners
  /owners:
    get:
      responses:
        '200':
          description: OK
`

func TestEditTouchesOnlyTheExpectedLines(t *testing.T) {
	summary := func(value string) apispec.Command {
		return apispec.Command{
			Kind:    apispec.CommandSetText,
			Pointer: apispec.OperationPointer("get", "/pets"),
			Field:   "summary",
			Value:   value,
		}
	}
	cases := []struct {
		name     string
		input    string
		commands []apispec.Command
		diff     string
	}{
		{
			"a new field goes after the fields before it",
			spliceBase,
			[]apispec.Command{summary("List")},
			"@@ -8 +8 @@\n+      summary: List\n",
		},
		{
			"a flow list stays a flow list",
			spliceBase,
			[]apispec.Command{
				{
					Kind:    apispec.CommandSetTags,
					Pointer: apispec.OperationPointer("get", "/pets"),
					Tags:    []string{"pets", "x"},
				},
			},
			"@@ -7 +7 @@\n-      tags: [pets]\n+      tags: [pets, x]\n",
		},
		{
			"the first item of a list is removed with its inline key",
			spliceBase,
			[]apispec.Command{{Kind: apispec.CommandRemoveParameter, Pointer: "/paths/~1pets/get/parameters/0"}},
			"@@ -9 +9 @@\n-        - name: a\n-          in: query\n-          schema: {type: string}\n",
		},
		{
			"the last parameter removed takes the parameters key with it",
			spliceBase,
			[]apispec.Command{
				{Kind: apispec.CommandRemoveParameter, Pointer: "/paths/~1pets/get/parameters/1"},
				{Kind: apispec.CommandRemoveParameter, Pointer: "/paths/~1pets/get/parameters/0"},
			},
			"@@ -8 +8 @@\n-      parameters:\n-        - name: a\n-          in: query\n-          schema: {type: string}\n" +
				"-        - name: b\n-          in: query\n-          schema: {type: string}\n",
		},
		{
			"the comment above a removed path goes with it",
			spliceBase,
			[]apispec.Command{
				{Kind: apispec.CommandRemoveOperation, Pointer: apispec.OperationPointer("get", "/owners")},
			},
			"@@ -18 +18 @@\n-  # about owners\n-  /owners:\n-    get:\n-      responses:\n-        '200':\n-          description: OK\n",
		},
		{
			"a new item is appended to a list",
			spliceBase,
			[]apispec.Command{
				{
					Kind:    apispec.CommandAddParameter,
					Pointer: apispec.OperationPointer("get", "/pets"),
					Name:    "c",
					In:      "query",
				},
			},
			"@@ -15 +15 @@\n+        - name: c\n+          in: query\n+          schema:\n+            type: string\n",
		},
		{
			"an empty paths key becomes an object",
			"openapi: 3.0.3\ninfo:\n  title: T\n  version: '1'\npaths:\n",
			[]apispec.Command{{Kind: apispec.CommandAddOperation, Path: "/a", Method: "get"}},
			"@@ -6 +6 @@\n+  /a:\n+    get:\n+      responses:\n+        \"200\":\n+          description: OK\n",
		},
		{
			"a key renamed keeps its value and comment",
			spliceBase,
			[]apispec.Command{
				{Kind: apispec.CommandSetMethod, Pointer: apispec.OperationPointer("get", "/owners"), Method: "head"},
			},
			"@@ -20 +20 @@\n-    get:\n+    head:\n",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := apispec.Edit([]byte(c.input), c.commands...)
			if err != nil {
				t.Fatal(err)
			}
			if diff := lineDiff(c.input, string(got)); diff != c.diff {
				t.Errorf("hunks:\n%s\nwant:\n%s", diff, c.diff)
			}
		})
	}
}

func TestEditKeepsCRLFLineEndings(t *testing.T) {
	input := strings.ReplaceAll(spliceBase, "\n", "\r\n")
	got, err := apispec.Edit([]byte(input), apispec.Command{
		Kind:    apispec.CommandSetText,
		Pointer: apispec.OperationPointer("get", "/pets"),
		Field:   "summary",
		Value:   "List",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(spliceBase, "      tags: [pets]\n", "      tags: [pets]\n      summary: List\n", 1)
	if string(got) != strings.ReplaceAll(want, "\n", "\r\n") {
		t.Errorf("got %q", got)
	}
}

func TestEditKeepsAMissingFinalNewline(t *testing.T) {
	input := strings.TrimSuffix(spliceBase, "\n")
	got, err := apispec.Edit([]byte(input), apispec.Command{
		Kind:    apispec.CommandSetText,
		Pointer: apispec.OperationPointer("get", "/owners"),
		Field:   "summary",
		Value:   "Owners",
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := input + "\n"; string(got) == want || strings.HasSuffix(string(got), "\n") {
		t.Errorf("got %q", got)
	}
}

func TestEditKeepsTheJSONIndentation(t *testing.T) {
	for _, indent := range []string{"  ", "    ", "   "} {
		input := "{\n" + indent + "\"openapi\": \"3.0.3\",\n" + indent + "\"info\": {\n" + indent + indent +
			"\"title\": \"T\",\n" + indent + indent + "\"version\": \"1\"\n" + indent + "},\n" + indent + "\"paths\": {}\n}\n"
		got, err := apispec.Edit([]byte(input), apispec.Command{
			Kind: apispec.CommandAddOperation, Path: "/a", Method: "get",
		})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(
			string(got),
			"{\n"+indent+"\"openapi\": \"3.0.3\",\n"+indent+"\"info\": {\n"+indent+indent+"\"title\"",
		) {
			t.Errorf("indent %q: got\n%s", indent, got)
		}
		if !strings.HasSuffix(string(got), "}\n") {
			t.Errorf("indent %q lost the final newline", indent)
		}
	}
}

func TestEditRefusesToGoThroughAnAlias(t *testing.T) {
	input := `openapi: 3.0.3
info: {title: T, version: "1"}
paths:
  /a:
    get: &shared
      responses:
        "200": {description: OK}
  /b:
    get: *shared
`
	_, err := apispec.Edit([]byte(input), apispec.Command{
		Kind: apispec.CommandSetText, Pointer: apispec.OperationPointer("get", "/b"), Field: "summary", Value: "x",
	})
	if err == nil || !strings.Contains(err.Error(), "alias") {
		t.Fatalf("got %v, want an alias refusal", err)
	}
}

func TestEditAStripeSizedDocumentChangesOneHunk(t *testing.T) {
	if testing.Short() {
		t.Skip("the Stripe document is 6.6 MB")
	}
	input := gunzip(t, filepath.Join("testdata", "stripe", "spec3.yaml.gz"))
	pointer := apispec.SchemaPointer("account")
	got, err := apispec.Edit(input, apispec.Command{
		Kind: apispec.CommandSetText, Pointer: pointer, Field: "description", Value: "Changed by the test.",
	})
	if err != nil {
		t.Fatal(err)
	}
	if hunks := strings.Count(lineDiff(string(input), string(got)), "@@ -"); hunks != 1 {
		t.Errorf("got %d hunks, want 1", hunks)
	}
}
