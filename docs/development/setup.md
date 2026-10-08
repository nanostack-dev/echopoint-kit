# Local setup

Use the Go version in [go.mod](../../go.mod), currently 1.26.5. Install Node.js for the WebAssembly parity test and `golangci-lint` for linting. The [CI workflow](../../.github/workflows/ci.yml) defines the Go tool setup; Node must be available for parity to execute rather than skip. No API target or credentials are required.

```sh
go mod download
go test -short ./...
```

`-short` is a fast first check: it skips expensive fixtures and the WebAssembly build, so follow [testing](testing.md) before delivery. A manual browser binary can be built independently:

```sh
GOOS=js GOARCH=wasm go build -o /tmp/apispec.wasm ./cmd/apispec-wasm
```

Use the `wasm_exec.js` from that same Go toolchain, as the [release workflow](../../.github/workflows/release.yml) does. Building the artifact does not publish or install it into a consumer.

## Agent harness setup

Codex, OpenCode and Grok Build discover the local `AGENTS.md` natively. Claude Code uses the project-local [.claude/settings.json](../../.claude/settings.json) SessionStart hook to read that guide from the Git checkout root, including sessions started in a nested directory. Approve project trust on first use and reload the session after adding or updating hooks. Personal overrides stay in ignored `.claude/settings.local.json`; required project guidance does not depend on the shared workspace or globally installed skills.
