# Troubleshooting

| Symptom | Established cause and repair | Verification |
| --- | --- | --- |
| Native tests pass but parity did not run | The parity test skips without Node or with `-short`. Install Node and run the full suite. | `TestWebAssemblyBuildAnswersLikeTheNativeEngine` executes successfully rather than skipping. |
| Canonical fixture changed unexpectedly | Parser/dependency/layout behavior moved. Investigate the actual diff; intentional byte changes require a new layout version and golden set. | Canonical fixtures, Stripe hash and idempotence tests pass for the intended layout. |
| Granular edit reflows unrelated lines | The edit path lost its byte-splice behavior. Repair the splice rather than replacing it with full-document serialization. | `TestEditTouchesOnlyTheExpectedLines` and the `changes.diff` fixtures match. |
| Browser artifact fails despite a Go upgrade | `wasm_exec.js` must match the Go toolchain which built the WASM. Use both assets from one kit release. | Native/browser parity and the consumer's representative editor operation agree. |

Use [Stripe fixture provenance](../../apispec/testdata/stripe/README.md) before replacing real-world fixtures. Add new recurring fixes after cause and verification are established.
