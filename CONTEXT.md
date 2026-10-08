# Domain context

| Term | Meaning |
| --- | --- |
| Document | Parsed, self-contained OpenAPI 3.0.x or 3.1.x input. |
| Canonical layout | Deterministic YAML representation used as a stored format; identified by `LayoutVersion`. |
| Live version | Previously published document/version used as the comparison baseline. |
| Publication | Validated document and computed semantic version returned by `apispec.Publish`. |
| Comparison | Located changes classified using pinned oasdiff behavior; comparison failure is explicit. |
| Edit command | Granular splice which preserves original bytes outside the touched regions. |
| Finding | Lint result with a stable rule ID and document location. |
| Bridge | JSON-in/JSON-out wrapper shared by native API callers and the WebAssembly entry point. |
| Kit release version | Go module/WebAssembly release version; distinct from a document's version and `LayoutVersion`. |

[API spec invariants](docs/technical/apispec.md) define compatibility obligations for each representation.
