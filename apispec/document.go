package apispec

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"go.yaml.in/yaml/v3"
)

var supportedOpenAPIVersion = regexp.MustCompile(`^3\.[01](\.\d+)?$`)

// Document is a parsed, self-contained OpenAPI 3.0 or 3.1 document. It keeps
// the YAML tree it was read from, for the canonical layout, and the OpenAPI
// model, for validation and comparison.
type Document struct {
	root    *yaml.Node
	spec    *openapi3.T
	version *string
}

// Parse reads an OpenAPI 3.0.x or 3.1.x document from YAML or JSON. It refuses
// Swagger 2.0, other OpenAPI versions, and documents with an external $ref.
// A document that parses can still fail Validate.
func Parse(data []byte) (*Document, error) {
	root, err := decodeRoot(data)
	if err != nil {
		return nil, err
	}
	if versionErr := checkOpenAPIVersion(root); versionErr != nil {
		return nil, versionErr
	}
	if refs := findExternalRefs(root); len(refs) > 0 {
		return nil, &ExternalRefError{Refs: refs}
	}

	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = false
	spec, err := loader.LoadFromData(data)
	if err != nil {
		return nil, &ValidationError{Problems: []string{loadProblem(err)}}
	}
	return &Document{root: root, spec: spec}, nil
}

// loadProblem drops the JSON half of kin-openapi's load error: the document
// already decoded as YAML, so only the YAML half says what is wrong.
func loadProblem(err error) string {
	message := err.Error()
	if _, yamlProblem, found := strings.Cut(message, "yaml error: "); found {
		return yamlProblem
	}
	return message
}

func decodeRoot(data []byte) (*yaml.Node, error) {
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrUnreadable, err.Error())
	}
	if document.Kind != yaml.DocumentNode || len(document.Content) == 0 {
		return nil, ErrUnreadable
	}
	root := resolveAlias(document.Content[0])
	if root.Kind != yaml.MappingNode {
		return nil, &ValidationError{Problems: []string{"the document root must be an object"}}
	}
	return root, nil
}

func checkOpenAPIVersion(root *yaml.Node) error {
	if lookup(root, "swagger") != nil {
		return ErrSwagger2
	}
	version := lookup(root, "openapi")
	if version == nil || version.Kind != yaml.ScalarNode {
		return fmt.Errorf("%w: the document has no openapi field", ErrUnsupportedVersion)
	}
	if !supportedOpenAPIVersion.MatchString(version.Value) {
		return fmt.Errorf("%w: found %q", ErrUnsupportedVersion, version.Value)
	}
	return nil
}

func findExternalRefs(root *yaml.Node) []ExternalRef {
	var refs []ExternalRef
	newLayouts().collectExternalRefs(root, object(kindRoot), "", &refs)
	return refs
}

func (l layouts) collectExternalRefs(node *yaml.Node, shape valueShape, pointer string, refs *[]ExternalRef) {
	if shape.kind == kindAny {
		return
	}
	node = resolveAlias(node)
	switch node.Kind {
	case yaml.MappingNode:
		for _, entry := range mappingEntries(node) {
			key := entry.key.Value
			value := resolveAlias(entry.value)
			childPointer := pointer + "/" + pointerToken(key)
			if key == refKey && shape.container == containerSingle && value.Kind == yaml.ScalarNode {
				if !strings.HasPrefix(value.Value, "#") {
					*refs = append(*refs, ExternalRef{Pointer: childPointer, Ref: value.Value})
				}
				continue
			}
			l.collectExternalRefs(value, l.entryShape(shape, key), childPointer, refs)
		}
	case yaml.SequenceNode:
		for index, item := range node.Content {
			l.collectExternalRefs(item, itemShape(shape), fmt.Sprintf("%s/%d", pointer, index), refs)
		}
	case yaml.DocumentNode, yaml.ScalarNode, yaml.AliasNode:
	}
}

// Validate checks the document against the OpenAPI specification of its
// version. Every problem found is reported in one ValidationError.
func (d *Document) Validate() error {
	options := []openapi3.ValidationOption{openapi3.EnableMultiError()}
	if d.spec.IsOpenAPI31OrLater() {
		options = append(options, openapi3.IsOpenAPI31OrLater())
	}
	err := d.spec.Validate(context.Background(), options...)
	if err == nil {
		return nil
	}
	return &ValidationError{Problems: validationProblems(err)}
}

func validationProblems(err error) []string {
	var multi openapi3.MultiError
	if !errors.As(err, &multi) {
		return []string{err.Error()}
	}
	problems := make([]string, 0, len(multi))
	for _, problem := range multi {
		problems = append(problems, validationProblems(problem)...)
	}
	return problems
}

// OpenAPIVersion is the document's openapi field, such as "3.1.0".
func (d *Document) OpenAPIVersion() string {
	return d.spec.OpenAPI
}

// Title is the document's info.title.
func (d *Document) Title() string {
	if d.spec.Info == nil {
		return ""
	}
	return d.spec.Info.Title
}

// Version is the document's info.version.
func (d *Document) Version() string {
	if d.spec.Info == nil {
		return ""
	}
	return d.spec.Info.Version
}

// WithVersion returns a copy of the document whose info.version is version.
// The receiver is left unchanged.
func (d *Document) WithVersion(version string) *Document {
	spec := *d.spec
	info := openapi3.Info{}
	if d.spec.Info != nil {
		info = *d.spec.Info
	}
	info.Version = version
	spec.Info = &info
	return &Document{root: d.root, spec: &spec, version: &version}
}
