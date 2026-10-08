# API spec invariants

Canonical output is a stored format. Any byte change in `Canonical` requires a `LayoutVersion` bump and a new `apispec/testdata/canonical/v<N>/` fixture set; retain previous layouts until consumers no longer need their stored Live versions. [Canonical tests](../../apispec/canonical_test.go) include a real Stripe-sized document and enforce idempotence.

`Edit` splices into original source bytes. Untouched comments, key order, blank lines, quoting and line endings stay identical. [Splice tests](../../apispec/edit_splice_test.go) and `apispec/testdata/edit` verify the actual diff; a whole-document re-encode does not satisfy this contract.

The publication bump table lives in [version.go](../../apispec/version.go) and comparison severity in [compare.go](../../apispec/compare.go), with publication tests in [publish_test.go](../../apispec/publish_test.go). `info.version` is ignored during comparison and assigned by publication. A failed comparison returns `ErrComparisonFailed`, not an empty successful comparison.

Lint rule IDs are stable shared data: server findings and browser results use the same engine. Renaming a rule ID is a breaking change. The existing thresholds and heuristics have focused tests in [lint_test.go](../../apispec/lint_test.go); change those deliberately and validate representative documents.

Dependency changes to kin-openapi or oasdiff may alter validation, diffs and layouts. Run the complete suite, the Stripe fixture and browser/native parity before treating such an upgrade as compatible. [README](../../README.md) owns the public API overview and version-bump table.
