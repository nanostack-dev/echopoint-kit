# Publication and consumer upgrades

This repository publishes a Go module and browser assets, not a standalone service. [release.yml](../../.github/workflows/release.yml) creates the next minor tag after every push to `main`, including documentation-only changes. Its release queue preserves main-push order.

The workflow builds `cmd/apispec-wasm` with the module's Go toolchain, gzip-compresses the binary, copies that toolchain's `wasm_exec.js`, computes `SHA256SUMS`, and publishes all three assets. Verify the release job and complete asset set; a tag alone does not prove the browser artifact exists.

In each Go consumer, replace the placeholder with the released version:

```sh
go get github.com/nanostack-dev/echopoint-kit@vX.Y.0
go mod tidy
```

Upgrade Echopoint and the CLI through linked PRs and run their affected spec/bridge tests. Echopoint's frontend selects the kit release matching its backend module version; verify that the binary, runtime shim and hashes come from that exact release and that a representative editor operation succeeds. The CLI must publish its own behavior release before users receive the new embedded kit.

Do not confuse kit semantic versions, stored canonical `LayoutVersion` and published OpenAPI document versions. Their compatibility evidence is separate.
