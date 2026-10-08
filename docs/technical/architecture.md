# Implemented architecture

`apispec` parses/validates self-contained OpenAPI, produces the canonical YAML layout, lints conventions, compares documents, computes publication versions and performs granular edits. Parsing uses kin-openapi and YAML tooling; comparison uses pinned oasdiff dependencies. [go.mod](../../go.mod) records their versions.

`apispec/bridge` exposes one JSON request/response interface for `lint`, `lint_changes`, `compare`, `edit` and `canonical`, including structured errors. `cmd/apispec-wasm` wraps the same bridge in `globalThis.apispecHandle(requestJSON)`, optionally calls `globalThis.apispecReady()`, then stays alive for subsequent calls. The [parity test](../../cmd/apispec-wasm/parity_test.go) compares native and Node-hosted WebAssembly responses.

Consumers are [Echopoint](https://github.com/nanostack-dev/echopoint) and [Echopoint CLI](https://github.com/nanostack-dev/echopoint-cli). The kit owns reusable OpenAPI behavior; consumer authorization, tenancy, persistence and product UI remain outside it. Consumers use pinned Go module releases; the frontend pairs the matching WebAssembly binary and Go runtime shim. No sibling checkout is required for ordinary kit development.

[API spec invariants](apispec.md) own stored-format and editing compatibility. [Publication](../runbooks/deployment.md) owns module/browser release coordination.
