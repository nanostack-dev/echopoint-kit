# echopoint-kit Agent Guide

Shared Go packages for Echopoint, public so the public `echopoint-cli` can import them. Consumers: `../echopoint` (API), `../echopoint-cli`, and later the browser through WebAssembly.

## Packages

- `apispec`: the OpenAPI spec engine of API specs (`nanostack-dev/echopoint#433`): parse and validate, the canonical YAML layout (ADR-0017 in echopoint), comparison with oasdiff, and the version bump.

## Invariants

- Nothing Echopoint-private goes here: no echopoint domain types, database, HTTP, or credentials. A package here must make sense to any OpenAPI tool.
- The canonical layout is a stored format. Any change to `Canonical` output, even one byte for one document, bumps `apispec.LayoutVersion` and adds goldens under `apispec/testdata/canonical/v<N>/`; the old directory stays until no Live version uses it.
- The bump table lives in `apispec/version.go` and `severityOf` in `apispec/compare.go`; each row has its own test in `apispec/publish_test.go`.
- A comparison that fails returns `ErrComparisonFailed`, never an empty comparison.
- oasdiff and kin-openapi are pinned; a bump can change diffs and the canonical layout, so run the Stripe golden test before merging one.
- Avoid comments — name variables and functions clearly instead. Comment only a genuinely complex algorithm.

## Commands

- Test: `go test -race ./...` (`-short` skips the 6.6 MB Stripe document). Lint: `golangci-lint run --path-mode=abs`.
- Regenerate goldens of the current layout version: `go test ./apispec -run Canonical -update`. Review the diff: an unexpected change means `LayoutVersion` must move.

## Releases

- `.github/workflows/release.yml` tags the next minor on every push to `main`. Then bump consumers: `go get github.com/nanostack-dev/echopoint-kit@vX.Y.0 && go mod tidy` in `../echopoint` and `../echopoint-cli`.
