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
