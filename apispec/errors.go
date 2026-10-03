package apispec

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrUnreadable         = errors.New("the document is not valid YAML or JSON")
	ErrSwagger2           = errors.New("swagger 2.0 is not supported: convert the document to OpenAPI 3.0 or 3.1")
	ErrUnsupportedVersion = errors.New("only OpenAPI 3.0.x and 3.1.x are supported")
	ErrExternalRef        = errors.New("external $ref is not supported: bundle the document into one file first")
	ErrInvalid            = errors.New("the document is not a valid OpenAPI document")
	ErrNothingToPublish   = errors.New("nothing to publish: the document is identical to the Live version")
	ErrComparisonFailed   = errors.New("comparison failed")
)

type ExternalRef struct {
	Pointer string
	Ref     string
}

type ExternalRefError struct {
	Refs []ExternalRef
}

func (e *ExternalRefError) Error() string {
	refs := make([]string, 0, len(e.Refs))
	for _, ref := range e.Refs {
		refs = append(refs, fmt.Sprintf("%s -> %s", ref.Pointer, ref.Ref))
	}
	return fmt.Sprintf("%s (%s)", ErrExternalRef, strings.Join(refs, ", "))
}

func (e *ExternalRefError) Unwrap() error {
	return ErrExternalRef
}

type ValidationError struct {
	Problems []string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", ErrInvalid, strings.Join(e.Problems, "; "))
}

func (e *ValidationError) Unwrap() error {
	return ErrInvalid
}

type ComparisonError struct {
	Reason string
}

func (e *ComparisonError) Error() string {
	return fmt.Sprintf("%s: %s", ErrComparisonFailed, e.Reason)
}

func (e *ComparisonError) Unwrap() error {
	return ErrComparisonFailed
}
