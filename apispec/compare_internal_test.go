package apispec

import (
	"errors"
	"testing"
)

func TestComparisonFailureIsReportedAndNeverEmpty(t *testing.T) {
	broken := &Document{}
	report, _, err := diffDocuments(broken, broken)
	if !errors.Is(err, ErrComparisonFailed) {
		t.Fatalf("diffDocuments() error = %v, want ErrComparisonFailed", err)
	}
	if report != nil {
		t.Error("a failed comparison returned a diff report")
	}
}
