# Rollback

Restore the last verified kit module version in each affected consumer, run `go mod tidy`, and restore the frontend's WASM/runtime-shim pair from that same release. Run the native/bridge/editor regressions before releasing the consumer. Releasing a kit revert alone does not change already pinned consumers.

Check whether a new canonical layout or stored finding/bridge contract was already persisted. Retain older layout fixtures and follow the consumer's data/version recovery procedure; changing a module pin does not rewrite stored documents or undo publication.

Prepare a focused source revert or fix PR with regression evidence. Every merge releases another minor; preserve earlier tags/assets as immutable history. Recovery is complete only after the affected consumers run the intended versions and representative operations succeed.
