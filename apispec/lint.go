package apispec

import (
	"cmp"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// LintRule names a convention rule. The names are stored with findings and
// shared with the browser, so renaming one is a breaking change.
type LintRule string

const (
	RulePropertyCasing           LintRule = "property-casing"
	RuleOperationIDCasing        LintRule = "operation-id-casing"
	RuleMissingStandardResponses LintRule = "missing-standard-responses"
	RuleErrorShape               LintRule = "error-shape"
	RuleMissingDescription       LintRule = "missing-description"
	RuleMissingExtension         LintRule = "missing-extension"
	RuleMissingSecurity          LintRule = "missing-security"
	RulePathParameters           LintRule = "path-parameters"
)

// Finding is one node of a document that departs from the document's own
// conventions. Pointer is an RFC 6901 JSON pointer into the document without
// the leading '#'. Convention says what the node departs from and Evidence
// what was seen.
type Finding struct {
	Rule       LintRule `json:"rule"`
	Pointer    string   `json:"pointer"`
	Message    string   `json:"message"`
	Convention string   `json:"convention"`
	Evidence   string   `json:"evidence"`
}

const (
	maxEvidenceItems = 4
	percentScale     = 100
)

var pathToken = regexp.MustCompile(`\{([^}]+)\}`)

// Lint reports every node of the document that departs from the document's
// own conventions: the casing most of its property names and operation IDs
// share, and what most operations of the same tag declare. A convention
// exists only when 80% of the document agrees (70% for property
// descriptions). Findings are sorted by pointer, then rule.
func (d *Document) Lint() []Finding {
	return sortedFindings(lintDocument(newBaseline(d), d))
}

// LintChanges reports the findings of next that base does not already have: a
// departure on a node that is new or changed. The conventions are those of
// base, so a change is judged by the document it amends. A nil base behaves
// like next.Lint().
func LintChanges(base, next *Document) []Finding {
	if base == nil {
		return next.Lint()
	}
	conventions := newBaseline(base)
	existing := make(map[Finding]bool)
	for _, finding := range lintDocument(conventions, base) {
		existing[finding] = true
	}
	var introduced []Finding
	for _, finding := range lintDocument(conventions, next) {
		if !existing[finding] {
			introduced = append(introduced, finding)
		}
	}
	return sortedFindings(introduced)
}

func sortedFindings(findings []Finding) []Finding {
	slices.SortFunc(findings, func(left, right Finding) int {
		return cmp.Or(
			cmp.Compare(left.Pointer, right.Pointer),
			cmp.Compare(left.Rule, right.Rule),
			cmp.Compare(left.Message, right.Message),
		)
	})
	return slices.Compact(findings)
}

func lintDocument(conventions *baseline, document *Document) []Finding {
	root := document.root
	operations := collectOperations(root)
	var findings []Finding
	for _, operation := range operations {
		findings = append(findings, conventions.lintOperation(root, operation)...)
	}
	for _, property := range lintedSchemaProperties(root, operations) {
		findings = append(findings, conventions.lintProperty(property)...)
	}
	for _, parameter := range lintedParameters(root, operations) {
		findings = append(findings, conventions.lintParameter(parameter)...)
	}
	return findings
}

func percent(share float64) string {
	return strconv.Itoa(int(math.Round(share*percentScale))) + "%"
}

func plural(count int, singular, pluralForm string) string {
	if pluralForm == "" {
		pluralForm = singular + "s"
	}
	noun := pluralForm
	if count == 1 {
		noun = singular
	}
	return groupThousands(count) + " " + noun
}

func groupThousands(count int) string {
	digits := strconv.Itoa(count)
	var grouped strings.Builder
	for index, digit := range digits {
		if index > 0 && (len(digits)-index)%3 == 0 {
			grouped.WriteByte(',')
		}
		grouped.WriteRune(digit)
	}
	return grouped.String()
}

func quoted(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, "`"+value+"`")
	}
	return result
}

func formatNames(names []string) string {
	shown := quoted(names[:min(len(names), maxEvidenceItems)])
	if rest := len(names) - len(shown); rest > 0 {
		return fmt.Sprintf("%s and %d more", strings.Join(shown, ", "), rest)
	}
	return strings.Join(shown, ", ")
}

func areOrIs(count int) string {
	if count > 1 {
		return "are"
	}
	return "is"
}

func scopedOperations(count int, label string) string {
	noun := "operation"
	if label != "" {
		noun = label + " operation"
	}
	return plural(count, noun, "")
}

