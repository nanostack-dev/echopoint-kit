package apispec

import (
	"bytes"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/oasdiff/oasdiff/checker"
	"github.com/oasdiff/oasdiff/checker/metaschema"
	"github.com/oasdiff/oasdiff/checker/rules"
	"github.com/oasdiff/oasdiff/diff"
	"github.com/oasdiff/oasdiff/load"
)

// Severity is what a change means for the API's clients, the grouping a
// reviewer reads a diff by.
type Severity string

const (
	// SeverityBreaking breaks clients that worked with the base version.
	SeverityBreaking Severity = "breaking"
	// SeverityRisky may break clients; it needs a look.
	SeverityRisky Severity = "risky"
	// SeverityAdditive adds to the API or loosens what it accepts.
	SeverityAdditive Severity = "additive"
	// SeverityEdit changes descriptions, examples, extensions, or other details
	// clients do not depend on.
	SeverityEdit Severity = "edit"
)

// Change is one difference between two versions of a document.
type Change struct {
	// ID names the kind of change, such as "endpoint-added": an oasdiff rule id,
	// or one of the ids this package adds for what oasdiff does not report
	// ("schema-added", "operation-edited", "document-edited").
	ID       string
	Severity Severity
	// Method and Path locate the operation the change is in, when it is in one.
	Method string
	Path   string
	// Pointer is the JSON pointer of the operation or component the change is
	// in, or empty when it concerns the whole document.
	Pointer string
	Text    string
}

// Comparison is the result of comparing a revision with its base.
type Comparison struct {
	Bump    Bump
	Changes []Change
}

const (
	changeSchemaAdded     = "schema-added"
	changeOperationEdited = "operation-edited"
	changeDocumentEdited  = "document-edited"
)

// Compare reports what changed from base to revision and the version bump it
// calls for. info.version is ignored on both sides: EchoPoint owns it, so
// writing it never counts as a change. A failure inside the comparison is
// returned as a ComparisonError and never as an empty comparison.
func Compare(base, revision *Document) (Comparison, error) {
	identical, err := sameContent(base, revision)
	if err != nil {
		return Comparison{}, err
	}
	if identical {
		return Comparison{Bump: BumpNone}, nil
	}

	report, sources, err := diffDocuments(base, revision)
	if err != nil {
		return Comparison{}, err
	}
	changes := checkerChanges(report, sources)
	changes = append(changes, uncheckedChanges(report, changes)...)
	if len(changes) == 0 {
		changes = append(changes, Change{
			ID:       changeDocumentEdited,
			Severity: SeverityEdit,
			Text:     "the document changed without changing what clients see (order or formatting of fields)",
		})
	}
	return Comparison{Bump: bumpFor(changes), Changes: changes}, nil
}

func sameContent(base, revision *Document) (bool, error) {
	baseYAML, err := base.WithVersion("").Canonical()
	if err != nil {
		return false, err
	}
	revisionYAML, err := revision.WithVersion("").Canonical()
	if err != nil {
		return false, err
	}
	return bytes.Equal(baseYAML, revisionYAML), nil
}

func diffDocuments(base, revision *Document) (*diff.Diff, *diff.OperationsSourcesMap, error) {
	var (
		report  *diff.Diff
		sources *diff.OperationsSourcesMap
		failure error
	)
	func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				report, sources = nil, nil
				failure = &ComparisonError{Reason: fmt.Sprint(recovered)}
			}
		}()
		baseInfo := &load.SpecInfo{Url: "base", Spec: withoutVersion(base.spec)}
		revisionInfo := &load.SpecInfo{Url: "revision", Spec: withoutVersion(revision.spec)}
		var err error
		report, sources, err = diff.GetWithOperationsSourcesMap(diff.NewConfig(), baseInfo, revisionInfo)
		if err != nil {
			failure = &ComparisonError{Reason: err.Error()}
		}
	}()
	if failure != nil {
		return nil, nil, failure
	}
	return report, sources, nil
}

