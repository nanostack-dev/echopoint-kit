// Package bridge exposes apispec to JavaScript as one JSON-in, JSON-out call,
// so the browser runs the same engine as the server through WebAssembly. The
// error codes are the ones the Echopoint API returns for the same failures.
package bridge

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/nanostack-dev/echopoint-kit/apispec"
)

// Operation names what a Request asks for.
type Operation string

const (
	OperationLint        Operation = "lint"
	OperationLintChanges Operation = "lint_changes"
	OperationCompare     Operation = "compare"
	OperationEdit        Operation = "edit"
	OperationCanonical   Operation = "canonical"
)

// Request is one call. Base is read by lint_changes and compare, Commands by edit.
type Request struct {
	Operation Operation         `json:"operation"`
	Document  string            `json:"document"`
	Base      string            `json:"base,omitempty"`
	Commands  []apispec.Command `json:"commands,omitempty"`
}

// Response holds either Result or Error.
type Response struct {
	OK     bool   `json:"ok"`
	Result any    `json:"result,omitempty"`
	Error  *Error `json:"error,omitempty"`
}

// Error is a refusal, with the Echopoint API's error code for it.
type Error struct {
	Code     string        `json:"code"`
	Message  string        `json:"message"`
	Index    *int          `json:"index,omitempty"`
	Reason   string        `json:"reason,omitempty"`
	Problems []string      `json:"problems,omitempty"`
	Refs     []ExternalRef `json:"refs,omitempty"`
}

// ExternalRef is a $ref to another file or a URL, which a document may not have.
type ExternalRef struct {
	Pointer string `json:"pointer"`
	Ref     string `json:"ref"`
}

// Findings is the result of lint and lint_changes.
type Findings struct {
	Findings []apispec.Finding `json:"findings"`
}

// Comparison is the result of compare.
type Comparison struct {
	Bump    apispec.Bump `json:"bump"`
	Changes []Change     `json:"changes"`
}

// Change is one difference, as the API's SpecChange.
type Change struct {
	ID       string           `json:"id"`
	Severity apispec.Severity `json:"severity"`
	Method   string           `json:"method,omitempty"`
	Path     string           `json:"path,omitempty"`
	Pointer  string           `json:"pointer,omitempty"`
	Text     string           `json:"text"`
}

// Document is the result of edit and canonical.
type Document struct {
	Document string `json:"document"`
}

// Handle runs one JSON-encoded Request and returns the JSON-encoded Response.
// It never panics on bad input: a failure is a Response with an Error.
func Handle(request []byte) []byte {
	var decoded Request
	if err := json.Unmarshal(request, &decoded); err != nil {
		return encode(failure(&Error{Code: "BAD_REQUEST", Message: "the request is not valid JSON: " + err.Error()}))
	}
	result, err := run(decoded)
	if err != nil {
		return encode(failure(toError(err)))
	}
	return encode(Response{OK: true, Result: result})
}

func run(request Request) (any, error) {
	switch request.Operation {
	case OperationLint:
		document, err := apispec.Parse([]byte(request.Document))
		if err != nil {
			return nil, err
		}
		return Findings{Findings: nonNil(document.Lint())}, nil
	case OperationLintChanges:
		base, next, err := parsePair(request)
		if err != nil {
			return nil, err
		}
		return Findings{Findings: nonNil(apispec.LintChanges(base, next))}, nil
	case OperationCompare:
		base, next, err := parsePair(request)
		if err != nil {
			return nil, err
		}
		comparison, err := apispec.Compare(base, next)
		if err != nil {
			return nil, err
		}
		return toComparison(comparison), nil
	case OperationEdit:
		edited, err := apispec.Edit([]byte(request.Document), request.Commands...)
		if err != nil {
			return nil, err
		}
		return Document{Document: string(edited)}, nil
	case OperationCanonical:
		document, err := apispec.Parse([]byte(request.Document))
		if err != nil {
			return nil, err
		}
		canonical, err := document.Canonical()
		if err != nil {
			return nil, err
		}
		return Document{Document: string(canonical)}, nil
	}
	return nil, &Error{Code: "BAD_REQUEST", Message: fmt.Sprintf("unknown operation %q", request.Operation)}
}

func parsePair(request Request) (*apispec.Document, *apispec.Document, error) {
	base, err := apispec.Parse([]byte(request.Base))
	if err != nil {
		return nil, nil, fmt.Errorf("base: %w", err)
	}
	next, err := apispec.Parse([]byte(request.Document))
	if err != nil {
		return nil, nil, err
	}
	return base, next, nil
}

func toComparison(comparison apispec.Comparison) Comparison {
	changes := make([]Change, 0, len(comparison.Changes))
	for _, change := range comparison.Changes {
		changes = append(changes, Change(change))
	}
	return Comparison{Bump: comparison.Bump, Changes: changes}
}

func nonNil(findings []apispec.Finding) []apispec.Finding {
	if findings == nil {
		return []apispec.Finding{}
	}
	return findings
}

func (e *Error) Error() string { return e.Message }

func toError(err error) *Error {
	if bridgeError, ok := errors.AsType[*Error](err); ok {
		return bridgeError
	}
	if refused, ok := errors.AsType[*apispec.CommandError](err); ok {
		index := refused.Index
		return &Error{Code: "SPEC_COMMAND_REFUSED", Message: err.Error(), Index: &index, Reason: refused.Reason}
	}
	if invalid, ok := errors.AsType[*apispec.ValidationError](err); ok {
		return &Error{Code: "SPEC_DOCUMENT_INVALID", Message: err.Error(), Problems: invalid.Problems}
	}
	if external, ok := errors.AsType[*apispec.ExternalRefError](err); ok {
		refs := make([]ExternalRef, 0, len(external.Refs))
		for _, ref := range external.Refs {
			refs = append(refs, ExternalRef(ref))
		}
		return &Error{Code: "SPEC_EXTERNAL_REF", Message: err.Error(), Refs: refs}
	}
	switch {
	case errors.Is(err, apispec.ErrComparisonFailed):
		return &Error{Code: "SPEC_COMPARISON_FAILED", Message: err.Error()}
	case errors.Is(err, apispec.ErrSwagger2), errors.Is(err, apispec.ErrUnsupportedVersion):
		return &Error{Code: "SPEC_UNSUPPORTED_VERSION", Message: err.Error()}
	case errors.Is(err, apispec.ErrUnreadable):
		return &Error{Code: "SPEC_DOCUMENT_UNREADABLE", Message: err.Error()}
	}
	return &Error{Code: "UNEXPECTED", Message: err.Error()}
}

func failure(err *Error) Response {
	return Response{OK: false, Error: err}
}

func encode(response Response) []byte {
	encoded, err := json.Marshal(response)
	if err != nil {
		return []byte(`{"ok":false,"error":{"code":"UNEXPECTED","message":"encode response"}}`)
	}
	return encoded
}
