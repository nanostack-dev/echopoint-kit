// Package apispec is the OpenAPI spec engine of Echopoint's API specs. The
// API, the CLI, and the browser use it, so a document reads, compares, and
// versions the same everywhere.
//
//   - Parse reads an OpenAPI 3.0.x or 3.1.x document and refuses Swagger 2.0,
//     other versions, and external $ref. Validate checks it.
//   - Canonical writes the canonical YAML layout EchoPoint stores and
//     `spec pull` writes (ADR-0017). LayoutVersion names that layout.
//   - Edit applies granular commands (Command, addressed by JSON pointer) to a
//     YAML or JSON document, all or nothing. Bytes the commands do not touch
//     stay identical, so a diff of the result shows only the edit.
//   - Compare diffs two documents with oasdiff and computes the version bump.
//   - Lint reports the nodes that depart from the conventions the document itself follows,
//     and LintChanges only those a change introduces, judged by the conventions of the base.
//   - Publish turns a document into the next Live version.
package apispec
