# echopoint-kit

Shared Go packages for [Echopoint](https://echopoint.dev).

## apispec

The OpenAPI spec engine behind Echopoint's API specs and the `echopoint spec` CLI commands.

```go
document, err := apispec.Parse(data)          // OpenAPI 3.0.x or 3.1.x, self-contained
err = document.Validate()                     // every problem in one ValidationError
canonical, err := document.Canonical()        // the canonical YAML layout
comparison, err := apispec.Compare(base, document)
publication, err := apispec.Publish(&apispec.Live{Document: base, Version: "1.4.0"}, document)
```

### Canonical layout

OpenAPI fields in a fixed order (the order of the OpenAPI Specification; Schema Objects read identity, type, structure, limits, examples), `$ref` first, extensions and unknown fields after them sorted by name. Paths, schemas, properties, and responses keep the document's order. Two-space indentation, block style, no comments, no anchors (aliases and merge keys are expanded). Strings YAML 1.1 reads as booleans are quoted. `LayoutVersion` names the layout.

### Version bump

| Change | Bump |
| --- | --- |
| A change that breaks clients or may break them (oasdiff error or warning) | major |
| A new operation, schema, optional field, or enum value; looser limits | minor |
| Descriptions, examples, `x-` extensions, field order, other edits | patch |
| A mix | the highest of them |
| No change (identical to Live, `info.version` aside) | refused: `ErrNothingToPublish` |
| First version | `info.version` if it is a semantic version, else `1.0.0` |

`info.version` is owned by the publisher: it is ignored when comparing and written by `Publish`.

## License

MIT
