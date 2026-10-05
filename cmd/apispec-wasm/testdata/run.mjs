// Runs requests through the WebAssembly build: node run.mjs <wasm_exec.js> <apispec.wasm>
// reads a JSON array of requests on stdin and writes the array of responses.
import { readFileSync } from "node:fs"
import { createRequire } from "node:module"

const [wasmExec, wasmPath] = process.argv.slice(2)
createRequire(import.meta.url)(wasmExec)

const ready = new Promise((resolve) => {
  globalThis.apispecReady = resolve
})
const go = new globalThis.Go()
const { instance } = await WebAssembly.instantiate(readFileSync(wasmPath), go.importObject)
go.run(instance)
await ready

const requests = JSON.parse(readFileSync(0, "utf8"))
const responses = requests.map((request) => JSON.parse(globalThis.apispecHandle(JSON.stringify(request))))
process.stdout.write(JSON.stringify(responses))
process.exit(0)