func (c *casingConvention) finding(rule LintRule, pointer, subject, multiWord, name string) Finding {
	return Finding{
		Rule:    rule,
		Pointer: pointer,
		Message: fmt.Sprintf(
			"%s is %s, but this spec uses %s.", subject, classifyCasing(name).label(), c.casing.label(),
		),
		Convention: fmt.Sprintf(
			"%s of %s in this spec %s %s",
			percent(c.share), plural(c.total, multiWord, ""), areOrIs(c.total), c.casing.label(),
		),
		Evidence: formatNames([]string{name}),
	}
}

func (c *casingConvention) departs(name string) bool {
	kind := classifyCasing(name)
	return kind.isNamed() && kind != c.casing
}

func (b *baseline) lintProperty(property propertyRecord) []Finding {
	var findings []Finding
	if b.propertyCasing != nil && b.propertyCasing.departs(property.name) {
		findings = append(findings, b.propertyCasing.finding(
			RulePropertyCasing, property.pointer, "Property name", "multi-word property name", property.name,
		))
	}
	if !property.referenceOnly && !property.described && b.propertyShare.share >= propertyDescriptionMajority {
		findings = append(findings, Finding{
			Rule:    RuleMissingDescription,
			Pointer: property.pointer,
			Message: "Property has no description.",
			Convention: fmt.Sprintf(
				"%s of %s in this spec have a description",
				percent(b.propertyShare.share), plural(b.propertyShare.total, "property", "properties"),
			),
			Evidence: formatNames([]string{property.name}),
		})
	}
	return findings
}

func (b *baseline) lintParameter(parameter lintNode) []Finding {
	location, _ := stringValue(lookup(parameter.node, "in"))
	name, isString := stringValue(lookup(parameter.node, "name"))
	if location != "query" || !isString || b.propertyCasing == nil || !b.propertyCasing.departs(name) {
		return nil
	}
	return []Finding{b.propertyCasing.finding(
		RulePropertyCasing, parameter.pointer, "Parameter name", "multi-word property name", name,
	)}
}

func (b *baseline) lintOperation(root *yaml.Node, operation operationRecord) []Finding {
	profile, label := b.profileFor(operation.tag)
	var findings []Finding
	findings = append(findings, b.operationIDFinding(operation)...)
	findings = append(findings, missingStandardResponses(operation, profile, label)...)
	findings = append(findings, b.errorShapeFindings(operation)...)
	findings = append(findings, b.missingDescription(operation)...)
	findings = append(findings, missingExtensions(operation, profile, label)...)
	findings = append(findings, missingSecurity(operation, profile, label)...)
	findings = append(findings, pathParameterFindings(root, operation)...)
	return findings
}

func (b *baseline) operationIDFinding(operation operationRecord) []Finding {
	id, isString := stringValue(lookup(operation.node, "operationId"))
	if !isString || b.operationIDCasing == nil || !b.operationIDCasing.departs(id) {
		return nil
	}
	return []Finding{b.operationIDCasing.finding(
		RuleOperationIDCasing, operation.pointer, "Operation ID", "multi-word operation ID", id,
	)}
}

