# Testing

```sh
go test -race ./...
golangci-lint run --path-mode=abs
```

The [CI workflow](../../.github/workflows/ci.yml) runs lint and race-enabled tests with `-count=1 -timeout 10m`. Full tests exercise the large Stripe fixture and the WebAssembly parity test. Without Node, parity skips; `-short` also skips the WebAssembly build and large fixtures. Report these skips accurately.

For intentional fixture updates, use the existing test flags and review their resulting diffs:

```sh
go test ./apispec -run Canonical -update
go test ./apispec -run Edit -update
```

Canonical byte changes require the layout-version/new-goldens procedure in [API spec invariants](../technical/apispec.md). Review edit `changes.diff` to prove untouched source stayed unchanged. Consumer-facing bridge/rule changes also need the API and frontend/CLI regressions using the matching kit version.

Documentation-only changes need local-link, command-provenance and `git diff --check` validation. The repository has no Markdown linter configured.
