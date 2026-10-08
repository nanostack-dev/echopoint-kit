# Echopoint Kit

Public Go OpenAPI engine shared by Echopoint, its CLI and the browser WebAssembly build. A standalone checkout is sufficient for build and test.

- Before implementation or delivery, read [agent workflow](docs/development/agent-workflow.md) and [testing](docs/development/testing.md).
- Before parsing, editing, canonicalization, lint, comparison or versioning changes, read [architecture](docs/technical/architecture.md), [API spec invariants](docs/technical/apispec.md) and [domain context](CONTEXT.md). Preserve stored layout versions, rule IDs, untouched edit bytes and native/browser parity.
- Before startup or recurring failures, read [setup](docs/development/setup.md) or [troubleshooting](docs/development/troubleshooting.md).
- Before merge or consumer upgrades, read [publication](docs/runbooks/deployment.md). Every push to main, including documentation, tags another minor and publishes WebAssembly assets. For recovery, read [rollback](docs/runbooks/rollback.md).
- Packages must make sense to any OpenAPI tool; app-private domain/database/HTTP/credential concerns belong to their consumers. Keep required procedures local and update owning docs in the same PR.