func missingStandardResponses(operation operationRecord, profile *operationProfile, label string) []Finding {
	declared := make(map[string]bool)
	for _, response := range mappingOf(lookup(operation.node, "responses")) {
		declared[response.key.Value] = true
	}
	var missing []string
	lowest := 1.0
	for _, status := range sortedKeys(profile.statusCodes) {
		share := profile.share(profile.statusCodes[status])
		if isErrorStatus(status) && share >= majority && !declared[status] {
			missing = append(missing, status)
			lowest = math.Min(lowest, share)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	codes := strings.Join(quoted(missing), " and ")
	noun := "response"
	if len(missing) > 1 {
		noun = "responses"
	}
	return []Finding{{
		Rule:    RuleMissingStandardResponses,
		Pointer: operation.pointer,
		Message: fmt.Sprintf("Operation does not declare the %s %s its neighbours declare.", codes, noun),
		Convention: fmt.Sprintf(
			"%s of %s declare %s", percent(lowest), scopedOperations(profile.count, label), codes,
		),
		Evidence: "missing " + strings.Join(quoted(missing), ", "),
	}}
}

func (b *baseline) errorShapeFindings(operation operationRecord) []Finding {
	if b.errorShape.share < majority {
		return nil
	}
	var findings []Finding
	for _, response := range mappingOf(lookup(operation.node, "responses")) {
		status := response.key.Value
		if !isErrorStatus(status) || isSharedErrorShape(response.value) {
			continue
		}
		findings = append(findings, Finding{
			Rule:    RuleErrorShape,
			Pointer: childPointer(operation.pointer, "responses", status),
			Message: "Error response does not reuse the shared error shape.",
			Convention: fmt.Sprintf(
				"%s of %s in this spec reference a shared response or error schema",
				percent(b.errorShape.share), plural(b.errorShape.total, "error response", ""),
			),
			Evidence: fmt.Sprintf("`%s` declares an inline schema", status),
		})
	}
	return findings
}

func (b *baseline) missingDescription(operation operationRecord) []Finding {
	if isNonBlankString(lookup(operation.node, "description")) || b.overall.count == 0 {
		return nil
	}
	share := b.overall.share(b.overall.withDescription)
	if share < majority {
		return nil
	}
	return []Finding{{
		Rule:    RuleMissingDescription,
		Pointer: operation.pointer,
		Message: "Operation has no description.",
		Convention: fmt.Sprintf(
			"%s of %s in this spec have a description", percent(share), plural(b.overall.count, "operation", ""),
		),
		Evidence: formatNames([]string{operation.label()}),
	}}
}

func missingExtensions(operation operationRecord, profile *operationProfile, label string) []Finding {
	var missing []string
	lowest := 1.0
	for _, key := range sortedKeys(profile.extensions) {
		share := profile.share(profile.extensions[key])
		if share >= majority && lookup(operation.node, key) == nil {
			missing = append(missing, key)
			lowest = math.Min(lowest, share)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	keys := strings.Join(quoted(missing), " and ")
	noun := "extension"
	if len(missing) > 1 {
		noun = "extensions"
	}
	return []Finding{{
		Rule:    RuleMissingExtension,
		Pointer: operation.pointer,
		Message: fmt.Sprintf("Operation lacks the %s %s its neighbours carry.", keys, noun),
		Convention: fmt.Sprintf(
			"%s of %s set %s", percent(lowest), scopedOperations(profile.count, label), keys,
		),
		Evidence: "missing " + strings.Join(quoted(missing), ", "),
	}}
}

func missingSecurity(operation operationRecord, profile *operationProfile, label string) []Finding {
	if lookup(operation.node, "security") != nil || profile.count == 0 {
		return nil
	}
	share := profile.share(profile.withSecurity)
	if share < majority {
		return nil
	}
	return []Finding{{
		Rule:    RuleMissingSecurity,
		Pointer: operation.pointer,
		Message: "Operation declares no security requirement, unlike its neighbours.",
		Convention: fmt.Sprintf(
			"%s of %s declare security", percent(share), scopedOperations(profile.count, label),
		),
		Evidence: fmt.Sprintf("no `security` on `%s`", operation.label()),
	}}
}

func declaredPathParameters(root *yaml.Node, operation operationRecord) []string {
	var declared []string
	raw := slices.Concat(
		sequenceOf(lookup(operation.pathItem, "parameters")),
		sequenceOf(lookup(operation.node, "parameters")),
	)
	for _, parameter := range raw {
		resolved := resolveReferences(root, parameter)
		location, hasLocation := stringValue(lookup(resolved, "in"))
		name, hasName := stringValue(lookup(resolved, "name"))
		if hasLocation && hasName && location == "path" && !slices.Contains(declared, name) {
			declared = append(declared, name)
		}
	}
	return declared
}

func templatedNames(path string) []string {
	var templated []string
	for _, match := range pathToken.FindAllStringSubmatch(path, -1) {
		if !slices.Contains(templated, match[1]) {
			templated = append(templated, match[1])
		}
	}
	return templated
}

func pathParameterFindings(root *yaml.Node, operation operationRecord) []Finding {
	declared := declaredPathParameters(root, operation)
	templated := templatedNames(operation.path)
	undeclared := slices.DeleteFunc(slices.Clone(templated), func(name string) bool {
		return slices.Contains(declared, name)
	})
	unused := slices.DeleteFunc(slices.Clone(declared), func(name string) bool {
		return slices.Contains(templated, name)
	})
	if len(undeclared) == 0 && len(unused) == 0 {
		return nil
	}
	var messages []string
	if len(undeclared) > 0 {
		messages = append(messages, fmt.Sprintf("The path has %s with no path parameter.", formatNames(undeclared)))
	}
	if len(unused) > 0 {
		messages = append(messages, fmt.Sprintf(
			"%s %s declared as a path parameter but not in the path.", formatNames(unused), areOrIs(len(unused)),
		))
	}
	return []Finding{{
		Rule:    RulePathParameters,
		Pointer: operation.pointer,
		Message: strings.Join(messages, " "),
		Convention: "Every {name} in a path has a matching in: path parameter, " +
			"and every path parameter appears in the path",
		Evidence: "`" + operation.label() + "`",
	}}
}