func withoutVersion(spec *openapi3.T) *openapi3.T {
	clone := *spec
	info := openapi3.Info{}
	if spec.Info != nil {
		info = *spec.Info
	}
	info.Version = ""
	clone.Info = &info
	return &clone
}

func checkerChanges(report *diff.Diff, sources *diff.OperationsSourcesMap) []Change {
	ruleIndex := make(map[string]rules.Rule)
	for _, rule := range checker.GetAllRules().Metadata() {
		ruleIndex[rule.Id] = rule
	}

	config := checker.NewConfig(checker.GetAllChecks())
	localizer := checker.NewDefaultLocalizer()
	found := checker.CheckBackwardCompatibilityUntilLevel(config, report, sources, checker.INFO)

	changes := make([]Change, 0, len(found))
	for _, change := range found {
		rule := ruleIndex[change.GetId()]
		if rule.Area == rules.AreaInfo {
			continue
		}
		changes = append(changes, Change{
			ID:       change.GetId(),
			Severity: severityOf(change.GetLevel(), rule),
			Method:   change.GetOperation(),
			Path:     change.GetPath(),
			Pointer:  operationPointer(change.GetOperation(), change.GetPath()),
			Text:     change.GetUncolorizedText(localizer),
		})
	}
	return changes
}

// severityOf maps oasdiff's verdict onto the review grouping. Errors break
// clients and warnings may; among compatible changes, what widens the
// contract or adds to it is additive, and the rest is an edit.
func severityOf(level checker.Level, rule rules.Rule) Severity {
	switch {
	case level == checker.ERR:
		return SeverityBreaking
	case level == checker.WARN:
		return SeverityRisky
	case rule.Effect == rules.EffectWidens:
		return SeverityAdditive
	case rule.Effect == rules.EffectNone && slices.Contains(rule.Actions(), metaschema.ActionAdd):
		return SeverityAdditive
	}
	return SeverityEdit
}

// uncheckedChanges reports what oasdiff's checks do not: new component
// schemas, and operations that changed only in ways no check covers, such as
// a description.
func uncheckedChanges(report *diff.Diff, checked []Change) []Change {
	if report == nil {
		return nil
	}
	var changes []Change
	if report.ComponentsDiff != nil && report.ComponentsDiff.SchemasDiff != nil {
		for _, name := range report.ComponentsDiff.SchemasDiff.Added {
			changes = append(changes, Change{
				ID:       changeSchemaAdded,
				Severity: SeverityAdditive,
				Pointer:  "/components/schemas/" + pointerToken(name),
				Text:     fmt.Sprintf("added the schema '%s'", name),
			})
		}
	}

	if report.PathsDiff == nil {
		return changes
	}
	for _, path := range slices.Sorted(maps.Keys(report.PathsDiff.Modified)) {
		pathDiff := report.PathsDiff.Modified[path]
		if pathDiff == nil || pathDiff.OperationsDiff == nil {
			continue
		}
		for _, method := range slices.Sorted(maps.Keys(pathDiff.OperationsDiff.Modified)) {
			if hasChangeAt(checked, method, path) {
				continue
			}
			changes = append(changes, Change{
				ID:       changeOperationEdited,
				Severity: SeverityEdit,
				Method:   method,
				Path:     path,
				Pointer:  operationPointer(method, path),
				Text:     fmt.Sprintf("edited the documentation of %s %s", method, path),
			})
		}
	}
	return changes
}

func hasChangeAt(changes []Change, method, path string) bool {
	return slices.ContainsFunc(changes, func(change Change) bool {
		return strings.EqualFold(change.Method, method) && change.Path == path
	})
}

func operationPointer(method, path string) string {
	if path == "" {
		return ""
	}
	pointer := "/paths/" + pointerToken(path)
	if method != "" {
		pointer += "/" + strings.ToLower(method)
	}
	return pointer
}
