# echopoint-kit Agent Guide

Shared Go packages for Echopoint, public so the public `echopoint-cli` can import them. Consumers: `../echopoint` (API), `../echopoint-cli`, and later the browser through WebAssembly.

## Packages

- `apispec`: the OpenAPI spec engine of API specs (`nanostack-dev/echopoint#433`): parse and validate, the canonical YAML layout (ADR-0017 in echopoint), comparison with oasdiff, the version bump, and `Edit`, the granular edit commands.

## Invariants

- Nothing Echopoint-private goes here: no echopoint domain types, database, HTTP, or credentials. A package here must make sense to any OpenAPI tool.
- The canonical layout is a stored format. Any change to `Canonical` output, even one byte for one document, bumps `apispec.LayoutVersion` and adds goldens under `apispec/testdata/canonical/v<N>/`; the old directory stays until no Live version uses it.
- `apispec.Edit` splices into the original bytes: every byte a command does not touch stays identical (comments, key order, blank lines, quoting). Do not replace the splice with a whole-document encode; the diff-based goldens in `apispec/testdata/edit/<case>/{input.yaml|input.json,commands.json,output.*,changes.diff}` fail when untouched lines move. Regenerate them with `go test ./apispec -run Edit -update` and review `changes.diff`.
- The bump table lives in `apispec/version.go` and `severityOf` in `apispec/compare.go`; each row has its own test in `apispec/publish_test.go`.
- A comparison that fails returns `ErrComparisonFailed`, never an empty comparison.
- oasdiff and kin-openapi are pinned; a bump can change diffs and the canonical layout, so run the Stripe golden test before merging one.
- Lint rule IDs (`apispec.Rule*`, such as `property-casing`) are a stored and shared contract: the server stores findings and the browser runs this same code, so renaming a rule ID is a breaking change. Conventions are derived from the document itself and every threshold (80% majority, 70% for property descriptions, 3-operation tag baseline) is ported from the prototype in echopoint `features/api-specs/engine/lint.ts`.
- Avoid comments — name variables and functions clearly instead. Comment only a genuinely complex algorithm.

## Commands

- Test: `go test -race ./...` (`-short` skips the 6.6 MB Stripe document). Lint: `golangci-lint run --path-mode=abs`.
- Regenerate goldens of the current layout version: `go test ./apispec -run Canonical -update`. Review the diff: an unexpected change means `LayoutVersion` must move.

## Releases

- `.github/workflows/release.yml` tags the next minor on every push to `main`. Then bump consumers: `go get github.com/nanostack-dev/echopoint-kit@vX.Y.0 && go mod tidy` in `../echopoint` and `../echopoint-cli`.
